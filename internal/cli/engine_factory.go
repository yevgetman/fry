package cli

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/yevgetman/fry/internal/config"
	"github.com/yevgetman/fry/internal/engine"
	frlog "github.com/yevgetman/fry/internal/log"
)

type enginePlanner struct {
	mu              sync.Mutex
	activeName      string
	pinned          bool
	fallbackName    string
	disableFailover bool
	switchCallback  func(from, to string)
}

func newEnginePlanner(initialName string) *enginePlanner {
	return &enginePlanner{
		activeName:      initialName,
		fallbackName:    fallbackEngine,
		disableFailover: noEngineFailover,
	}
}

func (p *enginePlanner) Current() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.activeName
}

func (p *enginePlanner) SetDefault(name string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.pinned {
		p.activeName = name
	}
}

func (p *enginePlanner) SetSwitchCallback(fn func(from, to string)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.switchCallback = fn
}

func (p *enginePlanner) Build(requestedName string, engineOpts ...engine.EngineOpt) (engine.Engine, error) {
	p.mu.Lock()
	primaryName := strings.TrimSpace(requestedName)
	if p.pinned && p.activeName != "" {
		primaryName = p.activeName
	}
	fallbackName := p.fallbackName
	disableFailover := p.disableFailover || p.pinned
	p.mu.Unlock()

	primary, err := newBaseResilientEngine(primaryName, engineOpts...)
	if err != nil {
		return nil, err
	}
	if disableFailover {
		return primary, nil
	}

	resolvedFallback, explicitFallback, err := resolveFallbackEngine(primaryName, fallbackName)
	if err != nil {
		return nil, err
	}
	if resolvedFallback == "" {
		return primary, nil
	}

	fallback, err := newBaseResilientEngine(resolvedFallback, engineOpts...)
	if err != nil {
		if explicitFallback {
			return nil, err
		}
		frlog.Log("WARNING: automatic engine failover unavailable for %s: %v", resolvedFallback, err)
		return primary, nil
	}

	return engine.NewFailoverEngine(primary, fallback,
		engine.WithFailoverLogFunc(frlog.Log),
		engine.WithFailoverSwitchFunc(func(from, to string) {
			var switchCallback func(string, string)
			p.mu.Lock()
			p.activeName = to
			p.pinned = true
			switchCallback = p.switchCallback
			p.mu.Unlock()
			if switchCallback != nil {
				switchCallback(from, to)
			}
		}),
	), nil
}

func resolveFallbackEngine(primaryName, override string) (string, bool, error) {
	override = strings.TrimSpace(override)
	if override != "" {
		name, err := engine.NormalizeEngineName(override)
		if err != nil {
			return "", true, err
		}
		if name == primaryName {
			return "", true, fmt.Errorf("fallback engine %q must differ from primary engine %q", name, primaryName)
		}
		return name, true, nil
	}

	switch primaryName {
	case "claude":
		return "codex", false, nil
	case "codex":
		return "claude", false, nil
	default:
		return "", false, nil
	}
}

// BuildOneShot creates a standalone engine from the fallback name without
// pinning or affecting the active engine. Used for one-shot reporting retries.
func (p *enginePlanner) BuildOneShot(engineOpts ...engine.EngineOpt) (engine.Engine, string, error) {
	p.mu.Lock()
	primaryName := p.activeName
	fallbackName := p.fallbackName
	p.mu.Unlock()

	resolved, _, err := resolveFallbackEngine(primaryName, fallbackName)
	if err != nil || resolved == "" {
		return nil, "", fmt.Errorf("no fallback engine available for %s", primaryName)
	}
	eng, err := newBaseResilientEngine(resolved, engineOpts...)
	if err != nil {
		return nil, "", fmt.Errorf("create fallback engine %s: %w", resolved, err)
	}
	return eng, resolved, nil
}

func newBaseResilientEngine(name string, engineOpts ...engine.EngineOpt) (engine.Engine, error) {
	eng, err := engine.NewEngine(name, engineOpts...)
	if err != nil {
		return nil, err
	}
	return engine.NewResilientEngine(eng,
		engine.WithMaxRetries(config.RateLimitMaxRetries),
		engine.WithBaseDelay(time.Duration(config.RateLimitBaseDelaySec)*time.Second),
		engine.WithMaxDelay(time.Duration(config.RateLimitMaxDelaySec)*time.Second),
		engine.WithJitter(config.RateLimitJitter),
		engine.WithLogFunc(frlog.Log),
	), nil
}
