package sprint

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yevgetman/fry/internal/config"
)

func TestMechanicalCompactionNoIterationEntries(t *testing.T) {
	t.Parallel()

	input := "Some progress notes\nwith multiple lines\nbut no iteration headers"
	result := mechanicalCompaction(input)
	assert.Equal(t, strings.TrimSpace(input), result)
}

func TestMechanicalCompactionEmptyString(t *testing.T) {
	t.Parallel()

	result := mechanicalCompaction("")
	assert.Equal(t, "", result)
}

func TestMechanicalCompactionSingleIterationEntry(t *testing.T) {
	t.Parallel()

	input := "Preamble text\n\n## Iteration 1 — 2026-01-01\nDid some work\nMore notes"
	result := mechanicalCompaction(input)
	assert.Equal(t, "## Iteration 1 — 2026-01-01\nDid some work\nMore notes", result)
}

func TestMechanicalCompactionMultipleIterationEntries(t *testing.T) {
	t.Parallel()

	// Should return content from the LAST Iteration/Heal entry onward.
	input := "## Iteration 1 — 2026-01-01\nFirst iteration\n\n## Iteration 2 — 2026-01-02\nSecond iteration\n\n## Iteration 3 — 2026-01-03\nThird iteration"
	result := mechanicalCompaction(input)
	assert.Equal(t, "## Iteration 3 — 2026-01-03\nThird iteration", result)
}

func TestMechanicalCompactionMixedIterationAndHealEntries(t *testing.T) {
	t.Parallel()

	input := "## Iteration 1 — 2026-01-01\nFirst\n\n--- Heal attempt 1\nHealing\n\n--- Heal attempt 2\nMore healing"
	result := mechanicalCompaction(input)
	assert.Equal(t, "--- Heal attempt 2\nMore healing", result)
}

func TestMechanicalCompactionLastEntryIsHeal(t *testing.T) {
	t.Parallel()

	input := "## Iteration 1 — 2026-01-01\nIteration done\n\n--- Heal attempt 1\nHeal content"
	result := mechanicalCompaction(input)
	assert.Equal(t, "--- Heal attempt 1\nHeal content", result)
}

func TestCompactSprintProgressNonAgentMode(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	fryDir := filepath.Join(dir, ".fry")
	require.NoError(t, os.MkdirAll(fryDir, 0o755))

	progressContent := "## Iteration 1 — 2026-01-01\nCompleted task A\nCompleted task B"
	require.NoError(t, os.WriteFile(filepath.Join(dir, config.SprintProgressFile), []byte(progressContent), 0o644))

	result, err := CompactSprintProgress(context.Background(), dir, 1, "Foundation", "PASSED", nil, false, "")
	require.NoError(t, err)

	assert.Contains(t, result, "## Sprint 1: Foundation — PASSED")
	assert.Contains(t, result, "## Iteration 1 — 2026-01-01")
	assert.Contains(t, result, "Completed task A")
}

func TestCompactSprintProgressNonAgentFormatPrefix(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	fryDir := filepath.Join(dir, ".fry")
	require.NoError(t, os.MkdirAll(fryDir, 0o755))

	require.NoError(t, os.WriteFile(filepath.Join(dir, config.SprintProgressFile), []byte("some notes"), 0o644))

	result, err := CompactSprintProgress(context.Background(), dir, 3, "Finish", "SKIPPED", nil, false, "")
	require.NoError(t, err)

	assert.True(t, strings.HasPrefix(result, "## Sprint 3: Finish — SKIPPED\n\n"))
	assert.True(t, strings.HasSuffix(result, "\n"))
}
