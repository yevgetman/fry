package docker

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/yevgetman/fry/internal/config"
)

type dockerDeps struct {
	execCommandContext func(ctx context.Context, name string, args ...string) *exec.Cmd
	lookPath           func(file string) (string, error)
	sleep              func(d time.Duration)
	now                func() time.Time
}

var defaultDeps = dockerDeps{
	execCommandContext: exec.CommandContext,
	lookPath:           exec.LookPath,
	sleep:              time.Sleep,
	now:                time.Now,
}

func DetectComposeCommand(ctx context.Context) (string, error) {
	return detectComposeCommand(ctx, defaultDeps)
}

func detectComposeCommand(ctx context.Context, deps dockerDeps) (string, error) {
	if _, err := deps.lookPath("docker"); err == nil {
		cmd := deps.execCommandContext(ctx, "bash", "-c", "docker compose version")
		if cmd.Run() == nil {
			return "docker compose", nil
		}
	}

	if _, err := deps.lookPath("docker-compose"); err == nil {
		cmd := deps.execCommandContext(ctx, "bash", "-c", "docker-compose version")
		if cmd.Run() == nil {
			return "docker-compose", nil
		}
	}

	return "", fmt.Errorf("docker compose not found")
}

func ComposeFileExists(projectDir string) bool {
	for _, name := range []string{"docker-compose.yml", "compose.yml"} {
		if info, err := os.Stat(filepath.Join(projectDir, name)); err == nil && !info.IsDir() {
			return true
		}
	}
	return false
}

func EnsureDockerUp(ctx context.Context, projectDir string, readyCmd string, timeout int) error {
	return ensureDockerUp(ctx, projectDir, readyCmd, timeout, defaultDeps)
}

func ensureDockerUp(ctx context.Context, projectDir string, readyCmd string, timeout int, deps dockerDeps) error {
	if !ComposeFileExists(projectDir) {
		return nil
	}

	composeCmd, err := detectComposeCommand(ctx, deps)
	if err != nil {
		return err
	}

	psOutput, err := runCompose(ctx, projectDir, composeCmd+" ps", deps)
	if err == nil && containersAlreadyRunning(psOutput) {
		return nil
	}

	if _, err := runCompose(ctx, projectDir, composeCmd+" up -d", deps); err != nil {
		return fmt.Errorf("docker up: %w", err)
	}

	waitSeconds := timeout
	if waitSeconds <= 0 {
		waitSeconds = config.DefaultDockerReadyTimeout
	}
	deadline := deps.now().Add(time.Duration(waitSeconds) * time.Second)

	for {
		if strings.TrimSpace(readyCmd) != "" {
			if err := runReadyCommand(ctx, projectDir, readyCmd, deps); err == nil {
				return nil
			}
		} else {
			output, err := runCompose(ctx, projectDir, composeCmd+" ps", deps)
			if err == nil && composeHealthy(output) {
				return nil
			}
		}

		if deps.now().After(deadline) {
			return fmt.Errorf("docker readiness timeout after %d seconds", waitSeconds)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(1 * time.Second):
		}
	}
}

func runCompose(ctx context.Context, projectDir, cmd string, deps dockerDeps) (string, error) {
	command := deps.execCommandContext(ctx, "bash", "-c", cmd)
	command.Dir = projectDir
	output, err := command.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("%s: %w", cmd, err)
	}
	return string(output), nil
}

func runReadyCommand(ctx context.Context, projectDir, readyCmd string, deps dockerDeps) error {
	command := deps.execCommandContext(ctx, "bash", "-c", readyCmd)
	command.Dir = projectDir
	return command.Run()
}

func containersAlreadyRunning(output string) bool {
	lines := serviceStatusLines(output)
	if len(lines) == 0 {
		return false
	}

	for _, line := range lines {
		if serviceStateReady(line) {
			return true
		}
	}
	return false
}

func composeHealthy(output string) bool {
	lines := serviceStatusLines(output)
	if len(lines) == 0 {
		return false
	}

	for _, line := range lines {
		if serviceStateBlocked(line) || !serviceStateReady(line) {
			return false
		}
	}
	return true
}

func serviceStatusLines(output string) []string {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) <= 1 {
		return nil
	}

	var statuses []string
	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if line != "" {
			statuses = append(statuses, line)
		}
	}
	return statuses
}

func serviceStateReady(line string) bool {
	normalized := strings.ToLower(line)
	return (strings.Contains(normalized, " up ") || strings.Contains(normalized, " running")) &&
		!serviceStateBlocked(normalized)
}

func serviceStateBlocked(line string) bool {
	normalized := strings.ToLower(line)
	return strings.Contains(normalized, "starting") ||
		strings.Contains(normalized, "unhealthy") ||
		strings.Contains(normalized, "exited") ||
		strings.Contains(normalized, "dead") ||
		strings.Contains(normalized, "created") ||
		strings.Contains(normalized, "restarting")
}
