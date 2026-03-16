package audit

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/yevgetman/fry/internal/config"
	"github.com/yevgetman/fry/internal/engine"
	"github.com/yevgetman/fry/internal/epic"
	frylog "github.com/yevgetman/fry/internal/log"
	"github.com/yevgetman/fry/internal/textutil"
)

type AuditOpts struct {
	ProjectDir string
	Sprint     *epic.Sprint
	Epic       *epic.Epic
	Engine     engine.Engine
	GitDiff    string                // initial diff; used if DiffFn is nil
	DiffFn     func() (string, error) // if set, called before each audit pass to refresh the diff
	Verbose    bool
	Mode       string
}

type AuditResult struct {
	Passed      bool
	Blocking    bool   // true when CRITICAL or HIGH issues remain after all iterations
	Iterations  int
	MaxSeverity string // "CRITICAL", "HIGH", "MODERATE", "LOW", or ""
}

func RunAuditLoop(ctx context.Context, opts AuditOpts) (*AuditResult, error) {
	if opts.Epic == nil || opts.Sprint == nil {
		return nil, fmt.Errorf("run audit loop: epic and sprint are required")
	}
	if opts.Engine == nil {
		return nil, fmt.Errorf("run audit loop: engine is required")
	}

	maxIter := opts.Epic.MaxAuditIterations
	if maxIter <= 0 {
		maxIter = config.DefaultMaxAuditIterations
	}

	buildLogsDir := filepath.Join(opts.ProjectDir, config.BuildLogsDir)
	if err := os.MkdirAll(buildLogsDir, 0o755); err != nil {
		return nil, fmt.Errorf("run audit loop: create build logs dir: %w", err)
	}

	for iter := 1; iter <= maxIter; iter++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Refresh diff if a diff function is available
		if opts.DiffFn != nil {
			if freshDiff, diffErr := opts.DiffFn(); diffErr == nil {
				opts.GitDiff = freshDiff
			}
		}

		// Build and write audit prompt
		auditPrompt := buildAuditPrompt(opts)
		promptPath := filepath.Join(opts.ProjectDir, config.AuditPromptFile)
		if err := os.MkdirAll(filepath.Dir(promptPath), 0o755); err != nil {
			return nil, fmt.Errorf("run audit loop: create fry dir: %w", err)
		}
		if err := os.WriteFile(promptPath, []byte(auditPrompt), 0o644); err != nil {
			return nil, fmt.Errorf("run audit loop: write audit prompt: %w", err)
		}

		frylog.Log(
			"▶ AUDIT  sprint %d/%d \"%s\"  pass %d/%d  engine=%s",
			opts.Sprint.Number,
			opts.Epic.TotalSprints,
			opts.Sprint.Name,
			iter,
			maxIter,
			opts.Engine.Name(),
		)

		// Run audit agent
		auditLogPath := filepath.Join(
			buildLogsDir,
			fmt.Sprintf("sprint%d_audit%d_%s.log", opts.Sprint.Number, iter, time.Now().Format("20060102_150405")),
		)
		if _, err := runAgentWithLog(ctx, opts, config.AuditInvocationPrompt, auditLogPath); err != nil {
			return nil, err
		}

		// Read and parse audit findings
		auditFilePath := filepath.Join(opts.ProjectDir, config.SprintAuditFile)
		content, err := os.ReadFile(auditFilePath)
		if err != nil {
			if os.IsNotExist(err) {
				// Audit agent didn't write findings — treat as pass
				frylog.Log("  AUDIT: no findings file written — treating as pass.")
				return &AuditResult{Passed: true, Iterations: iter}, nil
			}
			return nil, fmt.Errorf("run audit loop: read audit file: %w", err)
		}

		maxSev := parseAuditSeverity(string(content))
		if isAuditPass(maxSev) {
			frylog.Log("  AUDIT: pass (max severity: %s)", displaySeverity(maxSev))
			return &AuditResult{Passed: true, Iterations: iter, MaxSeverity: maxSev}, nil
		}

		frylog.Log("  AUDIT: %s issues found — running fix agent...", maxSev)

		// Build and write fix prompt
		fixPrompt := buildAuditFixPrompt(opts, string(content))
		if err := os.WriteFile(promptPath, []byte(fixPrompt), 0o644); err != nil {
			return nil, fmt.Errorf("run audit loop: write fix prompt: %w", err)
		}

		// Run fix agent
		fixLogPath := filepath.Join(
			buildLogsDir,
			fmt.Sprintf("sprint%d_auditfix_%d_%s.log", opts.Sprint.Number, iter, time.Now().Format("20060102_150405")),
		)
		if _, err := runAgentWithLog(ctx, opts, config.AuditFixInvocationPrompt, fixLogPath); err != nil {
			return nil, err
		}

		// Remove stale audit file before re-audit
		_ = os.Remove(auditFilePath)
	}

	// Refresh diff for final pass
	if opts.DiffFn != nil {
		if freshDiff, diffErr := opts.DiffFn(); diffErr == nil {
			opts.GitDiff = freshDiff
		}
	}

	// Final audit-only pass to check current state
	auditPrompt := buildAuditPrompt(opts)
	promptPath := filepath.Join(opts.ProjectDir, config.AuditPromptFile)
	if err := os.WriteFile(promptPath, []byte(auditPrompt), 0o644); err != nil {
		return nil, fmt.Errorf("run audit loop: write final audit prompt: %w", err)
	}

	frylog.Log(
		"▶ AUDIT  sprint %d/%d \"%s\"  final pass  engine=%s",
		opts.Sprint.Number,
		opts.Epic.TotalSprints,
		opts.Sprint.Name,
		opts.Engine.Name(),
	)

	finalLogPath := filepath.Join(
		buildLogsDir,
		fmt.Sprintf("sprint%d_audit_final_%s.log", opts.Sprint.Number, time.Now().Format("20060102_150405")),
	)
	if _, err := runAgentWithLog(ctx, opts, config.AuditInvocationPrompt, finalLogPath); err != nil {
		return nil, err
	}

	auditFilePath := filepath.Join(opts.ProjectDir, config.SprintAuditFile)
	content, err := os.ReadFile(auditFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return &AuditResult{Passed: true, Iterations: maxIter}, nil
		}
		return nil, fmt.Errorf("run audit loop: read final audit file: %w", err)
	}

	maxSev := parseAuditSeverity(string(content))
	if isAuditPass(maxSev) {
		return &AuditResult{Passed: true, Iterations: maxIter, MaxSeverity: maxSev}, nil
	}

	return &AuditResult{
		Passed:      false,
		Blocking:    isBlockingSeverity(maxSev),
		Iterations:  maxIter,
		MaxSeverity: maxSev,
	}, nil
}

func buildAuditPrompt(opts AuditOpts) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("# SPRINT AUDIT — Sprint %d: %s\n\n", opts.Sprint.Number, opts.Sprint.Name))

	b.WriteString("## Your Role\n")
	if opts.Mode == "writing" {
		b.WriteString("You are a content auditor. Review the written content completed in this sprint.\n")
		b.WriteString("Do NOT modify any content files. Only write your findings.\n\n")
	} else {
		b.WriteString("You are a code auditor. Review the work completed in this sprint.\n")
		b.WriteString("Do NOT modify any source code. Only write your findings.\n\n")
	}

	// Executive context (condensed)
	executivePath := filepath.Join(opts.ProjectDir, config.ExecutiveFile)
	if data, err := os.ReadFile(executivePath); err == nil {
		executive := string(data)
		if len(executive) > 2000 {
			executive = executive[:2000] + "\n...(truncated)"
		}
		b.WriteString("## Project Context\n")
		b.WriteString(executive)
		b.WriteString("\n\n")
	}

	// Sprint goals
	b.WriteString("## Sprint Goals\n")
	b.WriteString(opts.Sprint.Prompt)
	b.WriteString("\n\n")

	// Sprint progress (capped to avoid oversized prompts)
	progressPath := filepath.Join(opts.ProjectDir, config.SprintProgressFile)
	if data, err := os.ReadFile(progressPath); err == nil && len(data) > 0 {
		progress := string(data)
		const maxProgressBytes = 50_000
		if len(progress) > maxProgressBytes {
			progress = textutil.TruncateUTF8(progress, maxProgressBytes) + "\n...(sprint progress truncated at 50KB)"
		}
		b.WriteString("## What Was Done\n")
		b.WriteString(progress)
		b.WriteString("\n\n")
	}

	// Git diff
	b.WriteString("## Changes Made This Sprint\n")
	diff := opts.GitDiff
	if len(diff) > config.MaxAuditDiffBytes {
		diff = diff[:config.MaxAuditDiffBytes] + "\n...(diff truncated at 100KB)"
	}
	if strings.TrimSpace(diff) == "" {
		diff = "(no changes detected)"
	}
	b.WriteString("```diff\n")
	b.WriteString(diff)
	b.WriteString("\n```\n\n")

	// Audit criteria
	b.WriteString("## Audit Criteria\n")
	if opts.Mode == "writing" {
		b.WriteString("Review the sprint's written content against these criteria:\n")
		b.WriteString("1. **Coherence** — Does the content flow logically and tell a consistent story?\n")
		b.WriteString("2. **Accuracy** — Are factual claims correct and properly supported?\n")
		b.WriteString("3. **Completeness** — Does the content cover all required topics at sufficient depth?\n")
		b.WriteString("4. **Tone & Voice** — Is the writing voice consistent and appropriate for the audience?\n")
		b.WriteString("5. **Structure** — Are sections well-organized with clear headings and transitions?\n")
		b.WriteString("6. **Depth** — Is the content substantive rather than superficial or padded?\n\n")

		b.WriteString("## Severity Levels\n")
		b.WriteString("| Level | Description |\n")
		b.WriteString("|---|---|\n")
		b.WriteString("| CRITICAL | Factual errors, contradictions, or missing core content |\n")
		b.WriteString("| HIGH | Major structural problems or significant gaps in coverage |\n")
		b.WriteString("| MODERATE | Weak transitions, inconsistent voice, or shallow treatment |\n")
		b.WriteString("| LOW | Minor style, formatting, or word choice issues |\n\n")
	} else {
		b.WriteString("Review the sprint's work against these criteria:\n")
		b.WriteString("1. **Correctness** — Does the code do what the sprint goals require?\n")
		b.WriteString("2. **Usability** — Are APIs, CLIs, and interfaces intuitive and consistent?\n")
		b.WriteString("3. **Edge Cases** — Are boundary conditions and error paths handled?\n")
		b.WriteString("4. **Security** — Are there injection, auth, or data-exposure risks?\n")
		b.WriteString("5. **Performance** — Are there obvious bottlenecks or resource leaks?\n")
		b.WriteString("6. **Code Quality** — Is the code readable, well-structured, and idiomatic?\n\n")

		b.WriteString("## Severity Levels\n")
		b.WriteString("| Level | Description |\n")
		b.WriteString("|---|---|\n")
		b.WriteString("| CRITICAL | Data loss, security breach, or crash under normal use |\n")
		b.WriteString("| HIGH | Significant bug; affects core functionality |\n")
		b.WriteString("| MODERATE | Edge case gaps, poor error handling, quality concerns |\n")
		b.WriteString("| LOW | Style, naming, cosmetic |\n\n")
	}

	// Output instructions
	b.WriteString("## Output\n")
	b.WriteString("Write your findings to .fry/sprint-audit.txt in this format:\n\n")
	b.WriteString("```\n")
	b.WriteString("## Summary\n")
	b.WriteString("<1-2 sentence overview>\n\n")
	b.WriteString("## Findings\n")
	b.WriteString("For each issue:\n")
	b.WriteString("- **Location:** <file:line>\n")
	b.WriteString("- **Description:** <what is wrong>\n")
	b.WriteString("- **Severity:** CRITICAL | HIGH | MODERATE | LOW\n")
	b.WriteString("- **Recommended Fix:** <how to fix>\n\n")
	b.WriteString("## Verdict\n")
	b.WriteString("PASS (no issues or all LOW) or FAIL (CRITICAL/HIGH/MODERATE found)\n")
	b.WriteString("```\n")

	return b.String()
}

func buildAuditFixPrompt(opts AuditOpts, auditFindings string) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("# AUDIT FIX — Sprint %d: %s\n\n", opts.Sprint.Number, opts.Sprint.Name))
	if opts.Mode == "writing" {
		b.WriteString("The content audit found issues. Fix ONLY the CRITICAL, HIGH, and MODERATE\n")
		b.WriteString("issues listed below. Do NOT fix LOW issues. Make minimal editorial changes.\n\n")
	} else {
		b.WriteString("The sprint audit found issues. Fix ONLY the CRITICAL, HIGH, and MODERATE\n")
		b.WriteString("issues listed below. Do NOT fix LOW issues. Make minimal changes.\n\n")
	}

	b.WriteString("## Audit Findings\n")
	b.WriteString(auditFindings)
	b.WriteString("\n\n")

	b.WriteString("## Context\n")
	b.WriteString(fmt.Sprintf("- Read %s for what was built\n", config.SprintProgressFile))
	b.WriteString(fmt.Sprintf("- Read %s for strategic context\n\n", config.PlanFile))

	b.WriteString(fmt.Sprintf("Append a brief note to %s about what you fixed.\n", config.SprintProgressFile))

	return b.String()
}

// severityLabelRe matches lines containing a severity label (e.g., "**Severity:**" or "Severity:").
var severityLabelRe = regexp.MustCompile(`(?i)\bseverity\b`)

// severityWordRe matches whole-word severity keywords to avoid false positives
// from substrings like "HIGHLY", "HIGHLIGHTED", "CRITICALLY", "ALLOW", etc.
var severityWordRe = regexp.MustCompile(`\b(CRITICAL|HIGH|MODERATE|LOW)\b`)

func parseAuditSeverity(content string) string {
	// Look for severity markers in structured finding lines only.
	// Matches patterns like "**Severity:** CRITICAL" or "Severity: HIGH"
	// to avoid false positives from words appearing in diffs or prose.
	// Uses word-boundary matching to prevent substring false positives
	// (e.g., "HIGHLY" should not match "HIGH").
	// Only the FIRST severity keyword after the "Severity" label on each line
	// is considered — additional keywords in trailing prose are ignored.
	maxSev := ""
	for _, line := range strings.Split(content, "\n") {
		// Match lines containing "severity" as a label
		if !severityLabelRe.MatchString(line) {
			continue
		}
		upper := strings.ToUpper(line)
		// Extract only the first severity keyword on this line
		m := severityWordRe.FindString(upper)
		if m == "" {
			continue
		}
		if severityRank(m) > severityRank(maxSev) {
			maxSev = m
		}
		if maxSev == "CRITICAL" {
			return "CRITICAL" // highest possible, return immediately
		}
	}
	return maxSev
}

func severityRank(sev string) int {
	switch sev {
	case "CRITICAL":
		return 4
	case "HIGH":
		return 3
	case "MODERATE":
		return 2
	case "LOW":
		return 1
	default:
		return 0
	}
}

func isAuditPass(maxSeverity string) bool {
	return maxSeverity == "" || maxSeverity == "LOW"
}

func isBlockingSeverity(maxSeverity string) bool {
	return maxSeverity == "CRITICAL" || maxSeverity == "HIGH"
}

func Cleanup(projectDir string) error {
	for _, rel := range []string{config.SprintAuditFile, config.AuditPromptFile} {
		path := filepath.Join(projectDir, rel)
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("audit cleanup: %w", err)
		}
	}
	return nil
}

func displaySeverity(sev string) string {
	if sev == "" {
		return "none"
	}
	return sev
}

func runAgentWithLog(ctx context.Context, opts AuditOpts, prompt, logPath string) (string, error) {
	logFile, err := os.Create(logPath)
	if err != nil {
		return "", fmt.Errorf("run audit loop: create log: %w", err)
	}
	defer logFile.Close()

	runOpts := engine.RunOpts{
		Model:   opts.Epic.AuditModel,
		WorkDir: opts.ProjectDir,
	}

	if opts.Verbose {
		writer := io.MultiWriter(os.Stdout, logFile)
		runOpts.Stdout = writer
		runOpts.Stderr = writer
		output, _, runErr := opts.Engine.Run(ctx, prompt, runOpts)
		if runErr != nil && ctx.Err() == nil {
			frylog.Log("WARNING: agent exited with error (non-fatal): %v", runErr)
			return output, nil
		}
		return output, runErr
	}

	runOpts.Stdout = logFile
	runOpts.Stderr = logFile
	output, _, runErr := opts.Engine.Run(ctx, prompt, runOpts)
	if runErr != nil && ctx.Err() == nil {
		frylog.Log("WARNING: agent exited with error (non-fatal): %v", runErr)
		return output, nil
	}
	return output, runErr
}
