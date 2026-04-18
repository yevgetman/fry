package wake

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yevgetman/fry/internal/state"
	"github.com/yevgetman/fry/internal/wakelog"
)

func newTestMission(t *testing.T, promptBody string) (string, *state.Mission) {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "prompt.md"), []byte(promptBody), 0o644))
	m := &state.Mission{
		MissionID:       "testm",
		CreatedAt:       time.Now().UTC().Add(-30 * time.Minute),
		InputMode:       "prompt",
		Effort:          "fast",
		IntervalSeconds: 300,
		DurationHours:   1,
		OvertimeHours:   0,
		CurrentWake:     0,
		Status:          state.StatusActive,
	}
	return dir, m
}

func TestAssemble_MinimalPromptOnly(t *testing.T) {
	t.Parallel()
	dir, m := newTestMission(t, "# Minimal mission\nDo the thing.")

	out, err := Assemble(m, dir, 5, time.Now().UTC())
	require.NoError(t, err)

	// L0: wake contract always present.
	assert.Contains(t, out, "# Wake Contract")
	assert.Contains(t, out, PromiseToken, "preamble must name the promise token so agent knows what to emit")

	// L1: mission overview includes prompt body.
	assert.Contains(t, out, "# Mission Overview")
	assert.Contains(t, out, "Do the thing.")

	// L5: current-wake directive names wake number + PromiseToken.
	assert.Contains(t, out, "# Current Wake Directive")
	assert.Contains(t, out, "This is wake 1.")
}

func TestAssemble_IncludesPlanWhenPresent(t *testing.T) {
	t.Parallel()
	dir, m := newTestMission(t, "# Mission")

	require.NoError(t, os.WriteFile(filepath.Join(dir, "plan.md"), []byte("## Plan details\nStep 1."), 0o644))

	out, err := Assemble(m, dir, 5, time.Now().UTC())
	require.NoError(t, err)

	assert.Contains(t, out, "# Plan")
	assert.Contains(t, out, "Step 1.")
}

func TestAssemble_OmitsPlanSectionWhenAbsent(t *testing.T) {
	t.Parallel()
	dir, m := newTestMission(t, "# Mission")

	out, err := Assemble(m, dir, 5, time.Now().UTC())
	require.NoError(t, err)

	assert.NotContains(t, out, "# Plan\n", "plan header must be absent when plan.md is missing")
}

func TestAssemble_IncludesNotesSections(t *testing.T) {
	t.Parallel()
	dir, m := newTestMission(t, "# Mission")

	notesContent := `# Mission Notes — testm

## Current Focus
Building M4

## Next Wake Should
Start M5

## Decisions
2026-04-18T15:00Z (wake 2): chose mkdir for lock

## Open Questions
Is stale lock recovery needed?

## Supervisor Injections
focus on error paths
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "notes.md"), []byte(notesContent), 0o644))

	out, err := Assemble(m, dir, 5, time.Now().UTC())
	require.NoError(t, err)

	assert.Contains(t, out, "# Current Focus")
	assert.Contains(t, out, "Building M4")
	assert.Contains(t, out, "# Prior Wake Handoff")
	assert.Contains(t, out, "Start M5")
	assert.Contains(t, out, "# Supervisor Injections")
	assert.Contains(t, out, "focus on error paths")
	assert.Contains(t, out, "# Prior Decisions")
	assert.Contains(t, out, "chose mkdir for lock")
	assert.Contains(t, out, "# Open Questions")
	assert.Contains(t, out, "stale lock recovery")
}

func TestAssemble_IncludesRecentWakeLog(t *testing.T) {
	t.Parallel()
	dir, m := newTestMission(t, "# Mission")

	require.NoError(t, wakelog.Append(dir, wakelog.Entry{WakeNumber: 1, Phase: "building", WakeGoal: "did thing A", Blockers: []string{}}))
	require.NoError(t, wakelog.Append(dir, wakelog.Entry{WakeNumber: 2, Phase: "building", WakeGoal: "did thing B", Blockers: []string{}}))

	out, err := Assemble(m, dir, 5, time.Now().UTC())
	require.NoError(t, err)

	assert.Contains(t, out, "# Recent Wake Log")
	assert.Contains(t, out, "did thing A")
	assert.Contains(t, out, "did thing B")
}

func TestAssemble_MissingPromptIsError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir() // no prompt.md

	m := &state.Mission{MissionID: "x", CreatedAt: time.Now().UTC(), Status: state.StatusActive}
	_, err := Assemble(m, dir, 5, time.Now().UTC())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "prompt.md")
}

func TestAssemble_LayerOrderStableAcrossWakes(t *testing.T) {
	t.Parallel()
	// The layered structure (L0 wake contract → L1 mission → ...) must stay
	// constant for prompt cache hit rate. Two wakes with the same mission
	// content must share an identical prefix up through the L2 plan section.
	dir, m := newTestMission(t, "# Mission body that is stable.")

	// First wake — no notes, no log yet.
	out1, err := Assemble(m, dir, 5, time.Now().UTC())
	require.NoError(t, err)

	// Second wake — still no notes / log.
	m.CurrentWake = 1
	out2, err := Assemble(m, dir, 5, time.Now().UTC().Add(10*time.Minute))
	require.NoError(t, err)

	// L0+L1 should be byte-identical up to at least the end of "# Mission Overview" section.
	idx1 := indexEnd(out1, "Mission body that is stable.")
	idx2 := indexEnd(out2, "Mission body that is stable.")
	require.Greater(t, idx1, 0)
	require.Greater(t, idx2, 0)
	assert.Equal(t, out1[:idx1], out2[:idx2], "L0+L1 stable prefix must be byte-identical across wakes")
}

func indexEnd(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i + len(sub)
		}
	}
	return -1
}
