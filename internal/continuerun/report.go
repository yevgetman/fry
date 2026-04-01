package continuerun

import (
	"fmt"
	"strings"

	"github.com/yevgetman/fry/internal/archive"
)

// FormatReport renders a BuildState into a human-readable markdown report
// suitable for both terminal display and LLM prompt inclusion.
func FormatReport(state *BuildState) string {
	var b strings.Builder

	b.WriteString("# Build State Report\n\n")
	modeDisplay := state.Mode
	if modeDisplay == "" {
		modeDisplay = "software"
	}
	b.WriteString(fmt.Sprintf("## Epic: %s (%d sprints, engine: %s, effort: %s, mode: %s)\n\n",
		state.EpicName, state.TotalSprints, state.Engine, state.EffortLevel, modeDisplay))

	// Build exit reason (why the last run stopped)
	if state.ExitReason != "" {
		b.WriteString("## Last Run Stopped\n")
		b.WriteString(state.ExitReason)
		b.WriteString("\n\n")
	}

	if state.ResumePoint != nil {
		b.WriteString("## Resume Point\n")
		b.WriteString(fmt.Sprintf("- Verdict: %s\n", state.ResumePoint.Verdict))
		if state.ResumePoint.Sprint > 0 {
			b.WriteString(fmt.Sprintf("- Sprint: %d (%s)\n", state.ResumePoint.Sprint, state.ResumePoint.SprintName))
		}
		if state.ResumePoint.Phase != "" {
			b.WriteString(fmt.Sprintf("- Phase: %s\n", state.ResumePoint.Phase))
		}
		if state.ResumePoint.Reason != "" {
			b.WriteString(fmt.Sprintf("- Reason: %s\n", state.ResumePoint.Reason))
		}
		if state.ResumePoint.RecommendedCommand != "" {
			b.WriteString(fmt.Sprintf("- Recommended command: `%s`\n", state.ResumePoint.RecommendedCommand))
		}
		b.WriteByte('\n')
	}

	// Completed sprints
	b.WriteString("## Completed Sprints\n")
	if len(state.CompletedSprints) == 0 {
		b.WriteString("None\n")
	} else {
		for _, cs := range state.CompletedSprints {
			b.WriteString(fmt.Sprintf("- Sprint %d: %s \u2014 %s\n", cs.Number, cs.Name, cs.Status))
		}
	}
	b.WriteByte('\n')

	// Failed sprints
	if len(state.FailedSprints) > 0 {
		b.WriteString("## Failed Sprints\n")
		for _, fs := range state.FailedSprints {
			b.WriteString(fmt.Sprintf("- Sprint %d: %s \u2014 %s\n", fs.Number, fs.Name, fs.Status))
		}
		b.WriteByte('\n')
	}

	// Build audit status
	b.WriteString("## Build Audit\n")
	if !state.AuditConfigured {
		b.WriteString("- Sentinel: N/A (build audit not configured for this build)\n")
	} else if state.BuildAuditComplete {
		b.WriteString("- Sentinel: PRESENT (build audit completed successfully)\n")
	} else {
		b.WriteString("- Sentinel: ABSENT (build audit has not completed)\n")
	}
	b.WriteByte('\n')

	// Next sprint
	next := findNextSprint(state.CompletedSprints, state.TotalSprints)
	if next == 0 {
		b.WriteString("## All sprints complete\n\n")
	} else {
		name := ""
		if next >= 1 && next <= len(state.SprintNames) {
			name = state.SprintNames[next-1]
		}
		b.WriteString(fmt.Sprintf("## Next Sprint: %d (%s)\n\n", next, name))
	}

	// Active/partial sprints
	if len(state.ActiveSprints) > 0 {
		b.WriteString(fmt.Sprintf("## Partial Work Detected (%d incomplete sprint(s))\n\n", len(state.ActiveSprints)))
		for _, a := range state.ActiveSprints {
			b.WriteString(fmt.Sprintf("### Sprint %d: %s\n", a.Number, a.Name))
			b.WriteString(fmt.Sprintf("- %d iterations completed, %d audit passes, %d alignment attempts\n",
				a.IterationCount, a.AuditCount, a.HealCount))
			if a.HasResumeLog {
				b.WriteString("- Has prior resume attempt\n")
			}
			if a.AuditSeverity != "" {
				b.WriteString(fmt.Sprintf("- Last audit severity: %s\n", a.AuditSeverity))
			}
			if a.LastLogTail != "" {
				b.WriteString("- Last log tail:\n```\n")
				b.WriteString(a.LastLogTail)
				b.WriteString("\n```\n")
			}
			if a.ProgressExcerpt != "" {
				b.WriteString("- Sprint progress excerpt:\n> ")
				lines := strings.Split(a.ProgressExcerpt, "\n")
				b.WriteString(strings.Join(lines, "\n> "))
				b.WriteByte('\n')
			}
			b.WriteByte('\n')
		}
	}

	// Environment
	b.WriteString("## Environment\n")
	if state.DockerRequired {
		if state.DockerAvailable {
			b.WriteString("- Docker: RUNNING (required)\n")
		} else {
			b.WriteString("- Docker: NOT RUNNING (required)\n")
		}
	} else {
		b.WriteString("- Docker: not required for next sprint\n")
	}
	if len(state.RequiredTools) > 0 {
		b.WriteString("- Required tools: ")
		parts := make([]string, len(state.RequiredTools))
		for i, t := range state.RequiredTools {
			status := "ok"
			if !t.Available {
				status = "MISSING"
			}
			parts[i] = fmt.Sprintf("%s (%s)", t.Name, status)
		}
		b.WriteString(strings.Join(parts, ", "))
		b.WriteByte('\n')
	}
	if state.GitBranch != "" {
		cleanStr := "clean"
		if !state.GitClean {
			cleanStr = "uncommitted changes"
		}
		b.WriteString(fmt.Sprintf("- Git: %s, branch %s", cleanStr, state.GitBranch))
		if state.LastAutoCommit != "" {
			b.WriteString(fmt.Sprintf(", last commit \"%s\"", state.LastAutoCommit))
		}
		b.WriteByte('\n')
	}
	b.WriteByte('\n')

	// Deferred failures
	b.WriteString("## Deferred Failures\n")
	if len(state.DeferredFailures) == 0 {
		b.WriteString("None\n")
	} else {
		for _, line := range state.DeferredFailures {
			b.WriteString(line)
			b.WriteByte('\n')
		}
	}
	b.WriteByte('\n')

	// Deviations
	b.WriteString(fmt.Sprintf("## Deviations: %d\n", state.DeviationCount))

	// Sprint list for reference
	b.WriteString("\n## All Sprints\n")
	for i, name := range state.SprintNames {
		b.WriteString(fmt.Sprintf("- Sprint %d: %s\n", i+1, name))
	}

	return b.String()
}

const maxDisplayedArchives = 10

// FormatInactiveSummary renders a report for when no active build exists,
// showing archived and worktree builds if any are found.
func FormatInactiveSummary(projectDir string, archives, worktrees []archive.BuildSummary) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("No active build found in %s\n", projectDir))

	if len(archives) == 0 && len(worktrees) == 0 {
		b.WriteString("Run 'fry run' to start a build.\n")
		return b.String()
	}

	if len(archives) > 0 {
		b.WriteString(fmt.Sprintf("\nArchived Builds (%d)\n", len(archives)))
		limit := len(archives)
		if limit > maxDisplayedArchives {
			limit = maxDisplayedArchives
		}
		for _, a := range archives[:limit] {
			b.WriteString(formatBuildLine(a))
			if a.ExitReason != "" {
				b.WriteString(fmt.Sprintf("    Exit: %s\n", a.ExitReason))
			}
		}
		if len(archives) > maxDisplayedArchives {
			b.WriteString(fmt.Sprintf("  ... and %d more archived builds\n", len(archives)-maxDisplayedArchives))
		}
	}

	if len(worktrees) > 0 {
		b.WriteString(fmt.Sprintf("\nWorktree Builds (%d)\n", len(worktrees)))
		for _, w := range worktrees {
			b.WriteString(fmt.Sprintf("  %s/  %s  %s", w.Dir, w.EpicName, formatBuildStatus(w)))
			if w.ExitReason != "" {
				b.WriteString(fmt.Sprintf("    Exit: %s\n", w.ExitReason))
			}
		}
	}

	b.WriteString("\nRun 'fry run' to start a new build.\n")
	return b.String()
}

func formatBuildLine(s archive.BuildSummary) string {
	ts := "(unknown date)  "
	if !s.Timestamp.IsZero() {
		ts = s.Timestamp.Format("2006-01-02 15:04")
	}
	return fmt.Sprintf("  %s  %s  %s", ts, s.EpicName, formatBuildStatus(s))
}

func formatBuildStatus(s archive.BuildSummary) string {
	mode := s.Mode
	if mode == "" {
		mode = "software"
	}

	var parts []string
	parts = append(parts, fmt.Sprintf("%d/%d sprints passed", s.CompletedCount, s.TotalSprints))
	if s.FailedCount > 0 {
		parts = append(parts, fmt.Sprintf("%d failed", s.FailedCount))
	}
	parts = append(parts, mode)

	return fmt.Sprintf("(%s)\n", strings.Join(parts, ", "))
}
