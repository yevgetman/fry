package monitor

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/yevgetman/fry/internal/color"
)

// RenderEvent writes a single enriched event line to w.
func RenderEvent(w io.Writer, evt EnrichedEvent, useColor bool) {
	ts := evt.Timestamp.Format("15:04:05")
	elapsed := formatDuration(evt.ElapsedBuild)

	var parts []string
	parts = append(parts, fmt.Sprintf("[%s]", ts))
	parts = append(parts, fmt.Sprintf("+%-7s", elapsed))

	evtType := evt.Type
	if evt.Synthetic {
		evtType = "*" + evtType
	}
	if useColor {
		evtType = colorizeEventType(evtType)
	}
	parts = append(parts, fmt.Sprintf("%-20s", evtType))

	if evt.SprintOf != "" {
		parts = append(parts, evt.SprintOf)
	}

	for _, k := range sortedKeys(evt.Data) {
		parts = append(parts, fmt.Sprintf("%s=%s", k, evt.Data[k]))
	}

	if evt.PhaseChange != "" {
		marker := "[" + evt.PhaseChange + "]"
		if useColor {
			marker = color.CyanText(marker)
		}
		parts = append(parts, marker)
	}

	fmt.Fprintln(w, strings.Join(parts, "  "))
}

// RenderDashboard writes a dashboard view of the build state.
// If clearScreen is true and output is a TTY, ANSI escape codes are used
// to overwrite the previous output.
func RenderDashboard(w io.Writer, snap Snapshot, useColor bool, clearScreen bool) {
	if clearScreen {
		fmt.Fprint(w, "\033[H\033[2J")
	}

	ts := snap.Timestamp.Format("15:04:05")
	header := fmt.Sprintf("Fry Monitor%s%s", strings.Repeat(" ", 30), ts)
	if snap.PID > 0 {
		header = fmt.Sprintf("Fry Monitor%sPID %d  %s", strings.Repeat(" ", 25), snap.PID, ts)
	}
	if useColor {
		header = color.Colorize(header, color.Bold)
	}
	fmt.Fprintln(w, header)

	sep := strings.Repeat("\u2500", 64)
	fmt.Fprintln(w, sep)

	// Build info from BuildStatus.
	if st := snap.BuildStatus; st != nil {
		fmt.Fprintf(w, "Epic: %-28s Engine: %s\n", truncate(st.Build.Epic, 28), st.Build.Engine)
		fmt.Fprintf(w, "Mode: %-28s Effort: %s\n", st.Build.Mode, st.Build.Effort)
		phase := snap.Phase
		if phase == "" {
			phase = st.Build.Phase
		}
		branchStr := ""
		if st.Build.GitBranch != "" {
			branchStr = "Branch: " + st.Build.GitBranch
		}
		fmt.Fprintf(w, "Phase: %-28s %s\n", phase, branchStr)
		fmt.Fprintln(w, sep)

		// Sprint table.
		for _, sp := range st.Sprints {
			statusStr := sp.Status
			if useColor {
				statusStr = colorizeStatus(sp.Status)
			}
			durStr := ""
			if sp.DurationSec > 0 {
				durStr = formatDuration(time.Duration(sp.DurationSec * float64(time.Second)))
			} else if sp.Status == "running" && sp.StartedAt != nil {
				durStr = "+" + formatDuration(time.Since(*sp.StartedAt))
			}

			label := fmt.Sprintf("Sprint %d/%d: %s", sp.Number, st.Build.TotalSprints, sp.Name)
			dots := 50 - len(label)
			if dots < 2 {
				dots = 2
			}
			fmt.Fprintf(w, "%s %s %s  %s\n", label, strings.Repeat(".", dots), statusStr, durStr)
		}

		// Show pending sprints.
		for i := len(st.Sprints) + 1; i <= st.Build.TotalSprints; i++ {
			label := fmt.Sprintf("Sprint %d/%d", i, st.Build.TotalSprints)
			dots := 50 - len(label)
			if dots < 2 {
				dots = 2
			}
			pendingStr := "pending"
			if useColor {
				pendingStr = color.Colorize(pendingStr, color.Dim)
			}
			fmt.Fprintf(w, "%s %s %s\n", label, strings.Repeat(".", dots), pendingStr)
		}

		fmt.Fprintln(w, sep)
	} else {
		// No build status yet.
		phase := snap.Phase
		if phase == "" {
			phase = "unknown"
		}
		fmt.Fprintf(w, "Phase: %s\n", phase)
		if snap.BuildActive {
			fmt.Fprintf(w, "Build active (PID %d)\n", snap.PID)
		}
		fmt.Fprintln(w, sep)
	}

	// Latest event.
	if len(snap.Events) > 0 {
		last := snap.Events[len(snap.Events)-1]
		evtDesc := fmt.Sprintf("[%s] %s", last.Timestamp.Format("15:04:05"), last.Type)
		if last.SprintOf != "" {
			evtDesc += " (" + last.SprintOf + ")"
		}
		for _, k := range sortedKeys(last.Data) {
			evtDesc += fmt.Sprintf(" %s=%s", k, last.Data[k])
		}
		fmt.Fprintf(w, "Latest: %s\n", evtDesc)
	}

	// Build ended banner.
	if snap.BuildEnded {
		fmt.Fprintln(w, sep)
		msg := "Build complete"
		if snap.ExitReason != "" {
			msg = "Build ended: " + snap.ExitReason
		}
		if useColor {
			switch {
			case snap.ExitReason == "":
				msg = color.GreenText(msg)
			case snap.ExitReason == "process exited unexpectedly":
				msg = color.YellowText(msg)
			default:
				msg = color.RedText(msg)
			}
		}
		fmt.Fprintln(w, msg)
	}
}

// RenderLogTail writes the active log tail.
func RenderLogTail(w io.Writer, snap Snapshot) {
	if snap.ActiveLogPath == "" {
		fmt.Fprintln(w, "No active build log.")
		return
	}
	fmt.Fprintf(w, "--- %s ---\n", snap.ActiveLogPath)
	if snap.ActiveLogTail != "" {
		fmt.Fprintln(w, snap.ActiveLogTail)
	}
}

// RenderWaiting writes a waiting message.
func RenderWaiting(w io.Writer, projectDir string) {
	fmt.Fprintf(w, "Waiting for build to start in %s ...\n", projectDir)
}

// RenderBuildEnded writes a build-ended summary line.
func RenderBuildEnded(w io.Writer, snap Snapshot, useColor bool) {
	var msg string
	if snap.ExitReason != "" {
		msg = fmt.Sprintf("Build ended: %s", snap.ExitReason)
	} else {
		msg = "Build complete."
	}
	if useColor {
		switch {
		case snap.ExitReason == "":
			msg = color.GreenText(msg)
		case snap.ExitReason == "process exited unexpectedly":
			msg = color.YellowText(msg)
		default:
			msg = color.RedText(msg)
		}
	}
	fmt.Fprintln(w, msg)
}

// colorizeEventType applies color based on event type.
func colorizeEventType(evtType string) string {
	switch {
	case strings.HasSuffix(evtType, "_complete") || strings.HasSuffix(evtType, "_done") || evtType == "build_end":
		return color.GreenText(evtType)
	case strings.HasSuffix(evtType, "_start"):
		return color.CyanText(evtType)
	case strings.Contains(evtType, "decision") || strings.Contains(evtType, "pause") || strings.Contains(evtType, "directive"):
		return color.YellowText(evtType)
	default:
		return evtType
	}
}

// colorizeStatus applies color based on sprint status.
func colorizeStatus(status string) string {
	switch {
	case strings.HasPrefix(status, "PASS"):
		return color.GreenText(status)
	case strings.HasPrefix(status, "FAIL"):
		return color.RedText(status)
	case status == "running":
		return color.CyanText(status)
	case status == "SKIPPED":
		return color.YellowText(status)
	default:
		return status
	}
}

// formatDuration formats a duration compactly (e.g., "5m10s", "2h3m", "45s").
func formatDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	d = d.Round(time.Second)

	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60

	switch {
	case h > 0:
		return fmt.Sprintf("%dh%dm", h, m)
	case m > 0:
		return fmt.Sprintf("%dm%ds", m, s)
	default:
		return fmt.Sprintf("%ds", s)
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}
