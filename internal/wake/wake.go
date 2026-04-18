package wake

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/yevgetman/fry/internal/scheduler"
	"github.com/yevgetman/fry/internal/state"
	"github.com/yevgetman/fry/internal/wakelog"
)

// Execute runs a single wake for the mission at missionDir.
// It acquires the overlap lock, invokes claude, writes the log entry,
// updates state, and handles any status transitions.
func Execute(ctx context.Context, missionDir string, m *state.Mission) (*wakelog.Entry, error) {
	now := time.Now().UTC()

	// Hard deadline: fire but don't call claude; just mark complete.
	if !m.HardDeadlineUTC.IsZero() && now.After(m.HardDeadlineUTC) {
		return hardStop(missionDir, m, now)
	}

	lk, err := Acquire(missionDir)
	if err != nil {
		return nil, err
	}
	defer lk.Release()

	prompt, err := buildPrompt(missionDir, m, now)
	if err != nil {
		return nil, fmt.Errorf("wake: build prompt: %w", err)
	}

	cap := time.Duration(m.IntervalSeconds)*time.Second - 30*time.Second
	if cap < 60*time.Second || cap > 540*time.Second {
		cap = 540 * time.Second
	}

	res, claudeErr := RunClaude(ctx, ClaudeRequest{
		MissionDir:   missionDir,
		Effort:       m.Effort,
		Prompt:       prompt,
		WallClockCap: cap,
	})

	wakeNum := m.CurrentWake + 1
	entry := wakelog.Entry{
		WakeNumber:       wakeNum,
		TimestampUTC:     now.Format(time.RFC3339),
		ElapsedHours:     m.ElapsedHours(now),
		Phase:            currentPhase(m, now),
		CurrentMilestone: "M3",
		WakeGoal:         "Execute claude invocation + check promise token",
		Blockers:         []string{},
	}

	if claudeErr != nil {
		entry.Blockers = append(entry.Blockers, claudeErr.Error())
		entry.ExitCode = -1
	} else {
		entry.ExitCode = res.ExitCode
		entry.WallClockSeconds = res.WallClockSeconds
		entry.PromiseTokenFound = res.PromiseFound
		entry.CostUSD = res.CostUSD
	}

	// Check for status transition signal in output.
	if res != nil && len(res.Stdout) > 0 {
		if newStatus, ok := ExtractStatusTransition(res.Stdout); ok {
			applyTransition(missionDir, m, state.Status(newStatus))
		}
	}

	// Advance state.
	m.CurrentWake = wakeNum
	m.LastWakeAt = now

	// Compute deadline status if not already complete/stopped.
	if m.Status == state.StatusActive || m.Status == state.StatusOvertime {
		m.Status = deadlineStatus(m, now)
	}

	if err := m.Save(missionDir); err != nil {
		return nil, fmt.Errorf("wake: save state: %w", err)
	}

	// If mission is complete or stopped, uninstall scheduler.
	if m.Status == state.StatusComplete || m.Status == state.StatusStopped {
		sched := scheduler.New()
		_ = sched.Uninstall(m) // best-effort; log if fails
	}

	if err := wakelog.Append(missionDir, entry); err != nil {
		return nil, fmt.Errorf("wake: append log: %w", err)
	}

	return &entry, nil
}

// buildPrompt assembles the wake prompt (M3 stub: reads prompt.md + appends directive).
func buildPrompt(missionDir string, m *state.Mission, now time.Time) (string, error) {
	promptPath := filepath.Join(missionDir, "prompt.md")
	data, err := os.ReadFile(promptPath)
	if err != nil {
		return "", fmt.Errorf("read prompt.md: %w", err)
	}
	elapsed := m.ElapsedHours(now)
	directive := fmt.Sprintf(
		"\n\n---\nThis is wake %d. Elapsed: %.2fh. Mission directory: %s\n"+
			"Do one unit of work described above. When done, output %s as the final line.\n",
		m.CurrentWake+1, elapsed, missionDir, PromiseToken,
	)
	return string(data) + directive, nil
}

// hardStop writes a terminal log entry and marks the mission complete without calling claude.
func hardStop(missionDir string, m *state.Mission, now time.Time) (*wakelog.Entry, error) {
	m.Status = state.StatusComplete
	m.CurrentWake++
	m.LastWakeAt = now
	if err := m.Save(missionDir); err != nil {
		return nil, fmt.Errorf("hardStop: save: %w", err)
	}
	sched := scheduler.New()
	_ = sched.Uninstall(m)

	entry := wakelog.Entry{
		WakeNumber:       m.CurrentWake,
		TimestampUTC:     now.Format(time.RFC3339),
		ElapsedHours:     m.ElapsedHours(now),
		Phase:            "complete",
		CurrentMilestone: "done",
		WakeGoal:         "hard deadline reached — shutting down without claude call",
		Blockers:         []string{"hard_deadline"},
	}
	if err := wakelog.Append(missionDir, entry); err != nil {
		return nil, fmt.Errorf("hardStop: append log: %w", err)
	}
	return &entry, nil
}

// applyTransition validates and applies a status transition from the agent.
func applyTransition(missionDir string, m *state.Mission, newStatus state.Status) {
	if state.CanTransition(m.Status, newStatus) {
		m.Status = newStatus
	}
}

// currentPhase returns the current phase string for the log entry.
func currentPhase(m *state.Mission, now time.Time) string {
	elapsed := m.ElapsedHours(now)
	hardH := m.DurationHours + m.OvertimeHours
	switch {
	case elapsed >= hardH:
		return "complete"
	case elapsed >= m.DurationHours:
		return "building (overtime)"
	default:
		return "building"
	}
}

// deadlineStatus returns the appropriate status given current time.
func deadlineStatus(m *state.Mission, now time.Time) state.Status {
	elapsed := now.Sub(m.CreatedAt)
	soft := time.Duration(m.DurationHours * float64(time.Hour))
	hard := soft + time.Duration(m.OvertimeHours*float64(time.Hour))
	switch {
	case elapsed >= hard:
		return state.StatusComplete
	case elapsed >= soft:
		return state.StatusOvertime
	default:
		return state.StatusActive
	}
}
