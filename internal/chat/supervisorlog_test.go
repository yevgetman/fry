package chat

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAppendSupervisorLog_CreatesFileOnFirstCall(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	err := AppendSupervisorLog(dir, "query", "hello", nil)
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(dir, "supervisor_log.jsonl"))
	require.NoError(t, err)
	assert.NotEmpty(t, data)
}

func TestAppendSupervisorLog_RecordsAllFields(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	err := AppendSupervisorLog(dir, "intervention", "nudged focus to error paths", []string{"notes.md"})
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(dir, "supervisor_log.jsonl"))
	require.NoError(t, err)

	// Exactly one JSONL line.
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	require.Len(t, lines, 1)

	var e SupervisorEntry
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &e))
	assert.Equal(t, "intervention", e.Type)
	assert.Equal(t, "nudged focus to error paths", e.Summary)
	assert.Equal(t, []string{"notes.md"}, e.FieldsChanged)
	assert.Equal(t, "chat", e.Operator)
	assert.NotEmpty(t, e.TimestampUTC)
}

func TestAppendSupervisorLog_AppendsOnSubsequentCalls(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	require.NoError(t, AppendSupervisorLog(dir, "query", "one", nil))
	require.NoError(t, AppendSupervisorLog(dir, "query", "two", nil))
	require.NoError(t, AppendSupervisorLog(dir, "query", "three", nil))

	data, err := os.ReadFile(filepath.Join(dir, "supervisor_log.jsonl"))
	require.NoError(t, err)

	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	require.Len(t, lines, 3)

	for i, expected := range []string{"one", "two", "three"} {
		var e SupervisorEntry
		require.NoError(t, json.Unmarshal([]byte(lines[i]), &e))
		assert.Equal(t, expected, e.Summary)
	}
}

func TestAppendSupervisorLog_NilFieldsChangedIsOk(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	err := AppendSupervisorLog(dir, "query", "no field edits", nil)
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(dir, "supervisor_log.jsonl"))
	require.NoError(t, err)

	var e SupervisorEntry
	require.NoError(t, json.Unmarshal(data[:len(data)-1], &e)) // strip trailing newline
	assert.Nil(t, e.FieldsChanged)
}

func TestAppendSupervisorLog_ErrorOnUnwritableDir(t *testing.T) {
	t.Parallel()
	// A path whose parent doesn't exist should fail the OpenFile call.
	err := AppendSupervisorLog("/nonexistent/deeply/missing/path", "query", "x", nil)
	require.Error(t, err)
}
