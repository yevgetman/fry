package team

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

var errNoEligibleTask = errors.New("no eligible task")

func AssignTasksFromFile(projectDir, teamID, path string) ([]Task, error) {
	inputs, err := readTaskFile(path)
	if err != nil {
		return nil, fmt.Errorf("read task file: %w", err)
	}
	if len(inputs) == 0 {
		return nil, nil
	}
	now := time.Now().UTC()
	created := make([]Task, 0, len(inputs))
	for idx, input := range inputs {
		id := strings.TrimSpace(input.ID)
		if id == "" {
			id = fmt.Sprintf("task-%03d", idx+1)
		}
		task := Task{
			ID:              id,
			Title:           strings.TrimSpace(input.Title),
			Description:     strings.TrimSpace(input.Description),
			Role:            strings.TrimSpace(input.Role),
			Status:          TaskPending,
			Priority:        input.Priority,
			Command:         strings.TrimSpace(input.Command),
			BlockedBy:       append([]string(nil), input.BlockedBy...),
			AcceptanceHints: append([]string(nil), input.AcceptanceHints...),
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		if task.Title == "" {
			task.Title = task.ID
		}
		if err := SaveTask(projectDir, teamID, &task); err != nil {
			return nil, err
		}
		if err := emitEvent(projectDir, teamID, Event{
			Type:   "team_task_created",
			TeamID: teamID,
			TaskID: task.ID,
			Data: map[string]string{
				"role": task.Role,
			},
		}); err != nil {
			return nil, err
		}
		created = append(created, task)
	}
	if err := writePlanSummary(projectDir, teamID, created); err != nil {
		return nil, err
	}
	return created, nil
}

func ClaimNextTask(ctx context.Context, projectDir, teamID, workerID, role, workDir string) (*Task, error) {
	lockPath := AssignmentLockPath(projectDir, teamID)
	var claimed *Task
	err := withFileLock(ctx, lockPath, func() error {
		tasks, err := ListTasks(projectDir, teamID)
		if err != nil {
			return err
		}
		sort.SliceStable(tasks, func(i, j int) bool {
			if tasks[i].Priority == tasks[j].Priority {
				return tasks[i].ID < tasks[j].ID
			}
			return tasks[i].Priority > tasks[j].Priority
		})
		for i := range tasks {
			task := tasks[i]
			if !taskClaimable(task, tasks, role) {
				continue
			}
			now := time.Now().UTC()
			task.Status = TaskAssigned
			task.Owner = workerID
			task.WorkDir = workDir
			task.UpdatedAt = now
			if err := SaveTask(projectDir, teamID, &task); err != nil {
				return err
			}
			claimed = &task
			return nil
		}
		return errNoEligibleTask
	})
	if err != nil {
		return nil, err
	}
	if claimed != nil {
		_ = emitEvent(projectDir, teamID, Event{
			Type:     "team_task_assigned",
			TeamID:   teamID,
			TaskID:   claimed.ID,
			WorkerID: workerID,
			Data: map[string]string{
				"role": role,
			},
		})
	}
	return claimed, nil
}

func MarkTaskRunning(projectDir, teamID string, task *Task, workerID string) error {
	now := time.Now().UTC()
	task.Owner = workerID
	task.Status = TaskInProgress
	task.Attempts++
	task.StartedAt = &now
	task.UpdatedAt = now
	if err := SaveTask(projectDir, teamID, task); err != nil {
		return err
	}
	return emitEvent(projectDir, teamID, Event{
		Type:     "team_task_started",
		TeamID:   teamID,
		TaskID:   task.ID,
		WorkerID: workerID,
	})
}

func MarkTaskFinished(projectDir, teamID string, task *Task, workerID string, exitCode int, lastErr, logPath string) error {
	now := time.Now().UTC()
	task.Owner = workerID
	task.ExitCode = exitCode
	task.LogPath = logPath
	task.LastError = strings.TrimSpace(lastErr)
	task.FinishedAt = &now
	task.UpdatedAt = now
	eventType := "team_task_completed"
	if exitCode == 0 {
		task.Status = TaskCompleted
	} else {
		task.Status = TaskFailed
		eventType = "team_task_failed"
	}
	if err := SaveTask(projectDir, teamID, task); err != nil {
		return err
	}
	data := map[string]string{}
	if task.LogPath != "" {
		data["log"] = task.LogPath
	}
	if task.LastError != "" {
		data["error"] = task.LastError
	}
	return emitEvent(projectDir, teamID, Event{
		Type:     eventType,
		TeamID:   teamID,
		TaskID:   task.ID,
		WorkerID: workerID,
		Data:     data,
	})
}

func RequeueOwnedTasks(projectDir, teamID, workerID, reason string) error {
	tasks, err := ListTasks(projectDir, teamID)
	if err != nil {
		return err
	}
	for i := range tasks {
		task := tasks[i]
		if task.Owner != workerID {
			continue
		}
		if task.Status != TaskAssigned && task.Status != TaskInProgress {
			continue
		}
		task.Status = TaskPending
		task.Owner = ""
		task.LastError = reason
		if err := SaveTask(projectDir, teamID, &task); err != nil {
			return err
		}
	}
	return nil
}

func taskClaimable(task Task, tasks []Task, role string) bool {
	if task.Status != TaskPending {
		return false
	}
	if task.Role != "" && task.Role != role {
		return false
	}
	if len(task.BlockedBy) == 0 {
		return true
	}
	deps := make(map[string]TaskStatus, len(tasks))
	for _, other := range tasks {
		deps[other.ID] = other.Status
	}
	for _, dep := range task.BlockedBy {
		status, ok := deps[dep]
		if !ok {
			return false
		}
		if status != TaskCompleted {
			return false
		}
	}
	return true
}

func writePlanSummary(projectDir, teamID string, tasks []Task) error {
	var b strings.Builder
	b.WriteString("# Team Plan Summary\n\n")
	for _, task := range tasks {
		fmt.Fprintf(&b, "- `%s` [%s] %s\n", task.ID, defaultString(task.Role, "any"), task.Title)
		if task.Command != "" {
			fmt.Fprintf(&b, "  command: `%s`\n", task.Command)
		}
	}
	return os.WriteFile(PlanSummaryPath(projectDir, teamID), []byte(b.String()), 0o644)
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
