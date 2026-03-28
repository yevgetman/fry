package steering

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/yevgetman/fry/internal/config"
)

// IsPaused checks if a pause sentinel file exists.
func IsPaused(projectDir string) bool {
	path := filepath.Join(projectDir, config.AgentPauseFile)
	_, err := os.Stat(path)
	return err == nil
}

// ClearPause removes the pause sentinel file.
func ClearPause(projectDir string) error {
	path := filepath.Join(projectDir, config.AgentPauseFile)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("clear pause: %w", err)
	}
	return nil
}
