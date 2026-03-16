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
}

// RunBuildAudit performs a final holistic audit of the entire codebase after
// all sprints have completed. It launches a single agent session that iteratively
// audits, classifies, reports, and remediates issues across the full build.
func RunBuildAudit(ctx context.Context, opts BuildAuditOpts) error {
	if opts.Epic == nil {
		return fmt.Errorf("run build audit: epic is required")
	}
	if opts.Engine == nil {
		return fmt.Errorf("run build audit: engine is required")
	}

	frylog.Log("▶ BUILD AUDIT  running final holistic audit for %q", opts.Epic.Name)

	prompt := buildBuildAuditPrompt(opts)

	// Write prompt file
	promptPath := filepath.Join(opts.ProjectDir, config.BuildAuditPromptFile)
	if err := os.MkdirAll(filepath.Dir(promptPath), 0o755); err != nil {
		return fmt.Errorf("run build audit: create dir: %w", err)
	}
	if err := os.WriteFile(promptPath, []byte(prompt), 0o644); err != nil {
		return fmt.Errorf("run build audit: write prompt: %w", err)
	}

	// Create log file
	buildLogsDir := filepath.Join(opts.ProjectDir, config.BuildLogsDir)
	if err := os.MkdirAll(buildLogsDir, 0o755); err != nil {
		return fmt.Errorf("run build audit: create logs dir: %w", err)
	}
	logPath := filepath.Join(buildLogsDir,
		fmt.Sprintf("build_audit_%s.log", time.Now().Format("20060102_150405")),
	)
	logFile, err := os.Create(logPath)
	if err != nil {
		return fmt.Errorf("run build audit: create log: %w", err)
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

	_, _, runErr := opts.Engine.Run(ctx, config.BuildAuditInvocationPrompt, runOpts)
	if runErr != nil && ctx.Err() == nil {
		frylog.Log("  BUILD AUDIT: agent exited with error (non-fatal): %v", runErr)
	} else if runErr != nil {
		return fmt.Errorf("run build audit: %w", runErr)
	}

	// Verify audit file was produced
	auditPath := filepath.Join(opts.ProjectDir, config.BuildAuditFile)
	if _, err := os.Stat(auditPath); err != nil {
		frylog.Log("  BUILD AUDIT: WARNING — agent did not produce %s", config.BuildAuditFile)
		return nil
	}

	frylog.Log("  BUILD AUDIT: report written to %s", config.BuildAuditFile)

	// Cleanup prompt file
	_ = os.Remove(promptPath)

	return nil
}

func buildBuildAuditPrompt(opts BuildAuditOpts) string {
	var b strings.Builder

	b.WriteString("# FINAL BUILD AUDIT\n\n")
	b.WriteString(fmt.Sprintf("Holistic post-build audit for epic: **%s**\n\n", opts.Epic.Name))

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
		b.WriteString(fmt.Sprintf("| %d | %s | %s | %s |\n",
			r.Number, r.Name, r.Status, r.Duration.Round(time.Second)))
	}
	b.WriteString("\n")

	// Deferred verification failures
	if strings.TrimSpace(opts.DeferredFailures) != "" {
		b.WriteString("## Deferred Verification Failures\n\n")
		b.WriteString("The following verification checks failed during sprints but were below the\n")
		b.WriteString("failure threshold. Attempt to fix these as part of your audit remediation.\n\n")
		b.WriteString(opts.DeferredFailures)
		b.WriteString("\n\n")
	}

	// --- Iterative audit instructions (adapted from audit-prompt.md) ---

	b.WriteString("---\n\n")
	b.WriteString("## Instructions\n\n")
	b.WriteString("Repeat the following cycle until the EXIT CONDITION is met (max 10 iterations).\n\n")

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
	b.WriteString(fmt.Sprintf("Save a comprehensive report to `%s` in the project root. For each finding include: location, description, severity, and recommended fix.\n\n", config.BuildAuditFile))

	b.WriteString("### Step 4 — Evaluate exit condition\n\n")
	b.WriteString(fmt.Sprintf("- **If no issues exist, or all remaining issues are LOW severity → save the final report to `%s` and stop.** ← EXIT CONDITION\n", config.BuildAuditFile))
	b.WriteString("- **Otherwise → continue to Step 5.**\n\n")

	b.WriteString("### Step 5 — Remediate\n\n")
	b.WriteString(fmt.Sprintf("Using `%s` as your plan, fix **all** issues (including LOW severity). Also fix any deferred verification failures listed above. Then return to **Step 1** and re-audit the modified codebase.\n\n", config.BuildAuditFile))

	b.WriteString("---\n\n")
	b.WriteString("### Guardrails\n\n")
	b.WriteString(fmt.Sprintf("- Each iteration must produce a fresh `%s` that reflects the *current* state of the code.\n", config.BuildAuditFile))
	b.WriteString("- Do not skip issues to force an early exit; report honestly.\n")
	b.WriteString("- If you reach 10 iterations without meeting the exit condition, stop and report the remaining issues with an explanation of why they persist.\n")

	return b.String()
}
