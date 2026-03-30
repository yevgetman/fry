package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yevgetman/fry/internal/config"
)

func TestDestroyTargetsEmpty(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	targets := destroyTargets(dir)
	assert.Empty(t, targets, "empty directory should have no destroy targets")
}

func TestDestroyTargetsFindsAllArtifacts(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Create all possible fry artifacts.
	dirs := []string{
		config.FryDir,
		config.ArchiveDir,
		config.GitWorktreeDir,
		config.PlansDir,
		config.AssetsDir,
		config.MediaDir,
	}
	for _, d := range dirs {
		require.NoError(t, os.MkdirAll(filepath.Join(dir, d), 0o755))
	}

	files := []string{
		config.BuildAuditFile,
		config.SummaryFile,
		config.BuildAuditSARIFFile,
	}
	for _, f := range files {
		require.NoError(t, os.WriteFile(filepath.Join(dir, f), []byte("x"), 0o644))
	}

	targets := destroyTargets(dir)
	assert.Len(t, targets, len(dirs)+len(files), "should find all fry artifacts")
}

func TestDestroyTargetsPartialArtifacts(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Only create .fry/ and plans/
	require.NoError(t, os.MkdirAll(filepath.Join(dir, config.FryDir), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, config.PlansDir), 0o755))

	targets := destroyTargets(dir)
	assert.Len(t, targets, 2)
	assert.Contains(t, targets, filepath.Join(dir, config.FryDir))
	assert.Contains(t, targets, filepath.Join(dir, config.PlansDir))
}

func TestDestroyTargetsIncludesNestedContent(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Create .fry/ with nested files (they should all be destroyed via RemoveAll).
	fryDir := filepath.Join(dir, config.FryDir)
	require.NoError(t, os.MkdirAll(filepath.Join(fryDir, "build-logs"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(fryDir, "codebase.md"), []byte("x"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(fryDir, "build-logs", "sprint1.log"), []byte("x"), 0o644))

	targets := destroyTargets(dir)
	require.Len(t, targets, 1)
	assert.Equal(t, fryDir, targets[0])

	// Verify RemoveAll would clean nested content.
	require.NoError(t, os.RemoveAll(fryDir))
	_, err := os.Stat(fryDir)
	assert.True(t, os.IsNotExist(err))
}
