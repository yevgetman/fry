package archive

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/yevgetman/fry/internal/config"
)

// Archive moves .fry/ and root-level build outputs into a timestamped folder
// under .fry-archive/. Returns the archive destination path.
func Archive(projectDir string) (string, error) {
	fryPath := filepath.Join(projectDir, config.FryDir)
	if _, err := os.Stat(fryPath); os.IsNotExist(err) {
		return "", fmt.Errorf("archive: %s does not exist", config.FryDir)
	} else if err != nil {
		return "", fmt.Errorf("archive: stat %s: %w", config.FryDir, err)
	}

	archiveRoot := filepath.Join(projectDir, config.ArchiveDir)
	if err := os.MkdirAll(archiveRoot, 0o755); err != nil {
		return "", fmt.Errorf("archive: create %s: %w", config.ArchiveDir, err)
	}

	archiveName := config.ArchivePrefix + time.Now().Format("20060102-150405")
	destPath := filepath.Join(archiveRoot, archiveName)

	if err := os.Rename(fryPath, destPath); err != nil {
		return "", fmt.Errorf("archive: move %s: %w", config.FryDir, err)
	}

	for _, name := range []string{config.BuildAuditFile, config.SummaryFile} {
		src := filepath.Join(projectDir, name)
		if _, err := os.Stat(src); os.IsNotExist(err) {
			continue
		} else if err != nil {
			fmt.Fprintf(os.Stderr, "fry: warning: stat %s: %v\n", name, err)
			continue
		}
		dst := filepath.Join(destPath, name)
		if err := os.Rename(src, dst); err != nil {
			return "", fmt.Errorf("archive: move %s: %w", name, err)
		}
	}

	return destPath, nil
}
