package settings

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/yevgetman/fry/internal/config"
)

type Settings struct {
	Engine string `json:"engine,omitempty"`
}

type rawSettings map[string]json.RawMessage

func Load(projectDir string) (Settings, error) {
	path := filepath.Join(projectDir, config.ProjectConfigFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Settings{}, nil
		}
		return Settings{}, fmt.Errorf("read config: %w", err)
	}

	var raw rawSettings
	if err := json.Unmarshal(data, &raw); err != nil {
		return Settings{}, fmt.Errorf("parse config: %w", err)
	}
	var settings Settings
	if rawEngine, ok := raw["engine"]; ok {
		if err := json.Unmarshal(rawEngine, &settings.Engine); err != nil {
			return Settings{}, fmt.Errorf("parse config engine: %w", err)
		}
	}
	if settings.Engine != "" {
		if err := validateEngine(settings.Engine); err != nil {
			return Settings{}, err
		}
	}
	return settings, nil
}

func Save(projectDir string, settings Settings) error {
	if settings.Engine != "" {
		if err := validateEngine(settings.Engine); err != nil {
			return err
		}
	}

	path := filepath.Join(projectDir, config.ProjectConfigFile)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	raw := rawSettings{}
	if existing, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(existing, &raw); err != nil {
			return fmt.Errorf("parse existing config: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("read existing config: %w", err)
	}

	if settings.Engine == "" {
		delete(raw, "engine")
	} else {
		engineJSON, err := json.Marshal(settings.Engine)
		if err != nil {
			return fmt.Errorf("marshal config engine: %w", err)
		}
		raw["engine"] = engineJSON
	}

	data, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	data = append(data, '\n')

	tmp, err := os.CreateTemp(filepath.Dir(path), "config-*.json")
	if err != nil {
		return fmt.Errorf("create temp config: %w", err)
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("write temp config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close temp config: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("replace config: %w", err)
	}
	return nil
}

func GetEngine(projectDir string) (string, error) {
	settings, err := Load(projectDir)
	if err != nil {
		return "", err
	}
	return settings.Engine, nil
}

func SetEngine(projectDir, engineName string) error {
	settings, err := Load(projectDir)
	if err != nil {
		return err
	}
	settings.Engine = engineName
	return Save(projectDir, settings)
}

func validateEngine(engineName string) error {
	switch engineName {
	case "codex", "claude", "ollama":
		return nil
	default:
		return fmt.Errorf("unsupported engine %q; valid: codex, claude, ollama", engineName)
	}
}
