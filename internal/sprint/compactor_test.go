package sprint

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yevgetman/fry/internal/config"
	"github.com/yevgetman/fry/internal/engine"
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
			name:     "no iteration or alignment markers",
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
			name:     "mix of iteration and alignment attempt entries",
			input:    "## Iteration 1\nfirst\n--- Alignment attempt 1\nheal work\n## Iteration 2\nsecond",
			expected: "## Iteration 2\nsecond",
		},
		{
			name:     "last entry is alignment attempt",
			input:    "## Iteration 1\nfirst work\n--- Alignment attempt 1\nheal lines\nmore heal",
			expected: "--- Alignment attempt 1\nheal lines\nmore heal",
		},
		{
			name:     "entry at very last line",
			input:    "Some content\n## Iteration 1",
			expected: "## Iteration 1",
		},
		{
			name:     "iteration with date suffix",
			input:    "Preamble text\n\n## Iteration 1 — 2026-01-01\nDid some work\nMore notes",
			expected: "## Iteration 1 — 2026-01-01\nDid some work\nMore notes",
		},
		{
			name:     "multiple dated iterations",
			input:    "## Iteration 1 — 2026-01-01\nFirst\n\n## Iteration 2 — 2026-01-02\nSecond\n\n## Iteration 3 — 2026-01-03\nThird",
			expected: "## Iteration 3 — 2026-01-03\nThird",
		},
		{
			name:     "multiple alignment attempts",
			input:    "## Iteration 1 — 2026-01-01\nFirst\n\n--- Alignment attempt 1\nAligning\n\n--- Alignment attempt 2\nMore aligning",
			expected: "--- Alignment attempt 2\nMore aligning",
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
	require.NoError(t, os.MkdirAll(filepath.Join(dir, config.FryDir), 0o755))
	progressContent := "## Iteration 1 — 2026-01-01\nCompleted task A\nCompleted task B"
	require.NoError(t, os.WriteFile(filepath.Join(dir, config.SprintProgressFile), []byte(progressContent), 0o644))

	result, err := CompactSprintProgress(context.Background(), dir, 1, "Foundation", "PASSED", nil, false, "")
	require.NoError(t, err)
	assert.Contains(t, result, "## Sprint 1: Foundation — PASSED")
	assert.Contains(t, result, "## Iteration 1")
	assert.Contains(t, result, "Completed task A")
}

func TestCompactSprintProgressNonAgentFormatPrefix(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, config.FryDir), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, config.SprintProgressFile), []byte("some notes"), 0o644))

	result, err := CompactSprintProgress(context.Background(), dir, 3, "Finish", "SKIPPED", nil, false, "")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(result, "## Sprint 3: Finish — SKIPPED\n\n"))
}

func TestCompactSprintProgressNoMarkers(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, config.FryDir), 0o755))
	progressContent := "Some progress notes\nAll done\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, config.SprintProgressFile), []byte(progressContent), 0o644))

	result, err := CompactSprintProgress(context.Background(), dir, 1, "TestSprint", "COMPLETE", nil, false, "")
	require.NoError(t, err)
	assert.Contains(t, result, "Some progress notes")
	assert.Contains(t, result, "All done")
}

func TestCompactSprintProgressMissingFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	result, err := CompactSprintProgress(context.Background(), dir, 1, "TestSprint", "COMPLETE", nil, false, "")
	require.NoError(t, err)
	assert.Contains(t, result, "## Sprint 1: TestSprint — COMPLETE")
}

func TestCompactSprintProgressAgentNilEngine(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, config.FryDir), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, config.SprintProgressFile), []byte("## Iteration 1\nsome work"), 0o644))

	_, err := CompactSprintProgress(context.Background(), dir, 1, "Sprint", "PASS", nil, true, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "engine is required")
}

func TestCompactSprintProgressWithAgent(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, config.FryDir), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, config.SprintProgressFile), []byte("## Iteration 1\nwork done"), 0o644))

	stub := &stubEngine{name: "stub", outputs: []string{"Agent summarized: work done"}}
	result, err := CompactSprintProgress(context.Background(), dir, 2, "API", "PASSED", stub, true, "gpt-4")
	require.NoError(t, err)
	assert.Contains(t, result, "## Sprint 2: API — PASSED")
	assert.Contains(t, result, "Agent summarized: work done")
}

type failingCompactionEngine struct {
	name     string
	output   string
	exitCode int
	err      error
}

func (f *failingCompactionEngine) Run(_ context.Context, _ string, _ engine.RunOpts) (string, int, error) {
	return f.output, f.exitCode, f.err
}

func (f *failingCompactionEngine) Name() string {
	return f.name
}

func TestCompactSprintProgressWithAgentIncludesFailureDetails(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, config.FryDir), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, config.SprintProgressFile), []byte("## Iteration 1\nwork done"), 0o644))

	_, err := CompactSprintProgress(context.Background(), dir, 2, "API", "PASSED", &failingCompactionEngine{
		name:     "claude",
		output:   "authentication expired\nplease run claude login",
		exitCode: 1,
		err:      errors.New("exit status 1"),
	}, true, "sonnet")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "compact sprint progress with agent: engine=claude model=sonnet exit_code=1")
	assert.Contains(t, err.Error(), "authentication expired")
}
