package consciousness

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yevgetman/fry/internal/config"
)

func TestReadLocalStatus(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, writeJSONAtomic(filepath.Join(dir, config.ConsciousnessSessionFile), SessionState{
		SessionID:              "session-1",
		Status:                 SessionStatusRunning,
		CurrentSprint:          2,
		TotalSprints:           4,
		CheckpointsPersisted:   3,
		ParseFailures:          1,
		RepairSuccesses:        2,
		DistillationsSucceeded: 2,
		DistillationsFailed:    1,
		UploadAttempts:         5,
		UploadSuccesses:        4,
		SessionResumedCount:    1,
		LastUpdatedAt:          time.Now().UTC(),
		LastFlushedAt:          time.Now().UTC(),
	}))
	require.NoError(t, writeJSONAtomic(filepath.Join(dir, config.ConsciousnessDistilledDir, "0001.json"), CheckpointSummary{
		SessionID: "session-1",
		Sequence:  1,
		Summary:   "checkpoint one",
	}))
	require.NoError(t, writeJSONAtomic(filepath.Join(dir, config.ConsciousnessUploadQueueDir, "pending.json"), UploadEnvelope{
		ID:        "pending",
		EventType: UploadEventCheckpointSummary,
	}))

	status, err := ReadLocalStatus(dir)
	require.NoError(t, err)
	require.NotNil(t, status)
	assert.Equal(t, 3, status.CheckpointsPersisted)
	assert.Equal(t, 1, status.CheckpointSummaries)
	assert.Equal(t, 1, status.PendingUploads)
	assert.Contains(t, FormatLocalStatus(status), "Parse failures:        1")
}
