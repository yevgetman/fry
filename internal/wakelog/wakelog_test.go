package wakelog

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAppend_CreatesAndAppends(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	e1 := Entry{WakeNumber: 1, TimestampUTC: "2026-04-18T10:00:00Z", Phase: "building", WakeGoal: "first", Blockers: []string{}}
	e2 := Entry{WakeNumber: 2, TimestampUTC: "2026-04-18T10:10:00Z", Phase: "building", WakeGoal: "second", Blockers: []string{}}

	require.NoError(t, Append(dir, e1))
	require.NoError(t, Append(dir, e2))

	data, err := os.ReadFile(filepath.Join(dir, "wake_log.jsonl"))
	require.NoError(t, err)
	// Exactly two newline-terminated JSON lines.
	assert.Equal(t, 2, countLines(data))
	assert.Contains(t, string(data), `"wake_number":1`)
	assert.Contains(t, string(data), `"wake_number":2`)
}

func TestAppend_MarshalsAllFields(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	e := Entry{
		WakeNumber:        7,
		TimestampUTC:      "2026-04-18T10:00:00Z",
		ElapsedHours:      2.5,
		Phase:             "building",
		CurrentMilestone:  "M5",
		WakeGoal:          "test",
		ActionsTaken:      []string{"a", "b"},
		ArtifactsTouched:  []string{"x.go"},
		Blockers:          []string{},
		NextWakePlan:      "next",
		SelfAssessment:    "fine",
		PromiseTokenFound: true,
		ExitCode:          0,
		WallClockSeconds:  120,
		CostUSD:           0.05,
		Overtime:          false,
	}
	require.NoError(t, Append(dir, e))

	got, err := TailN(dir, 10)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, e, got[0])
}

func TestTailN_EmptyFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// No wake_log.jsonl at all.
	entries, err := TailN(dir, 5)
	require.NoError(t, err)
	assert.Nil(t, entries)
}

func TestTailN_FewerThanN(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for i := 1; i <= 3; i++ {
		require.NoError(t, Append(dir, Entry{WakeNumber: i, Phase: "building", Blockers: []string{}}))
	}
	entries, err := TailN(dir, 10)
	require.NoError(t, err)
	require.Len(t, entries, 3)
	assert.Equal(t, 1, entries[0].WakeNumber)
	assert.Equal(t, 3, entries[2].WakeNumber)
}

func TestTailN_MoreThanN(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for i := 1; i <= 10; i++ {
		require.NoError(t, Append(dir, Entry{WakeNumber: i, Phase: "building", Blockers: []string{}}))
	}
	entries, err := TailN(dir, 3)
	require.NoError(t, err)
	require.Len(t, entries, 3)
	// Should be the LAST three.
	assert.Equal(t, 8, entries[0].WakeNumber)
	assert.Equal(t, 9, entries[1].WakeNumber)
	assert.Equal(t, 10, entries[2].WakeNumber)
}

func TestTailN_SkipsMalformedLines(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "wake_log.jsonl")
	// Intentionally write a malformed line in the middle.
	content := `{"wake_number":1,"phase":"a","blockers":[]}
this is not json
{"wake_number":2,"phase":"b","blockers":[]}
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	entries, err := TailN(dir, 10)
	require.NoError(t, err)
	require.Len(t, entries, 2)
	assert.Equal(t, 1, entries[0].WakeNumber)
	assert.Equal(t, 2, entries[1].WakeNumber)
}

func TestTailN_TrailingNewlineHandled(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, Append(dir, Entry{WakeNumber: 1, Phase: "x", Blockers: []string{}}))

	// Append writes a trailing newline; TailN should not return an empty extra entry.
	entries, err := TailN(dir, 10)
	require.NoError(t, err)
	require.Len(t, entries, 1)
}

func countLines(b []byte) int {
	n := 0
	for _, c := range b {
		if c == '\n' {
			n++
		}
	}
	return n
}
