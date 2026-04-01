package engine

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type failoverResult struct {
	output   string
	exitCode int
	err      error
}

type failoverStubEngine struct {
	name       string
	results    []failoverResult
	runCalls   int
	runOpts    []RunOpts
	lastPrompt string
}

func (s *failoverStubEngine) Run(_ context.Context, prompt string, opts RunOpts) (string, int, error) {
	s.runCalls++
	s.lastPrompt = prompt
	s.runOpts = append(s.runOpts, opts)
	if len(s.results) == 0 {
		return "", 0, nil
	}
	result := s.results[0]
	if len(s.results) > 1 {
		s.results = s.results[1:]
	}
	return result.output, result.exitCode, result.err
}

func (s *failoverStubEngine) Name() string {
	return s.name
}

func TestFailoverEnginePromotesFallbackAndSticks(t *testing.T) {
	t.Parallel()

	primary := &failoverStubEngine{
		name: "claude",
		results: []failoverResult{
			{output: "rate limit exceeded", exitCode: 1, err: errors.New("429 too many requests")},
		},
	}
	fallback := &failoverStubEngine{
		name: "codex",
		results: []failoverResult{
			{output: "fallback ok", exitCode: 0, err: nil},
			{output: "still on fallback", exitCode: 0, err: nil},
		},
	}

	var switchedFrom, switchedTo string
	eng := NewFailoverEngine(primary, fallback, WithFailoverSwitchFunc(func(from, to string) {
		switchedFrom = from
		switchedTo = to
	}))

	opts := RunOpts{
		Model:       "sonnet",
		SessionType: SessionSprint,
		EffortLevel: "standard",
	}

	output, exitCode, err := eng.Run(context.Background(), "build it", opts)
	require.NoError(t, err)
	assert.Equal(t, "fallback ok", output)
	assert.Equal(t, 0, exitCode)
	assert.Equal(t, "claude", switchedFrom)
	assert.Equal(t, "codex", switchedTo)
	assert.Equal(t, "codex", eng.Name())
	assert.Equal(t, 1, primary.runCalls)
	assert.Equal(t, 1, fallback.runCalls)
	require.Len(t, fallback.runOpts, 1)
	assert.Equal(t, ResolveModelForSession("codex", "standard", SessionSprint), fallback.runOpts[0].Model)

	output, exitCode, err = eng.Run(context.Background(), "second call", opts)
	require.NoError(t, err)
	assert.Equal(t, "still on fallback", output)
	assert.Equal(t, 0, exitCode)
	assert.Equal(t, 1, primary.runCalls)
	assert.Equal(t, 2, fallback.runCalls)
	require.Len(t, fallback.runOpts, 2)
	assert.Equal(t, ResolveModelForSession("codex", "standard", SessionSprint), fallback.runOpts[1].Model)
	assert.Equal(t, "second call", fallback.lastPrompt)
}

func TestFailoverEngineDoesNotSwitchOnDeterministicError(t *testing.T) {
	t.Parallel()

	primary := &failoverStubEngine{
		name: "claude",
		results: []failoverResult{
			{output: "invalid model", exitCode: 1, err: errors.New("invalid model")},
		},
	}
	fallback := &failoverStubEngine{name: "codex"}
	eng := NewFailoverEngine(primary, fallback)

	output, exitCode, err := eng.Run(context.Background(), "build it", RunOpts{
		Model:       "sonnet",
		SessionType: SessionReview,
		EffortLevel: "high",
	})

	require.Error(t, err)
	assert.Equal(t, "invalid model", output)
	assert.Equal(t, 1, exitCode)
	assert.Equal(t, "claude", eng.Name())
	assert.Equal(t, 1, primary.runCalls)
	assert.Zero(t, fallback.runCalls)
}

func TestAdaptRunOptsForEngineClearsInvalidModelWithoutSession(t *testing.T) {
	t.Parallel()

	opts, changed := AdaptRunOptsForEngine("codex", RunOpts{Model: "sonnet"})
	assert.True(t, changed)
	assert.Empty(t, opts.Model)
}

func TestDetectFailoverCondition(t *testing.T) {
	t.Parallel()

	rateLimit := DetectFailoverCondition("claude", "rate limit exceeded", errors.New("429"))
	assert.True(t, rateLimit.Detected)
	assert.Equal(t, "rate limit", rateLimit.Reason)

	timeout := DetectFailoverCondition("codex", "", errors.New("deadline exceeded"))
	assert.True(t, timeout.Detected)
	assert.Equal(t, "timeout", timeout.Reason)

	auth := DetectFailoverCondition("claude", "authentication expired", errors.New("exit status 1"))
	assert.False(t, auth.Detected)
}
