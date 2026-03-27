package git

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/yevgetman/fry/internal/archive"
	"github.com/yevgetman/fry/internal/config"
)

// ScanWorktreeBuilds looks in .fry-worktrees/ for subdirectories containing
// .fry/epic.md and returns a summary for each. Returns nil, nil if the
// worktree directory does not exist.
func ScanWorktreeBuilds(projectDir string) ([]archive.BuildSummary, error) {
	wtRoot := filepath.Join(projectDir, config.GitWorktreeDir)
	entries, err := os.ReadDir(wtRoot)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("scan worktree builds: %w", err)
	}

	var summaries []archive.BuildSummary
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		fryDir := filepath.Join(wtRoot, entry.Name(), config.FryDir)
		summary, err := archive.ScanBuildDir(fryDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "fry: warning: scan worktree %s: %v\n", entry.Name(), err)
			continue
		}
		if summary == nil {
			continue
		}
		// Use the worktree subdirectory as Dir for display
		summary.Dir = filepath.Join(config.GitWorktreeDir, entry.Name())
		// Use directory mod time as timestamp
		if info, infoErr := entry.Info(); infoErr == nil {
			summary.Timestamp = info.ModTime()
		}
		summaries = append(summaries, *summary)
	}

	return summaries, nil
}
