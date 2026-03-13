package epic

import (
	"fmt"
	"strings"
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
	CompactWithAgent     bool
	ReviewBetweenSprints bool
	ReviewEngine         string
	ReviewModel          string
	MaxDeviationScope    int
	AuditAfterSprint     bool
	MaxAuditIterations   int
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
