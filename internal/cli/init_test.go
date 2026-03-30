package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yevgetman/fry/internal/config"
)

func TestScaffoldProject(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	created, err := scaffoldProject(dir)
	require.NoError(t, err)

	// plans/, assets/, media/ directories should exist
	for _, sub := range []string{config.PlansDir, config.AssetsDir, config.MediaDir} {
		info, err := os.Stat(filepath.Join(dir, sub))
		require.NoError(t, err, "directory %s should exist", sub)
		assert.True(t, info.IsDir())
	}

	// plan.example.md should exist with template content
	examplePath := filepath.Join(dir, config.PlansDir, "plan.example.md")
	data, err := os.ReadFile(examplePath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "# Build Plan")
	assert.Contains(t, string(data), "## Goals")

	// Should have created 3 dirs + 1 file
	assert.Len(t, created, 4)
}

func TestScaffoldProjectSkipsExistingDirs(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Pre-create all directories
	for _, sub := range []string{config.PlansDir, config.AssetsDir, config.MediaDir} {
		require.NoError(t, os.MkdirAll(filepath.Join(dir, sub), 0o755))
	}

	created, err := scaffoldProject(dir)
	require.NoError(t, err)

	// Only plan.example.md should be in created (dirs already existed)
	assert.Len(t, created, 1)
	assert.Equal(t, filepath.Join(dir, config.PlansDir, "plan.example.md"), created[0])
}

func TestScaffoldProjectDoesNotCreatePlanMd(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	_, err := scaffoldProject(dir)
	require.NoError(t, err)

	// plan.md should NOT be created by init
	_, err = os.Stat(filepath.Join(dir, config.PlanFile))
	assert.True(t, os.IsNotExist(err), "plan.md should not be created by init")

	// plan.example.md SHOULD exist
	_, err = os.Stat(filepath.Join(dir, config.PlansDir, "plan.example.md"))
	assert.NoError(t, err, "plan.example.md should exist")
}

func TestScaffoldProjectIdempotent(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// First run creates dirs + example
	created1, err := scaffoldProject(dir)
	require.NoError(t, err)
	assert.Len(t, created1, 4)

	// Second run only rewrites example (dirs already exist)
	created2, err := scaffoldProject(dir)
	require.NoError(t, err)
	assert.Len(t, created2, 1)
	assert.Equal(t, filepath.Join(dir, config.PlansDir, "plan.example.md"), created2[0])
}

func TestCodebaseIndexExistsReturnsFalseWhenMissing(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	assert.False(t, codebaseIndexExists(dir), "should be false when neither file exists")
}

func TestCodebaseIndexExistsReturnsFalseWithPartialIndex(t *testing.T) {
	t.Parallel()

	t.Run("only file-index.txt", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(dir, config.FryDir), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, config.FileIndexFile), []byte("index"), 0o644))
		assert.False(t, codebaseIndexExists(dir))
	})

	t.Run("only codebase.md", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(dir, config.FryDir), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, config.CodebaseFile), []byte("codebase"), 0o644))
		assert.False(t, codebaseIndexExists(dir))
	})
}

func TestCodebaseIndexExistsReturnsTrueWhenBothExist(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, config.FryDir), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, config.FileIndexFile), []byte("index"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, config.CodebaseFile), []byte("codebase"), 0o644))
	assert.True(t, codebaseIndexExists(dir))
}
