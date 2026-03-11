package engine

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/yevgetman/fry/internal/config"
)

type Engine interface {
	Run(ctx context.Context, prompt string, opts RunOpts) (output string, exitCode int, err error)
	Name() string
}

type RunOpts struct {
	Model      string
	ExtraFlags []string
	WorkDir    string
	Stdout     io.Writer
	Stderr     io.Writer
	LogFiles   []string
}

func ResolveEngine(cliFlag, epicDirective, envVar string) (string, error) {
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
		name = config.DefaultEngine
	}

	switch name {
	case "codex", "claude":
		return name, nil
	default:
		return "", fmt.Errorf("unsupported engine: %s", name)
	}
}

func NewEngine(name string) (Engine, error) {
	switch name {
	case "codex":
		return &CodexEngine{}, nil
	case "claude":
		return &ClaudeEngine{}, nil
	default:
		return nil, fmt.Errorf("unsupported engine: %s", name)
	}
}
