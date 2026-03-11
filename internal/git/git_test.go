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
