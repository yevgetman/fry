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

// stubEngine implements engine.Engine for testing.
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

func TestBuildExperiencePrompt(t *testing.T) {
	t.Parallel()

	opts := SummarizeOpts{
		Record: BuildRecord{
			Engine:       "claude",
			EffortLevel:  "high",
			TotalSprints: 3,
			Observations: []BuildObservation{
				{Timestamp: time.Now(), WakePoint: "after_sprint", SprintNum: 1, Thoughts: "Sprint 1 completed cleanly with no issues."},
				{Timestamp: time.Now(), WakePoint: "build_end", SprintNum: 3, Thoughts: "Build finished successfully."},
			},
		},
		SprintResults: []SprintOutcome{
			{Number: 1, Name: "Add auth", Status: "PASS", HealAttempts: 0},
			{Number: 2, Name: "Add tests", Status: "PASS (aligned)", HealAttempts: 2},
			{Number: 3, Name: "Add docs", Status: "PASS", HealAttempts: 0},
		},
		BuildOutcome: "success",
	}

	prompt := buildExperiencePrompt(opts)

	// Metadata present
	assert.Contains(t, prompt, "claude")
	assert.Contains(t, prompt, "high")
	assert.Contains(t, prompt, "3")
	assert.Contains(t, prompt, "success")

	// Sprint results present
	assert.Contains(t, prompt, "Add auth")
	assert.Contains(t, prompt, "PASS (aligned)")
	assert.Contains(t, prompt, "Add tests")

	// Observations present
	assert.Contains(t, prompt, "Sprint 1 completed cleanly")
	assert.Contains(t, prompt, "Build finished successfully")

	// Instructions present
	assert.Contains(t, prompt, "Synthesize")
	assert.Contains(t, prompt, "Do NOT include project-specific")
}

func TestBuildExperiencePrompt_EmptyObservations(t *testing.T) {
	t.Parallel()

	opts := SummarizeOpts{
		Record: BuildRecord{
			Engine:       "codex",
			EffortLevel:  "fast",
			TotalSprints: 1,
		},
		BuildOutcome: "success",
	}

	prompt := buildExperiencePrompt(opts)

	assert.Contains(t, prompt, "codex")
	assert.Contains(t, prompt, "No observations were recorded")
	assert.Contains(t, prompt, "Synthesize")
}

func TestSummarizeExperience_NilEngine(t *testing.T) {
	t.Parallel()

	_, err := SummarizeExperience(context.Background(), SummarizeOpts{
		ProjectDir: t.TempDir(),
		Engine:     nil,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "engine is required")
}

func TestSummarizeExperience_ContextCancelled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := SummarizeExperience(ctx, SummarizeOpts{
		ProjectDir: t.TempDir(),
		Engine:     &stubEngine{output: "summary"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "context canceled")
}

func TestSummarizeExperience_Success(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	eng := &stubEngine{output: "This build went smoothly with three sprints completing without issues."}

	summary, err := SummarizeExperience(context.Background(), SummarizeOpts{
		ProjectDir: dir,
		Engine:     eng,
		Record: BuildRecord{
			Engine:       "claude",
			EffortLevel:  "standard",
			TotalSprints: 2,
		},
		BuildOutcome: "success",
	})
	require.NoError(t, err)
	assert.Contains(t, summary, "three sprints")
}

func TestSummarizeExperience_EmptyOutput(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	eng := &stubEngine{output: ""}

	_, err := SummarizeExperience(context.Background(), SummarizeOpts{
		ProjectDir: dir,
		Engine:     eng,
		Record: BuildRecord{
			Engine:       "claude",
			EffortLevel:  "standard",
			TotalSprints: 1,
		},
		BuildOutcome: "success",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty output")
}

func TestSummarizeExperience_EngineErrorEmptyOutput(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	eng := &stubEngine{output: "", err: errors.New("engine down")}

	result, err := SummarizeExperience(context.Background(), SummarizeOpts{
		ProjectDir: dir,
		Engine:     eng,
	})

	assert.Equal(t, "", result)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "engine down")
}

func TestSummarizeExperience_EngineErrorWithOutput(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	eng := &stubEngine{output: "partial output", err: errors.New("engine degraded")}

	result, err := SummarizeExperience(context.Background(), SummarizeOpts{
		ProjectDir: dir,
		Engine:     eng,
	})

	// Sprint 1 fix: partial output must be rejected when engine returns an error.
	assert.Equal(t, "", result)
	require.Error(t, err)
}

func TestSummarizeExperience_Verbose(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	eng := &stubEngine{output: "summary text"}
	var buf bytes.Buffer

	result, err := SummarizeExperience(context.Background(), SummarizeOpts{
		ProjectDir: dir,
		Engine:     eng,
		Verbose:    true,
		Stdout:     &buf,
	})

	require.NoError(t, err)
	assert.Equal(t, "summary text", result)
	assert.NotEmpty(t, buf.String())
}
