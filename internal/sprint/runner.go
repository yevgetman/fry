package sprint

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/yevgetman/fry/internal/config"
	"github.com/yevgetman/fry/internal/engine"
	"github.com/yevgetman/fry/internal/epic"
	"github.com/yevgetman/fry/internal/heal"
	"github.com/yevgetman/fry/internal/shellhook"
	frylog "github.com/yevgetman/fry/internal/log"
	"github.com/yevgetman/fry/internal/verify"
)

const (
	StatusPass = "PASS"
	// PASS healed: keep this unparenthesized form in the file for regex-based verification compatibility.
	StatusPassHealed                           = "PASS (healed)"
	StatusPassVerificationPassedNoPromise      = "PASS (verification passed, no promise)"
	StatusPassHealedNoPromise                  = "PASS (healed, no promise)"
	StatusFailVerificationFailedHealExhausted  = "FAIL (verification failed, heal exhausted)"
	StatusFailNoPromiseVerificationHealExhaust = "FAIL (no promise, verification failed, heal exhausted)"
	StatusFailNoPrompt                         = "FAIL (no prompt)"
	StatusSkipped                              = "SKIPPED"
)

type SprintResult struct {
	Number       int
	Name         string
	Status       string
	Duration     time.Duration
	AuditWarning string // non-empty when MODERATE audit issues remain (advisory)
}

type RunConfig struct {
	ProjectDir  string
	Epic        *epic.Epic
	Sprint      *epic.Sprint
	Engine      engine.Engine
	Verbose     bool
	DryRun      bool
	UserPrompt  string
	StartSprint int
	EndSprint   int
}

func RunSprint(ctx context.Context, cfg RunConfig) (*SprintResult, error) {
	started := time.Now()
	if cfg.Epic == nil || cfg.Sprint == nil {
		return nil, fmt.Errorf("run sprint: epic and sprint are required")
	}

	if err := InitSprintProgress(cfg.ProjectDir, cfg.Sprint.Number, cfg.Sprint.Name); err != nil {
		return nil, fmt.Errorf("run sprint: %w", err)
	}

	if strings.TrimSpace(cfg.Sprint.Prompt) == "" {
		return &SprintResult{
			Number:   cfg.Sprint.Number,
			Name:     cfg.Sprint.Name,
			Status:   StatusFailNoPrompt,
			Duration: time.Since(started),
		}, nil
	}
	if cfg.Engine == nil {
		return nil, fmt.Errorf("run sprint: engine is required")
	}

	userPrompt := cfg.UserPrompt
	if strings.TrimSpace(userPrompt) == "" {
		userPrompt = readOptionalPromptFile(filepath.Join(cfg.ProjectDir, config.UserPromptFile))
	}

	if _, err := AssemblePrompt(PromptOpts{
		ProjectDir:         cfg.ProjectDir,
		UserPrompt:         userPrompt,
		SprintPrompt:       cfg.Sprint.Prompt,
		SprintProgressFile: config.SprintProgressFile,
		EpicProgressFile:   config.EpicProgressFile,
		Promise:            cfg.Sprint.Promise,
		EffortLevel:        cfg.Epic.EffortLevel,
	}); err != nil {
		return nil, fmt.Errorf("run sprint: %w", err)
	}

	buildLogsDir := filepath.Join(cfg.ProjectDir, config.BuildLogsDir)
	if err := os.MkdirAll(buildLogsDir, 0o755); err != nil {
		return nil, fmt.Errorf("run sprint: create build logs dir: %w", err)
	}

	sprintStamp := time.Now().Format("20060102_150405")
	sprintLogPath := filepath.Join(buildLogsDir, fmt.Sprintf("sprint%d_%s.log", cfg.Sprint.Number, sprintStamp))

	promiseToken := "===PROMISE: " + cfg.Sprint.Promise + "==="
	promiseFound := false

	// Pre-load verification checks for no-op early exit detection
	checks, checkErr := loadVerificationChecks(cfg.ProjectDir, cfg.Epic.VerificationFile)
	if checkErr != nil {
		return nil, checkErr
	}

	consecutiveNoop := 0

	frylog.Log("=========================================")
	frylog.Log("STARTING SPRINT %d: %s", cfg.Sprint.Number, cfg.Sprint.Name)
	frylog.Log("Max iterations: %d", cfg.Sprint.MaxIterations)
	if cfg.Epic.EffortLevel != "" {
		frylog.Log("Effort level: %s", cfg.Epic.EffortLevel)
	}
	frylog.Log("=========================================")

	for iter := 1; iter <= cfg.Sprint.MaxIterations; iter++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		if err := shellhook.Run(ctx, cfg.ProjectDir, cfg.Epic.PreIterationCmd); err != nil {
			return nil, fmt.Errorf("run sprint pre-iteration hook: %w", err)
		}

		frylog.AgentBanner(cfg.Sprint.Number, cfg.Epic.TotalSprints, cfg.Sprint.Name, iter, cfg.Sprint.MaxIterations, cfg.Engine.Name(), cfg.Epic.AgentModel)

		// Snapshot working tree before agent runs (for no-op detection)
		preIterDiff := gitDiffStat(ctx, cfg.ProjectDir)

		iterPath := filepath.Join(buildLogsDir, fmt.Sprintf("sprint%d_iter%d_%s.log", cfg.Sprint.Number, iter, time.Now().Format("20060102_150405")))
		output, err := runAgentWithDualLogs(ctx, cfg, config.AgentInvocationPrompt, iterPath, sprintLogPath)
		if err != nil {
			return nil, err
		}

		if strings.Contains(output, promiseToken) {
			promiseFound = true
			break
		}

		// No-op detection: early exit when agent is stuck in audit loops
		postIterDiff := gitDiffStat(ctx, cfg.ProjectDir)
		if preIterDiff == postIterDiff {
			consecutiveNoop++
		} else {
			consecutiveNoop = 0
		}

		noopThreshold := 2
		if cfg.Epic.EffortLevel == epic.EffortMax {
			noopThreshold = 3
		}
		if consecutiveNoop >= noopThreshold && cfg.Sprint.Promise != "" && len(checks) > 0 {
			_, passCount, totalCount := verify.RunChecks(ctx, checks, cfg.Sprint.Number, cfg.ProjectDir)
			if totalCount > 0 && passCount == totalCount {
				frylog.Log("  No file changes for %d consecutive iterations and verification passes — exiting early.", consecutiveNoop)
				break
			}
		}
	}

	if len(checks) > 0 {
		frylog.Log("  Running verification checks...")
	}
	results, passCount, totalCount := verify.RunChecks(ctx, checks, cfg.Sprint.Number, cfg.ProjectDir)
	if totalCount > 0 {
		frylog.Log("  Verification: %d/%d checks passed.", passCount, totalCount)
	}

	status, err := determineOutcome(ctx, cfg, checks, promiseFound, results, passCount, totalCount, sprintLogPath)
	if err != nil {
		return nil, err
	}

	elapsed := time.Since(started)
	frylog.Log("SPRINT %d %s (%s)", cfg.Sprint.Number, status, elapsed.Round(time.Second))

	return &SprintResult{
		Number:   cfg.Sprint.Number,
		Name:     cfg.Sprint.Name,
		Status:   status,
		Duration: elapsed,
	}, nil
}

func determineOutcome(ctx context.Context, cfg RunConfig, checks []verify.Check, promiseFound bool, results []verify.CheckResult, passCount, totalCount int, sprintLogPath string) (string, error) {
	hasChecks := totalCount > 0
	checksPass := totalCount == passCount

	switch {
	case promiseFound && !hasChecks:
		return StatusPass, nil
	case promiseFound && checksPass:
		return StatusPass, nil
	case promiseFound && hasChecks && !checksPass:
		healed, err := heal.RunHealLoop(ctx, heal.HealOpts{
			ProjectDir:    cfg.ProjectDir,
			Sprint:        cfg.Sprint,
			Epic:          cfg.Epic,
			Engine:        cfg.Engine,
			Checks:        checks,
			UserPrompt:    cfg.UserPrompt,
			Verbose:       cfg.Verbose,
			SprintLogFile: sprintLogPath,
		})
		if err != nil {
			return "", err
		}
		if healed {
			return StatusPassHealed, nil
		}
		return StatusFailVerificationFailedHealExhausted, nil
	case !promiseFound && !hasChecks:
		return fmt.Sprintf("FAIL (no promise after %d iters)", cfg.Sprint.MaxIterations), nil
	case !promiseFound && checksPass:
		return StatusPassVerificationPassedNoPromise, nil
	default:
		healed, err := heal.RunHealLoop(ctx, heal.HealOpts{
			ProjectDir:    cfg.ProjectDir,
			Sprint:        cfg.Sprint,
			Epic:          cfg.Epic,
			Engine:        cfg.Engine,
			Checks:        checks,
			UserPrompt:    cfg.UserPrompt,
			Verbose:       cfg.Verbose,
			SprintLogFile: sprintLogPath,
		})
		if err != nil {
			return "", err
		}
		if healed {
			return StatusPassHealedNoPromise, nil
		}
		return StatusFailNoPromiseVerificationHealExhaust, nil
	}
}

func loadVerificationChecks(projectDir, verificationFile string) ([]verify.Check, error) {
	path := verificationFile
	if path == "" {
		path = config.DefaultVerificationFile
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(projectDir, path)
	}

	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	return verify.ParseVerification(path)
}

func runAgentWithDualLogs(ctx context.Context, cfg RunConfig, prompt, iterPath, sprintLogPath string) (string, error) {
	if cfg.Engine == nil {
		return "", fmt.Errorf("run sprint: engine is required")
	}

	iterLog, err := os.Create(iterPath)
	if err != nil {
		return "", fmt.Errorf("create iteration log: %w", err)
	}
	defer iterLog.Close()

	sprintLog, err := os.OpenFile(sprintLogPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return "", fmt.Errorf("open sprint log: %w", err)
	}
	defer sprintLog.Close()

	opts := engine.RunOpts{
		Model:      cfg.Epic.AgentModel,
		ExtraFlags: strings.Fields(cfg.Epic.AgentFlags),
		WorkDir:    cfg.ProjectDir,
	}

	if cfg.Verbose {
		writer := io.MultiWriter(os.Stdout, iterLog, sprintLog)
		opts.Stdout = writer
		opts.Stderr = writer
		output, _, runErr := cfg.Engine.Run(ctx, prompt, opts)
		if runErr != nil && ctx.Err() == nil {
			return output, nil
		}
		return output, runErr
	}

	opts.Stdout = iterLog
	opts.Stderr = iterLog
	output, _, runErr := cfg.Engine.Run(ctx, prompt, opts)
	// iterLog is flushed (Go file writes are unbuffered); defer handles close.
	iterBytes, err := os.ReadFile(iterPath)
	if err != nil {
		return output, fmt.Errorf("read iteration log: %w", err)
	}
	if _, err := sprintLog.Write(iterBytes); err != nil {
		return output, fmt.Errorf("append iteration log to sprint log: %w", err)
	}
	if runErr != nil && ctx.Err() == nil {
		return output, nil
	}
	return output, runErr
}

func gitDiffStat(ctx context.Context, projectDir string) string {
	// Exclude .fry/sprint-progress.txt and .fry/epic-progress.txt — the agent
	// appends to sprint-progress.txt every iteration, so including it would
	// make pre/post diffs always differ, defeating no-op detection entirely.
	cmd := exec.CommandContext(ctx, "git", "diff", "--stat", "HEAD", "--",
		".", ":!.fry/sprint-progress.txt", ":!.fry/epic-progress.txt")
	cmd.Dir = projectDir
	out, err := cmd.Output()
	if err != nil {
		// Return a unique value so pre/post diffs never falsely compare equal
		// when git is broken, which would trigger premature early exit.
		frylog.Log("WARNING: git diff --stat failed: %v", err)
		return fmt.Sprintf("__git_error_%d__", time.Now().UnixNano())
	}
	return string(out)
}

