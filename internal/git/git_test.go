package git

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitGit(t *testing.T) {
	projectDir := t.TempDir()
	require.NoError(t, InitGit(projectDir))

	info, err := os.Stat(projectDir + "/.git")
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestGitCheckpoint(t *testing.T) {
	projectDir := t.TempDir()
	require.NoError(t, InitGit(projectDir))
	require.NoError(t, os.WriteFile(projectDir+"/file.txt", []byte("data\n"), 0o644))
	require.NoError(t, GitCheckpoint(projectDir, "Epic Name", 2, "complete"))

	cmd := exec.Command("bash", "-c", "git log -1 --pretty=%s")
	cmd.Dir = projectDir
	output, err := cmd.Output()
	require.NoError(t, err)
	assert.Equal(t, "Epic Name: Sprint 2 complete [automated]", strings.TrimSpace(string(output)))
}

func TestInitGitIdempotent(t *testing.T) {
	projectDir := t.TempDir()
	require.NoError(t, InitGit(projectDir))
	require.NoError(t, InitGit(projectDir))
}

func TestGitDiffForAudit(t *testing.T) {
	projectDir := t.TempDir()
	require.NoError(t, InitGit(projectDir))

	// Create a tracked file and commit it
	require.NoError(t, os.WriteFile(projectDir+"/existing.txt", []byte("original\n"), 0o644))
	cmd := exec.Command("bash", "-c", "git add -A && git commit -m 'add existing'")
	cmd.Dir = projectDir
	require.NoError(t, cmd.Run())

	// Modify the tracked file
	require.NoError(t, os.WriteFile(projectDir+"/existing.txt", []byte("modified\n"), 0o644))

	// Add a new untracked file
	require.NoError(t, os.WriteFile(projectDir+"/newfile.txt", []byte("new content\n"), 0o644))

	diff, err := GitDiffForAudit(projectDir)
	require.NoError(t, err)

	// Should contain changes to tracked file
	assert.Contains(t, diff, "existing.txt")
	assert.Contains(t, diff, "modified")

	// Should contain new file content
	assert.Contains(t, diff, "newfile.txt")
	assert.Contains(t, diff, "new content")

	// Verify no staged changes remain after GitDiffForAudit
	statusCmd := exec.Command("bash", "-c", "git diff --cached --name-only")
	statusCmd.Dir = projectDir
	out, err := statusCmd.Output()
	require.NoError(t, err)
	assert.Empty(t, strings.TrimSpace(string(out)), "no files should be staged after GitDiffForAudit")
}

func TestGitDiffForAuditExcludesFryDir(t *testing.T) {
	projectDir := t.TempDir()
	require.NoError(t, InitGit(projectDir))

	// Create a file in .fry/ — should be excluded from diff
	require.NoError(t, os.MkdirAll(projectDir+"/.fry", 0o755))
	require.NoError(t, os.WriteFile(projectDir+"/.fry/sprint-progress.txt", []byte("progress\n"), 0o644))

	// Create a normal file — should be included
	require.NoError(t, os.WriteFile(projectDir+"/code.go", []byte("package main\n"), 0o644))

	diff, err := GitDiffForAudit(projectDir)
	require.NoError(t, err)

	assert.Contains(t, diff, "code.go")
	assert.NotContains(t, diff, "sprint-progress.txt")
}
