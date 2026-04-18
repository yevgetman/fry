package wake

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yevgetman/fry/internal/notes"
	"github.com/yevgetman/fry/internal/state"
	"github.com/yevgetman/fry/internal/wakelog"
)

// Assemble builds the layered wake prompt:
//
//	L0: static wake-contract preamble (stable — cache-friendly)
//	L1: mission overview (prompt.md)
//	L2: plan body (plan.md, if present)
//	L3: notes.md — current focus, next-wake handoff, supervisor injections, decisions
//	L4: last N wake_log entries
//	L5: current-wake directive (changes every wake)
func Assemble(m *state.Mission, missionDir string, lastN int, now time.Time) (string, error) {
	var sb strings.Builder
	wakeNum := m.CurrentWake + 1

	// L0 — static preamble
	sb.WriteString("# Wake Contract\n\n")
	sb.WriteString("You are a wake agent in an autonomous builder mission managed by fry.\n")
	sb.WriteString("You operate in a single discrete chunk and exit at end of this wake.\n")
	sb.WriteString("Another agent instance wakes up ~interval minutes after you exit.\n\n")
	sb.WriteString("Rules:\n")
	sb.WriteString("- Do exactly one bounded unit of work described in the directive below.\n")
	sb.WriteString("- Your final stdout line MUST be ===WAKE_DONE=== (the promise token).\n")
	sb.WriteString("- Do NOT run launchctl, systemctl, or any scheduler command directly.\n")
	sb.WriteString("- Signal mission complete via FRY_STATUS_TRANSITION=complete on stdout.\n\n")

	// L1 — mission overview from prompt.md
	promptData, err := os.ReadFile(filepath.Join(missionDir, "prompt.md"))
	if err != nil {
		return "", fmt.Errorf("prompt.Assemble: read prompt.md: %w", err)
	}
	sb.WriteString("# Mission Overview\n\n")
	sb.Write(promptData)
	sb.WriteString("\n\n")

	// L2 — plan.md if present (optional input mode)
	if planData, err := os.ReadFile(filepath.Join(missionDir, "plan.md")); err == nil {
		sb.WriteString("# Plan\n\n")
		sb.Write(planData)
		sb.WriteString("\n\n")
	}

	// L3 — notes.md sections (changes each wake)
	if n, err := notes.Load(missionDir); err == nil {
		if n.CurrentFocus != "" {
			sb.WriteString("# Current Focus\n\n")
			sb.WriteString(n.CurrentFocus)
			sb.WriteString("\n\n")
		}
		if n.NextWakeShould != "" {
			sb.WriteString("# Prior Wake Handoff\n\n")
			sb.WriteString(n.NextWakeShould)
			sb.WriteString("\n\n")
		}
		if len(n.SupervisorInjects) > 0 {
			sb.WriteString("# Supervisor Injections\n\n")
			for _, inj := range n.SupervisorInjects {
				sb.WriteString("- ")
				sb.WriteString(inj)
				sb.WriteString("\n")
			}
			sb.WriteString("\n")
		}
		if len(n.Decisions) > 0 {
			sb.WriteString("# Prior Decisions\n\n")
			for _, d := range n.Decisions {
				sb.WriteString("- ")
				sb.WriteString(d)
				sb.WriteString("\n")
			}
			sb.WriteString("\n")
		}
		if len(n.OpenQuestions) > 0 {
			sb.WriteString("# Open Questions\n\n")
			for _, q := range n.OpenQuestions {
				sb.WriteString("- ")
				sb.WriteString(q)
				sb.WriteString("\n")
			}
			sb.WriteString("\n")
		}
	}

	// L4 — last N wake_log entries (changing)
	if entries, err := wakelog.TailN(missionDir, lastN); err == nil && len(entries) > 0 {
		sb.WriteString("# Recent Wake Log\n\n")
		for _, e := range entries {
			data, _ := json.Marshal(e)
			sb.Write(data)
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	// L5 — current-wake directive (changes every wake)
	elapsed := m.ElapsedHours(now)
	sb.WriteString("# Current Wake Directive\n\n")
	sb.WriteString(fmt.Sprintf(
		"This is wake %d. Elapsed: %.2fh. Status: %s. Mission directory: %s\n\n"+
			"Do one unit of work as described above.\n"+
			"When done, output %s as the final line of stdout.\n",
		wakeNum, elapsed, m.Status, missionDir, PromiseToken,
	))

	return sb.String(), nil
}
