package scan

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsExistingProject_EmptyDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	assert.False(t, IsExistingProject(context.Background(), dir))
}

func TestIsExistingProject_WithGoMod(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n"), 0o644))
	assert.True(t, IsExistingProject(context.Background(), dir))
}

func TestIsExistingProject_WithPackageJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"test"}`), 0o644))
	assert.True(t, IsExistingProject(context.Background(), dir))
}

func TestIsExistingProject_WithCargoToml(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "Cargo.toml"), []byte("[package]\nname = \"test\"\n"), 0o644))
	assert.True(t, IsExistingProject(context.Background(), dir))
}

func TestIsExistingProject_WithRequirementsTxt(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "requirements.txt"), []byte("flask==2.0\n"), 0o644))
	assert.True(t, IsExistingProject(context.Background(), dir))
}

func TestIsExistingProject_WithManyFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// Create minNonDotFiles + 1 regular files (no markers).
	for i := 0; i <= minNonDotFiles; i++ {
		path := filepath.Join(dir, "file"+string(rune('a'+i))+".txt")
		require.NoError(t, os.WriteFile(path, []byte("content"), 0o644))
	}
	assert.True(t, IsExistingProject(context.Background(), dir))
}

func TestIsExistingProject_FewFilesNoMarkers(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// Create fewer than minNonDotFiles files.
	for i := 0; i < 3; i++ {
		path := filepath.Join(dir, "file"+string(rune('a'+i))+".txt")
		require.NoError(t, os.WriteFile(path, []byte("content"), 0o644))
	}
	assert.False(t, IsExistingProject(context.Background(), dir))
}

func TestIsExistingProject_OnlyHiddenFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// Hidden files should not count.
	for i := 0; i < 20; i++ {
		path := filepath.Join(dir, ".hidden"+string(rune('a'+i)))
		require.NoError(t, os.WriteFile(path, []byte("content"), 0o644))
	}
	assert.False(t, IsExistingProject(context.Background(), dir))
}

func TestIsExistingProject_OnlyDirectories(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// Directories should not count toward the file threshold.
	for i := 0; i < 20; i++ {
		require.NoError(t, os.MkdirAll(filepath.Join(dir, "dir"+string(rune('a'+i))), 0o755))
	}
	assert.False(t, IsExistingProject(context.Background(), dir))
}

func TestIsExistingProject_GitHistoryMultipleCommits(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	ctx := context.Background()

	// Initialize a git repo with 2 commits.
	run := func(args ...string) {
		cmd := exec.CommandContext(ctx, "git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		require.NoError(t, cmd.Run())
	}

	run("init")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o644))
	run("add", ".")
	run("commit", "-m", "first")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.txt"), []byte("b"), 0o644))
	run("add", ".")
	run("commit", "-m", "second")

	assert.True(t, IsExistingProject(ctx, dir))
}

func TestIsExistingProject_GitHistorySingleCommit(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	ctx := context.Background()

	run := func(args ...string) {
		cmd := exec.CommandContext(ctx, "git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		require.NoError(t, cmd.Run())
	}

	run("init")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o644))
	run("add", ".")
	run("commit", "-m", "initial")

	// Single commit should NOT qualify (could be fry init's own commit).
	assert.False(t, IsExistingProject(ctx, dir))
}

func TestHasProjectMarker_SLNGlob(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "MyApp.sln"), []byte("solution"), 0o644))
	assert.True(t, hasProjectMarker(dir))
}

func TestCountNonDotFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".hidden"), []byte("h"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "subdir"), 0o755))

	// Should count only "a.txt" — .hidden and subdir are excluded.
	assert.Equal(t, 1, countNonDotFiles(dir))
}
