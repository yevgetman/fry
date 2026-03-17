package continuerun

import "fmt"

// ContinueVerdict represents the LLM's decision about how to resume a build.
type ContinueVerdict string

const (
	VerdictResumeRetry  ContinueVerdict = "RESUME_RETRY"
	VerdictResumeFresh  ContinueVerdict = "RESUME_FRESH"
	VerdictContinueNext ContinueVerdict = "CONTINUE_NEXT"
	VerdictAllComplete  ContinueVerdict = "ALL_COMPLETE"
	VerdictBlocked      ContinueVerdict = "BLOCKED"
)

// ErrNoPreviousBuild indicates no .fry/ directory or build artifacts exist.
var ErrNoPreviousBuild = fmt.Errorf("no previous build found; run `fry run` to start a new build")

// ContinueDecision is the parsed result from the LLM analysis agent.
type ContinueDecision struct {
	Verdict       ContinueVerdict
	StartSprint   int
	Reason        string
	Preconditions []string // actions the user must take before resuming
}

// BuildState is the programmatically-collected snapshot of a build's current state.
type BuildState struct {
	EpicName     string
	TotalSprints int
	Engine       string
	EffortLevel  string

	// Build mode (software, planning, writing) from .fry/build-mode.txt
	Mode string

	// Per-sprint completion from epic-progress.txt
	CompletedSprints []CompletedSprint
	HighestCompleted int // 0 if none

	// Evidence of started-but-not-passed sprints (may be multiple)
	ActiveSprints []ActiveSprintState // empty if no partial work detected

	// Environment checks
	DockerAvailable bool
	DockerRequired  bool // true if epic has docker_from_sprint <= next sprint
	RequiredTools   []ToolStatus
	GitClean        bool
	GitBranch       string
	LastAutoCommit  string // message of most recent [automated] commit

	// History
	DeviationCount   int
	DeferredFailures []string // summary lines from deferred-failures.md

	// Sprint names for the LLM prompt (number → name)
	SprintNames []string
}

// CompletedSprint records one sprint that has passed.
type CompletedSprint struct {
	Number int
	Name   string
	Status string // "PASS", "PASS (healed)", etc.
}

// ActiveSprintState describes a sprint that was started but did not pass.
type ActiveSprintState struct {
	Number          int
	Name            string
	IterationCount  int
	AuditCount      int
	HealCount       int
	HasRetryLog     bool
	LastLogTail     string // tail ~100 lines of most recent log
	AuditSeverity   string // from sprint-audit.txt if present
	ProgressExcerpt string // last entry from sprint-progress.txt
}

// ToolStatus records whether a required tool is available.
type ToolStatus struct {
	Name      string
	Available bool
}
