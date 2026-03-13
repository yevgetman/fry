package sprint

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/yevgetman/fry/internal/config"
)

func InitSprintProgress(projectDir string, sprintNum int, sprintName string) error {
	path := filepath.Join(projectDir, config.SprintProgressFile)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create sprint progress dir: %w", err)
	}
	content := fmt.Sprintf("# Sprint %d: %s — Progress\n\n", sprintNum, sprintName)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write sprint progress: %w", err)
	}
	return nil
}

func InitEpicProgress(projectDir string, epicName string) error {
	path := filepath.Join(projectDir, config.EpicProgressFile)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create epic progress dir: %w", err)
	}
	content := fmt.Sprintf("# Epic Progress — %s\n\n", epicName)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write epic progress: %w", err)
	}
	return nil
}

func ShouldResetEpicProgress(startSprint, currentSprint, endSprint, totalSprints int) bool {
	return startSprint == 1 && currentSprint == 1 && endSprint == totalSprints
}

func AppendToSprintProgress(projectDir string, content string) error {
	return appendFile(filepath.Join(projectDir, config.SprintProgressFile), content)
}

func AppendToEpicProgress(projectDir string, content string) error {
	return appendFile(filepath.Join(projectDir, config.EpicProgressFile), content)
}

func ReadSprintProgress(projectDir string) (string, error) {
	return readFile(filepath.Join(projectDir, config.SprintProgressFile))
}

func ReadEpicProgress(projectDir string) (string, error) {
	return readFile(filepath.Join(projectDir, config.EpicProgressFile))
}

func appendFile(path string, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create parent dir: %w", err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open append file: %w", err)
	}
	defer file.Close()

	if _, err := file.WriteString(content); err != nil {
		return fmt.Errorf("append file: %w", err)
	}
	return nil
}

func readFile(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("read file: %w", err)
	}
	return string(content), nil
}
