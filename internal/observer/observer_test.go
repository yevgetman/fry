package observer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yevgetman/fry/internal/config"
	"github.com/yevgetman/fry/internal/engine"
	"github.com/yevgetman/fry/internal/epic"
)

// --- stub engine ---

type stubObserverEngine struct {
	output string
	err    error
}

func (s *stubObserverEngine) Run(_ context.Context, prompt string, opts engine.RunOpts) (string, int, error) {
	if opts.Stdout != nil {
		_, _ = opts.Stdout.Write([]byte(s.output))
	}
	return s.output, 0, s.err
}

func (s *stubObserverEngine) Name() string { return "stub" }

// --- ShouldWakeUp tests ---

func TestShouldWakeUp_LowEffort(t *testing.T) {
	t.Parallel()

	assert.False(t, ShouldWakeUp(epic.EffortLow, WakeAfterSprint))
	assert.False(t, ShouldWakeUp(epic.EffortLow, WakeAfterBuildAudit))
	assert.False(t, ShouldWakeUp(epic.EffortLow, WakeBuildEnd))
}

func TestShouldWakeUp_MediumEffort(t *testing.T) {
	t.Parallel()

	assert.False(t, ShouldWakeUp(epic.EffortMedium, WakeAfterSprint))
	assert.False(t, ShouldWakeUp(epic.EffortMedium, WakeAfterBuildAudit))
	assert.True(t, ShouldWakeUp(epic.EffortMedium, WakeBuildEnd))
}

func TestShouldWakeUp_HighEffort(t *testing.T) {
	t.Parallel()

	assert.True(t, ShouldWakeUp(epic.EffortHigh, WakeAfterSprint))
	assert.True(t, ShouldWakeUp(epic.EffortHigh, WakeAfterBuildAudit))
	assert.True(t, ShouldWakeUp(epic.EffortHigh, WakeBuildEnd))
}

func TestShouldWakeUp_MaxEffort(t *testing.T) {
	t.Parallel()

	assert.True(t, ShouldWakeUp(epic.EffortMax, WakeAfterSprint))
	assert.True(t, ShouldWakeUp(epic.EffortMax, WakeAfterBuildAudit))
	assert.True(t, ShouldWakeUp(epic.EffortMax, WakeBuildEnd))
}

func TestShouldWakeUp_EmptyEffort(t *testing.T) {
	t.Parallel()

	// Empty effort is treated like medium
	assert.False(t, ShouldWakeUp("", WakeAfterSprint))
	assert.True(t, ShouldWakeUp("", WakeBuildEnd))
}

// --- InitBuild tests ---

func TestInitBuild_CreatesDirAndResetsScratchpad(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	err := InitBuild(dir, "TestEpic", "high", 3)
	require.NoError(t, err)

	// Verify observer directory exists
	info, err := os.Stat(filepath.Join(dir, config.ObserverDir))
	require.NoError(t, err)
	assert.True(t, info.IsDir())

	// Verify scratchpad is empty
	content, err := ReadScratchpad(dir)
	require.NoError(t, err)
	assert.Empty(t, content)

	// Verify build_start event emitted
	events, err := ReadEvents(dir)
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, EventBuildStart, events[0].Type)
	assert.Equal(t, "TestEpic", events[0].Data["epic"])
	assert.Equal(t, "high", events[0].Data["effort"])
	assert.Equal(t, "3", events[0].Data["total_sprints"])
}

func TestInitBuild_ClearsStaleEvents(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Simulate a prior build that left events behind
	require.NoError(t, EmitEvent(dir, Event{Timestamp: "2025-01-01T00:00:00Z", Type: EventBuildStart}))
	require.NoError(t, EmitEvent(dir, Event{Timestamp: "2025-01-01T00:01:00Z", Type: EventSprintComplete, Sprint: 1}))
	staleEvents, err := ReadEvents(dir)
	require.NoError(t, err)
	require.Len(t, staleEvents, 2)

	// Init a new build — should clear stale events and emit fresh build_start
	err = InitBuild(dir, "NewEpic", "high", 2)
	require.NoError(t, err)

	events, err := ReadEvents(dir)
	require.NoError(t, err)
	require.Len(t, events, 1, "should only have the new build_start event")
	assert.Equal(t, EventBuildStart, events[0].Type)
	assert.Equal(t, "NewEpic", events[0].Data["epic"])
}

func TestInitBuild_PreservesIdentity(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Write a custom identity before init
	identityPath := filepath.Join(dir, config.ObserverIdentityFile)
	require.NoError(t, os.MkdirAll(filepath.Dir(identityPath), 0o755))
	customIdentity := "# Custom Identity\nI have been here before.\n"
	require.NoError(t, os.WriteFile(identityPath, []byte(customIdentity), 0o644))

	err := InitBuild(dir, "TestEpic", "high", 3)
	require.NoError(t, err)

	// Verify custom identity was preserved
	content, err := ReadIdentity(dir)
	require.NoError(t, err)
	assert.Equal(t, customIdentity, content)
}

// --- Scratchpad tests ---

func TestWriteAndReadScratchpad(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	err := WriteScratchpad(dir, "# Build Notes\nSprint 1 was interesting.\n")
	require.NoError(t, err)

	content, err := ReadScratchpad(dir)
	require.NoError(t, err)
	assert.Equal(t, "# Build Notes\nSprint 1 was interesting.\n", content)
}

func TestReadScratchpad_Missing(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	content, err := ReadScratchpad(dir)
	require.NoError(t, err)
	assert.Empty(t, content)
}

func TestAppendScratchpad(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	err := WriteScratchpad(dir, "line1\n")
	require.NoError(t, err)

	err = AppendScratchpad(dir, "line2\n")
	require.NoError(t, err)

	err = AppendScratchpad(dir, "line3\n")
	require.NoError(t, err)

	content, err := ReadScratchpad(dir)
	require.NoError(t, err)
	assert.Equal(t, "line1\nline2\nline3\n", content)
}

func TestAppendScratchpad_CreatesMissingFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	err := AppendScratchpad(dir, "first entry\n")
	require.NoError(t, err)

	content, err := ReadScratchpad(dir)
	require.NoError(t, err)
	assert.Equal(t, "first entry\n", content)
}

// --- WakeUp tests ---

func TestWakeUp_Success(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, InitBuild(dir, "TestEpic", "high", 3))

	eng := &stubObserverEngine{
		output: `<thoughts>
The build started cleanly. No issues observed in sprint 1.
</thoughts>

<scratchpad>
Sprint 1 completed in 3 iterations. Watch for test flakiness in sprint 2.
</scratchpad>`,
	}

	obs, err := WakeUp(context.Background(), ObserverOpts{
		ProjectDir:   dir,
		Engine:       eng,
		EpicName:     "TestEpic",
		WakePoint:    WakeAfterSprint,
		SprintNum:    1,
		TotalSprints: 3,
		EffortLevel:  epic.EffortHigh,
	})

	require.NoError(t, err)
	require.NotNil(t, obs)
	assert.Contains(t, obs.Thoughts, "build started cleanly")
	assert.Contains(t, obs.ScratchpadDelta, "Sprint 1 completed")
	assert.False(t, obs.IdentityEdited)
}

func TestWakeUp_UpdatesScratchpad(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, InitBuild(dir, "TestEpic", "high", 3))

	eng := &stubObserverEngine{
		output: `<thoughts>Observation 1.</thoughts>
<scratchpad>Note from wake 1.</scratchpad>`,
	}

	_, err := WakeUp(context.Background(), ObserverOpts{
		ProjectDir:   dir,
		Engine:       eng,
		EpicName:     "TestEpic",
		WakePoint:    WakeAfterSprint,
		SprintNum:    1,
		TotalSprints: 3,
		EffortLevel:  epic.EffortHigh,
	})
	require.NoError(t, err)

	content, err := ReadScratchpad(dir)
	require.NoError(t, err)
	assert.Contains(t, content, "Note from wake 1")
}

func TestWakeUp_UpdatesIdentity(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, InitBuild(dir, "TestEpic", "high", 3))

	newIdentity := "# Updated Observer Identity\nI have learned things.\n"
	eng := &stubObserverEngine{
		output: fmt.Sprintf(`<thoughts>Significant learning happened.</thoughts>
<scratchpad>Updating identity.</scratchpad>
<identity_update>%s</identity_update>`, newIdentity),
	}

	obs, err := WakeUp(context.Background(), ObserverOpts{
		ProjectDir:   dir,
		Engine:       eng,
		EpicName:     "TestEpic",
		WakePoint:    WakeBuildEnd,
		SprintNum:    3,
		TotalSprints: 3,
		EffortLevel:  epic.EffortHigh,
	})
	require.NoError(t, err)
	assert.True(t, obs.IdentityEdited)

	content, err := ReadIdentity(dir)
	require.NoError(t, err)
	assert.Contains(t, content, "Updated Observer Identity")
}

func TestWakeUp_SkipsIdentityWhenEmpty(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, InitBuild(dir, "TestEpic", "high", 3))

	// Get initial identity
	initialIdentity, err := ReadIdentity(dir)
	require.NoError(t, err)

	eng := &stubObserverEngine{
		output: `<thoughts>Nothing special.</thoughts>
<scratchpad>No notes.</scratchpad>
<identity_update>
</identity_update>`,
	}

	obs, err := WakeUp(context.Background(), ObserverOpts{
		ProjectDir:   dir,
		Engine:       eng,
		EpicName:     "TestEpic",
		WakePoint:    WakeAfterSprint,
		SprintNum:    1,
		TotalSprints: 3,
		EffortLevel:  epic.EffortHigh,
	})
	require.NoError(t, err)
	assert.False(t, obs.IdentityEdited)

	// Identity should be unchanged
	content, err := ReadIdentity(dir)
	require.NoError(t, err)
	assert.Equal(t, initialIdentity, content)
}

func TestWakeUp_EngineFailure(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, InitBuild(dir, "TestEpic", "high", 3))

	eng := &stubObserverEngine{
		output: "",
		err:    fmt.Errorf("engine crashed"),
	}

	obs, err := WakeUp(context.Background(), ObserverOpts{
		ProjectDir:   dir,
		Engine:       eng,
		EpicName:     "TestEpic",
		WakePoint:    WakeAfterSprint,
		SprintNum:    1,
		TotalSprints: 3,
		EffortLevel:  epic.EffortHigh,
	})

	// Should not return error for non-fatal engine failure
	require.NoError(t, err)
	require.NotNil(t, obs)
}

func TestWakeUp_ContextCancelled(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, InitBuild(dir, "TestEpic", "high", 3))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	eng := &stubObserverEngine{
		output: "",
		err:    ctx.Err(),
	}

	_, err := WakeUp(ctx, ObserverOpts{
		ProjectDir:   dir,
		Engine:       eng,
		EpicName:     "TestEpic",
		WakePoint:    WakeAfterSprint,
		SprintNum:    1,
		TotalSprints: 3,
		EffortLevel:  epic.EffortHigh,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "context canceled")
}

func TestWakeUp_NilEngine(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	_, err := WakeUp(context.Background(), ObserverOpts{
		ProjectDir:   dir,
		Engine:       nil,
		EpicName:     "TestEpic",
		WakePoint:    WakeAfterSprint,
		SprintNum:    1,
		TotalSprints: 3,
		EffortLevel:  epic.EffortHigh,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "engine is required")
}
