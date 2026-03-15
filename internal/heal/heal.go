package heal

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yevgetman/fry/internal/config"
	"github.com/yevgetman/fry/internal/engine"
	"github.com/yevgetman/fry/internal/epic"
	frylog "github.com/yevgetman/fry/internal/log"
	"github.com/yevgetman/fry/internal/shellhook"
	"github.com/yevgetman/fry/internal/verify"
)

type HealOpts struct {
	ProjectDir          string
	Sprint              *epic.Sprint
	Epic                *epic.Epic
	Engine              engine.Engine
	Checks              []verify.Check     // Initial checks (used if VerificationFile is empty)
	VerificationFile    string             // When set, re-parsed each heal attempt so on-disk edits take effect
	UserPrompt          string
	Verbose             bool
	SprintLogFile       string
	MaxAttemptsOverride int // When > 0, overrides epic/sprint max heal attempts
}

func RunHealLoop(ctx context.Context, opts HealOpts) (bool, error) {
	if opts.Epic == nil || opts.Sprint == nil {
		return false, fmt.Errorf("run heal loop: epic and sprint are required")
	}
	if opts.Engine == nil {
		return false, fmt.Errorf("run heal loop: engine is required")
	}

	maxAttempts := opts.Epic.MaxHealAttempts
	if opts.Sprint.MaxHealAttempts != nil {
		maxAttempts = *opts.Sprint.MaxHealAttempts
	}
	if maxAttempts <= 0 {
		maxAttempts = config.DefaultMaxHealAttempts
	}
	if opts.MaxAttemptsOverride > 0 {
		maxAttempts = opts.MaxAttemptsOverride
	}

	buildLogsDir := filepath.Join(opts.ProjectDir, config.BuildLogsDir)
	if err := os.MkdirAll(buildLogsDir, 0o755); err != nil {
		return false, fmt.Errorf("run heal loop: create build logs dir: %w", err)
	}

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		default:
		}

		checks, reloadErr := reloadChecks(opts)
		if reloadErr != nil {
			return false, fmt.Errorf("run heal loop: reload checks: %w", reloadErr)
		}

		results, passCount, totalCount := verify.RunChecks(ctx, checks, opts.Sprint.Number, opts.ProjectDir)
		if totalCount == passCount {
			return true, nil
		}

		failureReport := verify.CollectFailures(results, passCount, totalCount)
		prompt := buildHealPrompt(opts, failureReport)
		promptPath := filepath.Join(opts.ProjectDir, config.PromptFile)
		if err := os.WriteFile(promptPath, []byte(prompt), 0o644); err != nil {
			return false, fmt.Errorf("run heal loop: write heal prompt: %w", err)
		}

		frylog.Log(
			"▶ AGENT  sprint %d/%d \"%s\"  heal %d/%d  engine=%s  model=%s",
			opts.Sprint.Number,
			opts.Epic.TotalSprints,
			opts.Sprint.Name,
			attempt,
			maxAttempts,
			opts.Engine.Name(),
			agentModel(opts.Epic.AgentModel),
		)

		healLogPath := filepath.Join(
			buildLogsDir,
			fmt.Sprintf("sprint%d_heal%d_%s.log", opts.Sprint.Number, attempt, time.Now().Format("20060102_150405")),
		)
		if _, err := runAgentWithDualLogs(ctx, opts, config.HealInvocationPrompt, healLogPath); err != nil {
			return false, err
		}

		if err := shellhook.Run(ctx, opts.ProjectDir, opts.Epic.PreSprintCmd); err != nil {
			return false, fmt.Errorf("run heal loop: pre-sprint hook: %w", err)
		}

		frylog.Log("  Re-running verification after heal attempt %d...", attempt)
		// Re-read verification file so on-disk fixes by the agent take effect.
		checks, reloadErr = reloadChecks(opts)
		if reloadErr != nil {
			return false, fmt.Errorf("run heal loop: reload checks after attempt %d: %w", attempt, reloadErr)
		}
		results, passCount, totalCount = verify.RunChecks(ctx, checks, opts.Sprint.Number, opts.ProjectDir)
		if totalCount == passCount {
			frylog.Log("  Heal attempt %d SUCCEEDED — all checks now pass.", attempt)
			return true, nil
		}

		frylog.Log("  Heal attempt %d — %d/%d checks still failing.", attempt, totalCount-passCount, totalCount)
		failureReport = verify.CollectFailures(results, passCount, totalCount)
		entry := fmt.Sprintf("--- Heal attempt %d failed ---\n\n%s\n\n", attempt, failureReport)
		if err := appendToSprintProgress(opts.ProjectDir, entry); err != nil {
			return false, fmt.Errorf("run heal loop: append failure report: %w", err)
		}
	}

	frylog.Log("  All %d heal attempts exhausted.", maxAttempts)
	return false, nil
}

// reloadChecks re-parses the verification file from disk when VerificationFile
// is set, so that on-disk edits by the healing agent take effect between
// attempts. Falls back to the pre-loaded Checks slice otherwise.
func reloadChecks(opts HealOpts) ([]verify.Check, error) {
	if opts.VerificationFile == "" {
		return opts.Checks, nil
	}
	path := opts.VerificationFile
	if !filepath.IsAbs(path) {
		path = filepath.Join(opts.ProjectDir, path)
	}
	checks, err := verify.ParseVerification(path)
	if err != nil {
		return nil, err
	}
	return checks, nil
}

func buildHealPrompt(opts HealOpts, failureReport string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("# HEAL MODE — Sprint %d: %s\n\n", opts.Sprint.Number, opts.Sprint.Name))
	b.WriteString("## What happened\n")
	b.WriteString("The sprint finished its work but FAILED independent verification checks.\n")
	b.WriteString("Your job is to fix ONLY the issues described below. Do not start the sprint over.\n")
	b.WriteString("Do not refactor or reorganize. Make the minimum changes needed to pass the checks.\n\n")
	b.WriteString("## Failed verification checks\n\n")
	b.WriteString(failureReport)
	b.WriteString("\n\n")
	b.WriteString("## Instructions\n")
	b.WriteString(fmt.Sprintf("1. Read %s for context on what was built this sprint\n", config.SprintProgressFile))
	b.WriteString(fmt.Sprintf("2. Read %s for context on what was built in prior sprints\n", config.EpicProgressFile))
	b.WriteString("3. Read the failed checks above carefully\n")
	b.WriteString("4. Fix each failure — create missing files, fix build errors, correct config\n")
	b.WriteString("5. After fixing, do a final sanity check (e.g., run the build command if applicable)\n")
	b.WriteString(fmt.Sprintf("6. Append a brief note to %s about what you fixed in this heal pass\n\n", config.SprintProgressFile))
	b.WriteString("## Context files\n")
	b.WriteString(fmt.Sprintf("- Read %s for current sprint iteration history\n", config.SprintProgressFile))
	b.WriteString(fmt.Sprintf("- Read %s for prior sprint summaries\n", config.EpicProgressFile))
	b.WriteString(fmt.Sprintf("- Read %s for the overall project plan\n", config.PlanFile))
	// Only reference executive.md if the file actually exists
	executivePath := filepath.Join(opts.ProjectDir, config.ExecutiveFile)
	if _, err := os.Stat(executivePath); err == nil {
		b.WriteString(fmt.Sprintf("- Read %s for executive context\n", config.ExecutiveFile))
	}
	if strings.TrimSpace(opts.UserPrompt) != "" {
		b.WriteString(fmt.Sprintf("- User directive: %s\n", strings.TrimSpace(opts.UserPrompt)))
	}
	b.WriteString("\n")
	b.WriteString("Do NOT output any promise tokens. Just fix the issues.\n")
	return b.String()
}

func runAgentWithDualLogs(ctx context.Context, opts HealOpts, prompt, iterPath string) (string, error) {
	iterLog, err := os.Create(iterPath)
	if err != nil {
		return "", fmt.Errorf("run heal loop: create iteration log: %w", err)
	}
	defer iterLog.Close()

	sprintLog, err := os.OpenFile(opts.SprintLogFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return "", fmt.Errorf("run heal loop: open sprint log: %w", err)
	}
	defer sprintLog.Close()

	runOpts := engine.RunOpts{
		Model:      opts.Epic.AgentModel,
		ExtraFlags: strings.Fields(opts.Epic.AgentFlags),
		WorkDir:    opts.ProjectDir,
	}

	if opts.Verbose {
		writer := io.MultiWriter(os.Stdout, iterLog, sprintLog)
		runOpts.Stdout = writer
		runOpts.Stderr = writer
		output, _, runErr := opts.Engine.Run(ctx, prompt, runOpts)
		if runErr != nil && ctx.Err() == nil {
			frylog.Log("WARNING: agent exited with error (non-fatal): %v", runErr)
			return output, nil
		}
		return output, runErr
	}

	runOpts.Stdout = iterLog
	runOpts.Stderr = iterLog
	output, _, runErr := opts.Engine.Run(ctx, prompt, runOpts)
	// iterLog is flushed (Go file writes are unbuffered); defer handles close.
	iterBytes, err := os.ReadFile(iterPath)
	if err != nil {
		return output, fmt.Errorf("run heal loop: read iteration log: %w", err)
	}
	if _, err := sprintLog.Write(iterBytes); err != nil {
		return output, fmt.Errorf("run heal loop: append iteration log: %w", err)
	}
	if runErr != nil && ctx.Err() == nil {
		frylog.Log("WARNING: agent exited with error (non-fatal): %v", runErr)
		return output, nil
	}
	return output, runErr
}

func appendToSprintProgress(projectDir, entry string) error {
	progressPath := filepath.Join(projectDir, config.SprintProgressFile)
	if err := os.MkdirAll(filepath.Dir(progressPath), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(progressPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.WriteString(entry)
	return err
}

func agentModel(model string) string {
	if model == "" {
		return "default"
	}
	return model
}
