package wakelog

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Entry is one line in wake_log.jsonl.
type Entry struct {
	WakeNumber        int      `json:"wake_number"`
	TimestampUTC      string   `json:"timestamp_utc"`
	ElapsedHours      float64  `json:"elapsed_hours"`
	Phase             string   `json:"phase"`
	CurrentMilestone  string   `json:"current_milestone,omitempty"`
	WakeGoal          string   `json:"wake_goal"`
	ActionsTaken      []string `json:"actions_taken,omitempty"`
	ArtifactsTouched  []string `json:"artifacts_touched,omitempty"`
	Blockers          []string `json:"blockers"`
	NextWakePlan      string   `json:"next_wake_plan,omitempty"`
	SelfAssessment    string   `json:"self_assessment,omitempty"`
	PromiseTokenFound bool     `json:"promise_token_found"`
	ExitCode          int      `json:"exit_code"`
	WallClockSeconds  int      `json:"wall_clock_seconds"`
	CostUSD           float64  `json:"cost_usd"`
	Overtime          bool     `json:"overtime"`
}

// Append adds a single entry to wake_log.jsonl in missionDir.
func Append(missionDir string, e Entry) error {
	data, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("wakelog.Append: marshal: %w", err)
	}
	path := filepath.Join(missionDir, "wake_log.jsonl")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("wakelog.Append: open: %w", err)
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "%s\n", data)
	return err
}

// TailN returns the last n entries from wake_log.jsonl.
func TailN(missionDir string, n int) ([]Entry, error) {
	path := filepath.Join(missionDir, "wake_log.jsonl")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("wakelog.TailN: %w", err)
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	var entries []Entry
	for _, line := range lines {
		if line == "" {
			continue
		}
		var e Entry
		if json.Unmarshal([]byte(line), &e) == nil {
			entries = append(entries, e)
		}
	}
	return entries, nil
}
