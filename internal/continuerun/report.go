package continuerun

import (
	"fmt"
	"strings"
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

	// Active/partial sprint
	if state.ActiveSprint != nil {
		a := state.ActiveSprint
		b.WriteString(fmt.Sprintf("## Partial Work Detected for Sprint %d\n", a.Number))
		b.WriteString(fmt.Sprintf("- %d iterations completed, %d audit passes, %d heal attempts\n",
			a.IterationCount, a.AuditCount, a.HealCount))
		if a.HasRetryLog {
			b.WriteString("- Has prior retry attempt\n")
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
			// Indent all lines with >
			lines := strings.Split(a.ProgressExcerpt, "\n")
			b.WriteString(strings.Join(lines, "\n> "))
			b.WriteByte('\n')
		}
		b.WriteByte('\n')
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
