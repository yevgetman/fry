package report

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/yevgetman/fry/internal/metrics"
)

// BuildReport is the top-level JSON build report written by --json-report.
type BuildReport struct {
	EpicName  string        `json:"epic_name"`
	StartTime time.Time     `json:"start_time"`
	EndTime   time.Time     `json:"end_time"`
	Duration  time.Duration `json:"duration_ns"`
	Sprints   []SprintResult `json:"sprints"`
}

// SprintResult captures the outcome of a single sprint within the build.
type SprintResult struct {
	SprintNum    int                 `json:"sprint_num"`
	Name         string              `json:"name"`
	StartTime    time.Time           `json:"start_time"`
	EndTime      time.Time           `json:"end_time"`
	Passed       bool                `json:"passed"`
	HealAttempts int                 `json:"alignment_attempts"`
	Verification *VerificationResult `json:"verification,omitempty"`
	TokenUsage   *metrics.TokenUsage `json:"token_usage,omitempty"`
}

// VerificationResult summarises the sanity checks run for a sprint.
type VerificationResult struct {
	TotalChecks  int           `json:"total_checks"`
	PassedChecks int           `json:"passed_checks"`
	FailedChecks int           `json:"failed_checks"`
	CheckResults []CheckResult `json:"check_results,omitempty"`
}

// CheckResult records the outcome of a single sanity check.
type CheckResult struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Passed  bool   `json:"passed"`
	Message string `json:"message,omitempty"`
}

// Write marshals report to indented JSON and writes it atomically to path.
// The file is first written to a temp file in the same directory, then renamed
// to path so that build-report.json is either absent or fully written — never partial.
// The parent directory is created if it does not exist.
func Write(path string, r BuildReport) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("report: create directory: %w", err)
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("report: marshal: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".build-report-*.json")
	if err != nil {
		return fmt.Errorf("report: create temp file: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("report: write temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("report: sync temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("report: close temp file: %w", err)
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("report: chmod temp file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("report: rename temp file: %w", err)
	}
	return nil
}
