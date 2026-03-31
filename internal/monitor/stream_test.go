package monitor

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yevgetman/fry/internal/agent"
	"github.com/yevgetman/fry/internal/config"
)

func setupTestProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, config.ObserverDir), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, config.FryDir), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, config.BuildLogsDir), 0o755))
	return dir
}

func writeTestEvent(t *testing.T, dir string, typ string, sprint int) {
	t.Helper()
	evt := agent.BuildEvent{
		Type:      typ,
		Timestamp: time.Now(),
		Sprint:    sprint,
		Data:      map[string]string{"test": "true"},
	}
	data, _ := json.Marshal(evt)
	eventsPath := filepath.Join(dir, config.ObserverEventsFile)
	f, err := os.OpenFile(eventsPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	require.NoError(t, err)
	_, err = f.Write(append(data, '\n'))
	require.NoError(t, err)
	f.Close()
}

func writeTestLock(t *testing.T, dir string) {
	t.Helper()
	lockPath := filepath.Join(dir, config.LockFile)
	require.NoError(t, os.WriteFile(lockPath, []byte(fmt.Sprintf("%d\n", os.Getpid())), 0o644))
}

func removeTestLock(t *testing.T, dir string) {
	t.Helper()
	os.Remove(filepath.Join(dir, config.LockFile))
}

func TestMonitor_Snapshot(t *testing.T) {
	t.Parallel()
	dir := setupTestProject(t)

	writeTestLock(t, dir)
	writeTestEvent(t, dir, "build_start", 0)
	writeTestEvent(t, dir, "sprint_start", 1)

	require.NoError(t, os.WriteFile(
		filepath.Join(dir, config.BuildPhaseFile),
		[]byte("sprint\n"), 0o644,
	))

	mon := New(Config{
		ProjectDir:  dir,
		WorktreeDir: dir,
	})

	snap, err := mon.Snapshot()
	require.NoError(t, err)

	assert.True(t, snap.BuildActive)
	assert.Equal(t, os.Getpid(), snap.PID)
	assert.Equal(t, "sprint", snap.Phase)
	assert.Len(t, snap.Events, 2)
	assert.Equal(t, "build_start", snap.Events[0].Type)
	assert.Equal(t, "sprint_start", snap.Events[1].Type)
}

func TestMonitor_SnapshotIgnoresStaleExitReasonWhileBuildActive(t *testing.T) {
	t.Parallel()
	dir := setupTestProject(t)

	writeTestLock(t, dir)
	writeTestEvent(t, dir, "build_start", 0)
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, config.BuildExitReasonFile),
		[]byte("After sprint 4: prior failure\n"), 0o644,
	))

	mon := New(Config{
		ProjectDir:  dir,
		WorktreeDir: dir,
	})

	snap, err := mon.Snapshot()
	require.NoError(t, err)

	assert.True(t, snap.BuildActive)
	assert.False(t, snap.BuildEnded)
	assert.Equal(t, "After sprint 4: prior failure", snap.ExitReason)
}

func TestMonitor_RunDetectsBuildEnd(t *testing.T) {
	t.Parallel()
	dir := setupTestProject(t)

	writeTestLock(t, dir)
	writeTestEvent(t, dir, "build_start", 0)

	mon := New(Config{
		ProjectDir:  dir,
		WorktreeDir: dir,
		Interval:    50 * time.Millisecond,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch := mon.Run(ctx)

	// Collect first snapshot.
	snap := <-ch
	assert.True(t, snap.BuildActive)
	assert.False(t, snap.BuildEnded)

	// Write build_end event.
	writeTestEvent(t, dir, "build_end", 0)

	// Should get a final snapshot with BuildEnded=true.
	var finalSnap Snapshot
	for s := range ch {
		finalSnap = s
	}
	assert.True(t, finalSnap.BuildEnded)
}

func TestMonitor_RunDetectsProcessDeath(t *testing.T) {
	t.Parallel()
	dir := setupTestProject(t)

	writeTestLock(t, dir)
	writeTestEvent(t, dir, "build_start", 0)

	mon := New(Config{
		ProjectDir:  dir,
		WorktreeDir: dir,
		Interval:    50 * time.Millisecond,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch := mon.Run(ctx)

	// Wait for first snapshot.
	snap := <-ch
	assert.True(t, snap.BuildActive)

	// Remove lock to simulate process death.
	removeTestLock(t, dir)

	// Should detect death and close.
	var finalSnap Snapshot
	for s := range ch {
		finalSnap = s
	}
	assert.True(t, finalSnap.BuildEnded)
	assert.Contains(t, finalSnap.ExitReason, "exited unexpectedly")
}

func TestMonitor_NoWaitExitsWhenNoBuild(t *testing.T) {
	t.Parallel()
	dir := setupTestProject(t)

	mon := New(Config{
		ProjectDir:  dir,
		WorktreeDir: dir,
		Interval:    50 * time.Millisecond,
		Wait:        false,
	})

	snap, err := mon.Snapshot()
	require.NoError(t, err)
	assert.False(t, snap.BuildActive)
	assert.Empty(t, snap.Events)
}

func TestMonitor_ConfigDefaults(t *testing.T) {
	t.Parallel()

	mon := New(Config{ProjectDir: t.TempDir()})
	assert.Equal(t, time.Duration(config.MonitorDefaultIntervalSec)*time.Second, mon.cfg.Interval)
	assert.Equal(t, config.MonitorDefaultLogTailLines, mon.cfg.LogTailLines)
}
