package consciousness

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yevgetman/fry/internal/engine"
)

type stubEngine struct {
	output string
	err    error
}

func (s *stubEngine) Run(_ context.Context, prompt string, opts engine.RunOpts) (string, int, error) {
	if opts.Stdout != nil {
		_, _ = opts.Stdout.Write([]byte(s.output))
	}
	return s.output, 0, s.err
}

func (s *stubEngine) Name() string { return "stub" }

func TestBuildExperiencePrompt_UsesCheckpointSummaries(t *testing.T) {
	t.Parallel()

	prompt := buildExperiencePrompt(SummarizeOpts{
		Record: BuildRecord{
			Engine:          "claude",
			EffortLevel:     "high",
			TotalSprints:    3,
			CheckpointCount: 2,
			ParseFailures:   1,
			CheckpointSummaries: []CheckpointSummary{
				{Sequence: 1, CheckpointType: CheckpointTypeObservation, Summary: "Sprint 1 stayed focused.", Lessons: []string{"Keep audit loops bounded."}},
				{Sequence: 2, CheckpointType: CheckpointTypeObservation, Summary: "Sprint 2 slowed on tests.", RiskSignals: []string{"Flaky tests surfaced again."}},
			},
		},
		SprintResults: []SprintOutcome{{Number: 1, Name: "Auth", Status: "PASS", HealAttempts: 0}},
		BuildOutcome:  "success",
	})

	assert.Contains(t, prompt, "Checkpoint Summaries")
	assert.Contains(t, prompt, "Sprint 1 stayed focused.")
	assert.Contains(t, prompt, "Flaky tests surfaced again.")
	assert.NotContains(t, prompt, "Observer Observations")
}

func TestDistillCheckpoint_Success(t *testing.T) {
	t.Parallel()

	summary, err := DistillCheckpoint(context.Background(), DistillOpts{
		ProjectDir:  t.TempDir(),
		Engine:      &stubEngine{output: `{"summary":"Sprint 1 stayed focused and low-risk.","lessons":["Keep prompts bounded."],"risk_signals":["Minor test churn."]}`},
		Model:       "gpt-5.4-mini",
		EffortLevel: "high",
		Record:      BuildRecord{SessionID: "session-1", Engine: "claude", EffortLevel: "high", TotalSprints: 3},
		Checkpoint: ObservationCheckpoint{
			Sequence:       1,
			CheckpointType: CheckpointTypeObservation,
			WakePoint:      "after_sprint",
			SprintNum:      1,
			ParseStatus:    ParseStatusOK,
			Observation: &BuildObservation{
				Timestamp: time.Now().UTC(),
				WakePoint: "after_sprint",
				SprintNum: 1,
				Thoughts:  "Sprint 1 stayed focused and low-risk.",
			},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, 1, summary.Sequence)
	assert.Contains(t, summary.Summary, "focused")
	assert.Equal(t, []string{"Keep prompts bounded."}, summary.Lessons)
}

func TestDistillCheckpoint_RepairsTranscriptPreamble(t *testing.T) {
	t.Parallel()

	summary, err := DistillCheckpoint(context.Background(), DistillOpts{
		ProjectDir:  t.TempDir(),
		Engine:      &stubEngine{output: "Reading prompt from stdin...\n```json\n{\"summary\":\"Checkpoint stayed clean.\",\"lessons\":[\"Use fenced JSON recovery.\"],\"risk_signals\":[]}\n```"},
		Model:       "gpt-5.4-mini",
		EffortLevel: "high",
		Record:      BuildRecord{SessionID: "session-1", Engine: "claude", EffortLevel: "high", TotalSprints: 3},
		Checkpoint: ObservationCheckpoint{
			Sequence:       1,
			CheckpointType: CheckpointTypeObservation,
			WakePoint:      "after_sprint",
			SprintNum:      1,
			ParseStatus:    ParseStatusOK,
			Observation: &BuildObservation{
				Timestamp: time.Now().UTC(),
				WakePoint: "after_sprint",
				SprintNum: 1,
				Thoughts:  "Checkpoint stayed clean.",
			},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "Checkpoint stayed clean.", summary.Summary)
}

func TestSummarizeExperience_Success(t *testing.T) {
	t.Parallel()

	summary, err := SummarizeExperience(context.Background(), SummarizeOpts{
		ProjectDir:  t.TempDir(),
		Engine:      &stubEngine{output: `{"summary":"The build progressed in measured steps, surfaced one recurring testing risk, and converted that experience into durable guidance."}`},
		Model:       "gpt-5.4-mini",
		EffortLevel: "high",
		Record: BuildRecord{
			SessionID:           "session-1",
			Engine:              "claude",
			EffortLevel:         "high",
			TotalSprints:        2,
			CheckpointCount:     2,
			CheckpointSummaries: []CheckpointSummary{{Sequence: 1, Summary: "Sprint 1 stayed focused."}},
		},
		BuildOutcome: "success",
	})
	require.NoError(t, err)
	assert.Contains(t, summary, "measured steps")
}

func TestSummarizeExperience_RequiresCheckpointSummaries(t *testing.T) {
	t.Parallel()

	_, err := SummarizeExperience(context.Background(), SummarizeOpts{
		ProjectDir: t.TempDir(),
		Engine:     &stubEngine{output: `{"summary":"unused"}`},
		Record:     BuildRecord{},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "checkpoint summaries are required")
}

func TestSummarizeExperience_RejectsEngineErrorWithOutput(t *testing.T) {
	t.Parallel()

	result, err := SummarizeExperience(context.Background(), SummarizeOpts{
		ProjectDir: t.TempDir(),
		Engine: &stubEngine{
			output: `{"summary":"partial structured output"}`,
			err:    errors.New("engine degraded"),
		},
		Record: BuildRecord{
			CheckpointSummaries: []CheckpointSummary{{Sequence: 1, Summary: "Checkpoint"}},
		},
	})
	assert.Empty(t, result)
	require.Error(t, err)
}

func TestSummarizeExperience_Verbose(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	result, err := SummarizeExperience(context.Background(), SummarizeOpts{
		ProjectDir: t.TempDir(),
		Engine:     &stubEngine{output: `{"summary":"summary text"}`},
		Record: BuildRecord{
			CheckpointSummaries: []CheckpointSummary{{Sequence: 1, Summary: "Checkpoint"}},
		},
		Verbose: true,
		Stdout:  &buf,
	})
	require.NoError(t, err)
	assert.Equal(t, "summary text", result)
	assert.NotEmpty(t, buf.String())
}
