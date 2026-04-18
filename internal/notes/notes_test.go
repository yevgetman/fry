package notes

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const sampleNotes = `# Mission Notes — testmission

## Current Focus
Building M4 prompt assembly

## Next Wake Should
Implement M5 chat session

## Decisions
<!-- Format: YYYY-MM-DDTHH:MMZ (wake N): <decision> -->
2026-04-18T15:00Z (wake 2): chose mkdir for lock

## Open Questions
<!-- Add open questions here as they arise -->
Is stale lock recovery needed in M2?

## Supervisor Injections
<!-- Chat sessions append here; wakes read and honor. -->
`

func TestParse(t *testing.T) {
	n := parse(sampleNotes)
	assert.Equal(t, "testmission", n.MissionID)
	assert.Equal(t, "Building M4 prompt assembly", n.CurrentFocus)
	assert.Equal(t, "Implement M5 chat session", n.NextWakeShould)
	require.Len(t, n.Decisions, 1)
	assert.Contains(t, n.Decisions[0], "chose mkdir for lock")
	require.Len(t, n.OpenQuestions, 1)
	assert.Contains(t, n.OpenQuestions[0], "stale lock")
	assert.Empty(t, n.SupervisorInjects)
}

func TestRenderRoundtrip(t *testing.T) {
	n := parse(sampleNotes)
	rendered := n.render()
	n2 := parse(rendered)

	assert.Equal(t, n.MissionID, n2.MissionID)
	assert.Equal(t, n.CurrentFocus, n2.CurrentFocus)
	assert.Equal(t, n.NextWakeShould, n2.NextWakeShould)
	assert.Equal(t, n.Decisions, n2.Decisions)
	assert.Equal(t, n.OpenQuestions, n2.OpenQuestions)
	assert.Equal(t, n.SupervisorInjects, n2.SupervisorInjects)
}

func TestSaveLoad(t *testing.T) {
	dir := t.TempDir()

	n := &Notes{
		MissionID:      "savemission",
		CurrentFocus:   "testing save/load",
		NextWakeShould: "verify it works",
		Decisions:      []string{"2026-04-18T15:00Z (wake 1): initial decision"},
	}
	require.NoError(t, n.Save(dir))

	loaded, err := Load(dir)
	require.NoError(t, err)
	assert.Equal(t, n.MissionID, loaded.MissionID)
	assert.Equal(t, n.CurrentFocus, loaded.CurrentFocus)
	assert.Equal(t, n.NextWakeShould, loaded.NextWakeShould)
	assert.Equal(t, n.Decisions, loaded.Decisions)
}

func TestLoadMissing(t *testing.T) {
	dir := t.TempDir()
	n, err := Load(dir)
	require.NoError(t, err)
	assert.NotNil(t, n)
	assert.Empty(t, n.CurrentFocus)
}

func TestAppendDecision(t *testing.T) {
	n := &Notes{MissionID: "m"}
	n.AppendDecision(3, "use atomic writes")
	require.Len(t, n.Decisions, 1)
	assert.True(t, strings.Contains(n.Decisions[0], "(wake 3)"))
	assert.True(t, strings.Contains(n.Decisions[0], "use atomic writes"))
}

func TestAppendInjection(t *testing.T) {
	n := &Notes{MissionID: "m"}
	n.AppendInjection("focus on error handling next wake")
	require.Len(t, n.SupervisorInjects, 1)
	assert.Contains(t, n.SupervisorInjects[0], "focus on error handling next wake")
}

func TestEmptyNotes(t *testing.T) {
	n := &Notes{MissionID: "empty"}
	rendered := n.render()
	n2 := parse(rendered)
	assert.Equal(t, "empty", n2.MissionID)
	assert.Equal(t, "<one sentence — what this wake is for>", n2.CurrentFocus)
}
