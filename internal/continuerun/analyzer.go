package continuerun

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/yevgetman/fry/internal/config"
	"github.com/yevgetman/fry/internal/engine"
	frylog "github.com/yevgetman/fry/internal/log"
	"github.com/yevgetman/fry/internal/textutil"
)

// AnalyzeOpts configures the LLM analysis agent.
type AnalyzeOpts struct {
	ProjectDir string
	State      *BuildState
	Engine     engine.Engine
	Model      string
	Verbose    bool
}

var (
	verdictRe = regexp.MustCompile(`<verdict>(RESUME_RETRY|RESUME_FRESH|CONTINUE_NEXT|ALL_COMPLETE|BLOCKED)</verdict>`)
	sprintRe  = regexp.MustCompile(`<sprint>(\d+)</sprint>`)
	reasonRe  = regexp.MustCompile(`(?s)<reason>(.*?)</reason>`)
)

// Analyze runs the LLM analysis agent to determine where to resume a build.
func Analyze(ctx context.Context, opts AnalyzeOpts) (*ContinueDecision, error) {
	if opts.Engine == nil {
		return nil, fmt.Errorf("continue analyze: engine is required")
	}

	report := FormatReport(opts.State)

	// Write report to disk for user inspection
	reportPath := filepath.Join(opts.ProjectDir, config.ContinueReportFile)
	if err := os.MkdirAll(filepath.Dir(reportPath), 0o755); err != nil {
		return nil, fmt.Errorf("continue analyze: create dir: %w", err)
	}
	if err := os.WriteFile(reportPath, []byte(report), 0o644); err != nil {
		return nil, fmt.Errorf("continue analyze: write report: %w", err)
	}

	// Build and write analysis prompt
	prompt := buildAnalysisPrompt(opts.State, report)
	promptPath := filepath.Join(opts.ProjectDir, config.ContinuePromptFile)
	if err := os.WriteFile(promptPath, []byte(prompt), 0o644); err != nil {
		return nil, fmt.Errorf("continue analyze: write prompt: %w", err)
	}

	frylog.Log("▶ CONTINUE  analyzing with engine=%s...", opts.Engine.Name())

	// Create log file
	buildLogsDir := filepath.Join(opts.ProjectDir, config.BuildLogsDir)
	if err := os.MkdirAll(buildLogsDir, 0o755); err != nil {
		return nil, fmt.Errorf("continue analyze: create logs dir: %w", err)
	}
	logPath := filepath.Join(buildLogsDir, fmt.Sprintf("continue_%s.log", time.Now().Format("20060102_150405")))
	logFile, err := os.Create(logPath)
	if err != nil {
		return nil, fmt.Errorf("continue analyze: create log: %w", err)
	}
	defer logFile.Close()

	runOpts := engine.RunOpts{
		Model:   opts.Model,
		WorkDir: opts.ProjectDir,
	}
	if opts.Verbose {
		writer := io.MultiWriter(os.Stdout, logFile)
		runOpts.Stdout = writer
		runOpts.Stderr = writer
	} else {
		runOpts.Stdout = logFile
		runOpts.Stderr = logFile
	}

	output, _, runErr := opts.Engine.Run(ctx, config.ContinueInvocationPrompt, runOpts)
	if runErr != nil && ctx.Err() == nil {
		frylog.Log("WARNING: continue agent exited with error (non-fatal): %v", runErr)
	} else if runErr != nil {
		return nil, runErr
	}

	// Read decision file if the agent wrote one
	decisionPath := filepath.Join(opts.ProjectDir, config.ContinueDecisionFile)
	if data, err := os.ReadFile(decisionPath); err == nil {
		output = string(data)
	}

	decision := ParseDecision(output, opts.State.TotalSprints)

	frylog.Log("▶ CONTINUE  decision: %s sprint %d — %q",
		decision.Verdict, decision.StartSprint, truncate(decision.Reason, 80))

	return decision, nil
}

// ParseDecision extracts a ContinueDecision from LLM output.
func ParseDecision(output string, totalSprints int) *ContinueDecision {
	decision := &ContinueDecision{
		Verdict: VerdictBlocked,
		Reason:  "could not parse agent decision",
	}

	if m := verdictRe.FindStringSubmatch(output); len(m) > 1 {
		decision.Verdict = ContinueVerdict(m[1])
	}

	if m := sprintRe.FindStringSubmatch(output); len(m) > 1 {
		if n, err := strconv.Atoi(m[1]); err == nil && n >= 1 && n <= totalSprints {
			decision.StartSprint = n
		}
	}

	if m := reasonRe.FindStringSubmatch(output); len(m) > 1 {
		decision.Reason = strings.TrimSpace(m[1])
	}

	// Extract preconditions from markdown checklist
	decision.Preconditions = parsePreconditions(output)

	return decision
}

// parsePreconditions extracts "- [ ] ..." lines from the output.
func parsePreconditions(output string) []string {
	var items []string
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- [ ] ") {
			items = append(items, strings.TrimPrefix(trimmed, "- [ ] "))
		}
	}
	return items
}

func buildAnalysisPrompt(state *BuildState, report string) string {
	var b strings.Builder

	b.WriteString("# Continue Analysis — Build Resume Decision\n\n")
	b.WriteString("## Your Role\n")
	b.WriteString("You are a build analyst. Review the build state report below and decide\n")
	b.WriteString("how the build should be resumed. Do NOT modify any source code.\n\n")

	b.WriteString("## Build State Report\n\n")
	b.WriteString(report)
	b.WriteString("\n\n")

	b.WriteString("## Decision Options\n\n")
	b.WriteString("| Verdict | When to use |\n")
	b.WriteString("|---|---|\n")
	b.WriteString("| RESUME_RETRY | Sprint was partially completed — code work exists but verification failed. Skip to verification + healing. |\n")
	b.WriteString("| RESUME_FRESH | Sprint needs a full re-run from scratch (e.g., work was corrupted or insufficient). |\n")
	b.WriteString("| CONTINUE_NEXT | The active sprint is actually done but wasn't recorded as complete. Start the next unstarted sprint. |\n")
	b.WriteString("| ALL_COMPLETE | All sprints have passed. Nothing to do. |\n")
	b.WriteString("| BLOCKED | Cannot continue without user action (e.g., Docker not running, missing tools). |\n\n")

	b.WriteString("## Important Guidelines\n\n")
	b.WriteString("- If the last failure was an environment issue (Docker not running, missing tool) and the\n")
	b.WriteString("  environment is now fixed, recommend RESUME_RETRY so verification re-runs.\n")
	b.WriteString("- If the environment issue is still present, recommend BLOCKED with preconditions.\n")
	b.WriteString("- If partial work exists (iterations ran, code was written) but verification failed\n")
	b.WriteString("  for code reasons, recommend RESUME_RETRY to attempt healing.\n")
	b.WriteString("- If no work exists for the next sprint, recommend RESUME_FRESH.\n")
	b.WriteString("- If there's evidence of successful iterations but no PASS recorded, check\n")
	b.WriteString("  whether the work looks complete and recommend RESUME_RETRY or CONTINUE_NEXT.\n\n")

	b.WriteString("## Output Format\n\n")
	b.WriteString("Write your analysis to .fry/continue-decision.txt in EXACTLY this format:\n\n")
	b.WriteString("```\n")
	b.WriteString("## Analysis\n")
	b.WriteString("<2-5 sentences about what happened and why the build stopped>\n\n")
	b.WriteString("## Decision\n\n")
	b.WriteString("<verdict>VERDICT_HERE</verdict>\n")
	b.WriteString("<sprint>N</sprint>\n")
	b.WriteString("<reason>1-2 sentence explanation</reason>\n\n")
	b.WriteString("## Pre-conditions\n")
	b.WriteString("- [ ] Action the user must take (if any)\n\n")
	b.WriteString("## Recommended Command\n")
	b.WriteString("fry run --retry --sprint N  (or whatever is appropriate)\n")
	b.WriteString("```\n")

	return b.String()
}

func truncate(s string, max int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	return textutil.TruncateUTF8(s, max)
}
