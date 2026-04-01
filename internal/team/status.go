package team

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"time"
)

func SnapshotForTeam(ctx context.Context, projectDir, teamID string) (*Snapshot, error) {
	if err := Reconcile(ctx, projectDir, teamID); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	cfg, err := LoadConfig(projectDir, teamID)
	if err != nil {
		return nil, err
	}
	taskList, err := ListTasks(projectDir, teamID)
	if err != nil {
		return nil, err
	}
	workerIDs, err := ListWorkerIDs(projectDir, teamID)
	if err != nil {
		return nil, err
	}
	workers := make([]WorkerSnapshot, 0, len(workerIDs))
	snap := &Snapshot{
		Config: cfgCopy(cfg),
		Tasks:  taskList,
	}
	for _, workerID := range workerIDs {
		identity, err := LoadIdentity(projectDir, teamID, workerID)
		if err != nil {
			return nil, err
		}
		record, err := LoadWorkerRecord(projectDir, teamID, workerID)
		if err != nil {
			return nil, err
		}
		hb, _ := LoadHeartbeat(projectDir, teamID, workerID)
		alive := identity.WindowName == "" || DefaultTmux.WindowAlive(ctx, cfg.TMuxSession, identity.WindowName)
		workers = append(workers, WorkerSnapshot{
			Identity:  *identity,
			Record:    *record,
			Heartbeat: hb,
			Alive:     alive,
		})
		switch record.Status {
		case WorkerIdle:
			snap.IdleWorkers++
		case WorkerRunning:
			snap.RunningWorkers++
		case WorkerDead, WorkerStalled:
			snap.StalledWorkers++
		}
	}
	snap.Workers = workers
	for _, task := range taskList {
		switch task.Status {
		case TaskPending:
			snap.Pending++
		case TaskInProgress, TaskAssigned:
			snap.InProgress++
		case TaskCompleted:
			snap.Completed++
		case TaskFailed:
			snap.Failed++
		case TaskBlocked:
			snap.Blocked++
		}
	}
	snap.Active = cfg.Status == StatusRunning || cfg.Status == StatusPaused || cfg.Status == StatusDraining || cfg.Status == StatusStarting
	if cfg.GitIsolationMode == IsolationPerWorkerWorktree {
		if _, err := os.Stat(filepath.Join(IntegratedOutputDir(projectDir, teamID), ".git")); err == nil {
			snap.IntegratedDir = IntegratedOutputDir(projectDir, teamID)
		}
	} else if cfg.Status == StatusComplete {
		snap.IntegratedDir = cfg.ProjectDir
	}
	sort.Slice(snap.Workers, func(i, j int) bool {
		return snap.Workers[i].Identity.WorkerID < snap.Workers[j].Identity.WorkerID
	})
	return snap, nil
}

func ActiveSnapshot(ctx context.Context, projectDir string) (*Snapshot, error) {
	teamID, err := readActiveTeam(projectDir)
	if err != nil {
		return nil, err
	}
	return SnapshotForTeam(ctx, projectDir, teamID)
}

func cfgCopy(cfg *Config) Config {
	out := *cfg
	if out.UpdatedAt.IsZero() {
		out.UpdatedAt = time.Now().UTC()
	}
	return out
}
