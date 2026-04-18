package wake

import (
	"errors"
	"os"
	"path/filepath"
)

// ErrLocked is returned when another wake is already running.
var ErrLocked = errors.New("wake: mission is locked (another wake is running)")

// Lock represents a held overlap lock.
type Lock struct {
	path string
}

// Acquire tries to create the lock directory atomically.
// Returns ErrLocked if the lock is already held.
func Acquire(missionDir string) (*Lock, error) {
	p := filepath.Join(missionDir, "lock")
	if err := os.Mkdir(p, 0o755); err != nil {
		if os.IsExist(err) {
			return nil, ErrLocked
		}
		return nil, err
	}
	return &Lock{path: p}, nil
}

// Release removes the lock directory.
func (l *Lock) Release() error {
	return os.Remove(l.path)
}
