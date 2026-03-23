package audit

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/yevgetman/fry/internal/config"
	"github.com/yevgetman/fry/internal/engine"
	"github.com/yevgetman/fry/internal/epic"
	frylog "github.com/yevgetman/fry/internal/log"
	"github.com/yevgetman/fry/internal/textutil"
)

// Finding represents a single structured audit finding tracked across iterations.
type Finding struct {
	Location       string
	Description    string
	Severity       string
	RecommendedFix string
	OriginCycle    int  // which outer audit cycle discovered this finding
	Resolved       bool // whether this finding has been verified resolved
}

// key returns a normalized identity for deduplication and comparison across cycles.
func (f Finding) key() string {
	return strings.ToLower(strings.TrimSpace(f.Description))
}

// isActionable returns true if the finding has severity above LOW and is not resolved.
// This is the pass/fail predicate — LOW findings never affect pass/fail.
func (f Finding) isActionable() bool {
	return f.Severity != "" && f.Severity != "LOW" && !f.Resolved
}

// fixIncludesLow returns true if the effort level includes LOW findings in fix scope.
// At high and max effort, the fix agent addresses LOW findings alongside higher-severity items.
func fixIncludesLow(ep *epic.Epic) bool {
	return ep.EffortLevel == epic.EffortHigh || ep.EffortLevel == epic.EffortMax
}

type AuditOpts struct {
	ProjectDir string
	Sprint     *epic.Sprint
	Epic       *epic.Epic
	Engine     engine.Engine
	GitDiff    string                 // initial diff; used if DiffFn is nil
	DiffFn     func() (string, error) // if set, called before each audit pass to refresh the diff
	Verbose    bool
	Mode       string
	Stdout     io.Writer // optional; defaults to os.Stdout when Verbose is true
}

type AuditResult struct {
	Passed             bool
	Blocking           bool           // true when CRITICAL or HIGH issues remain after all cycles
	Iterations         int            // number of outer audit cycles completed
	MaxSeverity        string         // "CRITICAL", "HIGH", "MODERATE", "LOW", or ""
	SeverityCounts     map[string]int // count of findings per severity level
	UnresolvedFindings []Finding      // remaining findings after all cycles
}

const (
	maxStaleIterations      = 3 // outer loop stale threshold
	maxInnerStaleIterations = 2 // inner loop stale threshold
)

// RunAuditLoop runs a two-level audit loop for a sprint.
//
// Outer loop (audit cycles): each cycle runs the audit agent to discover issues,
// then enters an inner fix loop to resolve them. After the inner loop resolves all
// issues (or stalls), a re-audit verifies resolution and discovers new issues.
//
// Inner loop (fix iterations): for each audit report, the fix agent runs repeatedly
// until all issues above LOW severity are resolved, or the inner cap is reached.
// Issues are presented to the fix agent in FIFO order (oldest first).
func RunAuditLoop(ctx context.Context, opts AuditOpts) (*AuditResult, error) {
	if opts.Epic == nil || opts.Sprint == nil {
		return nil, fmt.Errorf("run audit loop: epic and sprint are required")
	}
	if opts.Engine == nil {
		return nil, fmt.Errorf("run audit loop: engine is required")
	}

	maxOuter, progressBased := effectiveOuterCycles(opts.Epic)
	maxInner := effectiveInnerIter(opts.Epic)
	includeLow := fixIncludesLow(opts.Epic)

	buildLogsDir := filepath.Join(opts.ProjectDir, config.BuildLogsDir)
	if err := os.MkdirAll(buildLogsDir, 0o755); err != nil {
		return nil, fmt.Errorf("run audit loop: create build logs dir: %w", err)
	}

	auditFilePath := filepath.Join(opts.ProjectDir, config.SprintAuditFile)
	promptPath := filepath.Join(opts.ProjectDir, config.AuditPromptFile)

	var knownFindings []Finding // tracked across outer cycles
	outerStaleCount := 0
	var lastCycle int

	for cycle := 1; cycle <= maxOuter; cycle++ {
		lastCycle = cycle

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		refreshDiff(&opts)

		// Build and write audit prompt (with known findings for verification on cycle 2+)
		auditPrompt := buildAuditPrompt(opts, knownFindings)
		if err := writePromptFile(promptPath, auditPrompt); err != nil {
			return nil, fmt.Errorf("run audit loop: write audit prompt: %w", err)
		}

		auditModel := engine.ResolveModel(opts.Epic.AuditModel, opts.Engine.Name(), string(opts.Epic.EffortLevel), engine.SessionAudit)
		if progressBased {
			frylog.Log(
				"▶ AUDIT  sprint %d/%d \"%s\"  cycle %d (progress-based, cap %d)  engine=%s  model=%s",
				opts.Sprint.Number, opts.Epic.TotalSprints, opts.Sprint.Name,
				cycle, maxOuter, opts.Engine.Name(), auditModel,
			)
		} else {
			frylog.Log(
				"▶ AUDIT  sprint %d/%d \"%s\"  cycle %d/%d  engine=%s  model=%s",
				opts.Sprint.Number, opts.Epic.TotalSprints, opts.Sprint.Name,
				cycle, maxOuter, opts.Engine.Name(), auditModel,
			)
		}

		// Run audit agent
		auditLogPath := filepath.Join(buildLogsDir,
			fmt.Sprintf("sprint%d_audit%d_%s.log", opts.Sprint.Number, cycle, time.Now().Format("20060102_150405")),
		)
		if _, err := runAgentWithLog(ctx, opts, config.AuditInvocationPrompt, auditLogPath, auditModel); err != nil {
			return nil, err
		}

		// Read audit findings
		content, err := os.ReadFile(auditFilePath)
		if err != nil {
			if os.IsNotExist(err) {
				frylog.Log("  AUDIT: no findings file written — treating as pass.")
				return &AuditResult{Passed: true, Iterations: cycle}, nil
			}
			return nil, fmt.Errorf("run audit loop: read audit file: %w", err)
		}
		_ = os.Remove(auditFilePath)

		// Quick severity check
		maxSev := parseAuditSeverity(string(content))
		counts := countAuditSeverities(string(content))
		if isAuditPass(maxSev) {
			frylog.Log("  AUDIT: pass (%s)", formatSeverityCounts(counts))
			return &AuditResult{
				Passed: true, Iterations: cycle,
				MaxSeverity: maxSev, SeverityCounts: counts,
			}, nil
		}

		// Parse structured findings
		currentFindings := parseFindings(string(content))

		// Fallback: severity indicates issues but no structured findings parsed
		if len(currentFindings) == 0 {
			currentFindings = []Finding{{
				Description: "Audit agent reported issues but structured findings could not be parsed. See raw audit output for details.",
				Severity:    maxSev,
				OriginCycle: cycle,
			}}
		}

		// Classify findings against known set
		var activeFindings []Finding
		if cycle > 1 && len(knownFindings) > 0 {
			resolved, persisting, newFindings := classifyFindings(knownFindings, currentFindings)
			for i := range newFindings {
				newFindings[i].OriginCycle = cycle
			}
			activeFindings = mergeFindings(persisting, newFindings)
			if len(resolved) > 0 {
				frylog.Log("  AUDIT: %d previously known issues resolved", len(resolved))
			}
			if len(newFindings) > 0 {
				frylog.Log("  AUDIT: %d new issues discovered", len(newFindings))
			}
		} else {
			for i := range currentFindings {
				currentFindings[i].OriginCycle = cycle
			}
			activeFindings = currentFindings
		}

		// Check actionable count
		actionable := countActionableFindings(activeFindings)
		if actionable == 0 {
			frylog.Log("  AUDIT: pass (no actionable issues)")
			return &AuditResult{
				Passed: true, Iterations: cycle,
				MaxSeverity: maxSev, SeverityCounts: counts,
			}, nil
		}

		// Outer stale detection (progress-based mode only)
		if progressBased && cycle > 1 {
			prevKeys := findingKeySet(knownFindings)
			currKeys := findingKeySet(activeFindings)
			if !hasProgress(prevKeys, currKeys) {
				outerStaleCount++
				frylog.Log("  AUDIT: no progress detected (%d/%d stale cycles)", outerStaleCount, maxStaleIterations)
				if outerStaleCount >= maxStaleIterations {
					frylog.Log("  AUDIT: stopping — no progress after %d cycles", cycle)
					break
				}
			} else {
				outerStaleCount = 0
			}
		}

		fixableCount := countFixable(activeFindings, includeLow)
		frylog.Log("  AUDIT: %s — entering fix loop (%d issues)...", formatSeverityCounts(counts), fixableCount)

		// Sort findings FIFO for fix agent
		sortFindingsFIFO(activeFindings)

		// Inner fix loop
		innerStaleCount := 0
		lastResolvedCount := 0

		for fixIter := 1; fixIter <= maxInner; fixIter++ {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
			}

			unresolved := filterFixable(activeFindings, includeLow)
			if len(unresolved) == 0 {
				break
			}

			fixModel := engine.ResolveModel(opts.Epic.AuditModel, opts.Engine.Name(), string(opts.Epic.EffortLevel), engine.SessionAuditFix)
			frylog.Log("  AUDIT FIX  cycle %d  fix %d/%d — targeting %d issues (oldest first)  engine=%s  model=%s",
				cycle, fixIter, maxInner, len(unresolved), opts.Engine.Name(), fixModel)

			// Build and write fix prompt
			fixPrompt := buildAuditFixPrompt(opts, unresolved)
			if err := writePromptFile(promptPath, fixPrompt); err != nil {
				return nil, fmt.Errorf("run audit loop: write fix prompt: %w", err)
			}

			// Run fix agent
			fixLogPath := filepath.Join(buildLogsDir,
				fmt.Sprintf("sprint%d_auditfix_%d_%d_%s.log", opts.Sprint.Number, cycle, fixIter, time.Now().Format("20060102_150405")),
			)
			if _, err := runAgentWithLog(ctx, opts, config.AuditFixInvocationPrompt, fixLogPath, fixModel); err != nil {
				return nil, err
			}

			// Remove stale audit file before verify
			_ = os.Remove(auditFilePath)

			// Build and write verify prompt
			verifyPrompt := buildVerifyPrompt(opts, unresolved)
			if err := writePromptFile(promptPath, verifyPrompt); err != nil {
				return nil, fmt.Errorf("run audit loop: write verify prompt: %w", err)
			}

			// Run verify agent
			verifyModel := engine.ResolveModel(opts.Epic.AuditModel, opts.Engine.Name(), string(opts.Epic.EffortLevel), engine.SessionAuditVerify)
			verifyLogPath := filepath.Join(buildLogsDir,
				fmt.Sprintf("sprint%d_auditverify_%d_%d_%s.log", opts.Sprint.Number, cycle, fixIter, time.Now().Format("20060102_150405")),
			)
			if _, err := runAgentWithLog(ctx, opts, config.AuditInvocationPrompt, verifyLogPath, verifyModel); err != nil {
				return nil, err
			}

			// Parse verification results
			verifyContent, verifyErr := os.ReadFile(auditFilePath)
			if verifyErr != nil {
				if os.IsNotExist(verifyErr) {
					// No file = all resolved (optimistic)
					markAllResolved(activeFindings)
					frylog.Log("  AUDIT VERIFY  cycle %d  fix %d/%d — all resolved (no findings file)", cycle, fixIter, maxInner)
					break
				}
				return nil, fmt.Errorf("run audit loop: read verify file: %w", verifyErr)
			}
			_ = os.Remove(auditFilePath)

			resolved := parseVerificationStatuses(string(verifyContent), unresolved)
			applyResolutionsByKey(activeFindings, unresolved, resolved)

			nowResolved := countResolved(activeFindings)
			totalFixable := countFixable(activeFindings, includeLow)
			remaining := filterFixable(activeFindings, includeLow)
			frylog.Log("  AUDIT VERIFY  cycle %d  fix %d/%d — %d of %d resolved",
				cycle, fixIter, maxInner, nowResolved, totalFixable)

			if len(remaining) == 0 {
				break
			}

			// Inner stale detection
			if nowResolved <= lastResolvedCount {
				innerStaleCount++
				if innerStaleCount >= maxInnerStaleIterations {
					frylog.Log("  AUDIT FIX: no progress after %d fix iterations — moving to re-audit", fixIter)
					break
				}
			} else {
				innerStaleCount = 0
			}
			lastResolvedCount = nowResolved
		}

		// Update known findings for next outer cycle
		knownFindings = collectUnresolved(activeFindings)
	}

	// Log remaining LOW findings at high/max effort
	if includeLow {
		lowRemaining := countUnresolvedLow(knownFindings)
		if lowRemaining > 0 {
			frylog.Log("  AUDIT: %d LOW issues remain (non-blocking)", lowRemaining)
		}
	}

	// Final audit pass to determine current state
	refreshDiff(&opts)
	finalPrompt := buildAuditPrompt(opts, knownFindings)
	if err := writePromptFile(promptPath, finalPrompt); err != nil {
		return nil, fmt.Errorf("run audit loop: write final audit prompt: %w", err)
	}

	finalAuditModel := engine.ResolveModel(opts.Epic.AuditModel, opts.Engine.Name(), string(opts.Epic.EffortLevel), engine.SessionAudit)
	frylog.Log(
		"▶ AUDIT  sprint %d/%d \"%s\"  final pass  engine=%s  model=%s",
		opts.Sprint.Number, opts.Epic.TotalSprints, opts.Sprint.Name,
		opts.Engine.Name(), finalAuditModel,
	)

	finalLogPath := filepath.Join(buildLogsDir,
		fmt.Sprintf("sprint%d_audit_final_%s.log", opts.Sprint.Number, time.Now().Format("20060102_150405")),
	)
	if _, err := runAgentWithLog(ctx, opts, config.AuditInvocationPrompt, finalLogPath, finalAuditModel); err != nil {
		return nil, err
	}

	content, err := os.ReadFile(auditFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return &AuditResult{Passed: true, Iterations: lastCycle}, nil
		}
		return nil, fmt.Errorf("run audit loop: read final audit file: %w", err)
	}

	maxSev := parseAuditSeverity(string(content))
	finalCounts := countAuditSeverities(string(content))
	if isAuditPass(maxSev) {
		frylog.Log("  AUDIT: pass after %d cycles (%s)", lastCycle, formatSeverityCounts(finalCounts))
		return &AuditResult{
			Passed: true, Iterations: lastCycle,
			MaxSeverity: maxSev, SeverityCounts: finalCounts,
		}, nil
	}

	return &AuditResult{
		Passed:             false,
		Blocking:           isBlockingSeverity(maxSev),
		Iterations:         lastCycle,
		MaxSeverity:        maxSev,
		SeverityCounts:     finalCounts,
		UnresolvedFindings: parseFindings(string(content)),
	}, nil
}

// --- Prompt builders ---

func buildAuditPrompt(opts AuditOpts, previousFindings []Finding) string {
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

	// Previously identified issues (cycle 2+)
	actionablePrev := filterUnresolved(previousFindings)
	if len(actionablePrev) > 0 {
		b.WriteString("## Previously Identified Issues\n\n")
		b.WriteString("The following issues were found in previous audit cycles. Verify whether\n")
		b.WriteString("each has been resolved. Include your verdict for each in your report.\n\n")
		for i, f := range actionablePrev {
			b.WriteString(fmt.Sprintf("%d. ", i+1))
			if f.Location != "" {
				b.WriteString(fmt.Sprintf("[%s] ", f.Location))
			}
			b.WriteString(fmt.Sprintf("%s (%s)\n", f.Description, f.Severity))
		}
		b.WriteString("\n")
	}

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

	if len(actionablePrev) > 0 {
		b.WriteString("## Verified Previous Issues\n")
		b.WriteString("For each previously identified issue:\n")
		b.WriteString("- **Issue:** <number from list above>\n")
		b.WriteString("- **Status:** RESOLVED | STILL PRESENT\n")
		b.WriteString("- **Notes:** <brief explanation>\n\n")
	}

	b.WriteString("## Findings\n")
	if len(actionablePrev) > 0 {
		b.WriteString("For each issue (include both STILL PRESENT previous issues and any NEW issues):\n")
	} else {
		b.WriteString("For each issue:\n")
	}
	b.WriteString("- **Location:** <file:line>\n")
	b.WriteString("- **Description:** <what is wrong>\n")
	b.WriteString("- **Severity:** CRITICAL | HIGH | MODERATE | LOW\n")
	b.WriteString("- **Recommended Fix:** <how to fix>\n\n")
	b.WriteString("## Verdict\n")
	b.WriteString("PASS (no issues or all LOW) or FAIL (CRITICAL/HIGH/MODERATE found)\n")
	b.WriteString("```\n")

	return b.String()
}

func buildAuditFixPrompt(opts AuditOpts, findings []Finding) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("# AUDIT FIX — Sprint %d: %s\n\n", opts.Sprint.Number, opts.Sprint.Name))
	skipLow := !fixIncludesLow(opts.Epic)
	if opts.Mode == "writing" {
		b.WriteString("The content audit found issues. Fix ONLY the issues listed below.\n")
		if skipLow {
			b.WriteString("Do NOT fix LOW issues. ")
		}
		b.WriteString("Make minimal editorial changes.\n\n")
	} else {
		b.WriteString("The sprint audit found issues. Fix ONLY the issues listed below.\n")
		if skipLow {
			b.WriteString("Do NOT fix LOW issues. ")
		}
		b.WriteString("Make minimal changes.\n\n")
	}

	b.WriteString("**Important:** Focus exclusively on fixing the listed issues. Do not search\n")
	b.WriteString("for new issues. Address the oldest issues first (listed in priority order).\n\n")

	b.WriteString("## Sprint Goals\n")
	b.WriteString(opts.Sprint.Prompt)
	b.WriteString("\n\n")

	b.WriteString("## Issues to Fix\n\n")
	b.WriteString("Issues are listed in priority order (oldest first, highest severity within age group).\n\n")

	// Group by origin cycle for clarity
	groups := groupByCycle(findings)
	for _, group := range groups {
		if len(groups) > 1 {
			b.WriteString(fmt.Sprintf("### From Audit Cycle %d\n\n", group.cycle))
		}
		for _, f := range group.findings {
			if f.Location != "" {
				b.WriteString(fmt.Sprintf("- **Location:** %s\n", f.Location))
			}
			b.WriteString(fmt.Sprintf("- **Description:** %s\n", f.Description))
			b.WriteString(fmt.Sprintf("- **Severity:** %s\n", f.Severity))
			if f.RecommendedFix != "" {
				b.WriteString(fmt.Sprintf("- **Recommended Fix:** %s\n", f.RecommendedFix))
			}
			b.WriteString("\n")
		}
	}

	b.WriteString("## Context\n")
	b.WriteString(fmt.Sprintf("- Read %s for what was built\n", config.SprintProgressFile))
	b.WriteString(fmt.Sprintf("- Read %s for strategic context\n\n", config.PlanFile))

	b.WriteString(fmt.Sprintf("Append a brief note to %s about what you fixed.\n", config.SprintProgressFile))

	return b.String()
}

func buildVerifyPrompt(opts AuditOpts, findings []Finding) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("# VERIFY FIXES — Sprint %d: %s\n\n", opts.Sprint.Number, opts.Sprint.Name))
	b.WriteString("Check whether the following issues have been resolved by recent changes.\n")
	b.WriteString("For each issue, inspect the specified location and verify whether it is fixed.\n\n")
	b.WriteString("Do NOT look for new issues. Only verify the listed issues.\n")
	b.WriteString("Do NOT modify any source code.\n\n")

	b.WriteString("Write your results to .fry/sprint-audit.txt in this format:\n\n")
	b.WriteString("For each issue:\n")
	b.WriteString("- **Issue:** <number>\n")
	b.WriteString("- **Status:** RESOLVED | STILL PRESENT\n\n")

	b.WriteString("## Issues to Verify\n\n")

	for i, f := range findings {
		b.WriteString(fmt.Sprintf("%d. ", i+1))
		if f.Location != "" {
			b.WriteString(fmt.Sprintf("[%s] ", f.Location))
		}
		b.WriteString(fmt.Sprintf("%s (%s)\n", f.Description, f.Severity))
	}

	return b.String()
}

// --- Parsers ---

// severityLabelRe matches lines containing a severity label (e.g., "**Severity:**" or "Severity:").
var severityLabelRe = regexp.MustCompile(`(?i)\bseverity\b`)

// severityWordRe matches whole-word severity keywords to avoid false positives
// from substrings like "HIGHLY", "HIGHLIGHTED", "CRITICALLY", "ALLOW", etc.
var severityWordRe = regexp.MustCompile(`\b(CRITICAL|HIGH|MODERATE|LOW)\b`)

func parseAuditSeverity(content string) string {
	maxSev := ""
	for _, line := range strings.Split(content, "\n") {
		if !severityLabelRe.MatchString(line) {
			continue
		}
		upper := strings.ToUpper(line)
		m := severityWordRe.FindString(upper)
		if m == "" {
			continue
		}
		if severityRank(m) > severityRank(maxSev) {
			maxSev = m
		}
		if maxSev == "CRITICAL" {
			return "CRITICAL"
		}
	}
	return maxSev
}

func countAuditSeverities(content string) map[string]int {
	counts := make(map[string]int)
	for _, line := range strings.Split(content, "\n") {
		if !severityLabelRe.MatchString(line) {
			continue
		}
		upper := strings.ToUpper(line)
		m := severityWordRe.FindString(upper)
		if m == "" {
			continue
		}
		counts[m]++
	}
	return counts
}

// Regexes for structured finding fields.
var (
	locationRe       = regexp.MustCompile(`(?i)\*?\*?Location:\*?\*?\s*(.+)`)
	descriptionRe    = regexp.MustCompile(`(?i)\*?\*?Description:\*?\*?\s*(.+)`)
	recommendedFixRe = regexp.MustCompile(`(?i)\*?\*?Recommended\s*Fix:\*?\*?\s*(.+)`)
)

// parseFindings extracts structured findings from audit output. Each finding
// is delimited by a new Location or Description line. Findings without a
// Description are discarded.
func parseFindings(content string) []Finding {
	var findings []Finding
	var current Finding
	hasCurrent := false

	emit := func() {
		if hasCurrent && strings.TrimSpace(current.Description) != "" {
			findings = append(findings, current)
		}
	}

	for _, line := range strings.Split(content, "\n") {
		// Check for Location (starts a new finding)
		if m := locationRe.FindStringSubmatch(line); len(m) >= 2 {
			emit()
			current = Finding{Location: strings.TrimSpace(m[1])}
			hasCurrent = true
			continue
		}

		// Check for Description (starts a new finding if current already has one)
		if m := descriptionRe.FindStringSubmatch(line); len(m) >= 2 {
			if hasCurrent && strings.TrimSpace(current.Description) != "" {
				emit()
				current = Finding{}
			}
			if !hasCurrent {
				current = Finding{}
				hasCurrent = true
			}
			current.Description = strings.TrimSpace(m[1])
			continue
		}

		// Check for Severity
		if hasCurrent && severityLabelRe.MatchString(line) {
			upper := strings.ToUpper(line)
			if m := severityWordRe.FindString(upper); m != "" {
				current.Severity = m
			}
			continue
		}

		// Check for Recommended Fix
		if hasCurrent {
			if m := recommendedFixRe.FindStringSubmatch(line); len(m) >= 2 {
				current.RecommendedFix = strings.TrimSpace(m[1])
				continue
			}
		}
	}

	emit()
	return findings
}

// Regexes for verification status parsing.
var (
	issueNumberRe = regexp.MustCompile(`(?i)\*?\*?Issue:\*?\*?\s*(\d+)`)
	statusRe      = regexp.MustCompile(`(?i)\*?\*?Status:\*?\*?\s*(RESOLVED|STILL\s*PRESENT)`)
)

// parseVerificationStatuses parses the verification agent's output to determine
// which findings are resolved. Returns a boolean slice aligned with the findings slice.
func parseVerificationStatuses(content string, findings []Finding) []bool {
	resolved := make([]bool, len(findings))

	currentIssue := -1
	for _, line := range strings.Split(content, "\n") {
		// Check for issue number
		if m := issueNumberRe.FindStringSubmatch(line); len(m) >= 2 {
			num, err := strconv.Atoi(strings.TrimSpace(m[1]))
			if err == nil && num >= 1 && num <= len(findings) {
				currentIssue = num - 1
			}
		}

		// Check for status (may be on same line or next line)
		if m := statusRe.FindStringSubmatch(line); len(m) >= 2 && currentIssue >= 0 {
			status := strings.ToUpper(strings.TrimSpace(m[1]))
			if strings.HasPrefix(status, "RESOLVED") {
				resolved[currentIssue] = true
			}
			currentIssue = -1
		}
	}

	return resolved
}

// --- Classification and comparison ---

// classifyFindings compares a set of known findings against newly parsed findings.
// Returns findings that were resolved (no longer present), findings that persist
// (still present from a previous cycle), and genuinely new findings.
func classifyFindings(known, current []Finding) (resolved, persisting, newFindings []Finding) {
	// Build a set of current finding keys for quick lookup.
	currentKeys := make(map[string]struct{})
	for _, f := range current {
		currentKeys[f.key()] = struct{}{}
	}

	// Classify known findings as resolved or persisting.
	knownKeys := make(map[string]struct{})
	for _, kf := range known {
		k := kf.key()
		knownKeys[k] = struct{}{}
		if _, exists := currentKeys[k]; exists {
			// Issue persists — keep origin cycle from known.
			// Find the matching current finding for updated fields.
			for _, cf := range current {
				if cf.key() == k {
					cf.OriginCycle = kf.OriginCycle
					persisting = append(persisting, cf)
					break
				}
			}
		} else {
			resolved = append(resolved, kf)
		}
	}

	// Collect genuinely new findings (not in known set). Use a seen set
	// to avoid emitting duplicates from the current list.
	seen := make(map[string]struct{})
	for _, cf := range current {
		k := cf.key()
		if _, isKnown := knownKeys[k]; isKnown {
			continue
		}
		if _, alreadySeen := seen[k]; alreadySeen {
			continue
		}
		seen[k] = struct{}{}
		newFindings = append(newFindings, cf)
	}

	return
}

// hasProgress returns true if the current finding set represents progress
// compared to the previous set. Progress means: fewer findings, or different
// findings (not all previous issues still present).
func hasProgress(previous, current map[string]struct{}) bool {
	if len(current) == 0 {
		return true
	}
	if len(previous) == 0 {
		return true
	}
	if len(current) < len(previous) {
		return true
	}
	matchCount := 0
	for key := range previous {
		if _, ok := current[key]; ok {
			matchCount++
		}
	}
	return matchCount < len(previous)
}

// findingKeySet extracts normalized description keys from actionable findings.
func findingKeySet(findings []Finding) map[string]struct{} {
	keys := make(map[string]struct{})
	for _, f := range findings {
		if f.isActionable() {
			keys[f.key()] = struct{}{}
		}
	}
	return keys
}

// --- Sorting and grouping ---

// sortFindingsFIFO sorts findings by OriginCycle ascending (oldest first),
// then by severity descending within the same cycle.
func sortFindingsFIFO(findings []Finding) {
	sort.SliceStable(findings, func(i, j int) bool {
		if findings[i].OriginCycle != findings[j].OriginCycle {
			return findings[i].OriginCycle < findings[j].OriginCycle
		}
		return severityRank(findings[i].Severity) > severityRank(findings[j].Severity)
	})
}

// mergeFindings combines persisting and new findings into a single FIFO-ordered list.
func mergeFindings(persisting, newFindings []Finding) []Finding {
	merged := make([]Finding, 0, len(persisting)+len(newFindings))
	merged = append(merged, persisting...)
	merged = append(merged, newFindings...)
	sortFindingsFIFO(merged)
	return merged
}

type findingGroup struct {
	cycle    int
	findings []Finding
}

// groupByCycle groups findings by their OriginCycle, returning groups in ascending cycle order.
func groupByCycle(findings []Finding) []findingGroup {
	cycleMap := make(map[int][]Finding)
	var cycles []int
	for _, f := range findings {
		if _, seen := cycleMap[f.OriginCycle]; !seen {
			cycles = append(cycles, f.OriginCycle)
		}
		cycleMap[f.OriginCycle] = append(cycleMap[f.OriginCycle], f)
	}
	sort.Ints(cycles)
	groups := make([]findingGroup, len(cycles))
	for i, c := range cycles {
		groups[i] = findingGroup{cycle: c, findings: cycleMap[c]}
	}
	return groups
}

// --- Filtering and counting helpers ---

func filterUnresolved(findings []Finding) []Finding {
	var result []Finding
	for _, f := range findings {
		if f.Severity != "" && f.Severity != "LOW" && !f.Resolved {
			result = append(result, f)
		}
	}
	return result
}

// filterFixable returns unresolved findings eligible for fix at the given effort level.
// At high/max effort (includeLow=true), LOW findings are included alongside higher-severity items.
// At other levels, only findings above LOW are returned (same as filterUnresolved).
func filterFixable(findings []Finding, includeLow bool) []Finding {
	var result []Finding
	for _, f := range findings {
		if f.Severity == "" || f.Resolved {
			continue
		}
		if !includeLow && f.Severity == "LOW" {
			continue
		}
		result = append(result, f)
	}
	return result
}

// countFixable counts findings in scope for the fix agent at the given effort level,
// regardless of resolution status. Used as the total denominator in progress logs.
func countFixable(findings []Finding, includeLow bool) int {
	n := 0
	for _, f := range findings {
		if f.Severity == "" {
			continue
		}
		if !includeLow && f.Severity == "LOW" {
			continue
		}
		n++
	}
	return n
}

// countUnresolvedLow counts unresolved LOW-severity findings.
func countUnresolvedLow(findings []Finding) int {
	n := 0
	for _, f := range findings {
		if f.Severity == "LOW" && !f.Resolved {
			n++
		}
	}
	return n
}

func countActionableFindings(findings []Finding) int {
	n := 0
	for _, f := range findings {
		if f.isActionable() {
			n++
		}
	}
	return n
}

func collectUnresolved(findings []Finding) []Finding {
	var result []Finding
	for _, f := range findings {
		if !f.Resolved {
			result = append(result, f)
		}
	}
	return result
}

// countAboveLow counts findings with severity above LOW, regardless of resolution status.
func countAboveLow(findings []Finding) int {
	n := 0
	for _, f := range findings {
		if f.Severity != "" && f.Severity != "LOW" {
			n++
		}
	}
	return n
}

func countResolved(findings []Finding) int {
	n := 0
	for _, f := range findings {
		if f.Resolved {
			n++
		}
	}
	return n
}

func markAllResolved(findings []Finding) {
	for i := range findings {
		findings[i].Resolved = true
	}
}

// applyResolutionsByKey marks findings as resolved based on the verification results.
// The resolved slice is aligned with the checked slice (a subset of all findings).
func applyResolutionsByKey(all []Finding, checked []Finding, resolved []bool) {
	for i, flag := range resolved {
		if !flag || i >= len(checked) {
			continue
		}
		key := checked[i].key()
		for j := range all {
			if all[j].key() == key && !all[j].Resolved {
				all[j].Resolved = true
				break
			}
		}
	}
}

// --- Severity helpers ---

var severityOrder = []string{"CRITICAL", "HIGH", "MODERATE", "LOW"}

func formatSeverityCounts(counts map[string]int) string {
	var parts []string
	for _, sev := range severityOrder {
		if n, ok := counts[sev]; ok && n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", n, sev))
		}
	}
	if len(parts) == 0 {
		return "none"
	}
	return strings.Join(parts, ", ")
}

// FormatCounts formats a severity count map for external callers.
func FormatCounts(counts map[string]int) string {
	return formatSeverityCounts(counts)
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

// --- Iteration limit helpers ---

// effectiveOuterCycles determines the maximum outer audit cycles and whether
// progress-based detection should be used.
func effectiveOuterCycles(ep *epic.Epic) (maxCycles int, progressBased bool) {
	if ep.MaxAuditIterationsSet {
		return ep.MaxAuditIterations, false
	}
	switch ep.EffortLevel {
	case epic.EffortMax:
		return config.MaxOuterCyclesMaxCap, true
	case epic.EffortHigh:
		return config.MaxOuterCyclesHighCap, true
	default:
		maxCycles = ep.MaxAuditIterations
		if maxCycles <= 0 {
			maxCycles = config.DefaultMaxOuterAuditCycles
		}
		return maxCycles, false
	}
}

// effectiveInnerIter determines the maximum inner fix iterations per audit cycle.
func effectiveInnerIter(ep *epic.Epic) int {
	switch ep.EffortLevel {
	case epic.EffortMax:
		return config.MaxInnerFixIterMax
	case epic.EffortHigh:
		return config.MaxInnerFixIterHigh
	default:
		return config.DefaultMaxInnerFixIter
	}
}

// --- File and agent helpers ---

func Cleanup(projectDir string) error {
	for _, rel := range []string{config.SprintAuditFile, config.AuditPromptFile} {
		path := filepath.Join(projectDir, rel)
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("audit cleanup: %w", err)
		}
	}
	return nil
}

func writePromptFile(path string, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

func refreshDiff(opts *AuditOpts) {
	if opts.DiffFn != nil {
		if freshDiff, diffErr := opts.DiffFn(); diffErr == nil {
			opts.GitDiff = freshDiff
		}
	}
}

func runAgentWithLog(ctx context.Context, opts AuditOpts, prompt, logPath, model string) (string, error) {
	logFile, err := os.Create(logPath)
	if err != nil {
		return "", fmt.Errorf("run audit loop: create log: %w", err)
	}
	defer logFile.Close()

	runOpts := engine.RunOpts{
		Model:   model,
		WorkDir: opts.ProjectDir,
	}

	if opts.Verbose {
		stdout := opts.Stdout
		if stdout == nil {
			stdout = os.Stdout
		}
		writer := io.MultiWriter(stdout, logFile)
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
