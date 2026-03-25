package docker

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectComposeCommand(t *testing.T) {
	t.Parallel()

	deps := dockerDeps{
		lookPath: func(file string) (string, error) {
			switch file {
			case "docker":
				return "/usr/bin/docker", nil
			case "docker-compose":
				return "", errors.New("missing")
			default:
				return "", errors.New("missing")
			}
		},
		execCommandContext: func(_ context.Context, name string, args ...string) *exec.Cmd {
			return exec.Command("bash", "-c", "exit 0")
		},
	}

	cmd, err := detectComposeCommand(context.Background(), deps)
	require.NoError(t, err)
	assert.Equal(t, "docker compose", cmd)
}

func TestDetectComposeCommand_FallbackToDockerCompose(t *testing.T) {
	t.Parallel()

	callCount := 0
	deps := dockerDeps{
		lookPath: func(file string) (string, error) {
			switch file {
			case "docker":
				return "/usr/bin/docker", nil
			case "docker-compose":
				return "/usr/local/bin/docker-compose", nil
			default:
				return "", errors.New("missing")
			}
		},
		execCommandContext: func(_ context.Context, name string, args ...string) *exec.Cmd {
			callCount++
			if callCount == 1 {
				// "docker compose version" fails
				return exec.Command("bash", "-c", "exit 1")
			}
			// "docker-compose version" succeeds
			return exec.Command("bash", "-c", "exit 0")
		},
	}

	cmd, err := detectComposeCommand(context.Background(), deps)
	require.NoError(t, err)
	assert.Equal(t, "docker-compose", cmd)
}

func TestDetectComposeCommand_NeitherAvailable(t *testing.T) {
	t.Parallel()

	deps := dockerDeps{
		lookPath: func(file string) (string, error) {
			return "", errors.New("not found")
		},
		execCommandContext: func(_ context.Context, name string, args ...string) *exec.Cmd {
			return exec.Command("bash", "-c", "exit 1")
		},
	}

	_, err := detectComposeCommand(context.Background(), deps)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "docker compose not found")
}

func TestComposeFileExists(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	assert.False(t, ComposeFileExists(projectDir))

	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "docker-compose.yml"), []byte("services:\n"), 0o644))
	assert.True(t, ComposeFileExists(projectDir))

	require.NoError(t, os.Remove(filepath.Join(projectDir, "docker-compose.yml")))
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "compose.yml"), []byte("services:\n"), 0o644))
	assert.True(t, ComposeFileExists(projectDir))
}

func TestComposeFileExists_DirectoryNotFile(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	// Create a directory with the same name — should not count
	require.NoError(t, os.MkdirAll(filepath.Join(projectDir, "docker-compose.yml"), 0o755))
	assert.False(t, ComposeFileExists(projectDir))
}

func TestEnsureDockerUp_NoComposeFile(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	// No compose file → should return nil immediately
	err := EnsureDockerUp(context.Background(), projectDir, "", 0)
	require.NoError(t, err)
}

// P3: containersAlreadyRunning

func TestContainersAlreadyRunning(t *testing.T) {
	t.Parallel()

	assert.False(t, containersAlreadyRunning(""))
	assert.False(t, containersAlreadyRunning("HEADER"))
	assert.False(t, containersAlreadyRunning("HEADER\n"))
	assert.False(t, containersAlreadyRunning("HEADER\n  \n"))
	assert.True(t, containersAlreadyRunning("HEADER\ncontainer1  running"))
	assert.True(t, containersAlreadyRunning("NAME\napp  Up 5 minutes"))
	assert.False(t, containersAlreadyRunning("NAME\napp  Exited (1)"))
	assert.False(t, containersAlreadyRunning("NAME\napp  Restarting (1)"))
}

// P3: composeHealthy

func TestComposeHealthy(t *testing.T) {
	t.Parallel()

	assert.True(t, composeHealthy("NAME  STATUS\napp  Up 5 minutes (healthy)"))
	assert.True(t, composeHealthy("NAME  STATUS\napp  running(healthy)"))
	assert.False(t, composeHealthy("NAME  STATUS\napp  Up 5 minutes (starting)"))
	assert.False(t, composeHealthy("NAME  STATUS\napp  Up 5 minutes (unhealthy)"))
	assert.False(t, composeHealthy("NAME  STATUS\napp  Exited (1)"))
	assert.False(t, composeHealthy("NAME  STATUS\napp  Created"))
	assert.False(t, composeHealthy(""))
}

func TestEnsureDockerUp_ContainersAlreadyRunning(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "docker-compose.yml"), []byte("services:\n"), 0o644))

	callCount := 0
	deps := dockerDeps{
		lookPath: func(file string) (string, error) {
			if file == "docker" {
				return "/usr/bin/docker", nil
			}
			return "", errors.New("not found")
		},
		execCommandContext: func(_ context.Context, _ string, args ...string) *exec.Cmd {
			callCount++
			if callCount == 1 {
				return exec.Command("bash", "-c", "exit 0")
			}
			return exec.Command("bash", "-c", `printf "NAME  STATUS\napp  Up 5 minutes"`)
		},
		sleep: func(d time.Duration) {},
		now:   time.Now,
	}

	err := ensureDockerUp(context.Background(), projectDir, "", 10, deps)
	require.NoError(t, err)
}

func TestEnsureDockerUp_StartupWithReadyCommand(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "docker-compose.yml"), []byte("services:\n"), 0o644))

	callCount := 0
	deps := dockerDeps{
		lookPath: func(file string) (string, error) {
			if file == "docker" {
				return "/usr/bin/docker", nil
			}
			return "", errors.New("not found")
		},
		execCommandContext: func(_ context.Context, _ string, args ...string) *exec.Cmd {
			callCount++
			switch callCount {
			case 1:
				return exec.Command("bash", "-c", "exit 0") // detect compose
			case 2:
				return exec.Command("bash", "-c", `echo "NAME  STATUS"`) // ps → not running
			case 3:
				return exec.Command("bash", "-c", "exit 0") // up -d
			default:
				return exec.Command("bash", "-c", "exit 0") // readyCmd → success
			}
		},
		sleep: func(d time.Duration) {},
		now:   time.Now,
	}

	err := ensureDockerUp(context.Background(), projectDir, "ready", 10, deps)
	require.NoError(t, err)
}

func TestEnsureDockerUp_StartupWithHealthCheckPolling(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "docker-compose.yml"), []byte("services:\n"), 0o644))

	callCount := 0
	deps := dockerDeps{
		lookPath: func(file string) (string, error) {
			if file == "docker" {
				return "/usr/bin/docker", nil
			}
			return "", errors.New("not found")
		},
		execCommandContext: func(_ context.Context, _ string, args ...string) *exec.Cmd {
			callCount++
			switch callCount {
			case 1:
				return exec.Command("bash", "-c", "exit 0") // detect compose
			case 2:
				return exec.Command("bash", "-c", `echo "NAME  STATUS"`) // ps → not running
			case 3:
				return exec.Command("bash", "-c", "exit 0") // up -d
			case 4:
				return exec.Command("bash", "-c", `printf "NAME  STATUS\napp  Starting"`) // ps poll 1 → not healthy
			default:
				return exec.Command("bash", "-c", `printf "NAME  STATUS\napp  Up 5 minutes (healthy)"`) // ps poll 2 → healthy
			}
		},
		sleep: func(d time.Duration) {},
		now:   time.Now,
	}

	err := ensureDockerUp(context.Background(), projectDir, "", 10, deps)
	require.NoError(t, err)
}

func TestEnsureDockerUp_TimeoutExceeded(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "docker-compose.yml"), []byte("services:\n"), 0o644))

	startTime := time.Now()
	nowCallCount := 0
	callCount := 0
	deps := dockerDeps{
		lookPath: func(file string) (string, error) {
			if file == "docker" {
				return "/usr/bin/docker", nil
			}
			return "", errors.New("not found")
		},
		execCommandContext: func(_ context.Context, _ string, args ...string) *exec.Cmd {
			callCount++
			switch callCount {
			case 1:
				return exec.Command("bash", "-c", "exit 0") // detect compose
			case 2:
				return exec.Command("bash", "-c", `echo "NAME  STATUS"`) // ps → not running
			case 3:
				return exec.Command("bash", "-c", "exit 0") // up -d
			default:
				return exec.Command("bash", "-c", `printf "NAME  STATUS\napp  Starting"`) // ps poll → not healthy
			}
		},
		sleep: func(d time.Duration) {},
		now: func() time.Time {
			nowCallCount++
			if nowCallCount == 1 {
				return startTime
			}
			return startTime.Add(1000 * time.Second)
		},
	}

	err := ensureDockerUp(context.Background(), projectDir, "", 1, deps)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "timeout")
}

func TestEnsureDockerUp_ContextCancelled(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "docker-compose.yml"), []byte("services:\n"), 0o644))

	callCount := 0
	deps := dockerDeps{
		lookPath: func(file string) (string, error) {
			if file == "docker" {
				return "/usr/bin/docker", nil
			}
			return "", errors.New("not found")
		},
		execCommandContext: func(_ context.Context, _ string, args ...string) *exec.Cmd {
			callCount++
			switch callCount {
			case 1:
				return exec.Command("bash", "-c", "exit 0") // detect compose
			case 2:
				return exec.Command("bash", "-c", `echo "NAME  STATUS"`) // ps → not running
			case 3:
				return exec.Command("bash", "-c", "exit 0") // up -d
			default:
				return exec.Command("bash", "-c", `printf "NAME  STATUS\napp  Starting"`) // ps poll → not healthy
			}
		},
		sleep: func(d time.Duration) {},
		now:   time.Now,
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := ensureDockerUp(ctx, projectDir, "", 10, deps)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}
