package engine

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// callTrackingEngine records calls and returns pre-configured outputs/errors.
type callTrackingEngine struct {
	name    string
	calls   int
	outputs []string
	errs    []error
}

func (e *callTrackingEngine) Run(ctx context.Context, prompt string, opts RunOpts) (string, int, error) {
	select {
	case <-ctx.Done():
		return "", 1, ctx.Err()
	default:
	}
	idx := e.calls
	e.calls++
	if idx >= len(e.outputs) {
		return "", 0, nil
	}
	return e.outputs[idx], 0, e.errs[idx]
}

func (e *callTrackingEngine) Name() string {
	if e.name != "" {
		return e.name
	}
	return "claude"
}

// tinyBackoff returns options for fast tests.
func tinyBackoff() []ResilientOpt {
	return []ResilientOpt{
		WithMaxRetries(3),
		WithBaseDelay(1 * time.Millisecond),
		WithMaxDelay(10 * time.Millisecond),
		WithJitter(0),
	}
}

func TestResilientEngine_NoRateLimit(t *testing.T) {
	t.Parallel()

	inner := &callTrackingEngine{
		outputs: []string{"good output"},
		errs:    []error{nil},
	}
	eng := NewResilientEngine(inner, tinyBackoff()...)

	output, exitCode, err := eng.Run(context.Background(), "prompt", RunOpts{})
	require.NoError(t, err)
	assert.Equal(t, "good output", output)
	assert.Equal(t, 0, exitCode)
	assert.Equal(t, 1, inner.calls, "should call engine exactly once")
}

func TestResilientEngine_RetryThenSucceed(t *testing.T) {
	t.Parallel()

	inner := &callTrackingEngine{
		outputs: []string{"rate_limit exceeded", "success output"},
		errs:    []error{fmt.Errorf("rate_limit exceeded"), nil},
	}
	eng := NewResilientEngine(inner, tinyBackoff()...)

	output, _, err := eng.Run(context.Background(), "prompt", RunOpts{})
	require.NoError(t, err)
	assert.Equal(t, "success output", output)
	assert.Equal(t, 2, inner.calls, "should call engine twice")
}

func TestResilientEngine_MaxRetriesExhausted(t *testing.T) {
	t.Parallel()

	rlErr := fmt.Errorf("429 rate limit")
	inner := &callTrackingEngine{
		outputs: []string{"429 rate limit", "429 rate limit", "429 rate limit", "429 rate limit"},
		errs:    []error{rlErr, rlErr, rlErr, rlErr},
	}
	eng := NewResilientEngine(inner, tinyBackoff()...)

	output, _, err := eng.Run(context.Background(), "prompt", RunOpts{})
	require.Error(t, err)
	assert.Equal(t, "429 rate limit", output)
	assert.Equal(t, 4, inner.calls, "maxRetries=3 means 1 initial + 3 retries = 4 total")
}

func TestResilientEngine_ContextCancelledDuringBackoff(t *testing.T) {
	t.Parallel()

	inner := &callTrackingEngine{
		outputs: []string{"rate_limit"},
		errs:    []error{fmt.Errorf("rate_limit")},
	}
	// Use a longer backoff so context cancels before retry fires.
	eng := NewResilientEngine(inner,
		WithMaxRetries(3),
		WithBaseDelay(500*time.Millisecond),
		WithMaxDelay(1*time.Second),
		WithJitter(0),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, _, err := eng.Run(ctx, "prompt", RunOpts{})
	require.Error(t, err)
	assert.True(t, errors.Is(err, context.DeadlineExceeded), "expected deadline exceeded, got: %v", err)
	assert.Equal(t, 1, inner.calls, "should not have retried before context expired")
}

func TestResilientEngine_RetryAfterHonored(t *testing.T) {
	t.Parallel()

	inner := &callTrackingEngine{
		outputs: []string{"rate_limit retry-after: 1", "ok"},
		errs:    []error{fmt.Errorf("rate_limit retry-after: 1"), nil},
	}
	eng := NewResilientEngine(inner,
		WithMaxRetries(3),
		WithBaseDelay(1*time.Millisecond),
		WithMaxDelay(2*time.Second), // must be >= retry-after (1s) to honor it
		WithJitter(0),
	)

	start := time.Now()
	output, _, err := eng.Run(context.Background(), "prompt", RunOpts{})
	elapsed := time.Since(start)

	require.NoError(t, err)
	assert.Equal(t, "ok", output)
	assert.Equal(t, 2, inner.calls)
	// retry-after: 1 means 1 second; verify we waited at least ~900ms
	assert.Greater(t, elapsed, 900*time.Millisecond, "should honor retry-after delay")
}

func TestResilientEngine_OllamaNoRetry(t *testing.T) {
	t.Parallel()

	inner := &callTrackingEngine{
		name:    "ollama",
		outputs: []string{"429 rate_limit"},
		errs:    []error{fmt.Errorf("429 rate_limit")},
	}
	eng := NewResilientEngine(inner, tinyBackoff()...)

	_, _, err := eng.Run(context.Background(), "prompt", RunOpts{})
	require.Error(t, err)
	assert.Equal(t, 1, inner.calls, "ollama should not retry on rate limit indicators")
}

func TestResilientEngine_NonRateLimitErrorPassesThrough(t *testing.T) {
	t.Parallel()

	inner := &callTrackingEngine{
		outputs: []string{"some output"},
		errs:    []error{fmt.Errorf("connection refused")},
	}
	eng := NewResilientEngine(inner, tinyBackoff()...)

	output, _, err := eng.Run(context.Background(), "prompt", RunOpts{})
	require.Error(t, err)
	assert.Equal(t, "some output", output)
	assert.Equal(t, 1, inner.calls, "non-rate-limit error should not trigger retry")
}

func TestResilientEngine_NameDelegation(t *testing.T) {
	t.Parallel()

	inner := &callTrackingEngine{name: "codex"}
	eng := NewResilientEngine(inner)
	assert.Equal(t, "codex", eng.Name())
}

func TestResilientEngine_InnerAccessor(t *testing.T) {
	t.Parallel()

	inner := &callTrackingEngine{name: "claude"}
	eng := NewResilientEngine(inner)
	assert.Same(t, inner, eng.Inner())
}

func TestResilientEngine_LogFuncCalled(t *testing.T) {
	t.Parallel()

	var logMessages []string
	inner := &callTrackingEngine{
		outputs: []string{"rate_limit", "ok"},
		errs:    []error{fmt.Errorf("rate_limit"), nil},
	}
	eng := NewResilientEngine(inner, append(tinyBackoff(),
		WithLogFunc(func(format string, args ...interface{}) {
			logMessages = append(logMessages, fmt.Sprintf(format, args...))
		}),
	)...)

	_, _, err := eng.Run(context.Background(), "prompt", RunOpts{})
	require.NoError(t, err)
	require.Len(t, logMessages, 1)
	assert.Contains(t, logMessages[0], "Rate limited")
	assert.Contains(t, logMessages[0], "attempt 2/4") // maxRetries=3 → total=4, this is retry 1→attempt 2
}

func TestResilientEngine_PatternCoverage(t *testing.T) {
	t.Parallel()

	patterns := []struct {
		name   string
		output string
	}{
		{"rate_limit", "rate_limit exceeded"},
		{"rate limit", "rate limit hit"},
		{"429", "HTTP 429"},
		{"overloaded", "overloaded_error"},
		{"too many requests", "too many requests"},
	}

	for _, p := range patterns {
		t.Run(p.name, func(t *testing.T) {
			t.Parallel()
			inner := &callTrackingEngine{
				outputs: []string{p.output, "ok"},
				errs:    []error{errors.New(p.output), nil},
			}
			eng := NewResilientEngine(inner, tinyBackoff()...)

			output, _, err := eng.Run(context.Background(), "prompt", RunOpts{})
			require.NoError(t, err)
			assert.Equal(t, "ok", output)
			assert.Equal(t, 2, inner.calls, "pattern %q should trigger retry", p.name)
		})
	}
}

func TestResilientEngine_MaxRetriesZero(t *testing.T) {
	t.Parallel()

	inner := &callTrackingEngine{
		outputs: []string{"rate_limit"},
		errs:    []error{fmt.Errorf("rate_limit")},
	}
	eng := NewResilientEngine(inner,
		WithMaxRetries(0),
		WithBaseDelay(1*time.Millisecond),
		WithJitter(0),
	)

	output, _, err := eng.Run(context.Background(), "prompt", RunOpts{})
	require.Error(t, err)
	assert.Equal(t, "rate_limit", output)
	assert.Equal(t, 1, inner.calls, "maxRetries=0 means single attempt, no retries")
}

func TestResilientEngine_NegativeMaxRetriesClamped(t *testing.T) {
	t.Parallel()

	inner := &callTrackingEngine{
		outputs: []string{"rate_limit"},
		errs:    []error{fmt.Errorf("rate_limit")},
	}
	eng := NewResilientEngine(inner,
		WithMaxRetries(-5),
		WithBaseDelay(1*time.Millisecond),
		WithJitter(0),
	)

	output, _, err := eng.Run(context.Background(), "prompt", RunOpts{})
	require.Error(t, err)
	assert.Equal(t, "rate_limit", output)
	assert.Equal(t, 1, inner.calls, "negative maxRetries clamped to 0, single attempt")
}

func TestResilientEngine_JitterBounds(t *testing.T) {
	t.Parallel()

	eng := &ResilientEngine{
		inner:      &callTrackingEngine{},
		maxRetries: 1,
		baseDelay:  100 * time.Millisecond,
		maxDelay:   10 * time.Second,
		jitter:     0.25,
	}

	// Run many samples to verify jitter bounds.
	minDelay := time.Duration(float64(100*time.Millisecond) * 0.75)
	maxDelay := time.Duration(float64(100*time.Millisecond) * 1.25)

	for i := 0; i < 200; i++ {
		d := eng.calculateDelay(0, 0)
		assert.GreaterOrEqual(t, d, minDelay, "delay below lower bound on iteration %d", i)
		assert.LessOrEqual(t, d, maxDelay, "delay above upper bound on iteration %d", i)
	}
}

func TestResilientEngine_RetryAfterCappedToMaxDelay(t *testing.T) {
	t.Parallel()

	inner := &callTrackingEngine{
		outputs: []string{"rate_limit retry-after: 999999", "ok"},
		errs:    []error{fmt.Errorf("rate_limit retry-after: 999999"), nil},
	}
	eng := NewResilientEngine(inner,
		WithMaxRetries(3),
		WithBaseDelay(1*time.Millisecond),
		WithMaxDelay(50*time.Millisecond),
		WithJitter(0),
	)

	start := time.Now()
	output, _, err := eng.Run(context.Background(), "prompt", RunOpts{})
	elapsed := time.Since(start)

	require.NoError(t, err)
	assert.Equal(t, "ok", output)
	// If retry-after were not capped, we'd wait ~11 days. With cap at 50ms, should be fast.
	assert.Less(t, elapsed, 200*time.Millisecond, "retry-after should be capped to maxDelay")
}

func TestNewResilientEngineFactory(t *testing.T) {
	t.Parallel()

	factory := NewResilientEngineFactory(tinyBackoff()...)

	eng, err := factory("claude")
	require.NoError(t, err)
	assert.Equal(t, "claude", eng.Name())

	resilient, ok := eng.(*ResilientEngine)
	require.True(t, ok, "factory should return *ResilientEngine")
	assert.Equal(t, "claude", resilient.Inner().Name())
}

func TestNewResilientEngineFactory_InvalidEngine(t *testing.T) {
	t.Parallel()

	factory := NewResilientEngineFactory()
	_, err := factory("nonexistent")
	require.Error(t, err)
}
