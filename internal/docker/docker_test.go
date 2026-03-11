package docker

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectComposeCommand(t *testing.T) {
	origLookPath := lookPath
	origExec := execCommandContext
	t.Cleanup(func() {
		lookPath = origLookPath
		execCommandContext = origExec
	})

	lookPath = func(file string) (string, error) {
		switch file {
		case "docker":
			return "/usr/bin/docker", nil
		case "docker-compose":
			return "", errors.New("missing")
		default:
			return "", errors.New("missing")
		}
	}
	execCommandContext = func(_ context.Context, name string, args ...string) *exec.Cmd {
		return exec.Command("bash", "-c", "exit 0")
	}

	cmd, err := DetectComposeCommand(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "docker compose", cmd)
}

func TestComposeFileExists(t *testing.T) {
	projectDir := t.TempDir()
	assert.False(t, ComposeFileExists(projectDir))

	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "docker-compose.yml"), []byte("services:\n"), 0o644))
	assert.True(t, ComposeFileExists(projectDir))

	require.NoError(t, os.Remove(filepath.Join(projectDir, "docker-compose.yml")))
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "compose.yml"), []byte("services:\n"), 0o644))
	assert.True(t, ComposeFileExists(projectDir))
}
