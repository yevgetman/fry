package prepare

import (
	"fmt"
	"strings"
)

// Mode represents the execution mode for a fry build.
type Mode string

const (
	ModeSoftware Mode = "software"
	ModePlanning Mode = "planning"
	ModeWriting  Mode = "writing"
)

// ParseMode parses a mode string, returning ModeSoftware for empty input.
// Parsing is case-insensitive.
func ParseMode(s string) (Mode, error) {
	switch Mode(strings.ToLower(strings.TrimSpace(s))) {
	case ModeSoftware, "":
		return ModeSoftware, nil
	case ModePlanning:
		return ModePlanning, nil
	case ModeWriting:
		return ModeWriting, nil
	default:
		return "", fmt.Errorf("unknown mode %q: must be software, planning, or writing", s)
	}
}
