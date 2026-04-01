package team

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var ErrNoActiveTeam = errors.New("no active team")

func ensureTeamLayout(projectDir, teamID string) error {
	dirs := []string{
		RootDir(projectDir),
		TeamDir(projectDir, teamID),
		TasksDir(projectDir, teamID),
		WorkersDir(projectDir, teamID),
		LocksDir(projectDir, teamID),
		ArtifactsDir(projectDir, teamID),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create team dir %s: %w", dir, err)
		}
	}
	return nil
}

func writeJSONAtomic(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp, err := os.CreateTemp(filepath.Dir(path), "fry-team-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}

func readJSON(path string, dest any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dest)
}

func writeTextAtomic(path, value string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "fry-team-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if _, err := tmp.WriteString(value); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}

func setActiveTeam(projectDir, teamID string) error {
	return writeTextAtomic(ActiveTeamPath(projectDir), teamID+"\n")
}

func clearActiveTeam(projectDir, teamID string) error {
	active, err := readActiveTeam(projectDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if active != teamID {
		return nil
	}
	if err := os.Remove(ActiveTeamPath(projectDir)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func readActiveTeam(projectDir string) (string, error) {
	data, err := os.ReadFile(ActiveTeamPath(projectDir))
	if err != nil {
		return "", err
	}
	teamID := strings.TrimSpace(string(data))
	if teamID == "" {
		return "", ErrNoActiveTeam
	}
	return teamID, nil
}

func ResolveTeamID(projectDir, requested string) (string, error) {
	if strings.TrimSpace(requested) != "" {
		return requested, nil
	}
	if active, err := readActiveTeam(projectDir); err == nil {
		return active, nil
	}
	entries, err := os.ReadDir(RootDir(projectDir))
	if err != nil {
		if os.IsNotExist(err) {
			return "", ErrNoActiveTeam
		}
		return "", err
	}
	var dirs []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dirs = append(dirs, entry.Name())
	}
	sort.Strings(dirs)
	if len(dirs) == 0 {
		return "", ErrNoActiveTeam
	}
	return dirs[len(dirs)-1], nil
}

func LoadConfig(projectDir, teamID string) (*Config, error) {
	var cfg Config
	if err := readJSON(ConfigPath(projectDir, teamID), &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func SaveConfig(projectDir string, cfg *Config) error {
	if cfg.Version == 0 {
		cfg.Version = 1
	}
	cfg.UpdatedAt = time.Now().UTC()
	return writeJSONAtomic(ConfigPath(projectDir, cfg.TeamID), cfg)
}

func LoadTask(projectDir, teamID, taskID string) (*Task, error) {
	var task Task
	if err := readJSON(TaskPath(projectDir, teamID, taskID), &task); err != nil {
		return nil, err
	}
	return &task, nil
}

func SaveTask(projectDir, teamID string, task *Task) error {
	task.UpdatedAt = time.Now().UTC()
	if task.CreatedAt.IsZero() {
		task.CreatedAt = task.UpdatedAt
	}
	return writeJSONAtomic(TaskPath(projectDir, teamID, task.ID), task)
}

func ListTasks(projectDir, teamID string) ([]Task, error) {
	entries, err := os.ReadDir(TasksDir(projectDir, teamID))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	tasks := make([]Task, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		var task Task
		if err := readJSON(filepath.Join(TasksDir(projectDir, teamID), entry.Name()), &task); err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	sort.Slice(tasks, func(i, j int) bool {
		if tasks[i].Priority == tasks[j].Priority {
			return tasks[i].ID < tasks[j].ID
		}
		return tasks[i].Priority > tasks[j].Priority
	})
	return tasks, nil
}

func SaveIdentity(projectDir, teamID string, identity *WorkerIdentity) error {
	return writeJSONAtomic(WorkerIdentityPath(projectDir, teamID, identity.WorkerID), identity)
}

func LoadIdentity(projectDir, teamID, workerID string) (*WorkerIdentity, error) {
	var identity WorkerIdentity
	if err := readJSON(WorkerIdentityPath(projectDir, teamID, workerID), &identity); err != nil {
		return nil, err
	}
	return &identity, nil
}

func SaveWorkerRecord(projectDir, teamID string, record *WorkerRecord) error {
	record.UpdatedAt = time.Now().UTC()
	return writeJSONAtomic(WorkerStatusPath(projectDir, teamID, record.WorkerID), record)
}

func LoadWorkerRecord(projectDir, teamID, workerID string) (*WorkerRecord, error) {
	var record WorkerRecord
	if err := readJSON(WorkerStatusPath(projectDir, teamID, workerID), &record); err != nil {
		return nil, err
	}
	return &record, nil
}

func SaveHeartbeat(projectDir, teamID string, hb *WorkerHeartbeat) error {
	if hb.LastSeenAt.IsZero() {
		hb.LastSeenAt = time.Now().UTC()
	}
	return writeJSONAtomic(WorkerHeartbeatPath(projectDir, teamID, hb.WorkerID), hb)
}

func LoadHeartbeat(projectDir, teamID, workerID string) (*WorkerHeartbeat, error) {
	var hb WorkerHeartbeat
	if err := readJSON(WorkerHeartbeatPath(projectDir, teamID, workerID), &hb); err != nil {
		return nil, err
	}
	return &hb, nil
}

func ListWorkerIDs(projectDir, teamID string) ([]string, error) {
	entries, err := os.ReadDir(WorkersDir(projectDir, teamID))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var ids []string
	for _, entry := range entries {
		if entry.IsDir() {
			ids = append(ids, entry.Name())
		}
	}
	sort.Strings(ids)
	return ids, nil
}

func appendTeamEvent(path string, evt Event) error {
	if evt.Timestamp == "" {
		evt.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	line, err := json.Marshal(evt)
	if err != nil {
		return err
	}
	line = append(line, '\n')
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(line)
	return err
}

func withFileLock(ctx context.Context, path string, fn func() error) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()

	for {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			_, _ = f.WriteString(fmt.Sprintf("%d\n", os.Getpid()))
			_ = f.Close()
			defer os.Remove(path)
			return fn()
		}
		if !os.IsExist(err) {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func readTaskFile(path string) ([]TaskInput, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return nil, nil
	}
	var wrapped TaskFile
	if err := json.Unmarshal(data, &wrapped); err == nil && len(wrapped.Tasks) > 0 {
		return wrapped.Tasks, nil
	}
	var tasks []TaskInput
	if err := json.Unmarshal(data, &tasks); err != nil {
		return nil, err
	}
	return tasks, nil
}
