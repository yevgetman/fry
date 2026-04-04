package cli

import (
	"bytes"
	"encoding/json"
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
		require.NoError(t, os.MkdirAll(filepath.Join(dir, config.FryConfigDir), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, config.FileIndexFile), []byte("index"), 0o644))
		assert.False(t, codebaseIndexExists(dir))
	})

	t.Run("only codebase.md", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(dir, config.FryConfigDir), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, config.CodebaseFile), []byte("codebase"), 0o644))
		assert.False(t, codebaseIndexExists(dir))
	})
}

func TestCodebaseIndexExistsReturnsTrueWhenBothExist(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, config.FryConfigDir), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, config.FileIndexFile), []byte("index"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, config.CodebaseFile), []byte("codebase"), 0o644))
	assert.True(t, codebaseIndexExists(dir))
}

func TestInitFryConfig_CreatesConfigDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	err := initFryConfig(dir)
	require.NoError(t, err)

	// .fry-config/ should exist
	info, err := os.Stat(filepath.Join(dir, config.FryConfigDir))
	require.NoError(t, err)
	assert.True(t, info.IsDir())

	// config.json should exist with default engine
	data, err := os.ReadFile(filepath.Join(dir, config.ProjectConfigFile))
	require.NoError(t, err)

	var raw map[string]string
	require.NoError(t, json.Unmarshal(data, &raw))
	assert.Equal(t, config.DefaultEngine, raw["engine"])
}

func TestInitFryConfig_DoesNotOverwrite(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, config.FryConfigDir), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, config.ProjectConfigFile), []byte(`{"engine":"codex"}`), 0o644))

	err := initFryConfig(dir)
	require.NoError(t, err)

	// config.json should NOT be overwritten
	data, err := os.ReadFile(filepath.Join(dir, config.ProjectConfigFile))
	require.NoError(t, err)
	assert.Contains(t, string(data), "codex")
}

func TestMigrateFromLegacyPaths_MovesFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, config.FryDir), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, config.FryConfigDir), 0o755))

	// Place legacy files under .fry/
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".fry/codebase.md"), []byte("old codebase"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".fry/file-index.txt"), []byte("old index"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".fry/config.json"), []byte(`{"engine":"codex"}`), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".fry/codebase-memories"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".fry/codebase-memories/001.md"), []byte("memory"), 0o644))

	cmd := newTestCmd(t, dir)
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	migrateFromLegacyPaths(dir, cmd)

	// Files should now be at new paths
	data, err := os.ReadFile(filepath.Join(dir, config.CodebaseFile))
	require.NoError(t, err)
	assert.Equal(t, "old codebase", string(data))

	data, err = os.ReadFile(filepath.Join(dir, config.FileIndexFile))
	require.NoError(t, err)
	assert.Equal(t, "old index", string(data))

	data, err = os.ReadFile(filepath.Join(dir, config.ProjectConfigFile))
	require.NoError(t, err)
	assert.Contains(t, string(data), "codex")

	data, err = os.ReadFile(filepath.Join(dir, config.CodebaseMemoriesDir, "001.md"))
	require.NoError(t, err)
	assert.Equal(t, "memory", string(data))

	// Old paths should be gone
	assert.False(t, fileExists(filepath.Join(dir, ".fry/codebase.md")))
	assert.False(t, fileExists(filepath.Join(dir, ".fry/file-index.txt")))
	assert.False(t, fileExists(filepath.Join(dir, ".fry/config.json")))
	assert.False(t, fileExists(filepath.Join(dir, ".fry/codebase-memories")))

	assert.Contains(t, buf.String(), "migrated")
}

func TestMigrateFromLegacyPaths_SkipsWhenDestExists(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, config.FryDir), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, config.FryConfigDir), 0o755))

	// Place legacy and new files
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".fry/codebase.md"), []byte("old"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, config.CodebaseFile), []byte("new"), 0o644))

	cmd := newTestCmd(t, dir)
	migrateFromLegacyPaths(dir, cmd)

	// New file should be untouched
	data, err := os.ReadFile(filepath.Join(dir, config.CodebaseFile))
	require.NoError(t, err)
	assert.Equal(t, "new", string(data))

	// Old file should still exist (not deleted since dest exists)
	assert.True(t, fileExists(filepath.Join(dir, ".fry/codebase.md")))
}

func TestMigrateFromLegacyPaths_SkipsWhenSourceMissing(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, config.FryConfigDir), 0o755))

	cmd := newTestCmd(t, dir)
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	migrateFromLegacyPaths(dir, cmd)

	// Nothing should be created
	assert.False(t, fileExists(filepath.Join(dir, config.CodebaseFile)))
	assert.NotContains(t, buf.String(), "migrated")
}
