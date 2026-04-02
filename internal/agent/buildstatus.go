package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/yevgetman/fry/internal/config"
)

// BuildStatus is a machine-readable snapshot of the entire build state,
// written atomically to .fry/build-status.json after every meaningful
// state change. Designed for agent LLMs to poll instead of parsing CLI output.
type BuildStatus struct {
	Version    int               `json:"version"`
	UpdatedAt  time.Time         `json:"updated_at"`
	Build      BuildInfo         `json:"build"`
	Sprints    []SprintStatus    `json:"sprints"`
	BuildAudit *BuildAuditStatus `json:"build_audit,omitempty"`
}

// BuildInfo contains the top-level build metadata.
type BuildInfo struct {
	Epic          string    `json:"epic"`
	Effort        string    `json:"effort"`
	Engine        string    `json:"engine"`
	Mode          string    `json:"mode"`
	GitBranch     string    `json:"git_branch,omitempty"`
	TotalSprints  int       `json:"total_sprints"`
	CurrentSprint int       `json:"current_sprint"`
	Status        string    `json:"status"`          // running, completed, failed, paused, triaging, preparing
	Phase         string    `json:"phase,omitempty"` // triage, prepare, sprint, audit, complete
	StartedAt     time.Time `json:"started_at"`
}

// SprintStatus captures the state of a single sprint.
type SprintStatus struct {
	Number           int                `json:"number"`
	Name             string             `json:"name"`
	Status           string             `json:"status"` // running, PASS, PASS (aligned), FAIL, SKIPPED, etc.
	StartedAt        *time.Time         `json:"started_at,omitempty"`
	FinishedAt       *time.Time         `json:"finished_at,omitempty"`
	DurationSec      float64            `json:"duration_sec,omitempty"`
	SanityChecks     *SanityCheckStatus `json:"sanity_checks,omitempty"`
	Alignment        *AlignmentStatus   `json:"alignment,omitempty"`
	Audit            *AuditStatus       `json:"audit,omitempty"`
	Review           *ReviewStatus      `json:"review,omitempty"`
	DeferredFailures int                `json:"deferred_failures,omitempty"`
	Warnings         []string           `json:"warnings,omitempty"`
}

// SanityCheckStatus summarizes sanity check results for a sprint.
type SanityCheckStatus struct {
	Passed  int                `json:"passed"`
	Total   int                `json:"total"`
	Results []CheckResultEntry `json:"results,omitempty"`
}

// CheckResultEntry is a single sanity check outcome.
type CheckResultEntry struct {
	Type   string `json:"type"`   // FILE, FILE_CONTAINS, CMD, CMD_OUTPUT, TEST
	Target string `json:"target"` // file path or command
	Passed bool   `json:"passed"`
	Output string `json:"output,omitempty"` // truncated check output (failures only)
}

// AlignmentStatus summarizes the alignment (heal) loop for a sprint.
type AlignmentStatus struct {
	Attempts int    `json:"attempts"`
	Outcome  string `json:"outcome"` // healed, exhausted, within_threshold, not_needed
}

type AuditMetricsSnapshot struct {
	TotalCalls        int     `json:"total_calls"`
	DurationMs        int64   `json:"duration_ms"`
	NoOpFixCalls      int     `json:"no_op_fix_calls"`
	NoOpRate          float64 `json:"no_op_rate"`
	VerifyCalls       int     `json:"verify_calls"`
	VerifyResolutions int     `json:"verify_resolutions"`
	VerifyYield       float64 `json:"verify_yield"`
}

// AuditStatus summarizes the per-sprint audit results.
type AuditStatus struct {
	Cycles         int                   `json:"cycles"`
	Findings       map[string]int        `json:"findings"` // severity -> count
	Outcome        string                `json:"outcome"`  // pass, failed, advisory, running
	Active         bool                  `json:"active,omitempty"`
	Stage          string                `json:"stage,omitempty"`           // auditing, fixing, verifying
	CurrentCycle   int                   `json:"current_cycle,omitempty"`   // current outer audit cycle
	MaxCycles      int                   `json:"max_cycles,omitempty"`      // configured outer audit cycle cap
	CurrentFix     int                   `json:"current_fix,omitempty"`     // current inner fix/verify iteration
	MaxFixes       int                   `json:"max_fixes,omitempty"`       // configured inner fix cap
	TargetIssues   int                   `json:"target_issues,omitempty"`   // issues currently being fixed/verified
	IssueHeadlines []string              `json:"issue_headlines,omitempty"` // compact descriptions of targeted issues
	Reopenings     int                   `json:"reopenings,omitempty"`      // findings suppressed as probable reopenings
	Complexity     string                `json:"complexity,omitempty"`
	Metrics        *AuditMetricsSnapshot `json:"metrics,omitempty"`
}

// ReviewStatus captures the sprint review verdict.
type ReviewStatus struct {
	Verdict string `json:"verdict"` // CONTINUE, DEVIATE
}

// BuildAuditStatus captures the final build-level audit results.
type BuildAuditStatus struct {
	Ran      bool           `json:"ran"`
	Passed   bool           `json:"passed"`
	Blocking bool           `json:"blocking"`
	Findings map[string]int `json:"findings,omitempty"`
}

// WriteBuildStatus atomically writes the build status to .fry/build-status.json.
// It writes to a temporary file first, then renames to avoid partial reads.
func WriteBuildStatus(projectDir string, status *BuildStatus) error {
	status.UpdatedAt = time.Now()
	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal build status: %w", err)
	}
	data = append(data, '\n')
	path := filepath.Join(projectDir, config.BuildStatusFile)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create status dir: %w", err)
	}
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return fmt.Errorf("write status tmp: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath) // clean up on rename failure
		return fmt.Errorf("rename status file: %w", err)
	}
	return nil
}

// ReadBuildStatus reads and parses .fry/build-status.json.
// Returns nil, nil if the file does not exist.
func ReadBuildStatus(projectDir string) (*BuildStatus, error) {
	path := filepath.Join(projectDir, config.BuildStatusFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read build status: %w", err)
	}
	var status BuildStatus
	if err := json.Unmarshal(data, &status); err != nil {
		return nil, fmt.Errorf("parse build status: %w", err)
	}
	return &status, nil
}
