package engine

import (
	"bytes"
	"context"
	"os/exec"
	"strings"

	"github.com/yevgetman/fry/internal/config"
)

type OllamaEngine struct{}

func (e *OllamaEngine) Run(ctx context.Context, prompt string, opts RunOpts) (string, int, error) {
	model := opts.Model
	if model == "" {
		model = config.DefaultOllamaModel
	}
	cmd := exec.CommandContext(ctx, "ollama", append([]string{"run", model}, opts.ExtraFlags...)...)
	cmd.Dir = opts.WorkDir
	cmd.Stdin = strings.NewReader(prompt)

	var buffer bytes.Buffer
	cmd.Stdout = combinedWriter(&buffer, opts.Stdout)
	cmd.Stderr = combinedWriter(&buffer, opts.Stderr)

	err := cmd.Run()
	exitCode := exitCodeFromError(err)
	return buffer.String(), exitCode, err
}

func (e *OllamaEngine) Name() string {
	return "ollama"
}

func ollamaArgs(opts RunOpts) []string {
	model := opts.Model
	if model == "" {
		model = config.DefaultOllamaModel
	}
	args := []string{"run", model}
	args = append(args, opts.ExtraFlags...)
	return args
}
