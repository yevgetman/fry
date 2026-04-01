package monitor

import (
	"time"

	"github.com/yevgetman/fry/internal/agent"
	"github.com/yevgetman/fry/internal/team"
)

// Snapshot is the composed build state from all monitored sources at a point
// in time. It is the single output type that all views render from.
type Snapshot struct {
	Timestamp      time.Time          `json:"timestamp"`
	ProjectDir     string             `json:"project_dir"`
	WorktreeDir    string             `json:"worktree_dir,omitempty"`
	BuildActive    bool               `json:"build_active"`
	PID            int                `json:"pid,omitempty"`
	Phase          string             `json:"phase,omitempty"`
	Events         []EnrichedEvent    `json:"events,omitempty"`
	NewEvents      []EnrichedEvent    `json:"new_events,omitempty"`
	BuildStatus    *agent.BuildStatus `json:"build_status,omitempty"`
	StatusChanged  bool               `json:"status_changed"`
	SprintProgress string             `json:"sprint_progress,omitempty"`
	EpicProgress   string             `json:"epic_progress,omitempty"`
	ActiveLogPath  string             `json:"active_log_path,omitempty"`
	ActiveLogTail  string             `json:"active_log_tail,omitempty"`
	BuildEnded     bool               `json:"build_ended"`
	ExitReason     string             `json:"exit_reason,omitempty"`
	Team           *team.Snapshot     `json:"team,omitempty"`
}

// EnrichedEvent extends agent.BuildEvent with computed context.
type EnrichedEvent struct {
	agent.BuildEvent

	ElapsedBuild  time.Duration `json:"elapsed_build"`
	ElapsedSprint time.Duration `json:"elapsed_sprint"`
	SprintOf      string        `json:"sprint_of,omitempty"`
	PhaseChange   string        `json:"phase_change,omitempty"`
	IsTerminal    bool          `json:"is_terminal"`
	Synthetic     bool          `json:"synthetic,omitempty"`
}
