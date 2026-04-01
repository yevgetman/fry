package team

import "time"

type TeamStatus string

const (
	StatusStarting TeamStatus = "starting"
	StatusRunning  TeamStatus = "running"
	StatusPaused   TeamStatus = "paused"
	StatusDraining TeamStatus = "draining"
	StatusFailed   TeamStatus = "failed"
	StatusComplete TeamStatus = "complete"
	StatusShutdown TeamStatus = "shutdown"
)

type TaskStatus string

const (
	TaskPending    TaskStatus = "pending"
	TaskAssigned   TaskStatus = "assigned"
	TaskInProgress TaskStatus = "in_progress"
	TaskCompleted  TaskStatus = "completed"
	TaskFailed     TaskStatus = "failed"
	TaskBlocked    TaskStatus = "blocked"
)

type WorkerStatus string

const (
	WorkerStarting WorkerStatus = "starting"
	WorkerIdle     WorkerStatus = "idle"
	WorkerRunning  WorkerStatus = "running"
	WorkerDraining WorkerStatus = "draining"
	WorkerStalled  WorkerStatus = "stalled"
	WorkerDead     WorkerStatus = "dead"
	WorkerStopped  WorkerStatus = "stopped"
)

type IsolationMode string

const (
	IsolationShared            IsolationMode = "shared"
	IsolationPerWorkerWorktree IsolationMode = "per-worker-worktree"
)

type Config struct {
	Version          int           `json:"version"`
	TeamID           string        `json:"team_id"`
	ProjectDir       string        `json:"project_dir"`
	BuildDir         string        `json:"build_dir"`
	Status           TeamStatus    `json:"status"`
	Engine           string        `json:"engine,omitempty"`
	TMuxSession      string        `json:"tmux_session"`
	LeaderPaneID     string        `json:"leader_pane_id,omitempty"`
	WorkerCount      int           `json:"worker_count"`
	MaxWorkers       int           `json:"max_workers"`
	GitIsolationMode IsolationMode `json:"git_isolation_mode"`
	CreatedAt        time.Time     `json:"created_at"`
	UpdatedAt        time.Time     `json:"updated_at"`
}

type Task struct {
	ID              string     `json:"id"`
	Title           string     `json:"title"`
	Description     string     `json:"description,omitempty"`
	Role            string     `json:"role,omitempty"`
	Status          TaskStatus `json:"status"`
	Owner           string     `json:"owner,omitempty"`
	Priority        int        `json:"priority"`
	Command         string     `json:"command,omitempty"`
	WorkDir         string     `json:"work_dir,omitempty"`
	BlockedBy       []string   `json:"blocked_by,omitempty"`
	AcceptanceHints []string   `json:"acceptance_hints,omitempty"`
	LogPath         string     `json:"log_path,omitempty"`
	LastError       string     `json:"last_error,omitempty"`
	ExitCode        int        `json:"exit_code,omitempty"`
	Attempts        int        `json:"attempts,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	StartedAt       *time.Time `json:"started_at,omitempty"`
	FinishedAt      *time.Time `json:"finished_at,omitempty"`
}

type TaskInput struct {
	ID              string   `json:"id"`
	Title           string   `json:"title"`
	Description     string   `json:"description,omitempty"`
	Role            string   `json:"role,omitempty"`
	Priority        int      `json:"priority,omitempty"`
	Command         string   `json:"command,omitempty"`
	BlockedBy       []string `json:"blocked_by,omitempty"`
	AcceptanceHints []string `json:"acceptance_hints,omitempty"`
}

type TaskFile struct {
	Tasks []TaskInput `json:"tasks"`
}

type WorkerIdentity struct {
	WorkerID        string       `json:"worker_id"`
	Role            string       `json:"role"`
	Engine          string       `json:"engine,omitempty"`
	Model           string       `json:"model,omitempty"`
	ReasoningEffort string       `json:"reasoning_effort,omitempty"`
	PaneID          string       `json:"pane_id,omitempty"`
	WindowName      string       `json:"window_name,omitempty"`
	WorkDir         string       `json:"work_dir"`
	WorktreeBranch  string       `json:"worktree_branch,omitempty"`
	Status          WorkerStatus `json:"status"`
}

type WorkerHeartbeat struct {
	WorkerID    string       `json:"worker_id"`
	Status      WorkerStatus `json:"status"`
	CurrentTask string       `json:"current_task,omitempty"`
	LastSeenAt  time.Time    `json:"last_seen_at"`
	Iteration   int          `json:"iteration"`
	Message     string       `json:"message,omitempty"`
}

type WorkerRecord struct {
	WorkerID      string       `json:"worker_id"`
	Status        WorkerStatus `json:"status"`
	DesiredStatus WorkerStatus `json:"desired_status,omitempty"`
	CurrentTask   string       `json:"current_task,omitempty"`
	LastError     string       `json:"last_error,omitempty"`
	LastExitCode  int          `json:"last_exit_code,omitempty"`
	PaneID        string       `json:"pane_id,omitempty"`
	WindowName    string       `json:"window_name,omitempty"`
	UpdatedAt     time.Time    `json:"updated_at"`
}

type Event struct {
	Timestamp string            `json:"ts"`
	Type      string            `json:"type"`
	TeamID    string            `json:"team_id"`
	WorkerID  string            `json:"worker_id,omitempty"`
	TaskID    string            `json:"task_id,omitempty"`
	Data      map[string]string `json:"data,omitempty"`
}

type WorkerSnapshot struct {
	Identity  WorkerIdentity   `json:"identity"`
	Record    WorkerRecord     `json:"record"`
	Heartbeat *WorkerHeartbeat `json:"heartbeat,omitempty"`
	Alive     bool             `json:"alive"`
}

type Snapshot struct {
	Config         Config           `json:"config"`
	Workers        []WorkerSnapshot `json:"workers"`
	Tasks          []Task           `json:"tasks"`
	Pending        int              `json:"pending"`
	InProgress     int              `json:"in_progress"`
	Completed      int              `json:"completed"`
	Failed         int              `json:"failed"`
	Blocked        int              `json:"blocked"`
	IdleWorkers    int              `json:"idle_workers"`
	RunningWorkers int              `json:"running_workers"`
	StalledWorkers int              `json:"stalled_workers"`
	Active         bool             `json:"active"`
	IntegratedDir  string           `json:"integrated_dir,omitempty"`
}

type StartOptions struct {
	ProjectDir       string
	TeamID           string
	Workers          int
	Roles            []string
	Engine           string
	GitIsolationMode IsolationMode
	ExecutablePath   string
	TaskFile         string
	MaxWorkers       int
}

type ScaleOptions struct {
	ProjectDir string
	TeamID     string
	Add        int
	Remove     string
}

type WorkerRunOptions struct {
	ProjectDir string
	TeamID     string
	WorkerID   string
	Once       bool
}
