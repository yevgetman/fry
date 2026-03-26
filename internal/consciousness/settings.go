package consciousness

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/yevgetman/fry/internal/config"
)

// Settings represents the user's ~/.fry/settings.json file.
// Only the fields needed now are defined; future stages may add more.
type Settings struct {
	Telemetry *bool `json:"telemetry,omitempty"`
}

// LoadSettings reads ~/.fry/settings.json. Returns zero-value Settings if the
// file does not exist or is malformed (non-fatal).
func LoadSettings() Settings {
	home, err := os.UserHomeDir()
	if err != nil {
		return Settings{}
	}
	return loadSettingsFromDir(home)
}

// loadSettingsFromDir reads settings from the given base directory.
func loadSettingsFromDir(baseDir string) Settings {
	path := filepath.Join(baseDir, config.SettingsFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return Settings{}
	}

	var s Settings
	if err := json.Unmarshal(data, &s); err != nil {
		fmt.Fprintf(os.Stderr, "fry: warning: could not parse %s: %v\n", path, err)
		return Settings{}
	}
	return s
}

// TelemetryEnabled resolves telemetry opt-in from the priority chain:
//
//	CLI flag > env var > settings file > default (false)
//
// cliFlag is nil when the flag was not provided.
func TelemetryEnabled(cliFlag *bool, settings Settings) bool {
	// 1. CLI flag (highest priority)
	if cliFlag != nil {
		return *cliFlag
	}

	// 2. Environment variable
	if env := os.Getenv(config.TelemetryEnvVar); env != "" {
		lower := strings.ToLower(env)
		if lower == "1" || lower == "true" {
			return true
		}
		if lower == "0" || lower == "false" {
			return false
		}
		// Unrecognized value — fall through
	}

	// 3. Settings file
	if settings.Telemetry != nil {
		return *settings.Telemetry
	}

	// 4. Default: off
	return false
}
