package consciousness

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yevgetman/fry/internal/config"
)

func TestNewCollector_NewSession(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	c, err := NewCollector(CollectorOptions{
		ProjectDir:   dir,
		EpicName:     "Test Epic",
		Engine:       "claude",
		EffortLevel:  "high",
		TotalSprints: 3,
		Mode:         SessionModeNew,
	})
	require.NoError(t, err)

	assert.NotEmpty(t, c.BuildID())
	assert.Equal(t, c.BuildID(), c.SessionID())
	assert.Equal(t, 0, c.ObservationCount())

	state := c.GetSessionState()
	assert.Equal(t, SessionStatusRunning, state.Status)
	assert.Equal(t, 0, state.CheckpointsPersisted)

	_, err = os.Stat(filepath.Join(dir, config.ConsciousnessSessionFile))
	require.NoError(t, err)
}

func TestNewCollector_ResumeSession(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	c1, err := NewCollector(CollectorOptions{
		ProjectDir:   dir,
		EpicName:     "Test Epic",
		Engine:       "claude",
		EffortLevel:  "high",
		TotalSprints: 3,
		Mode:         SessionModeNew,
	})
	require.NoError(t, err)

	checkpoint, err := c1.AddCheckpoint(ObservationCheckpoint{
		WakePoint:       "after_sprint",
		SprintNum:       1,
		ParseStatus:     ParseStatusOK,
		ScratchpadDelta: "watch audit loops",
		Observation: &BuildObservation{
			Timestamp: time.Now().UTC(),
			WakePoint: "after_sprint",
			SprintNum: 1,
			Thoughts:  "Sprint 1 stayed on track.",
		},
	})
	require.NoError(t, err)
	require.NoError(t, c1.RecordDistillation(CheckpointSummary{
		Sequence:       checkpoint.Sequence,
		CheckpointType: checkpoint.CheckpointType,
		Summary:        "Sprint 1 stayed on track with low risk.",
	}))

	c2, err := NewCollector(CollectorOptions{
		ProjectDir:    dir,
		EpicName:      "Test Epic",
		Engine:        "claude",
		EffortLevel:   "high",
		TotalSprints:  3,
		Mode:          SessionModeResume,
		CurrentSprint: 2,
	})
	require.NoError(t, err)

	assert.Equal(t, c1.BuildID(), c2.BuildID())
	assert.Equal(t, 1, c2.ObservationCount())
	assert.Len(t, c2.GetRecord().CheckpointSummaries, 1)
	assert.Equal(t, 1, c2.GetSessionState().SessionResumedCount)
	assert.Equal(t, 2, c2.GetSessionState().CurrentSprint)
}

func TestAddCheckpoint_PersistsCanonicalObservationAndScratchpadHistory(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	c, err := NewCollector(CollectorOptions{
		ProjectDir:   dir,
		EpicName:     "Test Epic",
		Engine:       "claude",
		EffortLevel:  "high",
		TotalSprints: 2,
		Mode:         SessionModeNew,
	})
	require.NoError(t, err)

	checkpoint, err := c.AddCheckpoint(ObservationCheckpoint{
		WakePoint:       "after_sprint",
		SprintNum:       1,
		ParseStatus:     ParseStatusRepaired,
		ScratchpadDelta: "tests still need a retry path",
		Observation: &BuildObservation{
			Timestamp: time.Now().UTC(),
			WakePoint: "after_sprint",
			SprintNum: 1,
			Thoughts:  "The sprint completed, but tests still look brittle.",
		},
	})
	require.NoError(t, err)

	assert.Equal(t, 1, c.ObservationCount())
	assert.Equal(t, 1, checkpoint.Sequence)
	assert.Equal(t, 1, c.GetSessionState().RepairSuccesses)

	_, err = os.Stat(filepath.Join(dir, config.ConsciousnessCheckpointsDir, "0001.json"))
	require.NoError(t, err)

	history, err := os.ReadFile(filepath.Join(dir, config.ConsciousnessScratchpadHistory))
	require.NoError(t, err)
	assert.Contains(t, string(history), "tests still need a retry path")
}

func TestAddCheckpoint_ParseFailureQuarantinesObservation(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	c, err := NewCollector(CollectorOptions{
		ProjectDir:   dir,
		EpicName:     "Test Epic",
		Engine:       "claude",
		EffortLevel:  "high",
		TotalSprints: 2,
		Mode:         SessionModeNew,
	})
	require.NoError(t, err)

	checkpoint, err := c.AddCheckpoint(ObservationCheckpoint{
		WakePoint:     "after_sprint",
		SprintNum:     1,
		ParseStatus:   ParseStatusFailed,
		ParseError:    "no valid JSON object found",
		RawOutputPath: ".fry/build-logs/observer_after_sprint.log",
	})
	require.NoError(t, err)

	assert.Equal(t, 0, c.ObservationCount())
	assert.Equal(t, ParseStatusFailed, checkpoint.ParseStatus)
	assert.Equal(t, 1, c.GetRecord().ParseFailures)

	data, err := os.ReadFile(filepath.Join(dir, config.ConsciousnessCheckpointsDir, "0001.json"))
	require.NoError(t, err)
	assert.Contains(t, string(data), `"parse_status": "failed"`)
	assert.Contains(t, string(data), `"raw_output_path": ".fry/build-logs/observer_after_sprint.log"`)
}

func TestFinalize_WritesExtendedBuildRecord(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	experiencesDir := filepath.Join(t.TempDir(), "experiences")
	c, err := NewCollector(CollectorOptions{
		ProjectDir:   dir,
		EpicName:     "Test Epic",
		Engine:       "codex",
		EffortLevel:  "max",
		TotalSprints: 4,
		Mode:         SessionModeNew,
	})
	require.NoError(t, err)
	c.outDir = experiencesDir

	checkpoint, err := c.AddCheckpoint(ObservationCheckpoint{
		WakePoint:   "build_end",
		SprintNum:   4,
		ParseStatus: ParseStatusOK,
		Observation: &BuildObservation{Timestamp: time.Now().UTC(), WakePoint: "build_end", SprintNum: 4, Thoughts: "The build finished with one lingering review concern."},
	})
	require.NoError(t, err)
	require.NoError(t, c.RecordDistillation(CheckpointSummary{
		Sequence:       checkpoint.Sequence,
		CheckpointType: checkpoint.CheckpointType,
		Summary:        "The build closed cleanly after one review concern was resolved.",
		Lessons:        []string{"Checkpoint summaries survive finalization."},
	}))
	c.SetSummary("The build closed cleanly and produced one durable checkpoint summary.")

	require.NoError(t, c.Finalize("success"))

	data, err := os.ReadFile(filepath.Join(experiencesDir, "build-"+c.BuildID()+".json"))
	require.NoError(t, err)

	var record BuildRecord
	require.NoError(t, json.Unmarshal(data, &record))
	assert.Equal(t, "success", record.Outcome)
	assert.Equal(t, 1, record.CheckpointCount)
	assert.Len(t, record.CheckpointSummaries, 1)
	assert.Equal(t, 0, record.ParseFailures)
	assert.False(t, record.EndTime.IsZero())
}

func TestBuildRecord_OldSchemaCompatibility(t *testing.T) {
	t.Parallel()

	oldJSON := `{
	  "id": "legacy-build",
	  "start_time": "2026-03-30T10:00:00Z",
	  "end_time": "2026-03-30T10:05:00Z",
	  "engine": "claude",
	  "effort_level": "high",
	  "total_sprints": 2,
	  "outcome": "success",
	  "observations": [{"timestamp":"2026-03-30T10:01:00Z","wake_point":"build_end","sprint_num":2,"thoughts":"legacy"}],
	  "summary": "legacy summary"
	}`

	var record BuildRecord
	require.NoError(t, json.Unmarshal([]byte(oldJSON), &record))
	assert.Equal(t, "legacy-build", record.ID)
	assert.Equal(t, "legacy summary", record.Summary)
	assert.Len(t, record.Observations, 1)
	assert.Zero(t, record.CheckpointCount)
}
