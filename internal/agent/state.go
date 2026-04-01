package agent

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/yevgetman/fry/internal/config"
	"github.com/yevgetman/fry/internal/observer"
)

// ReadBuildState assembles the current build state from .fry/ artifacts.
// Returns a state with Active=false if no build artifacts exist.
func ReadBuildState(projectDir string) (*BuildState, error) {
	absDir, err := filepath.Abs(projectDir)
	if err != nil {
		return nil, fmt.Errorf("resolve project dir: %w", err)
	}

	state := &BuildState{
		ProjectDir: absDir,
		Status:     "idle",
	}

	fryDir := filepath.Join(absDir, config.FryDir)
	if _, err := os.Stat(fryDir); os.IsNotExist(err) {
		return state, nil
	}

	// Parse epic for sprint info
	epicPath := filepath.Join(absDir, config.FryDir, "epic.md")
	epicInfo, err := parseEpicMetadata(epicPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("parse epic: %w", err)
	}
	if epicInfo != nil {
		state.Epic = epicInfo.name
		state.TotalSprints = epicInfo.totalSprints
		state.Effort = epicInfo.effort
		state.Engine = epicInfo.engine
	}

	// Read build mode
	if data, err := os.ReadFile(filepath.Join(absDir, config.BuildModeFile)); err == nil {
		state.Mode = strings.TrimSpace(string(data))
	}

	// Read events to determine current sprint and status
	events, err := observer.ReadEvents(absDir)
	if err != nil {
		return nil, fmt.Errorf("read events: %w", err)
	}
	if len(events) > 0 {
		last := events[len(events)-1]
		state.LastEvent = observerEventToBuildEvent(last)
		state.CurrentSprint = inferCurrentSprint(events)
		state.CurrentSprintName = inferCurrentSprintName(events)
		state.Status = inferStatus(events)
		state.StartedAt = parseTimestamp(events[0].Timestamp)
	}

	// Enrich from build-status.json when available (more complete than events).
	if bs, bsErr := ReadBuildStatus(absDir); bsErr == nil && bs != nil {
		if bs.Build.Epic != "" {
			state.Epic = bs.Build.Epic
		}
		if bs.Build.TotalSprints > 0 {
			state.TotalSprints = bs.Build.TotalSprints
		}
		if bs.Build.CurrentSprint > 0 {
			state.CurrentSprint = bs.Build.CurrentSprint
		}
		if bs.Build.Effort != "" {
			state.Effort = bs.Build.Effort
		}
		if bs.Build.Engine != "" {
			state.Engine = bs.Build.Engine
		}
		if bs.Build.Mode != "" {
			state.Mode = bs.Build.Mode
		}
		if bs.Build.Phase != "" {
			state.Phase = bs.Build.Phase
		}
		if !bs.Build.StartedAt.IsZero() {
			t := bs.Build.StartedAt
			state.StartedAt = &t
		}
		// Infer current sprint name from sprint list
		for _, sp := range bs.Sprints {
			if sp.Number == bs.Build.CurrentSprint {
				state.CurrentSprintName = sp.Name
				break
			}
		}
		// Use build-status.json status when event-based inference yielded idle
		if bs.Build.Status != "" && state.Status == "idle" {
			state.Status = bs.Build.Status
		}
	}

	// Read build phase (active during triage/prepare before events exist)
	if data, err := os.ReadFile(filepath.Join(absDir, config.BuildPhaseFile)); err == nil {
		phase := strings.TrimSpace(string(data))
		if phase != "" {
			state.Phase = phase
		}
		// If we have a phase but status is still idle, the build is in an early stage
		if state.Status == "idle" && phase != "" && phase != "complete" && phase != "failed" {
			switch {
			case phase == "triage":
				state.Status = "triaging"
			case strings.HasPrefix(phase, "prepare"):
				state.Status = "preparing"
			default:
				state.Status = "running"
			}
		}
	}

	// Git branch
	state.GitBranch = readGitBranch(absDir)

	// Check exit reason first — authoritative signal that the build finished.
	// Must come before PID check to avoid stale lock files overriding the result.
	if data, err := os.ReadFile(filepath.Join(absDir, config.BuildExitReasonFile)); err == nil {
		reason := strings.TrimSpace(string(data))
		if reason != "" {
			if strings.Contains(reason, "success") {
				state.Status = "completed"
			} else {
				state.Status = "failed"
			}
		}
	}

	// Check for running process via lock file — only if status is still ambiguous
	if state.Status != "completed" && state.Status != "failed" {
		lockPath := filepath.Join(absDir, config.LockFile)
		if pid, err := readPIDFromLock(lockPath); err == nil && pid > 0 {
			if processRunning(pid) {
				state.PID = pid
				if state.Status != "paused" {
					state.Status = "running"
				}
			}
		}
	}

	// If no running process and status is still "running", it crashed/exited
	if state.PID == 0 && state.Status == "running" {
		state.Status = "stopped"
	}

	// Derive Active from final status
	state.Active = state.Status == "running" || state.Status == "paused" || state.Status == "stopped"

	return state, nil
}

// ReadProgress reads sprint-progress.txt or epic-progress.txt.
// scope should be "sprint" or "epic".
func ReadProgress(projectDir string, scope string) (string, error) {
	absDir, err := filepath.Abs(projectDir)
	if err != nil {
		return "", fmt.Errorf("resolve project dir: %w", err)
	}
	projectDir = absDir
	var file string
	switch scope {
	case "sprint":
		file = config.SprintProgressFile
	case "epic":
		file = config.EpicProgressFile
	default:
		return "", fmt.Errorf("unknown progress scope: %s (use 'sprint' or 'epic')", scope)
	}
	data, err := os.ReadFile(filepath.Join(projectDir, file))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", fmt.Errorf("read %s progress: %w", scope, err)
	}
	return string(data), nil
}

// ReadLatestLog returns the last N lines of the most recent build log matching logType.
// logType: "sprint", "heal", "audit", "latest" (most recent of any type).
func ReadLatestLog(projectDir string, logType string, lines int) (string, error) {
	absDir, err := filepath.Abs(projectDir)
	if err != nil {
		return "", fmt.Errorf("resolve project dir: %w", err)
	}
	logsDir := filepath.Join(absDir, config.BuildLogsDir)
	entries, err := os.ReadDir(logsDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", fmt.Errorf("read build logs: %w", err)
	}

	// Filter and sort by modification time (newest first)
	var matched []os.DirEntry
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		switch logType {
		case "sprint":
			if strings.Contains(name, "_iter") || strings.Contains(name, "_heal") {
				continue
			}
			if strings.HasPrefix(name, "sprint") {
				matched = append(matched, entry)
			}
		case "heal":
			if strings.Contains(name, "_heal") {
				matched = append(matched, entry)
			}
		case "audit":
			if strings.Contains(name, "audit") {
				matched = append(matched, entry)
			}
		case "latest":
			if strings.HasSuffix(name, ".log") {
				matched = append(matched, entry)
			}
		default:
			if strings.HasSuffix(name, ".log") {
				matched = append(matched, entry)
			}
		}
	}

	if len(matched) == 0 {
		return "", nil
	}

	// Find most recent by name (timestamps in filenames sort lexicographically)
	latest := matched[0]
	for _, e := range matched[1:] {
		if e.Name() > latest.Name() {
			latest = e
		}
	}

	data, err := os.ReadFile(filepath.Join(logsDir, latest.Name()))
	if err != nil {
		return "", fmt.Errorf("read log %s: %w", latest.Name(), err)
	}

	content := string(data)
	if lines > 0 {
		content = lastNLines(content, lines)
	}
	return content, nil
}

// --- helpers ---

type epicMeta struct {
	name         string
	totalSprints int
	effort       string
	engine       string
}

func parseEpicMetadata(epicPath string) (*epicMeta, error) {
	data, err := os.ReadFile(epicPath)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(data), "\n")
	meta := &epicMeta{}
	sprintCount := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# ") && meta.name == "" {
			meta.name = strings.TrimPrefix(trimmed, "# ")
		}
		if strings.HasPrefix(trimmed, "## Sprint ") {
			sprintCount++
		}
		if strings.HasPrefix(trimmed, "@effort ") {
			meta.effort = strings.TrimPrefix(trimmed, "@effort ")
		}
		if strings.HasPrefix(trimmed, "@engine ") {
			meta.engine = strings.TrimPrefix(trimmed, "@engine ")
		}
	}
	meta.totalSprints = sprintCount
	if meta.engine == "" {
		meta.engine = config.DefaultEngine
	}
	return meta, nil
}

func observerEventToBuildEvent(evt observer.Event) *BuildEvent {
	ts, _ := time.Parse(time.RFC3339, evt.Timestamp)
	return &BuildEvent{
		Type:      string(evt.Type),
		Timestamp: ts,
		Sprint:    evt.Sprint,
		Data:      evt.Data,
	}
}

func inferCurrentSprint(events []observer.Event) int {
	sprint := 0
	for _, evt := range events {
		if evt.Sprint > sprint {
			sprint = evt.Sprint
		}
	}
	return sprint
}

func inferCurrentSprintName(events []observer.Event) string {
	// Walk backwards to find the last sprint_start event
	for i := len(events) - 1; i >= 0; i-- {
		if events[i].Type == observer.EventSprintStart {
			if name, ok := events[i].Data["name"]; ok {
				return name
			}
		}
	}
	return ""
}

func inferStatus(events []observer.Event) string {
	if len(events) == 0 {
		return "idle"
	}
	last := events[len(events)-1]
	switch last.Type {
	case observer.EventBuildEnd:
		if outcome, ok := last.Data["outcome"]; ok && outcome == "success" {
			return "completed"
		}
		return "failed"
	case observer.EventTriageStart:
		return "triaging"
	case observer.EventTriageComplete, observer.EventPrepareComplete:
		return "running"
	case observer.EventPrepareStart:
		return "preparing"
	case observer.EventBuildStart, observer.EventSprintStart:
		return "running"
	case observer.EventSprintComplete, observer.EventEngineFailover, observer.EventAlignmentComplete,
		observer.EventAuditComplete, observer.EventReviewComplete,
		observer.EventBuildAuditDone:
		return "running"
	default:
		return "unknown"
	}
}

func parseTimestamp(ts string) *time.Time {
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return nil
	}
	return &t
}

func readPIDFromLock(lockPath string) (int, error) {
	data, err := os.ReadFile(lockPath)
	if err != nil {
		return 0, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, err
	}
	return pid, nil
}

func processRunning(pid int) bool {
	// Signal 0 tests if process exists without actually sending a signal.
	// ESRCH = no such process. EPERM = process exists but owned by another user.
	err := syscall.Kill(pid, 0)
	if err == nil {
		return true
	}
	return !errors.Is(err, syscall.ESRCH)
}

func readGitBranch(projectDir string) string {
	headPath := filepath.Join(projectDir, ".git", "HEAD")
	data, err := os.ReadFile(headPath)
	if err != nil {
		return ""
	}
	ref := strings.TrimSpace(string(data))
	if strings.HasPrefix(ref, "ref: refs/heads/") {
		return strings.TrimPrefix(ref, "ref: refs/heads/")
	}
	return ""
}

func lastNLines(s string, n int) string {
	s = strings.TrimRight(s, "\n")
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}
