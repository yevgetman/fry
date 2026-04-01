package audit

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
	"github.com/yevgetman/fry/internal/sprint"
	"github.com/yevgetman/fry/internal/textutil"
)

const (
	maxBuildAuditExecutiveBytes  = 5_000
	maxBuildAuditPlanBytes       = 50_000
	maxBuildAuditEpicBytes       = 50_000
	maxBuildAuditUserPromptBytes = 2_000
)

type BuildAuditOpts struct {
	ProjectDir       string
	Epic             *epic.Epic
	Engine           engine.Engine
	Results          []sprint.SprintResult
	Verbose          bool
	Model            string
	DeferredFailures string // content of .fry/deferred-failures.md
	Mode             string
	Stdout           io.Writer // optional; defaults to os.Stdout when Verbose is true
}

// RunBuildAudit performs a final holistic audit of the entire codebase after
// all sprints have completed. It launches a single agent session that iteratively
// audits, classifies, reports, and remediates issues across the full build.
// The returned AuditResult contains structured findings parsed from the agent's
// report, enabling downstream consumers (e.g., build summary) to include audit results.
func RunBuildAudit(ctx context.Context, opts BuildAuditOpts) (*AuditResult, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	if opts.Epic == nil {
		return nil, fmt.Errorf("run build audit: epic is required")
	}
	if opts.Engine == nil {
		return nil, fmt.Errorf("run build audit: engine is required")
	}

	frylog.Log("▶ BUILD AUDIT  running final holistic audit for %q", opts.Epic.Name)

	prompt := buildBuildAuditPrompt(opts)

	// Write prompt file
	promptPath := filepath.Join(opts.ProjectDir, config.BuildAuditPromptFile)
	if err := os.MkdirAll(filepath.Dir(promptPath), 0o755); err != nil {
		return nil, fmt.Errorf("run build audit: create dir: %w", err)
	}
	if err := os.WriteFile(promptPath, []byte(prompt), 0o644); err != nil {
		return nil, fmt.Errorf("run build audit: write prompt: %w", err)
	}
	defer func() { _ = os.Remove(promptPath) }()

	// Create log file
	buildLogsDir := filepath.Join(opts.ProjectDir, config.BuildLogsDir)
	if err := os.MkdirAll(buildLogsDir, 0o755); err != nil {
		return nil, fmt.Errorf("run build audit: create logs dir: %w", err)
	}
	logPath := filepath.Join(buildLogsDir,
		fmt.Sprintf("build_audit_%s.log", time.Now().Format("20060102_150405")),
	)
	logFile, err := os.Create(logPath)
	if err != nil {
		return nil, fmt.Errorf("run build audit: create log: %w", err)
	}
	defer func() { _ = logFile.Close() }()

	runOpts := engine.RunOpts{
		Model:       opts.Model,
		SessionType: engine.SessionBuildAudit,
		EffortLevel: string(opts.Epic.EffortLevel),
		WorkDir:     opts.ProjectDir,
	}

	if opts.Verbose {
		stdout := opts.Stdout
		if stdout == nil {
			stdout = os.Stdout
		}
		writer := io.MultiWriter(stdout, logFile)
		runOpts.Stdout = writer
		runOpts.Stderr = writer
	} else {
		runOpts.Stdout = logFile
		runOpts.Stderr = logFile
	}

	output, _, runErr := opts.Engine.Run(ctx, config.BuildAuditInvocationPrompt, runOpts)
	if runErr != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("run build audit: agent run: %w", runErr)
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	// Parse audit results from the report file
	auditPath := filepath.Join(opts.ProjectDir, config.BuildAuditFile)
	content, readErr := readAuditOutput(
		auditPath,
		config.BuildAuditFile,
		"build audit session",
		"BUILD AUDIT",
		"run build audit",
		opts.ProjectDir,
		output,
		logPath,
	)
	if readErr != nil {
		return nil, readErr
	}

	frylog.Log("  BUILD AUDIT: report written to %s", config.BuildAuditFile)

	// Parse structured results from the report
	maxSev := parseAuditSeverity(string(content))
	counts := countAuditSeverities(string(content))
	findings := parseFindings(string(content))

	return &AuditResult{
		Passed:             isAuditPass(maxSev),
		Blocking:           isBlockingSeverity(maxSev),
		Iterations:         1,
		MaxSeverity:        maxSev,
		SeverityCounts:     counts,
		UnresolvedFindings: findings,
	}, nil
}

func buildBuildAuditPrompt(opts BuildAuditOpts) string {
	var b strings.Builder

	b.WriteString("# FINAL BUILD AUDIT\n\n")
	fmt.Fprintf(&b, "Holistic post-build audit for epic: **%s**\n\n", opts.Epic.Name)
	b.WriteString("Use the current repository state as primary evidence.\n")
	b.WriteString("Treat generated progress artifacts as supporting context, not proof.\n\n")

	appendCodebaseContext(&b, opts.ProjectDir)

	// --- Project context from selected artifacts ---

	// Executive summary
	executivePath := filepath.Join(opts.ProjectDir, config.ExecutiveFile)
	if data, err := os.ReadFile(executivePath); err == nil {
		content := string(data)
		if len(content) > maxBuildAuditExecutiveBytes {
			content = textutil.TruncateUTF8(content, maxBuildAuditExecutiveBytes) + "\n...(truncated)"
		}
		b.WriteString("## Project Context (executive.md)\n")
		b.WriteString(content)
		b.WriteString("\n\n")
	}

	// Implementation plan
	planPath := filepath.Join(opts.ProjectDir, config.PlanFile)
	if data, err := os.ReadFile(planPath); err == nil {
		content := string(data)
		if len(content) > maxBuildAuditPlanBytes {
			content = textutil.TruncateUTF8(content, maxBuildAuditPlanBytes) + "\n...(truncated)"
		}
		b.WriteString("## Implementation Plan (plan.md)\n")
		b.WriteString(content)
		b.WriteString("\n\n")
	}

	// Epic definition
	epicPath := filepath.Join(opts.ProjectDir, config.FryDir, "epic.md")
	if data, err := os.ReadFile(epicPath); err == nil {
		content := string(data)
		if len(content) > maxBuildAuditEpicBytes {
			content = textutil.TruncateUTF8(content, maxBuildAuditEpicBytes) + "\n...(truncated)"
		}
		b.WriteString("## Epic Definition (epic.md)\n")
		b.WriteString("```\n")
		b.WriteString(content)
		b.WriteString("\n```\n\n")
	}

	// Original user prompt
	userPromptPath := filepath.Join(opts.ProjectDir, config.UserPromptFile)
	if data, err := os.ReadFile(userPromptPath); err == nil && len(data) > 0 {
		content := string(data)
		if len(content) > maxBuildAuditUserPromptBytes {
			content = textutil.TruncateUTF8(content, maxBuildAuditUserPromptBytes) + "\n...(truncated)"
		}
		b.WriteString("## Original User Prompt\n")
		b.WriteString(content)
		b.WriteString("\n\n")
	}

	// Sprint results table
	b.WriteString("## Build Results\n\n")
	b.WriteString("| Sprint | Name | Status | Duration |\n")
	b.WriteString("|--------|------|--------|----------|\n")
	for _, r := range opts.Results {
		fmt.Fprintf(&b, "| %d | %s | %s | %s |\n",
			r.Number, r.Name, r.Status, r.Duration.Round(time.Second))
	}
	b.WriteString("\n")

	// Deferred sanity check failures
	if strings.TrimSpace(opts.DeferredFailures) != "" {
		b.WriteString("## Deferred Sanity Check Failures\n\n")
		b.WriteString("The following sanity checks failed during sprints but were below the\n")
		b.WriteString("failure threshold. Attempt to fix these as part of your audit remediation.\n\n")
		b.WriteString(opts.DeferredFailures)
		b.WriteString("\n\n")
	}

	// --- Iterative audit instructions (adapted from audit-prompt.md) ---

	b.WriteString("---\n\n")
	b.WriteString("## Instructions\n\n")
	iterCap := config.MaxOuterCyclesHighCap
	if opts.Epic != nil && opts.Epic.EffortLevel == epic.EffortMax {
		iterCap = config.MaxOuterCyclesMaxCap
	}
	fmt.Fprintf(&b, "Repeat the following cycle until the EXIT CONDITION is met (max %d iterations).\n\n", iterCap)

	b.WriteString("### Step 1 — Audit\n\n")
	if opts.Mode == "writing" {
		b.WriteString("Meticulously audit all written content against these criteria:\n\n")
		b.WriteString("- **Coherence** — Content flows logically and tells a consistent story throughout.\n")
		b.WriteString("- **Accuracy** — Factual claims are correct and properly supported.\n")
		b.WriteString("- **Completeness** — All required topics are covered at sufficient depth.\n")
		b.WriteString("- **Tone & Voice** — Writing voice is consistent and appropriate for the audience.\n")
		b.WriteString("- **Structure** — Sections are well-organized with clear headings and transitions.\n")
		b.WriteString("- **Depth** — Content is substantive rather than superficial or padded.\n\n")

		b.WriteString("### Step 2 — Classify\n\n")
		b.WriteString("Assign every finding a severity:\n\n")
		b.WriteString("| Severity | Definition |\n")
		b.WriteString("|----------|------------|\n")
		b.WriteString("| CRITICAL | Factual errors, contradictions, or missing core content |\n")
		b.WriteString("| HIGH | Major structural problems or significant gaps in coverage |\n")
		b.WriteString("| MODERATE | Weak transitions, inconsistent voice, or shallow treatment |\n")
		b.WriteString("| LOW | Minor style, formatting, or word choice issues |\n\n")
	} else {
		b.WriteString("Meticulously audit the entire codebase against these criteria:\n\n")
		b.WriteString("- **Correctness** — Code is coherent with the aim and function of the application; no bugs.\n")
		b.WriteString("- **Usability** — No UX friction, confusing flows, or accessibility gaps.\n")
		b.WriteString("- **Edge cases** — Boundary conditions, empty states, invalid input, and race conditions are handled.\n")
		b.WriteString("- **Security** — No vulnerabilities (injection, auth flaws, data exposure, etc.).\n")
		b.WriteString("- **Performance** — No bottlenecks, memory leaks, or unnecessary complexity.\n")
		b.WriteString("- **Code quality** — Clean style, consistent patterns, clear naming, appropriate abstractions.\n\n")

		b.WriteString("### Step 2 — Classify\n\n")
		b.WriteString("Assign every finding a severity:\n\n")
		b.WriteString("| Severity | Definition |\n")
		b.WriteString("|----------|------------|\n")
		b.WriteString("| CRITICAL | Data loss, security breach, or crash under normal use |\n")
		b.WriteString("| HIGH | Significant bug or vulnerability; affects core functionality |\n")
		b.WriteString("| MODERATE | Noticeable issue; degraded experience or maintainability risk |\n")
		b.WriteString("| LOW | Minor style, naming, or cosmetic concern |\n\n")
	}

	b.WriteString("### Step 3 — Report\n\n")
	fmt.Fprintf(&b, "Save a comprehensive report to `%s` in the project root. For each finding include: location, description, severity, and recommended fix.\n\n", config.BuildAuditFile)

	b.WriteString("### Step 4 — Evaluate exit condition\n\n")
	fmt.Fprintf(&b, "- **If no issues exist, or all remaining issues are LOW severity → save the final report to `%s` and stop.** ← EXIT CONDITION\n", config.BuildAuditFile)
	b.WriteString("- **Otherwise → continue to Step 5.**\n\n")

	b.WriteString("### Step 5 — Remediate\n\n")
	fmt.Fprintf(&b, "Using `%s` as your plan, fix **all** issues (including LOW severity). Also fix any deferred sanity check failures listed above. Then return to **Step 1** and re-audit the modified codebase.\n\n", config.BuildAuditFile)

	b.WriteString("---\n\n")
	b.WriteString("### Guardrails\n\n")
	fmt.Fprintf(&b, "- Each iteration must produce a fresh `%s` that reflects the *current* state of the code.\n", config.BuildAuditFile)
	b.WriteString("- Do not skip issues to force an early exit; report honestly.\n")
	fmt.Fprintf(&b, "- If you reach %d iterations without meeting the exit condition, stop and report the remaining issues with an explanation of why they persist.\n", iterCap)

	return b.String()
}
