package chat

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// SupervisorEntry is one line in supervisor_log.jsonl.
type SupervisorEntry struct {
	TimestampUTC  string   `json:"timestamp_utc"`
	Type          string   `json:"type"`
	Summary       string   `json:"summary"`
	FieldsChanged []string `json:"fields_changed"`
	Operator      string   `json:"operator"`
}

// AppendSupervisorLog appends one entry to supervisor_log.jsonl in missionDir.
func AppendSupervisorLog(missionDir string, entryType, summary string, fields []string) error {
	e := SupervisorEntry{
		TimestampUTC:  time.Now().UTC().Format(time.RFC3339),
		Type:          entryType,
		Summary:       summary,
		FieldsChanged: fields,
		Operator:      "chat",
	}
	data, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("supervisorlog: marshal: %w", err)
	}
	path := filepath.Join(missionDir, "supervisor_log.jsonl")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("supervisorlog: open: %w", err)
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "%s\n", data)
	return err
}
