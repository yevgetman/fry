package git

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScanWorktreeBuilds_NoWorktreeDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	summaries, err := ScanWorktreeBuilds(dir)
	require.NoError(t, err)
	assert.Nil(t, summaries)
}

func TestScanWorktreeBuilds_EmptyWorktreeDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".fry-worktrees"), 0o755))

	summaries, err := ScanWorktreeBuilds(dir)
	require.NoError(t, err)
	assert.Empty(t, summaries)
}

func TestScanWorktreeBuilds_WithBuild(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	wtFry := filepath.Join(dir, ".fry-worktrees", "my-epic", ".fry")
	require.NoError(t, os.MkdirAll(wtFry, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(wtFry, "epic.md"),
		[]byte("@epic My Worktree Epic\n@sprint Setup\n@sprint Build\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(wtFry, "epic-progress.txt"),
		[]byte("## Sprint 1: Setup \u2014 PASS\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(wtFry, "build-mode.txt"),
		[]byte("software"), 0o644))

	summaries, err := ScanWorktreeBuilds(dir)
	require.NoError(t, err)
	require.Len(t, summaries, 1)

	assert.Equal(t, "My Worktree Epic", summaries[0].EpicName)
	assert.Equal(t, 2, summaries[0].TotalSprints)
	assert.Equal(t, 1, summaries[0].CompletedCount)
	assert.Equal(t, 0, summaries[0].FailedCount)
	assert.Equal(t, ".fry-worktrees/my-epic", summaries[0].Dir)
}

func TestScanWorktreeBuilds_NoEpicInWorktree(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// Worktree directory exists but has no .fry/
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".fry-worktrees", "some-wt"), 0o755))

	summaries, err := ScanWorktreeBuilds(dir)
	require.NoError(t, err)
	assert.Empty(t, summaries)
}

func TestScanWorktreeBuilds_MultipleWorktrees(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Worktree with build
	wtFry1 := filepath.Join(dir, ".fry-worktrees", "alpha", ".fry")
	require.NoError(t, os.MkdirAll(wtFry1, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(wtFry1, "epic.md"),
		[]byte("@epic Alpha\n@sprint S1\n"), 0o644))

	// Worktree without .fry/ (skipped)
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".fry-worktrees", "beta"), 0o755))

	// Another worktree with build
	wtFry3 := filepath.Join(dir, ".fry-worktrees", "gamma", ".fry")
	require.NoError(t, os.MkdirAll(wtFry3, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(wtFry3, "epic.md"),
		[]byte("@epic Gamma\n@sprint S1\n@sprint S2\n"), 0o644))

	summaries, err := ScanWorktreeBuilds(dir)
	require.NoError(t, err)
	require.Len(t, summaries, 2)

	// Order depends on ReadDir (alphabetical)
	assert.Equal(t, "Alpha", summaries[0].EpicName)
	assert.Equal(t, "Gamma", summaries[1].EpicName)
}
