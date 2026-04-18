package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type Status string

const (
	StatusActive   Status = "active"
	StatusOvertime Status = "overtime"
	StatusComplete Status = "complete"
	StatusStopped  Status = "stopped"
	StatusFailed   Status = "failed"
)

type Mission struct {
	MissionID       string    `json:"mission_id"`
	CreatedAt       time.Time `json:"created_at"`
	PromptPath      string    `json:"prompt_path,omitempty"`
	PlanPath        string    `json:"plan_path,omitempty"`
	SpecDir         string    `json:"spec_dir,omitempty"`
	InputMode       string    `json:"input_mode"`
	Effort          string    `json:"effort"`
	IntervalSeconds int       `json:"interval_seconds"`
	DurationHours   float64   `json:"duration_hours"`
	OvertimeHours   float64   `json:"overtime_hours"`
	CurrentWake     int       `json:"current_wake"`
	LastWakeAt      time.Time `json:"last_wake_at,omitempty"`
	Status          Status    `json:"status"`
	HardDeadlineUTC time.Time `json:"hard_deadline_utc"`
}

func (m *Mission) SoftDeadline() time.Time {
	return m.CreatedAt.Add(time.Duration(float64(time.Hour) * m.DurationHours))
}

func (m *Mission) ElapsedHours(now time.Time) float64 {
	return now.Sub(m.CreatedAt).Hours()
}

const stateFile = "state.json"

func Load(missionDir string) (*Mission, error) {
	path := filepath.Join(missionDir, stateFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("state.Load: %w", err)
	}
	var m Mission
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("state.Load: unmarshal: %w", err)
	}
	return &m, nil
}

// Save writes state.json atomically via temp-file + rename.
func (m *Mission) Save(missionDir string) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("state.Save: marshal: %w", err)
	}
	path := filepath.Join(missionDir, stateFile)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("state.Save: write: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("state.Save: rename: %w", err)
	}
	return nil
}
