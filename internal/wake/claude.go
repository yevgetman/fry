package wake

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// effortArgs maps effort level to claude CLI flags.
var effortArgs = map[string][]string{
	"fast":     {"--model", "sonnet", "--effort", "low", "--max-budget-usd", "1"},
	"standard": {"--model", "sonnet", "--effort", "medium", "--max-budget-usd", "2"},
	"max":      {"--model", "opus", "--effort", "high", "--max-budget-usd", "5"},
}

// ClaudeRequest specifies a single non-interactive claude invocation.
type ClaudeRequest struct {
	MissionDir   string
	Effort       string
	Prompt       string
	WallClockCap time.Duration
}

// ClaudeResult holds the outcome of RunClaude.
type ClaudeResult struct {
	ExitCode         int
	Stdout           []byte
	WallClockSeconds int
	PromiseFound     bool
	ResultText       string
	CostUSD          float64
}

// RunClaude invokes `claude -p` with the given request and captures output.
func RunClaude(ctx context.Context, req ClaudeRequest) (*ClaudeResult, error) {
	eArgs, ok := effortArgs[req.Effort]
	if !ok {
		eArgs = effortArgs["fast"]
	}

	args := []string{
		"-p",
		"--output-format", "json",
		"--permission-mode", "bypassPermissions",
		"--dangerously-skip-permissions",
		"--add-dir", req.MissionDir,
	}
	args = append(args, eArgs...)

	cap := req.WallClockCap
	if cap <= 0 {
		cap = 540 * time.Second
	}
	ctx2, cancel := context.WithTimeout(ctx, cap)
	defer cancel()

	claudePath := claudeBinary()
	cmd := exec.CommandContext(ctx2, claudePath, args...)
	cmd.Stdin = strings.NewReader(req.Prompt)

	start := time.Now()
	out, err := cmd.Output()
	elapsed := int(time.Since(start).Seconds())

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
			// Include stderr in output for diagnostics
			out = append(out, exitErr.Stderr...)
		} else {
			return nil, fmt.Errorf("RunClaude: exec: %w", err)
		}
	}

	found, text := ExtractPromise(out)
	costUSD := ParseCostUSD(out)

	return &ClaudeResult{
		ExitCode:         exitCode,
		Stdout:           out,
		WallClockSeconds: elapsed,
		PromiseFound:     found,
		ResultText:       text,
		CostUSD:          costUSD,
	}, nil
}

// claudeBinary returns the path to the claude CLI binary.
func claudeBinary() string {
	// Check common locations; fall back to PATH lookup.
	candidates := []string{
		"/Users/julie/.local/bin/claude",
		os.ExpandEnv("$HOME/.local/bin/claude"),
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	if p, err := exec.LookPath("claude"); err == nil {
		return p
	}
	return "claude"
}
