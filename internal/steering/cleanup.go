package steering

import (
	"os"
	"path/filepath"

	"github.com/yevgetman/fry/internal/config"
)

// CleanupAll removes all steering sentinel and IPC files from the project.
// Called at build completion to prevent stale files from affecting the next run.
func CleanupAll(projectDir string) {
	files := []string{
		config.AgentDirectiveFile,
		config.AgentHoldFile,
		config.AgentPauseFile,
		config.DecisionNeededFile,
		config.ExitRequestFile,
		config.ResumePointFile,
	}
	for _, f := range files {
		_ = os.Remove(filepath.Join(projectDir, f))
	}
	// Also clean up any .consumed temp file from ConsumeDirective
	_ = os.Remove(filepath.Join(projectDir, config.AgentDirectiveFile+".consumed"))
}
