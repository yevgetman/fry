package engine

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// FailoverEngine wraps two engines. It starts on the primary engine and
// permanently promotes the fallback engine after the first successful failover.
type FailoverEngine struct {
	primary    Engine
	fallback   Engine
	active     Engine
	logFunc    func(format string, args ...interface{})
	switchFunc func(from, to string)
	mu         sync.RWMutex
}

// FailoverOpt configures a FailoverEngine.
type FailoverOpt func(*FailoverEngine)

// WithFailoverLogFunc sets the logger used for failover events.
func WithFailoverLogFunc(fn func(string, ...interface{})) FailoverOpt {
	return func(f *FailoverEngine) {
		if fn != nil {
			f.logFunc = fn
		}
	}
}

// WithFailoverSwitchFunc sets a callback fired after the fallback engine is
// promoted. The callback runs once, after a successful fallback invocation.
func WithFailoverSwitchFunc(fn func(from, to string)) FailoverOpt {
	return func(f *FailoverEngine) {
		if fn != nil {
			f.switchFunc = fn
		}
	}
}

// NewFailoverEngine wraps a primary engine with a sticky fallback engine.
func NewFailoverEngine(primary, fallback Engine, opts ...FailoverOpt) *FailoverEngine {
	f := &FailoverEngine{
		primary:    primary,
		fallback:   fallback,
		active:     primary,
		logFunc:    func(string, ...interface{}) {},
		switchFunc: func(string, string) {},
	}
	for _, opt := range opts {
		opt(f)
	}
	return f
}

// Run executes the active engine. When the primary engine fails with a
// failover-worthy error, the fallback engine is tried once and promoted on
// success for all future calls.
func (f *FailoverEngine) Run(ctx context.Context, prompt string, opts RunOpts) (string, int, error) {
	active := f.current()
	activeOpts, _ := AdaptRunOptsForEngine(active.Name(), opts)
	output, exitCode, err := active.Run(ctx, prompt, activeOpts)
	if err == nil || ctx.Err() != nil || f.fallback == nil {
		return output, exitCode, err
	}

	if active.Name() != f.primary.Name() {
		return output, exitCode, err
	}

	result := DetectFailoverCondition(active.Name(), output, err)
	if !result.Detected {
		return output, exitCode, err
	}

	fallbackOpts, modelChanged := AdaptRunOptsForEngine(f.fallback.Name(), opts)
	if modelChanged && strings.TrimSpace(opts.Model) != "" && strings.TrimSpace(fallbackOpts.Model) != "" {
		f.logFunc(
			"Engine failover remapped model from %q to %q for %s (%s/%s)",
			opts.Model,
			fallbackOpts.Model,
			f.fallback.Name(),
			opts.SessionType,
			normalizeEffort(opts.EffortLevel),
		)
	}
	f.logFunc(
		"Engine failover triggered: primary=%s reason=%s -> fallback=%s",
		f.primary.Name(),
		result.Reason,
		f.fallback.Name(),
	)

	fallbackOutput, fallbackExitCode, fallbackErr := f.fallback.Run(ctx, prompt, fallbackOpts)
	if fallbackErr != nil {
		if ctx.Err() != nil {
			return fallbackOutput, fallbackExitCode, ctx.Err()
		}
		return fallbackOutput, fallbackExitCode, fmt.Errorf(
			"primary engine %s failed (%s): %v; fallback engine %s failed: %w",
			f.primary.Name(),
			result.Reason,
			err,
			f.fallback.Name(),
			fallbackErr,
		)
	}

	f.promoteFallback()
	return fallbackOutput, fallbackExitCode, nil
}

// Name returns the currently active engine name.
func (f *FailoverEngine) Name() string {
	return f.current().Name()
}

func (f *FailoverEngine) current() Engine {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if f.active == nil {
		return f.primary
	}
	return f.active
}

func (f *FailoverEngine) promoteFallback() {
	f.mu.Lock()
	if f.active == f.fallback || f.fallback == nil {
		f.mu.Unlock()
		return
	}
	from := f.active.Name()
	to := f.fallback.Name()
	f.active = f.fallback
	f.mu.Unlock()
	f.switchFunc(from, to)
}

// AdaptRunOptsForEngine rewrites engine-specific run options for the target
// engine while preserving the current session tier selection behavior.
func AdaptRunOptsForEngine(engineName string, opts RunOpts) (RunOpts, bool) {
	adapted := opts
	requestedModel := strings.TrimSpace(opts.Model)
	if requestedModel == "" {
		if opts.SessionType != "" {
			adapted.Model = ResolveModelForSession(engineName, opts.EffortLevel, opts.SessionType)
		}
		return adapted, adapted.Model != opts.Model
	}
	if IsModelValidForEngine(engineName, requestedModel) {
		return adapted, false
	}
	if opts.SessionType != "" {
		adapted.Model = ResolveModelForSession(engineName, opts.EffortLevel, opts.SessionType)
		return adapted, adapted.Model != opts.Model
	}
	adapted.Model = ""
	return adapted, adapted.Model != opts.Model
}
