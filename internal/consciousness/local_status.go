package consciousness

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/yevgetman/fry/internal/config"
)

// ReadLocalStatus reads the project-local consciousness session state.
func ReadLocalStatus(projectDir string) (*localStatus, error) {
	state, err := loadSessionState(filepath.Join(projectDir, config.ConsciousnessSessionFile))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read local consciousness status: %w", err)
	}

	return &localStatus{
		SessionID:              state.SessionID,
		Status:                 state.Status,
		CurrentSprint:          state.CurrentSprint,
		TotalSprints:           state.TotalSprints,
		CheckpointsPersisted:   state.CheckpointsPersisted,
		CheckpointSummaries:    len(loadCheckpointSummaries(filepath.Join(projectDir, config.ConsciousnessDistilledDir))),
		ParseFailures:          state.ParseFailures,
		RepairSuccesses:        state.RepairSuccesses,
		DistillationsSucceeded: state.DistillationsSucceeded,
		DistillationsFailed:    state.DistillationsFailed,
		UploadAttempts:         state.UploadAttempts,
		UploadSuccesses:        state.UploadSuccesses,
		PendingUploads:         countJSONFiles(filepath.Join(projectDir, config.ConsciousnessUploadQueueDir)),
		SessionResumedCount:    state.SessionResumedCount,
		LastUpdatedAt:          state.LastUpdatedAt,
		LastFlushedAt:          state.LastFlushedAt,
	}, nil
}

// FormatLocalStatus renders local consciousness health for `fry status --consciousness`.
func FormatLocalStatus(status *localStatus) string {
	if status == nil {
		return "No consciousness session found.\n"
	}

	var b strings.Builder
	b.WriteString("Consciousness Session\n")
	fmt.Fprintf(&b, "  Session ID:            %s\n", status.SessionID)
	fmt.Fprintf(&b, "  Status:                %s\n", status.Status)
	if status.TotalSprints > 0 {
		fmt.Fprintf(&b, "  Current sprint:        %d / %d\n", status.CurrentSprint, status.TotalSprints)
	}
	fmt.Fprintf(&b, "  Checkpoints persisted: %d\n", status.CheckpointsPersisted)
	fmt.Fprintf(&b, "  Checkpoint summaries:  %d\n", status.CheckpointSummaries)
	fmt.Fprintf(&b, "  Parse failures:        %d\n", status.ParseFailures)
	fmt.Fprintf(&b, "  Repair successes:      %d\n", status.RepairSuccesses)
	fmt.Fprintf(&b, "  Distillations:         %d ok, %d failed\n", status.DistillationsSucceeded, status.DistillationsFailed)
	fmt.Fprintf(&b, "  Uploads:               %d attempts, %d succeeded, %d pending\n", status.UploadAttempts, status.UploadSuccesses, status.PendingUploads)
	fmt.Fprintf(&b, "  Session resumes:       %d\n", status.SessionResumedCount)
	if !status.LastUpdatedAt.IsZero() {
		fmt.Fprintf(&b, "  Last updated:          %s\n", status.LastUpdatedAt.Format("2006-01-02 15:04:05 MST"))
	}
	if !status.LastFlushedAt.IsZero() {
		fmt.Fprintf(&b, "  Last flush:            %s\n", status.LastFlushedAt.Format("2006-01-02 15:04:05 MST"))
	}
	return b.String()
}
