//go:build darwin

package scheduler

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/yevgetman/fry/internal/state"
)

type darwinScheduler struct{}

func newScheduler() Scheduler {
	return darwinScheduler{}
}

func plistLabel(missionID string) string {
	return fmt.Sprintf("com.fry.%s", missionID)
}

func plistDst(missionID string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home dir: %w", err)
	}
	return filepath.Join(home, "Library", "LaunchAgents", plistLabel(missionID)+".plist"), nil
}

func (s darwinScheduler) Install(m *state.Mission, plistPath string) error {
	dst, err := plistDst(m.MissionID)
	if err != nil {
		return err
	}
	if err := copyFileSched(plistPath, dst); err != nil {
		return fmt.Errorf("copy plist to LaunchAgents: %w", err)
	}
	out, err := exec.Command("launchctl", "load", dst).CombinedOutput()
	if err != nil {
		return fmt.Errorf("launchctl load: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (s darwinScheduler) Uninstall(m *state.Mission) error {
	dst, err := plistDst(m.MissionID)
	if err != nil {
		return err
	}
	out, err := exec.Command("launchctl", "unload", dst).CombinedOutput()
	if err != nil {
		// If it was never loaded, unload still returns an error — treat as warning.
		fmt.Fprintf(os.Stderr, "launchctl unload (warning): %s\n", strings.TrimSpace(string(out)))
	}
	if err := os.Remove(dst); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove plist: %w", err)
	}
	return nil
}

func (s darwinScheduler) Status(m *state.Mission) (SchedulerStatus, error) {
	label := plistLabel(m.MissionID)
	out, err := exec.Command("launchctl", "list").CombinedOutput()
	if err != nil {
		return SchedulerStatus{}, fmt.Errorf("launchctl list: %w", err)
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, label) {
			return SchedulerStatus{Running: true}, nil
		}
	}
	return SchedulerStatus{Running: false}, nil
}

func (s darwinScheduler) Kickstart(m *state.Mission) error {
	uid := fmt.Sprintf("%d", os.Getuid())
	label := plistLabel(m.MissionID)
	target := fmt.Sprintf("gui/%s/%s", uid, label)
	out, err := exec.Command("launchctl", "kickstart", "-k", target).CombinedOutput()
	if err != nil {
		return fmt.Errorf("launchctl kickstart: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func copyFileSched(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
