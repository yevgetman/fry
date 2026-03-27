package epic

import (
	"fmt"
	"strings"

	"github.com/yevgetman/fry/internal/config"
)

// EffortLevel controls sprint count, iteration budget, and review rigor.
type EffortLevel string

const (
	EffortLow    EffortLevel = "low"
	EffortMedium EffortLevel = "medium"
	EffortHigh   EffortLevel = "high"
	EffortMax    EffortLevel = "max"
)

// ParseEffortLevel parses a string into an EffortLevel.
// Empty string is accepted and means auto-detect.
func ParseEffortLevel(s string) (EffortLevel, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "low":
		return EffortLow, nil
	case "medium":
		return EffortMedium, nil
	case "high":
		return EffortHigh, nil
	case "max":
		return EffortMax, nil
	case "":
		return "", nil // auto-detect
	default:
		return "", fmt.Errorf("invalid effort level %q: must be low, medium, high, or max", s)
	}
}

// String returns the effort level as a display string. Empty means "auto".
func (e EffortLevel) String() string {
	if e == "" {
		return "auto"
	}
	return string(e)
}

// DefaultMaxIterations returns the default max_iterations for sprints
// at this effort level (used when @max_iterations is not set per-sprint).
func (e EffortLevel) DefaultMaxIterations() int {
	switch e {
	case EffortLow:
		return 12
	case EffortMedium:
		return 20
	case EffortHigh:
		return 25
	case EffortMax:
		return 40
	default:
		return 25 // default = high
	}
}

// MaxSprintCount returns the maximum number of sprints for this effort level.
func (e EffortLevel) MaxSprintCount() int {
	switch e {
	case EffortLow:
		return 2
	case EffortMedium:
		return 4
	case EffortHigh:
		return 10
	case EffortMax:
		return 10
	default:
		return 10
	}
}

// DefaultMaxHealAttempts returns the default number of alignment attempts for this
// effort level. Returns 0 for low (no alignment) and max (unlimited/progress-based).
func (e EffortLevel) DefaultMaxHealAttempts() int {
	switch e {
	case EffortLow:
		return 0
	case EffortMedium:
		return config.DefaultMaxHealAttempts
	case EffortHigh:
		return config.HealAttemptsHigh
	case EffortMax:
		return 0 // unlimited, governed by progress detection
	default:
		return config.DefaultMaxHealAttempts // auto = medium default
	}
}

// DefaultMaxFailPercent returns the default sanity check failure threshold
// for this effort level.
func (e EffortLevel) DefaultMaxFailPercent() int {
	switch e {
	case EffortMax:
		return config.MaxFailPercentMax
	default:
		return config.DefaultMaxFailPercent
	}
}

// HealUsesProgressDetection returns whether this effort level uses
// progress-based alignment (exit early if stuck).
func (e EffortLevel) HealUsesProgressDetection() bool {
	return e == EffortHigh || e == EffortMax
}

// HealStuckThreshold returns the number of consecutive no-progress alignment
// attempts before the loop exits. Only meaningful when HealUsesProgressDetection
// returns true.
func (e EffortLevel) HealStuckThreshold() int {
	switch e {
	case EffortHigh:
		return config.HealStuckThresholdHigh
	case EffortMax:
		return config.HealStuckThresholdMax
	default:
		return 0
	}
}

// HealHasHardCap returns whether this effort level has a fixed upper bound on
// alignment attempts. Max effort is unlimited (progress-based), all others are capped.
func (e EffortLevel) HealHasHardCap() bool {
	return e != EffortMax
}

// DeviationScopeUnlimited returns whether this effort level allows deviations
// to touch any remaining sprint in the epic (up to the safety cap).
// All effort levels except low expand deviation scope to cover remaining sprints.
func (e EffortLevel) DeviationScopeUnlimited() bool {
	return e != EffortLow
}

type Epic struct {
	Name                 string
	Engine               string
	EffortLevel          EffortLevel
	DockerFromSprint     int
	DockerReadyCmd       string
	DockerReadyTimeout   int
	RequiredTools        []string
	PreflightCmds        []string
	PreSprintCmd         string
	PreIterationCmd      string
	AgentModel           string
	AgentFlags           string
	VerificationFile     string
	MaxHealAttempts      int
	MaxHealAttemptsSet   bool
	MaxFailPercent       int
	MaxFailPercentSet    bool
	CompactWithAgent     bool
	ReviewBetweenSprints bool
	ReviewEngine         string
	ReviewModel          string
	MaxDeviationScope    int
	AuditAfterSprint     bool
	MaxAuditIterations    int
	MaxAuditIterationsSet bool
	AuditEngine          string
	AuditModel           string
	Sprints              []Sprint
	TotalSprints         int
}

type Sprint struct {
	Number          int
	Name            string
	MaxIterations   int
	Promise         string
	MaxHealAttempts *int
	Prompt          string
}
