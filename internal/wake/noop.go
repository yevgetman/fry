package wake

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/yevgetman/fry/internal/wakelog"
)

// DetectNoop returns (true, reason, nil) if the last n wake_log entries all
// indicate the agent produced no meaningful output (no promise token found).
// Requires at least n entries; returns (false, "", nil) if not enough history.
// If a no-op is detected, it appends a noop_warning to supervisor_log.jsonl
// but does NOT auto-stop the mission.
func DetectNoop(missionDir string, n int) (bool, string, error) {
	if n <= 0 {
		n = 3
	}
	entries, err := wakelog.TailN(missionDir, n)
	if err != nil {
		return false, "", err
	}
	if len(entries) < n {
		return false, "", nil
	}

	for _, e := range entries {
		if e.PromiseTokenFound {
			return false, "", nil
		}
	}

	reason := fmt.Sprintf("last %d wakes all had promise_token_found=false", n)
	if werr := appendNoopWarning(missionDir, reason); werr != nil {
		return true, reason, werr
	}
	return true, reason, nil
}

type noopSupervisorEntry struct {
	TimestampUTC string `json:"timestamp_utc"`
	Type         string `json:"type"`
	Summary      string `json:"summary"`
	Operator     string `json:"operator"`
}

func appendNoopWarning(missionDir, reason string) error {
	e := noopSupervisorEntry{
		TimestampUTC: time.Now().UTC().Format(time.RFC3339),
		Type:         "noop_warning",
		Summary:      reason,
		Operator:     "wake",
	}
	data, err := json.Marshal(e)
	if err != nil {
		return err
	}
	path := filepath.Join(missionDir, "supervisor_log.jsonl")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("noop: open supervisor_log: %w", err)
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "%s\n", data)
	return err
}
