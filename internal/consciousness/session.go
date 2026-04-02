package consciousness

import "time"

type SessionMode string

const (
	SessionModeNew    SessionMode = "new"
	SessionModeResume SessionMode = "resume"
)

type SessionStatus string

const (
	SessionStatusRunning     SessionStatus = "running"
	SessionStatusInterrupted SessionStatus = "interrupted"
	SessionStatusCompleted   SessionStatus = "completed"
	SessionStatusFailed      SessionStatus = "failed"
)

type ParseStatus string

const (
	ParseStatusOK       ParseStatus = "ok"
	ParseStatusRepaired ParseStatus = "repaired"
	ParseStatusFailed   ParseStatus = "failed"
)

type UploadState string

const (
	UploadStatePending UploadState = "pending"
	UploadStateSent    UploadState = "sent"
	UploadStateFailed  UploadState = "failed"
)

type CheckpointType string

const (
	CheckpointTypeObservation  CheckpointType = "observation"
	CheckpointTypeInterruption CheckpointType = "interruption"
)

// Directive is a structured observer output persisted with checkpoints.
type Directive struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

// RunSegment records one process lifetime inside a logical build session.
type RunSegment struct {
	Index     int        `json:"index"`
	StartedAt time.Time  `json:"started_at"`
	EndedAt   *time.Time `json:"ended_at,omitempty"`
	Status    string     `json:"status"`
	Resumed   bool       `json:"resumed"`
}

// SessionState is the durable runtime state for an in-progress consciousness session.
type SessionState struct {
	SessionID              string        `json:"session_id"`
	BuildID                string        `json:"build_id"`
	ProjectDir             string        `json:"project_dir"`
	EpicName               string        `json:"epic_name"`
	Engine                 string        `json:"engine"`
	EffortLevel            string        `json:"effort_level"`
	TotalSprints           int           `json:"total_sprints"`
	StartedAt              time.Time     `json:"started_at"`
	LastUpdatedAt          time.Time     `json:"last_updated_at"`
	LastFlushedAt          time.Time     `json:"last_flushed_at"`
	Status                 SessionStatus `json:"status"`
	CurrentSprint          int           `json:"current_sprint"`
	LastSequence           int           `json:"last_sequence"`
	CheckpointsPersisted   int           `json:"checkpoints_persisted"`
	ParseFailures          int           `json:"parse_failures"`
	RepairSuccesses        int           `json:"repair_successes"`
	DistillationsSucceeded int           `json:"distillations_succeeded"`
	DistillationsFailed    int           `json:"distillations_failed"`
	UploadAttempts         int           `json:"upload_attempts"`
	UploadSuccesses        int           `json:"upload_successes"`
	PendingUploads         int           `json:"pending_uploads"`
	SessionResumedCount    int           `json:"session_resumed_count"`
	RunSegments            []RunSegment  `json:"run_segments,omitempty"`
}

// ObservationCheckpoint is the durable unit persisted after each observer wake.
type ObservationCheckpoint struct {
	SessionID       string            `json:"session_id"`
	Sequence        int               `json:"sequence"`
	Timestamp       time.Time         `json:"timestamp"`
	CheckpointType  CheckpointType    `json:"checkpoint_type"`
	WakePoint       string            `json:"wake_point"`
	SprintNum       int               `json:"sprint_num,omitempty"`
	ParseStatus     ParseStatus       `json:"parse_status"`
	ParseError      string            `json:"parse_error,omitempty"`
	Observation     *BuildObservation `json:"observation,omitempty"`
	ScratchpadDelta string            `json:"scratchpad_delta,omitempty"`
	Directives      []Directive       `json:"directives,omitempty"`
	RawOutputPath   string            `json:"raw_output_path,omitempty"`
}

// CheckpointSummary is the distilled summary derived from one durable checkpoint.
type CheckpointSummary struct {
	SessionID      string         `json:"session_id"`
	Sequence       int            `json:"sequence"`
	CheckpointType CheckpointType `json:"checkpoint_type"`
	Timestamp      time.Time      `json:"timestamp"`
	Summary        string         `json:"summary"`
	Lessons        []string       `json:"lessons,omitempty"`
	RiskSignals    []string       `json:"risk_signals,omitempty"`
	UploadState    UploadState    `json:"upload_state,omitempty"`
}

type scratchpadHistoryEntry struct {
	SessionID string    `json:"session_id"`
	Sequence  int       `json:"sequence"`
	Timestamp time.Time `json:"timestamp"`
	Delta     string    `json:"delta"`
}

type distillResult struct {
	Summary     string   `json:"summary"`
	Lessons     []string `json:"lessons,omitempty"`
	RiskSignals []string `json:"risk_signals,omitempty"`
}

type summaryResult struct {
	Summary string `json:"summary"`
}

type localStatus struct {
	SessionID              string
	Status                 SessionStatus
	CurrentSprint          int
	TotalSprints           int
	CheckpointsPersisted   int
	CheckpointSummaries    int
	ParseFailures          int
	RepairSuccesses        int
	DistillationsSucceeded int
	DistillationsFailed    int
	UploadAttempts         int
	UploadSuccesses        int
	PendingUploads         int
	SessionResumedCount    int
	LastUpdatedAt          time.Time
	LastFlushedAt          time.Time
}
