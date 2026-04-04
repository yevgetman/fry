package team

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/yevgetman/fry/internal/git"
)

var DefaultGitExecutor git.Executor = git.DefaultExecutor

const heartbeatInterval = 2 * time.Second

func workerWindowName(workerID string) string {
	return strings.ReplaceAll(workerID, "_", "-")
}

func workerCommand(executable, projectDir, teamID, workerID string) string {
	return strings.Join([]string{
		shellQuote(executable),
		"team",
		"worker",
		"--project-dir",
		shellQuote(projectDir),
		"--team",
		shellQuote(teamID),
		"--worker",
		shellQuote(workerID),
	}, " ")
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

func StartWorkerHost(ctx context.Context, tmux Tmux, cfg *Config, identity *WorkerIdentity, executable string) error {
	window := workerWindowName(identity.WorkerID)
	command := workerCommand(executable, cfg.ProjectDir, cfg.TeamID, identity.WorkerID)
	paneID, err := tmux.NewWindow(ctx, cfg.TMuxSession, window, command)
	if err != nil {
		return err
	}
	identity.PaneID = paneID
	identity.WindowName = window
	identity.Status = WorkerStarting
	if err := SaveIdentity(cfg.ProjectDir, cfg.TeamID, identity); err != nil {
		return err
	}
	record := &WorkerRecord{
		WorkerID:      identity.WorkerID,
		Status:        WorkerStarting,
		DesiredStatus: WorkerRunning,
		PaneID:        paneID,
		WindowName:    window,
	}
	if err := SaveWorkerRecord(cfg.ProjectDir, cfg.TeamID, record); err != nil {
		return err
	}
	return emitEvent(cfg.ProjectDir, cfg.TeamID, Event{
		Type:     "team_worker_ready",
		TeamID:   cfg.TeamID,
		WorkerID: identity.WorkerID,
		Data: map[string]string{
			"role":   identity.Role,
			"window": window,
		},
	})
}

func RunWorker(ctx context.Context, opts WorkerRunOptions) error {
	cfg, err := LoadConfig(opts.ProjectDir, opts.TeamID)
	if err != nil {
		return fmt.Errorf("load team config: %w", err)
	}
	identity, err := LoadIdentity(opts.ProjectDir, opts.TeamID, opts.WorkerID)
	if err != nil {
		return fmt.Errorf("load worker identity: %w", err)
	}
	record, err := LoadWorkerRecord(opts.ProjectDir, opts.TeamID, opts.WorkerID)
	if err != nil {
		return fmt.Errorf("load worker status: %w", err)
	}

	iteration := 0
	for {
		iteration++
		cfg, err = LoadConfig(opts.ProjectDir, opts.TeamID)
		if err != nil {
			return err
		}
		record, err = LoadWorkerRecord(opts.ProjectDir, opts.TeamID, opts.WorkerID)
		if err != nil {
			return err
		}

		if cfg.Status == StatusShutdown || record.DesiredStatus == WorkerStopped {
			record.Status = WorkerStopped
			record.CurrentTask = ""
			if err := SaveWorkerRecord(opts.ProjectDir, opts.TeamID, record); err != nil {
				return err
			}
			_ = SaveHeartbeat(opts.ProjectDir, opts.TeamID, &WorkerHeartbeat{
				WorkerID:   opts.WorkerID,
				Status:     WorkerStopped,
				LastSeenAt: time.Now().UTC(),
				Iteration:  iteration,
				Message:    "worker exiting",
			})
			return nil
		}

		if record.DesiredStatus == WorkerDraining && record.CurrentTask == "" {
			record.Status = WorkerStopped
			record.CurrentTask = ""
			if err := SaveWorkerRecord(opts.ProjectDir, opts.TeamID, record); err != nil {
				return err
			}
			_ = SaveHeartbeat(opts.ProjectDir, opts.TeamID, &WorkerHeartbeat{
				WorkerID:   opts.WorkerID,
				Status:     WorkerStopped,
				LastSeenAt: time.Now().UTC(),
				Iteration:  iteration,
				Message:    "worker drained",
			})
			return nil
		}

		if cfg.Status == StatusPaused || record.DesiredStatus == WorkerDraining || cfg.Status == StatusDraining {
			idleStatus := WorkerIdle
			if record.DesiredStatus == WorkerDraining || cfg.Status == StatusDraining {
				idleStatus = WorkerDraining
			}
			record.Status = idleStatus
			record.CurrentTask = ""
			if err := SaveWorkerRecord(opts.ProjectDir, opts.TeamID, record); err != nil {
				return err
			}
			if err := SaveHeartbeat(opts.ProjectDir, opts.TeamID, &WorkerHeartbeat{
				WorkerID:   opts.WorkerID,
				Status:     idleStatus,
				LastSeenAt: time.Now().UTC(),
				Iteration:  iteration,
				Message:    "waiting for resume",
			}); err != nil {
				return err
			}
			if opts.Once {
				return nil
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(heartbeatInterval):
				continue
			}
		}

		task, err := ClaimNextTask(ctx, opts.ProjectDir, opts.TeamID, opts.WorkerID, identity.Role, identity.WorkDir)
		if err != nil {
			if errors.Is(err, errNoEligibleTask) {
				record.Status = WorkerIdle
				record.CurrentTask = ""
				if err := SaveWorkerRecord(opts.ProjectDir, opts.TeamID, record); err != nil {
					return err
				}
				if err := SaveHeartbeat(opts.ProjectDir, opts.TeamID, &WorkerHeartbeat{
					WorkerID:   opts.WorkerID,
					Status:     WorkerIdle,
					LastSeenAt: time.Now().UTC(),
					Iteration:  iteration,
					Message:    "idle",
				}); err != nil {
					return err
				}
				if opts.Once {
					return nil
				}
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(heartbeatInterval):
					continue
				}
			}
			return err
		}

		if err := MarkTaskRunning(opts.ProjectDir, opts.TeamID, task, opts.WorkerID); err != nil {
			return err
		}

		record.Status = WorkerRunning
		record.CurrentTask = task.ID
		record.LastError = ""
		if err := SaveWorkerRecord(opts.ProjectDir, opts.TeamID, record); err != nil {
			return err
		}

		exitCode, taskErr, logPath := executeTask(ctx, cfg, identity, task, iteration, opts)
		if err := MarkTaskFinished(opts.ProjectDir, opts.TeamID, task, opts.WorkerID, exitCode, taskErr, logPath); err != nil {
			return err
		}

		record.Status = WorkerIdle
		record.CurrentTask = ""
		record.LastError = taskErr
		record.LastExitCode = exitCode
		if exitCode != 0 {
			record.Status = WorkerIdle
		}
		if err := SaveWorkerRecord(opts.ProjectDir, opts.TeamID, record); err != nil {
			return err
		}
		if err := SaveHeartbeat(opts.ProjectDir, opts.TeamID, &WorkerHeartbeat{
			WorkerID:   opts.WorkerID,
			Status:     WorkerIdle,
			LastSeenAt: time.Now().UTC(),
			Iteration:  iteration,
			Message:    "task finished",
		}); err != nil {
			return err
		}

		if err := MaybeFinalize(ctx, cfg.ProjectDir, cfg.TeamID); err != nil {
			record.Status = WorkerStalled
			record.LastError = err.Error()
			_ = SaveWorkerRecord(opts.ProjectDir, opts.TeamID, record)
			return err
		}
		if opts.Once {
			return nil
		}
	}
}

func executeTask(ctx context.Context, cfg *Config, identity *WorkerIdentity, task *Task, iteration int, opts WorkerRunOptions) (int, string, string) {
	logPath := TaskLogPath(cfg.ProjectDir, cfg.TeamID, identity.WorkerID, task.ID)
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return 1, err.Error(), logPath
	}
	if task.Command == "" {
		return 1, "task has no command", logPath
	}
	logFile, err := os.Create(logPath)
	if err != nil {
		return 1, err.Error(), logPath
	}
	defer logFile.Close()

	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}
	cmd := exec.CommandContext(ctx, shell, "-lc", task.Command)
	cmd.Dir = identity.WorkDir
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Env = append(os.Environ(),
		"FRY_TEAM_ID="+cfg.TeamID,
		"FRY_TEAM_WORKER_ID="+identity.WorkerID,
		"FRY_TEAM_TASK_ID="+task.ID,
		"FRY_TEAM_ROLE="+identity.Role,
	)

	var wg sync.WaitGroup
	beatCtx, cancel := context.WithCancel(context.Background())
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(heartbeatInterval)
		defer ticker.Stop()
		count := iteration
		for {
			select {
			case <-beatCtx.Done():
				return
			case <-ticker.C:
				count++
				_ = SaveHeartbeat(cfg.ProjectDir, cfg.TeamID, &WorkerHeartbeat{
					WorkerID:    identity.WorkerID,
					Status:      WorkerRunning,
					CurrentTask: task.ID,
					LastSeenAt:  time.Now().UTC(),
					Iteration:   count,
					Message:     "running task",
				})
			}
		}
	}()

	err = cmd.Run()
	cancel()
	wg.Wait()

	exitCode := 0
	errMsg := ""
	if err != nil {
		exitCode = 1
		errMsg = err.Error()
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		}
	}

	if cfg.GitIsolationMode == IsolationPerWorkerWorktree && identity.WorktreeBranch != "" && exitCode == 0 {
		if commitErr := autoCommitWorktree(ctx, identity, cfg.TeamID, task.ID); commitErr != nil {
			exitCode = 1
			errMsg = commitErr.Error()
		}
	}

	return exitCode, errMsg, logPath
}

func autoCommitWorktree(ctx context.Context, identity *WorkerIdentity, teamID, taskID string) error {
	status, err := DefaultGitExecutor.StatusPorcelain(ctx, identity.WorkDir)
	if err != nil {
		return err
	}
	if strings.TrimSpace(status) == "" {
		return nil
	}
	add := exec.CommandContext(ctx, "git", "add", "-A")
	add.Dir = identity.WorkDir
	if output, err := add.CombinedOutput(); err != nil {
		return fmt.Errorf("git add: %s", strings.TrimSpace(string(output)))
	}
	msg := fmt.Sprintf("fry team %s worker %s task %s", teamID, identity.WorkerID, taskID)
	commit := exec.CommandContext(ctx, "git",
		"-c", "user.name=Fry Team",
		"-c", "user.email=fry-team@localhost",
		"commit", "-m", msg,
	)
	commit.Dir = identity.WorkDir
	if output, err := commit.CombinedOutput(); err != nil {
		return fmt.Errorf("git commit: %s", strings.TrimSpace(string(output)))
	}
	return nil
}

func workerNumber(workerID string) int {
	idx := strings.LastIndex(workerID, "-")
	if idx == -1 {
		return 0
	}
	n, _ := strconv.Atoi(workerID[idx+1:])
	return n
}
