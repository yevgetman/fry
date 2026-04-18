package scheduler

import (
	"errors"

	"github.com/yevgetman/fry/internal/state"
)

// ErrUnsupported is returned on platforms without a scheduler backend.
var ErrUnsupported = errors.New("scheduler: unsupported platform")

// SchedulerStatus describes whether the scheduler is currently loaded.
type SchedulerStatus struct {
	Running bool
	PID     int
}

// Scheduler installs and manages the per-mission LaunchAgent (or equivalent).
type Scheduler interface {
	Install(m *state.Mission, plistPath string) error
	Uninstall(m *state.Mission) error
	Status(m *state.Mission) (SchedulerStatus, error)
	Kickstart(m *state.Mission) error
}

// New returns the platform-appropriate Scheduler implementation.
func New() Scheduler {
	return newScheduler()
}
