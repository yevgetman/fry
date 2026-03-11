package engine

import (
	"bytes"
	"context"
	"io"
	"os/exec"
)

type CodexEngine struct{}

func (e *CodexEngine) Run(ctx context.Context, prompt string, opts RunOpts) (string, int, error) {
	args := codexArgs(prompt, opts)
	cmd := exec.CommandContext(ctx, "codex", args...)
	cmd.Dir = opts.WorkDir

	var buffer bytes.Buffer
	cmd.Stdout = combinedWriter(&buffer, opts.Stdout)
	cmd.Stderr = combinedWriter(&buffer, opts.Stderr)

	err := cmd.Run()
	exitCode := exitCodeFromError(err)
	return buffer.String(), exitCode, err
}

func (e *CodexEngine) Name() string {
	return "codex"
}

func codexArgs(prompt string, opts RunOpts) []string {
	args := []string{"exec", "--dangerously-bypass-approvals-and-sandbox"}
	if opts.Model != "" {
		args = append(args, "--model", opts.Model)
	}
	args = append(args, opts.ExtraFlags...)
	args = append(args, prompt)
	return args
}

func combinedWriter(buffer *bytes.Buffer, extra io.Writer) io.Writer {
	if extra == nil {
		return buffer
	}
	return io.MultiWriter(buffer, extra)
}
