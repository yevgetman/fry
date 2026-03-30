package archive

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/yevgetman/fry/internal/config"
)

// persistentArtifacts are .fry/ paths that survive fry clean.
// These are project-level infrastructure, not build artifacts.
var persistentArtifacts = []string{
	config.CodebaseFile,       // .fry/codebase.md
	config.FileIndexFile,      // .fry/file-index.txt
	config.CodebaseMemoriesDir, // .fry/codebase-memories/
}

// Archive moves .fry/ and root-level build outputs into a timestamped folder
// under .fry-archive/. Persistent artifacts (codebase.md, file-index.txt,
// codebase-memories/) are preserved and restored after archival.
// Returns the archive destination path.
func Archive(projectDir string) (string, error) {
	fryPath := filepath.Join(projectDir, config.FryDir)
	if _, err := os.Stat(fryPath); os.IsNotExist(err) {
		return "", fmt.Errorf("archive: %s does not exist", config.FryDir)
	} else if err != nil {
		return "", fmt.Errorf("archive: stat %s: %w", config.FryDir, err)
	}

	// Save persistent artifacts to a temp location before archival.
	tempDir, err := os.MkdirTemp("", "fry-persist-*")
	if err != nil {
		return "", fmt.Errorf("archive: create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	preserved := preserveArtifacts(projectDir, tempDir)

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

	// Restore persistent artifacts into fresh .fry/.
	if len(preserved) > 0 {
		restoreArtifacts(projectDir, tempDir, preserved)
	}

	return destPath, nil
}

// preserveArtifacts copies persistent artifacts from .fry/ to a temp directory.
// Returns the list of relative paths that were successfully preserved.
func preserveArtifacts(projectDir, tempDir string) []string {
	var preserved []string
	for _, relPath := range persistentArtifacts {
		src := filepath.Join(projectDir, relPath)
		info, err := os.Stat(src)
		if err != nil {
			continue
		}

		dst := filepath.Join(tempDir, relPath)
		if info.IsDir() {
			if copyErr := copyDir(src, dst); copyErr == nil {
				preserved = append(preserved, relPath)
			}
		} else {
			if copyErr := copyFile(src, dst); copyErr == nil {
				preserved = append(preserved, relPath)
			}
		}
	}
	return preserved
}

// restoreArtifacts copies preserved artifacts back into .fry/.
func restoreArtifacts(projectDir, tempDir string, preserved []string) {
	fryPath := filepath.Join(projectDir, config.FryDir)
	_ = os.MkdirAll(fryPath, 0o755)

	for _, relPath := range preserved {
		src := filepath.Join(tempDir, relPath)
		dst := filepath.Join(projectDir, relPath)

		info, err := os.Stat(src)
		if err != nil {
			continue
		}

		_ = os.MkdirAll(filepath.Dir(dst), 0o755)

		if info.IsDir() {
			_ = copyDir(src, dst)
		} else {
			_ = copyFile(src, dst)
		}
	}
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(path, target)
	})
}
