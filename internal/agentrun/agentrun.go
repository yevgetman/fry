package agentrun

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/yevgetman/fry/internal/engine"
)

// DualLogOpts holds the configuration for a dual-log agent invocation.
type DualLogOpts struct {
	Engine     engine.Engine // required
	Model      string        // resolved model string (e.g. "claude-opus-4-5")
	ExtraFlags []string      // agent extra flags from epic (e.g. --verbose)
	WorkDir    string        // project directory for the agent
	Verbose    bool          // if true, also write to os.Stdout
}

// RunWithDualLogs runs the engine with the given prompt, writing output to both
// iterPath (fresh per-iteration log) and sprintLogPath (appended cross-iteration log).
// Returns the agent's combined stdout+stderr output string.
func RunWithDualLogs(ctx context.Context, prompt, iterPath, sprintLogPath string, opts DualLogOpts) (string, error) {
	iterLog, err := os.Create(iterPath)
	if err != nil {
		return "", fmt.Errorf("open iter log: %w", err)
	}
	defer iterLog.Close()

	sprintLog, err := os.OpenFile(sprintLogPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return "", fmt.Errorf("open sprint log: %w", err)
	}
	defer sprintLog.Close()

	runOpts := engine.RunOpts{
		Model:      opts.Model,
		ExtraFlags: opts.ExtraFlags,
		WorkDir:    opts.WorkDir,
	}

	if opts.Verbose {
		writer := io.MultiWriter(os.Stdout, iterLog, sprintLog)
		runOpts.Stdout = writer
		runOpts.Stderr = writer
		output, _, runErr := opts.Engine.Run(ctx, prompt, runOpts)
		if runErr != nil && ctx.Err() == nil {
			fmt.Fprintf(os.Stderr, "fry: warning: agent exited with error (non-fatal): %v\n", runErr)
			return output, nil
		}
		return output, runErr
	}

	runOpts.Stdout = iterLog
	runOpts.Stderr = iterLog
	output, _, runErr := opts.Engine.Run(ctx, prompt, runOpts)
	iterBytes, err := os.ReadFile(iterPath)
	if err != nil {
		return output, fmt.Errorf("read iter log: %w", err)
	}
	n, err := sprintLog.Write(iterBytes)
	if err != nil {
		return output, fmt.Errorf("append iter log to sprint log: %w", err)
	}
	if n != len(iterBytes) {
		return output, fmt.Errorf("append iter log to sprint log: short write (%d/%d bytes)", n, len(iterBytes))
	}
	if err := sprintLog.Sync(); err != nil {
		return output, fmt.Errorf("sync sprint log: %w", err)
	}
	if runErr != nil && ctx.Err() == nil {
		fmt.Fprintf(os.Stderr, "fry: warning: agent exited with error (non-fatal): %v\n", runErr)
		return output, nil
	}
	return output, runErr
}
