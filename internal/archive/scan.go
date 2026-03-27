package archive

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/yevgetman/fry/internal/config"
)

// BuildSummary is a lightweight snapshot of a build's state,
// extracted from .fry/ artifacts without running CollectBuildState.
type BuildSummary struct {
	Dir            string    // path to the build artifacts directory
	Timestamp      time.Time // from archive dir name, or dir mod time for worktrees
	EpicName       string    // from @epic in epic.md
	Mode           string    // from build-mode.txt
	TotalSprints   int       // count of @sprint lines in epic.md
	CompletedCount int       // PASS entries in epic-progress.txt
	FailedCount    int       // FAIL entries in epic-progress.txt
	ExitReason     string    // from build-exit-reason.txt (trimmed)
}

// ScanArchives reads .fry-archive/ and returns a summary for each archived build,
// sorted newest-first. Returns nil, nil if the archive directory does not exist.
func ScanArchives(projectDir string) ([]BuildSummary, error) {
	archiveRoot := filepath.Join(projectDir, config.ArchiveDir)
	entries, err := os.ReadDir(archiveRoot)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("scan archives: %w", err)
	}

	var summaries []BuildSummary
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), config.ArchivePrefix) {
			continue
		}
		dir := filepath.Join(archiveRoot, entry.Name())
		summary, err := ScanBuildDir(dir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "fry: warning: scan archive %s: %v\n", entry.Name(), err)
			continue
		}
		if summary == nil {
			continue
		}
		ts, tsErr := parseTimestamp(entry.Name())
		if tsErr == nil {
			summary.Timestamp = ts
		}
		summaries = append(summaries, *summary)
	}

	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].Timestamp.After(summaries[j].Timestamp)
	})

	return summaries, nil
}

// ScanBuildDir extracts a BuildSummary from a directory containing .fry/ artifacts
// (epic.md, epic-progress.txt, etc.). For archives, the dir IS the old .fry/.
// For worktrees, pass the .fry/ subdirectory of the worktree.
// Returns nil, nil if epic.md is missing (graceful skip).
func ScanBuildDir(dir string) (*BuildSummary, error) {
	epicPath := filepath.Join(dir, "epic.md")
	f, err := os.Open(epicPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("open epic.md: %w", err)
	}
	defer f.Close()

	var epicName string
	var sprintCount int
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if epicName == "" && strings.HasPrefix(line, "@epic ") {
			epicName = strings.TrimSpace(strings.TrimPrefix(line, "@epic"))
		}
		if strings.HasPrefix(line, "@sprint ") {
			sprintCount++
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read epic.md: %w", err)
	}

	if epicName == "" {
		return nil, nil
	}

	summary := &BuildSummary{
		Dir:          dir,
		EpicName:     epicName,
		TotalSprints: sprintCount,
	}

	// Read epic-progress.txt for PASS/FAIL counts
	progressPath := filepath.Join(dir, "epic-progress.txt")
	if data, err := os.ReadFile(progressPath); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if strings.Contains(line, "\u2014 PASS") {
				summary.CompletedCount++
			} else if strings.Contains(line, "\u2014 FAIL") {
				summary.FailedCount++
			}
		}
	}

	// Read build-mode.txt
	modePath := filepath.Join(dir, "build-mode.txt")
	if data, err := os.ReadFile(modePath); err == nil {
		summary.Mode = strings.TrimSpace(string(data))
	}

	// Read build-exit-reason.txt
	exitPath := filepath.Join(dir, "build-exit-reason.txt")
	if data, err := os.ReadFile(exitPath); err == nil {
		summary.ExitReason = strings.TrimSpace(string(data))
	}

	return summary, nil
}

// parseTimestamp extracts a time.Time from an archive directory name
// of the form ".fry--build--YYYYMMDD-HHMMSS".
func parseTimestamp(name string) (time.Time, error) {
	ts := strings.TrimPrefix(name, config.ArchivePrefix)
	return time.ParseInLocation("20060102-150405", ts, time.Local)
}
