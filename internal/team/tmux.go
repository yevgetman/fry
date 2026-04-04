package team

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type Tmux interface {
	HasSession(ctx context.Context, session string) bool
	NewSession(ctx context.Context, session, window, command string) (string, error)
	NewWindow(ctx context.Context, session, window, command string) (string, error)
	KillSession(ctx context.Context, session string) error
	KillWindow(ctx context.Context, session, window string) error
	Attach(ctx context.Context, session string) error
	WindowAlive(ctx context.Context, session, window string) bool
}

var DefaultTmux Tmux = systemTmux{}

type systemTmux struct{}

func (systemTmux) HasSession(ctx context.Context, session string) bool {
	cmd := exec.CommandContext(ctx, "tmux", "has-session", "-t", session)
	return cmd.Run() == nil
}

func (systemTmux) NewSession(ctx context.Context, session, window, command string) (string, error) {
	if err := runTmux(ctx, "new-session", "-d", "-s", session, "-n", window, command); err != nil {
		return "", err
	}
	return paneIDForWindow(ctx, session, window)
}

func (systemTmux) NewWindow(ctx context.Context, session, window, command string) (string, error) {
	if err := runTmux(ctx, "new-window", "-d", "-t", session, "-n", window, command); err != nil {
		return "", err
	}
	return paneIDForWindow(ctx, session, window)
}

func (systemTmux) KillSession(ctx context.Context, session string) error {
	if !(systemTmux{}).HasSession(ctx, session) {
		return nil
	}
	return runTmux(ctx, "kill-session", "-t", session)
}

func (systemTmux) KillWindow(ctx context.Context, session, window string) error {
	return runTmux(ctx, "kill-window", "-t", session+":"+window)
}

func (systemTmux) Attach(ctx context.Context, session string) error {
	cmd := exec.CommandContext(ctx, "tmux", "attach-session", "-t", session)
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run()
}

func (systemTmux) WindowAlive(ctx context.Context, session, window string) bool {
	cmd := exec.CommandContext(ctx, "tmux", "list-windows", "-t", session, "-F", "#{window_name}")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return false
	}
	for _, line := range strings.Split(strings.TrimSpace(stdout.String()), "\n") {
		if strings.TrimSpace(line) == window {
			return true
		}
	}
	return false
}

func runTmux(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, "tmux", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("tmux %s: %s", strings.Join(args, " "), msg)
	}
	return nil
}

func paneIDForWindow(ctx context.Context, session, window string) (string, error) {
	cmd := exec.CommandContext(ctx, "tmux", "list-panes", "-t", session+":"+window, "-F", "#{pane_id}")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("tmux list-panes %s:%s: %s", session, window, msg)
	}
	return strings.TrimSpace(stdout.String()), nil
}
