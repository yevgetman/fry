package engine

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/yevgetman/fry/internal/config"
)

type Engine interface {
	Run(ctx context.Context, prompt string, opts RunOpts) (output string, exitCode int, err error)
	Name() string
}

type RunOpts struct {
	Model            string
	SessionType      SessionType
	SessionID        string
	StructuredOutput bool
	EffortLevel      string
	ExtraFlags       []string
	WorkDir          string
	Stdout           io.Writer
	Stderr           io.Writer
	LogFiles         []string
}

func ResolveEngine(cliFlag, epicDirective, envVar, defaultEngine string) (string, error) {
	name := cliFlag
	if name == "" {
		name = epicDirective
	}
	if name == "" {
		if envVar != "" {
			name = envVar
		} else {
			name = os.Getenv("FRY_ENGINE")
		}
	}
	if name == "" {
		if defaultEngine != "" {
			name = defaultEngine
		} else {
			name = config.DefaultEngine
		}
	}

	return NormalizeEngineName(name)
}

func NormalizeEngineName(name string) (string, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	switch name {
	case "codex", "claude", "ollama":
		return name, nil
	default:
		return "", fmt.Errorf("unsupported engine %q; valid: codex, claude, ollama", name)
	}
}

// EngineOpt configures an engine at creation time.
type EngineOpt func(Engine)

// WithMCPConfig sets the MCP server configuration file path.
// Only applies to ClaudeEngine; silently ignored by other engines.
func WithMCPConfig(path string) EngineOpt {
	return func(e Engine) {
		if ce, ok := e.(*ClaudeEngine); ok {
			ce.mcpConfig = path
		}
	}
}

func NewEngine(name string, opts ...EngineOpt) (Engine, error) {
	var eng Engine
	switch name {
	case "codex":
		eng = &CodexEngine{}
	case "claude":
		eng = &ClaudeEngine{}
	case "ollama":
		eng = &OllamaEngine{}
	default:
		return nil, fmt.Errorf("unsupported engine %q", name)
	}
	for _, opt := range opts {
		opt(eng)
	}
	return eng, nil
}
