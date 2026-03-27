package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestCmd(t *testing.T, projectDir string) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{}
	cmd.Flags().String("project-dir", projectDir, "")
	cmd.SetContext(context.Background())
	return cmd
}

func TestStatusCommandNoBuild(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	var buf bytes.Buffer
	fakeCmd := newTestCmd(t, dir)
	fakeCmd.SetOut(&buf)

	err := statusCmd.RunE(fakeCmd, []string{})
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "No active build")
}

func TestStatusCommandWithBuild(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	fryDir := filepath.Join(dir, ".fry")
	require.NoError(t, os.MkdirAll(fryDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(fryDir, "epic.md"), []byte("@epic Test Epic\n"), 0o644))

	var buf bytes.Buffer
	fakeCmd := newTestCmd(t, dir)
	fakeCmd.SetOut(&buf)

	err := statusCmd.RunE(fakeCmd, []string{})
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "Test Epic")
}

func TestStatusCommandNoBuild_WithArchives(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Create an archived build
	archiveDir := filepath.Join(dir, ".fry-archive", ".fry--build--20260327-120000")
	require.NoError(t, os.MkdirAll(archiveDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(archiveDir, "epic.md"),
		[]byte("@epic Archived Epic\n@sprint Setup\n@sprint Build\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(archiveDir, "epic-progress.txt"),
		[]byte("## Sprint 1: Setup \u2014 PASS\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(archiveDir, "build-mode.txt"),
		[]byte("software"), 0o644))

	var buf bytes.Buffer
	fakeCmd := newTestCmd(t, dir)
	fakeCmd.SetOut(&buf)

	err := statusCmd.RunE(fakeCmd, []string{})
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "No active build found")
	assert.Contains(t, output, "Archived Builds (1)")
	assert.Contains(t, output, "Archived Epic")
	assert.Contains(t, output, "1/2 sprints passed")
}

func TestStatusCommandNoBuild_WithWorktrees(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Create a worktree with a build
	wtFry := filepath.Join(dir, ".fry-worktrees", "my-epic", ".fry")
	require.NoError(t, os.MkdirAll(wtFry, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(wtFry, "epic.md"),
		[]byte("@epic Worktree Epic\n@sprint S1\n@sprint S2\n@sprint S3\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(wtFry, "build-mode.txt"),
		[]byte("software"), 0o644))

	var buf bytes.Buffer
	fakeCmd := newTestCmd(t, dir)
	fakeCmd.SetOut(&buf)

	err := statusCmd.RunE(fakeCmd, []string{})
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "No active build found")
	assert.Contains(t, output, "Worktree Builds (1)")
	assert.Contains(t, output, "Worktree Epic")
	assert.Contains(t, output, ".fry-worktrees/my-epic/")
}

func TestStatusCommandNoBuild_Empty(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	var buf bytes.Buffer
	fakeCmd := newTestCmd(t, dir)
	fakeCmd.SetOut(&buf)

	err := statusCmd.RunE(fakeCmd, []string{})
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "No active build found")
	assert.Contains(t, output, "Run 'fry run' to start a build.")
	assert.NotContains(t, output, "Archived")
	assert.NotContains(t, output, "Worktree")
}
