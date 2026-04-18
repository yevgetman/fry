package state

import "fmt"

// allowed maps each from-status to the set of valid to-statuses.
var allowed = map[Status][]Status{
	StatusActive:   {StatusOvertime, StatusComplete, StatusStopped, StatusFailed},
	StatusOvertime: {StatusComplete, StatusStopped, StatusFailed},
	StatusStopped:  {StatusActive},
	StatusComplete: {},
	StatusFailed:   {},
}

func CanTransition(from, to Status) bool {
	for _, s := range allowed[from] {
		if s == to {
			return true
		}
	}
	return false
}

// Transition validates and applies a status transition.
func (m *Mission) Transition(to Status) error {
	if !CanTransition(m.Status, to) {
		return fmt.Errorf("illegal transition %s → %s", m.Status, to)
	}
	m.Status = to
	return nil
}
