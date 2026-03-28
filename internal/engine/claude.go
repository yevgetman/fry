package engine

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"strings"
)

type ClaudeEngine struct {
	mcpConfig string
}

func (e *ClaudeEngine) Run(ctx context.Context, prompt string, opts RunOpts) (string, int, error) {
	args := claudeArgs(opts)
	if e.mcpConfig != "" {
		args = append(args, "--mcp-config", e.mcpConfig)
	}
	cmd := exec.CommandContext(ctx, "claude", args...)
	cmd.Dir = opts.WorkDir
	cmd.Stdin = strings.NewReader(prompt)

	var buffer bytes.Buffer
	cmd.Stdout = combinedWriter(&buffer, opts.Stdout)
	cmd.Stderr = combinedWriter(&buffer, opts.Stderr)

	err := cmd.Run()
	exitCode := exitCodeFromError(err)
	return buffer.String(), exitCode, err
}

func (e *ClaudeEngine) Name() string {
	return "claude"
}

func claudeArgs(opts RunOpts) []string {
	args := []string{"-p", "--dangerously-skip-permissions"}
	if opts.Model != "" {
		args = append(args, "--model", opts.Model)
	}
	args = append(args, opts.ExtraFlags...)
	return args
}

func exitCodeFromError(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if ok := errors.As(err, &exitErr); ok {
		return exitErr.ExitCode()
	}
	return -1
}
