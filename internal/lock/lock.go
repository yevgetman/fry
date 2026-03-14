package lock

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/yevgetman/fry/internal/config"
	frylog "github.com/yevgetman/fry/internal/log"
)

func Acquire(projectDir string) error {
	lockPath := filepath.Join(projectDir, config.LockFile)
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		return fmt.Errorf("acquire lock: %w", err)
	}

	if data, err := os.ReadFile(lockPath); err == nil {
		pid, parseErr := strconv.Atoi(strings.TrimSpace(string(data)))
		if parseErr == nil && pid > 0 && processAlive(pid) {
			return fmt.Errorf("another fry instance is running (PID %d)", pid)
		}
		if err := os.Remove(lockPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("acquire lock: remove stale lock: %w", err)
		}
		frylog.Log("WARNING: Removed stale fry lock file.")
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("acquire lock: %w", err)
	}

	pid := os.Getpid()
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		// Another process may have raced us to create the lock. Re-check.
		if data, readErr := os.ReadFile(lockPath); readErr == nil {
			if otherPid, parseErr := strconv.Atoi(strings.TrimSpace(string(data))); parseErr == nil && otherPid > 0 {
				return fmt.Errorf("another fry instance is running (PID %d)", otherPid)
			}
		}
		return fmt.Errorf("acquire lock: write lock file: %w", err)
	}
	_, writeErr := f.WriteString(strconv.Itoa(pid))
	closeErr := f.Close()
	if writeErr != nil {
		return fmt.Errorf("acquire lock: write lock file: %w", writeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("acquire lock: close lock file: %w", closeErr)
	}
	return nil
}

func Release(projectDir string) error {
	lockPath := filepath.Join(projectDir, config.LockFile)
	if err := os.Remove(lockPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("release lock: %w", err)
	}
	return nil
}

func AcquireIfNotDryRun(projectDir string, dryRun bool) error {
	if dryRun {
		return nil
	}
	return Acquire(projectDir)
}

func processAlive(pid int) bool {
	// On Unix, os.FindProcess always succeeds, so we skip it and go
	// straight to the signal-0 liveness check.
	err := syscall.Kill(pid, syscall.Signal(0))
	if err != nil {
		return !errors.Is(err, syscall.ESRCH)
	}
	return true
}
