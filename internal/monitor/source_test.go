package monitor

import (
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

func TestEventSource_EmptyDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	src := NewEventSource(dir)
	changed, err := src.Poll()
	require.NoError(t, err)
	assert.False(t, changed)
	assert.Empty(t, src.Events())
	assert.Empty(t, src.NewEvents())
}

func TestEventSource_ReadsAndTracks(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	eventsDir := filepath.Join(dir, config.ObserverDir)
	require.NoError(t, os.MkdirAll(eventsDir, 0o755))

	eventsPath := filepath.Join(dir, config.ObserverEventsFile)
	writeEvent := func(typ string, sprint int) {
		evt := agent.BuildEvent{
			Type:      typ,
			Timestamp: time.Now(),
			Sprint:    sprint,
		}
		data, _ := json.Marshal(evt)
		f, err := os.OpenFile(eventsPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		require.NoError(t, err)
		_, err = f.Write(append(data, '\n'))
		require.NoError(t, err)
		f.Close()
	}

	src := NewEventSource(dir)

	// Write two events.
	writeEvent("build_start", 0)
	writeEvent("sprint_start", 1)

	changed, err := src.Poll()
	require.NoError(t, err)
	assert.True(t, changed)
	assert.Len(t, src.Events(), 2)
	assert.Len(t, src.NewEvents(), 2)

	// Poll again with no changes.
	changed, err = src.Poll()
	require.NoError(t, err)
	assert.False(t, changed)
	assert.Len(t, src.Events(), 2)
	assert.Empty(t, src.NewEvents())

	// Add one more event.
	writeEvent("sprint_complete", 1)

	changed, err = src.Poll()
	require.NoError(t, err)
	assert.True(t, changed)
	assert.Len(t, src.Events(), 3)
	assert.Len(t, src.NewEvents(), 1)
	assert.Equal(t, "sprint_complete", src.NewEvents()[0].Type)
}

func TestPhaseSource_DetectsChanges(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	fryDir := filepath.Join(dir, config.FryDir)
	require.NoError(t, os.MkdirAll(fryDir, 0o755))

	src := NewPhaseSource(dir)

	// No phase file yet.
	changed, err := src.Poll()
	require.NoError(t, err)
	assert.False(t, changed)
	assert.Empty(t, src.Phase())

	// Write phase.
	phasePath := filepath.Join(dir, config.BuildPhaseFile)
	require.NoError(t, os.WriteFile(phasePath, []byte("triage\n"), 0o644))

	changed, err = src.Poll()
	require.NoError(t, err)
	assert.True(t, changed)
	assert.Equal(t, "triage", src.Phase())

	// Same phase, no change.
	changed, err = src.Poll()
	require.NoError(t, err)
	assert.False(t, changed)

	// Update phase.
	require.NoError(t, os.WriteFile(phasePath, []byte("sprint\n"), 0o644))
	// Need to ensure mtime changes.
	time.Sleep(10 * time.Millisecond)

	changed, err = src.Poll()
	require.NoError(t, err)
	assert.True(t, changed)
	assert.Equal(t, "sprint", src.Phase())
}

func TestStatusSource_ReadsAtomicJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	fryDir := filepath.Join(dir, config.FryDir)
	require.NoError(t, os.MkdirAll(fryDir, 0o755))

	src := NewStatusSource(dir)

	// No file yet.
	changed, err := src.Poll()
	require.NoError(t, err)
	assert.False(t, changed)
	assert.Nil(t, src.Status())

	// Write status.
	status := &agent.BuildStatus{
		Version: 1,
		Build: agent.BuildInfo{
			Epic:         "TestEpic",
			TotalSprints: 3,
			Status:       "running",
		},
	}
	data, _ := json.MarshalIndent(status, "", "  ")
	statusPath := filepath.Join(dir, config.BuildStatusFile)
	require.NoError(t, os.WriteFile(statusPath, data, 0o644))

	changed, err = src.Poll()
	require.NoError(t, err)
	assert.True(t, changed)
	require.NotNil(t, src.Status())
	assert.Equal(t, "TestEpic", src.Status().Build.Epic)
	assert.True(t, src.Changed())
}

func TestLockSource_DetectsLiveness(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	fryDir := filepath.Join(dir, config.FryDir)
	require.NoError(t, os.MkdirAll(fryDir, 0o755))

	src := NewLockSource(dir)

	// No lock.
	changed, err := src.Poll()
	require.NoError(t, err)
	assert.False(t, changed)
	assert.False(t, src.Active())

	// Write lock with current PID (alive).
	lockPath := filepath.Join(dir, config.LockFile)
	require.NoError(t, os.WriteFile(lockPath, []byte(fmt.Sprintf("%d\n", os.Getpid())), 0o644))

	changed, err = src.Poll()
	require.NoError(t, err)
	assert.True(t, changed)
	assert.True(t, src.Active())
	assert.Equal(t, os.Getpid(), src.PID())

	// Remove lock.
	require.NoError(t, os.Remove(lockPath))

	changed, err = src.Poll()
	require.NoError(t, err)
	assert.True(t, changed)
	assert.False(t, src.Active())
}

func TestProgressSource_DetectsSizeChanges(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	path := filepath.Join(dir, "progress.txt")
	src := NewProgressSource(path)

	// No file.
	changed, err := src.Poll()
	require.NoError(t, err)
	assert.False(t, changed)

	// Write content.
	require.NoError(t, os.WriteFile(path, []byte("# Sprint 1\n"), 0o644))

	changed, err = src.Poll()
	require.NoError(t, err)
	assert.True(t, changed)
	assert.Contains(t, src.Content(), "Sprint 1")

	// Same size.
	changed, err = src.Poll()
	require.NoError(t, err)
	assert.False(t, changed)

	// Append content.
	f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	_, _ = f.WriteString("## Iteration 1\n")
	f.Close()

	changed, err = src.Poll()
	require.NoError(t, err)
	assert.True(t, changed)
	assert.Contains(t, src.Content(), "Iteration 1")
}

func TestLogSource_FindsNewestLog(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	logsDir := filepath.Join(dir, config.BuildLogsDir)
	require.NoError(t, os.MkdirAll(logsDir, 0o755))

	src := NewLogSource(dir, 5)

	// No logs.
	changed, err := src.Poll()
	require.NoError(t, err)
	assert.False(t, changed)

	// Write two log files.
	log1 := filepath.Join(logsDir, "sprint1_iter1_20260330_100000.log")
	require.NoError(t, os.WriteFile(log1, []byte("line1\nline2\nline3\n"), 0o644))
	time.Sleep(10 * time.Millisecond)
	log2 := filepath.Join(logsDir, "sprint1_iter2_20260330_100100.log")
	require.NoError(t, os.WriteFile(log2, []byte("a\nb\nc\nd\ne\nf\ng\n"), 0o644))

	changed, err = src.Poll()
	require.NoError(t, err)
	assert.True(t, changed)
	assert.Equal(t, log2, src.ActivePath())
	// Tail should have last 5 lines.
	assert.Contains(t, src.Tail(), "c")
	assert.Contains(t, src.Tail(), "g")
}

func TestExitReasonSource_DetectsFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	fryDir := filepath.Join(dir, config.FryDir)
	require.NoError(t, os.MkdirAll(fryDir, 0o755))

	src := NewExitReasonSource(dir)

	// No file.
	changed, err := src.Poll()
	require.NoError(t, err)
	assert.False(t, changed)
	assert.False(t, src.Exists())

	// Write exit reason.
	path := filepath.Join(dir, config.BuildExitReasonFile)
	require.NoError(t, os.WriteFile(path, []byte("After sprint 2: audit failed\n"), 0o644))

	changed, err = src.Poll()
	require.NoError(t, err)
	assert.True(t, changed)
	assert.True(t, src.Exists())
	assert.Equal(t, "After sprint 2: audit failed", src.Reason())
}

func TestLastNLines(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		n        int
		expected string
	}{
		{"a\nb\nc\n", 2, "b\nc"},
		{"a\nb\nc\n", 5, "a\nb\nc"},
		{"single\n", 1, "single"},
		{"", 3, ""},
		{"a\nb\nc\nd\ne\n", 3, "c\nd\ne"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, lastNLines(tt.input, tt.n))
	}
}
