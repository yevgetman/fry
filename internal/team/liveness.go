package team

import (
	"context"
	"fmt"
	"time"
)

func Reconcile(ctx context.Context, projectDir, teamID string) error {
	cfg, err := LoadConfig(projectDir, teamID)
	if err != nil {
		return err
	}
	workerIDs, err := ListWorkerIDs(projectDir, teamID)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	for _, workerID := range workerIDs {
		identity, err := LoadIdentity(projectDir, teamID, workerID)
		if err != nil {
			return err
		}
		record, err := LoadWorkerRecord(projectDir, teamID, workerID)
		if err != nil {
			return err
		}
		hb, err := LoadHeartbeat(projectDir, teamID, workerID)
		if err != nil {
			hb = nil
		}
		alive := identity.WindowName == "" || DefaultTmux.WindowAlive(ctx, cfg.TMuxSession, identity.WindowName)
		stale := hb != nil && now.Sub(hb.LastSeenAt) > defaultHeartbeatGrace*time.Second
		if alive && !stale {
			continue
		}
		if !alive || stale {
			reason := "worker heartbeat stale"
			if !alive {
				reason = "worker tmux window missing"
			}
			record.Status = WorkerDead
			record.LastError = reason
			if err := SaveWorkerRecord(projectDir, teamID, record); err != nil {
				return err
			}
			if err := RequeueOwnedTasks(projectDir, teamID, workerID, reason); err != nil {
				return err
			}
			if err := emitEvent(projectDir, teamID, Event{
				Type:     "team_worker_stalled",
				TeamID:   teamID,
				WorkerID: workerID,
				Data: map[string]string{
					"reason": reason,
				},
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

func RestartDeadWorkers(ctx context.Context, projectDir, teamID, executable string) error {
	cfg, err := LoadConfig(projectDir, teamID)
	if err != nil {
		return err
	}
	workerIDs, err := ListWorkerIDs(projectDir, teamID)
	if err != nil {
		return err
	}
	for _, workerID := range workerIDs {
		record, err := LoadWorkerRecord(projectDir, teamID, workerID)
		if err != nil {
			return err
		}
		if record.Status != WorkerDead && record.Status != WorkerStopped {
			continue
		}
		identity, err := LoadIdentity(projectDir, teamID, workerID)
		if err != nil {
			return err
		}
		if err := StartWorkerHost(ctx, DefaultTmux, cfg, identity, executable); err != nil {
			return fmt.Errorf("restart %s: %w", workerID, err)
		}
	}
	return nil
}
