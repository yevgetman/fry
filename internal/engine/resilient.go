package engine

import (
	"context"
	"math"
	"math/rand"
	"time"

	"github.com/yevgetman/fry/internal/config"
)

// ResilientEngine wraps an Engine and retries on rate-limit errors with
// exponential backoff and jitter.
type ResilientEngine struct {
	inner      Engine
	maxRetries int
	baseDelay  time.Duration
	maxDelay   time.Duration
	jitter     float64 // 0.0–1.0 fraction of delay to randomize
	logFunc    func(format string, args ...interface{})
	engineOpts []EngineOpt
}

// ResilientOpt configures a ResilientEngine.
type ResilientOpt func(*ResilientEngine)

// WithMaxRetries sets the maximum number of retry attempts.
func WithMaxRetries(n int) ResilientOpt {
	return func(r *ResilientEngine) { r.maxRetries = n }
}

// WithBaseDelay sets the base delay for exponential backoff.
func WithBaseDelay(d time.Duration) ResilientOpt {
	return func(r *ResilientEngine) { r.baseDelay = d }
}

// WithMaxDelay sets the maximum delay cap.
func WithMaxDelay(d time.Duration) ResilientOpt {
	return func(r *ResilientEngine) { r.maxDelay = d }
}

// WithJitter sets the jitter factor (0.0–1.0).
func WithJitter(f float64) ResilientOpt {
	return func(r *ResilientEngine) { r.jitter = f }
}

// WithLogFunc sets the logging function for retry messages.
func WithLogFunc(fn func(string, ...interface{})) ResilientOpt {
	return func(r *ResilientEngine) { r.logFunc = fn }
}

// WithEngineOpts stores EngineOpt values to forward to NewEngine when using
// NewResilientEngineFactory.
func WithEngineOpts(opts ...EngineOpt) ResilientOpt {
	return func(r *ResilientEngine) { r.engineOpts = opts }
}

// NewResilientEngine wraps an engine with rate-limit retry logic.
func NewResilientEngine(inner Engine, opts ...ResilientOpt) *ResilientEngine {
	r := &ResilientEngine{
		inner:      inner,
		maxRetries: config.RateLimitMaxRetries,
		baseDelay:  time.Duration(config.RateLimitBaseDelaySec) * time.Second,
		maxDelay:   time.Duration(config.RateLimitMaxDelaySec) * time.Second,
		jitter:     config.RateLimitJitter,
		logFunc:    func(string, ...interface{}) {}, // no-op default
	}
	for _, opt := range opts {
		opt(r)
	}
	if r.maxRetries < 0 {
		r.maxRetries = 0
	}
	return r
}

// NewResilientEngineFactory returns a factory function that creates
// resilient-wrapped engines. Useful for packages that accept an
// engine factory (e.g., prepare.PrepareOpts.EngineFactory).
func NewResilientEngineFactory(opts ...ResilientOpt) func(string) (Engine, error) {
	probe := &ResilientEngine{}
	for _, opt := range opts {
		opt(probe)
	}
	engineOpts := probe.engineOpts
	return func(name string) (Engine, error) {
		inner, err := NewEngine(name, engineOpts...)
		if err != nil {
			return nil, err
		}
		return NewResilientEngine(inner, opts...), nil
	}
}

// Run executes the inner engine and retries on rate-limit errors.
func (r *ResilientEngine) Run(ctx context.Context, prompt string, opts RunOpts) (string, int, error) {
	for attempt := 0; attempt <= r.maxRetries; attempt++ {
		output, exitCode, err := r.inner.Run(ctx, prompt, opts)

		// Last attempt — return whatever we got.
		if attempt == r.maxRetries {
			return output, exitCode, err
		}

		// Success — no retry needed.
		if err == nil {
			return output, exitCode, err
		}

		// Check for rate-limit indicators.
		rl := DetectRateLimit(r.inner.Name(), output, err)
		if !rl.Detected {
			return output, exitCode, err
		}

		delay := r.calculateDelay(attempt, rl.RetryAfter)
		r.logFunc("Rate limited — waiting %s before retry (attempt %d/%d)",
			delay.Round(time.Second), attempt+2, r.maxRetries+1)

		select {
		case <-ctx.Done():
			return output, exitCode, ctx.Err()
		case <-time.After(delay):
			// continue to next attempt
		}
	}

	// Unreachable, but the compiler needs it.
	return "", -1, nil
}

// Name delegates to the inner engine.
func (r *ResilientEngine) Name() string { return r.inner.Name() }

// Inner returns the wrapped engine.
func (r *ResilientEngine) Inner() Engine { return r.inner }

// calculateDelay returns the backoff duration for an attempt, honoring a
// server-suggested retry-after when available.
func (r *ResilientEngine) calculateDelay(attempt int, retryAfter time.Duration) time.Duration {
	if retryAfter > 0 {
		if retryAfter > r.maxDelay {
			return r.maxDelay
		}
		return retryAfter
	}
	delay := float64(r.baseDelay) * math.Pow(2, float64(attempt))
	if delay > float64(r.maxDelay) {
		delay = float64(r.maxDelay)
	}
	// Apply jitter: delay * (1 ± jitter).
	jitterRange := delay * r.jitter
	delay = delay - jitterRange + rand.Float64()*2*jitterRange
	return time.Duration(delay)
}
