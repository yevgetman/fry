//go:build linux

package scheduler

import "github.com/yevgetman/fry/internal/state"

type linuxScheduler struct{}

func newScheduler() Scheduler {
	return linuxScheduler{}
}

func (s linuxScheduler) Install(m *state.Mission, plistPath string) error {
	return ErrUnsupported
}

func (s linuxScheduler) Uninstall(m *state.Mission) error {
	return ErrUnsupported
}

func (s linuxScheduler) Status(m *state.Mission) (SchedulerStatus, error) {
	return SchedulerStatus{}, ErrUnsupported
}

func (s linuxScheduler) Kickstart(m *state.Mission) error {
	return ErrUnsupported
}
