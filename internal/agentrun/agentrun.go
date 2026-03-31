package agentrun

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/yevgetman/fry/internal/engine"
	frylog "github.com/yevgetman/fry/internal/log"
	"github.com/yevgetman/fry/internal/textutil"
)

const maxNonFatalEngineOutputBytes = 1000

// DualLogOpts holds the configuration for a dual-log agent invocation.
type DualLogOpts struct {
	Engine     engine.Engine // required
	Model      string        // resolved model string (e.g. "claude-opus-4-5")
	ExtraFlags []string      // agent extra flags from epic (e.g. --verbose)
	WorkDir    string        // project directory for the agent
	Verbose    bool          // if true, also write to Stdout (or os.Stdout if nil)
	Stdout     io.Writer     // optional; defaults to os.Stdout when Verbose is true
}

// RunWithDualLogs runs the engine with the given prompt, writing output to both
// iterPath (fresh per-iteration log) and sprintLogPath (appended cross-iteration log).
// Returns the agent's combined stdout+stderr output string.
func RunWithDualLogs(ctx context.Context, prompt, iterPath, sprintLogPath string, opts DualLogOpts) (string, error) {
	stdout := opts.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}

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
		writer := io.MultiWriter(stdout, iterLog, sprintLog)
		runOpts.Stdout = writer
		runOpts.Stderr = writer
		output, exitCode, runErr := opts.Engine.Run(ctx, prompt, runOpts)
		if err := iterLog.Sync(); err != nil {
			return output, fmt.Errorf("sync iter log: %w", err)
		}
		if err := sprintLog.Sync(); err != nil {
			return output, fmt.Errorf("sync sprint log: %w", err)
		}
		if runErr != nil && ctx.Err() == nil {
			frylog.Log("WARNING: %s", formatNonFatalEngineError(opts.Engine, opts.Model, exitCode, runErr, output))
			return output, nil
		}
		return output, runErr
	}

	writer := io.MultiWriter(iterLog, sprintLog)
	runOpts.Stdout = writer
	runOpts.Stderr = writer
	output, exitCode, runErr := opts.Engine.Run(ctx, prompt, runOpts)
	if err := iterLog.Sync(); err != nil {
		return output, fmt.Errorf("sync iter log: %w", err)
	}
	if err := sprintLog.Sync(); err != nil {
		return output, fmt.Errorf("sync sprint log: %w", err)
	}
	if runErr != nil && ctx.Err() == nil {
		frylog.Log("WARNING: %s", formatNonFatalEngineError(opts.Engine, opts.Model, exitCode, runErr, output))
		return output, nil
	}
	return output, runErr
}

func formatNonFatalEngineError(eng engine.Engine, model string, exitCode int, runErr error, output string) string {
	msg := fmt.Sprintf("agent exited with error (non-fatal): engine=%s model=%s exit_code=%d err=%v", eng.Name(), model, exitCode, runErr)
	if details := summarizeEngineOutput(output); details != "" {
		msg += "\nengine output:\n" + details
	}
	return msg
}

func summarizeEngineOutput(output string) string {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return ""
	}
	if len(trimmed) > maxNonFatalEngineOutputBytes {
		return textutil.TruncateUTF8(trimmed, maxNonFatalEngineOutputBytes) + "\n...(truncated)"
	}
	return trimmed
}
