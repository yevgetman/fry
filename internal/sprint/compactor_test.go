package sprint

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMechanicalCompactionCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "no iteration or heal markers",
			input:    "Some progress\nMore progress\nDone",
			expected: "Some progress\nMore progress\nDone",
		},
		{
			name:     "one iteration entry",
			input:    "Header\n## Iteration 1\nsome work done\nresult: pass",
			expected: "## Iteration 1\nsome work done\nresult: pass",
		},
		{
			name:     "multiple iteration entries",
			input:    "## Iteration 1\nfirst work\n## Iteration 2\nsecond work\nmore lines",
			expected: "## Iteration 2\nsecond work\nmore lines",
		},
		{
			name:     "mix of iteration and heal attempt entries",
			input:    "## Iteration 1\nfirst\n--- Heal attempt 1\nheal work\n## Iteration 2\nsecond",
			expected: "## Iteration 2\nsecond",
		},
		{
			name:     "last entry is heal attempt",
			input:    "## Iteration 1\nfirst work\n--- Heal attempt 1\nheal lines\nmore heal",
			expected: "--- Heal attempt 1\nheal lines\nmore heal",
		},
		{
			name:     "entry at very last line",
			input:    "Some content\n## Iteration 1",
			expected: "## Iteration 1",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := mechanicalCompaction(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCompactSprintProgressWithIteration(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	fryDir := filepath.Join(dir, ".fry")
	require.NoError(t, os.MkdirAll(fryDir, 0o755))
	progressContent := "## Iteration 1\nDid some work\nResult: pass\n"
	require.NoError(t, os.WriteFile(filepath.Join(fryDir, "sprint-progress.txt"), []byte(progressContent), 0o644))

	result, err := CompactSprintProgress(context.Background(), dir, 1, "TestSprint", "COMPLETE", nil, false, "")
	require.NoError(t, err)
	assert.Contains(t, result, "## Sprint 1: TestSprint — COMPLETE")
	assert.Contains(t, result, "## Iteration 1")
}

func TestCompactSprintProgressNoMarkers(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	fryDir := filepath.Join(dir, ".fry")
	require.NoError(t, os.MkdirAll(fryDir, 0o755))
	progressContent := "Some progress notes\nAll done\n"
	require.NoError(t, os.WriteFile(filepath.Join(fryDir, "sprint-progress.txt"), []byte(progressContent), 0o644))

	result, err := CompactSprintProgress(context.Background(), dir, 1, "TestSprint", "COMPLETE", nil, false, "")
	require.NoError(t, err)
	assert.Contains(t, result, "Some progress notes")
	assert.Contains(t, result, "All done")
}

func TestCompactSprintProgressMissingFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// No .fry/sprint-progress.txt created; readFile treats missing file as empty.

	result, err := CompactSprintProgress(context.Background(), dir, 1, "TestSprint", "COMPLETE", nil, false, "")
	require.NoError(t, err)
	assert.Contains(t, result, "## Sprint 1: TestSprint — COMPLETE")
}
