package steering

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/yevgetman/fry/internal/config"
)

const (
	ResumeVerdictResume          = "RESUME"
	ResumeVerdictContinueNext    = "CONTINUE_NEXT"
	ResumeVerdictAuditIncomplete = "AUDIT_INCOMPLETE"
)

// StopRequest records a graceful-stop request from either `fry exit` or the
// legacy pause sentinel.
type StopRequest struct {
	Source      string
	RequestedAt time.Time
}

// ExitRequest is the persisted request written by `fry exit`.
type ExitRequest struct {
	Version     int       `json:"version"`
	RequestedAt time.Time `json:"requested_at"`
}

// ResumePoint is the structured checkpoint that `--continue` and
// `--simple-continue` can use as a deterministic pickup point.
type ResumePoint struct {
	Version            int       `json:"version"`
	Source             string    `json:"source,omitempty"`
	RequestedAt        time.Time `json:"requested_at,omitempty"`
	SettledAt          time.Time `json:"settled_at"`
	Sprint             int       `json:"sprint,omitempty"`
	SprintName         string    `json:"sprint_name,omitempty"`
	Phase              string    `json:"phase"`
	Verdict            string    `json:"verdict"`
	Reason             string    `json:"reason"`
	RecommendedCommand string    `json:"recommended_command,omitempty"`
}

// ExitRequestError signals that a graceful stop was requested and the caller
// should return to a stable checkpoint instead of continuing work.
type ExitRequestError struct {
	Phase  string
	Detail string
}

func (e *ExitRequestError) Error() string {
	if e == nil {
		return "graceful exit requested"
	}
	if e.Detail == "" {
		return fmt.Sprintf("graceful exit requested during %s", e.Phase)
	}
	return fmt.Sprintf("graceful exit requested during %s: %s", e.Phase, e.Detail)
}

// NewExitRequestError constructs an ExitRequestError with phase/detail context.
func NewExitRequestError(phase, detail string) error {
	return &ExitRequestError{
		Phase:  phase,
		Detail: detail,
	}
}

// RequestExit atomically writes a structured graceful-exit request.
// Returns created=false when a request already exists.
func RequestExit(projectDir string) (created bool, err error) {
	path := filepath.Join(projectDir, config.ExitRequestFile)
	if _, statErr := os.Stat(path); statErr == nil {
		return false, nil
	} else if !os.IsNotExist(statErr) {
		return false, fmt.Errorf("request exit: %w", statErr)
	}

	req := ExitRequest{
		Version:     1,
		RequestedAt: time.Now().UTC(),
	}
	data, err := json.MarshalIndent(req, "", "  ")
	if err != nil {
		return false, fmt.Errorf("request exit: marshal: %w", err)
	}
	if err := writeJSONAtomic(path, data); err != nil {
		return false, fmt.Errorf("request exit: %w", err)
	}
	return true, nil
}

// ReadExitRequest returns the structured exit request, or nil when absent.
func ReadExitRequest(projectDir string) (*ExitRequest, error) {
	path := filepath.Join(projectDir, config.ExitRequestFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read exit request: %w", err)
	}

	var req ExitRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, fmt.Errorf("read exit request: parse: %w", err)
	}
	return &req, nil
}

// ClearExitRequest removes the structured exit request file.
func ClearExitRequest(projectDir string) error {
	path := filepath.Join(projectDir, config.ExitRequestFile)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("clear exit request: %w", err)
	}
	return nil
}

// ReadStopRequest returns the first pending stop request, preferring the
// structured `fry exit` request over the legacy pause sentinel.
func ReadStopRequest(projectDir string) (*StopRequest, error) {
	req, err := ReadExitRequest(projectDir)
	if err != nil {
		return nil, err
	}
	if req != nil {
		return &StopRequest{
			Source:      "exit",
			RequestedAt: req.RequestedAt,
		}, nil
	}

	if IsPaused(projectDir) {
		return &StopRequest{
			Source: "pause",
		}, nil
	}

	return nil, nil
}

// HasStopRequest reports whether either `fry exit` or the pause sentinel has
// requested a graceful stop.
func HasStopRequest(projectDir string) bool {
	req, err := ReadStopRequest(projectDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fry: warning: reading stop request: %v\n", err)
	}
	return err == nil && req != nil
}

// ClearStopRequest removes both the structured exit request and the legacy
// pause sentinel.
func ClearStopRequest(projectDir string) error {
	if err := ClearExitRequest(projectDir); err != nil {
		return err
	}
	if err := ClearPause(projectDir); err != nil {
		return err
	}
	return nil
}

// WriteResumePoint persists the latest structured pause checkpoint.
func WriteResumePoint(projectDir string, point ResumePoint) error {
	if point.Version == 0 {
		point.Version = 1
	}
	if point.SettledAt.IsZero() {
		point.SettledAt = time.Now().UTC()
	}
	path := filepath.Join(projectDir, config.ResumePointFile)
	data, err := json.MarshalIndent(point, "", "  ")
	if err != nil {
		return fmt.Errorf("write resume point: marshal: %w", err)
	}
	if err := writeJSONAtomic(path, data); err != nil {
		return fmt.Errorf("write resume point: %w", err)
	}
	return nil
}

// ReadResumePoint returns the latest resume checkpoint, or nil when absent.
func ReadResumePoint(projectDir string) (*ResumePoint, error) {
	path := filepath.Join(projectDir, config.ResumePointFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read resume point: %w", err)
	}

	var point ResumePoint
	if err := json.Unmarshal(data, &point); err != nil {
		return nil, fmt.Errorf("read resume point: parse: %w", err)
	}
	return &point, nil
}

// ClearResumePoint removes the latest structured resume checkpoint.
func ClearResumePoint(projectDir string) error {
	path := filepath.Join(projectDir, config.ResumePointFile)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("clear resume point: %w", err)
	}
	return nil
}

func writeJSONAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data = append(data, '\n')

	tmp, err := os.CreateTemp(filepath.Dir(path), "fry-steering-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}
