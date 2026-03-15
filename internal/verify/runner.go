package verify

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/yevgetman/fry/internal/textutil"
)

// defaultCheckTimeout is the maximum time a single verification check command
// is allowed to run before being killed. This prevents hanging builds.
const defaultCheckTimeout = 120 * time.Second

func RunChecks(ctx context.Context, checks []Check, sprintNum int, projectDir string) ([]CheckResult, int, int) {
	var filtered []Check
	for _, check := range checks {
		if check.Sprint == sprintNum {
			filtered = append(filtered, check)
		}
	}

	results := make([]CheckResult, 0, len(filtered))
	passCount := 0

	for _, check := range filtered {
		result := runCheck(ctx, check, projectDir)
		if result.Passed {
			passCount++
		}
		results = append(results, result)
	}

	return results, passCount, len(filtered)
}

func runCheck(ctx context.Context, check Check, projectDir string) CheckResult {
	result := CheckResult{Check: check}

	switch check.Type {
	case CheckFile:
		info, err := os.Stat(filepath.Join(projectDir, check.Path))
		result.Passed = err == nil && info.Size() > 0
	case CheckFileContains:
		targetPath := filepath.Join(projectDir, check.Path)
		cmd := exec.CommandContext(ctx, "bash", "-c", fmt.Sprintf("grep -qE -- %s %s", textutil.ShellQuote(check.Pattern), textutil.ShellQuote(targetPath)))
		result.Passed = cmd.Run() == nil
	case CheckCmd:
		checkCtx, checkCancel := context.WithTimeout(ctx, defaultCheckTimeout)
		defer checkCancel()
		cmd := exec.CommandContext(checkCtx, "bash", "-c", check.Command)
		cmd.Dir = projectDir
		var combined cappedBuffer
		cmd.Stdout = &combined
		cmd.Stderr = &combined
		err := cmd.Run()
		result.Output = combined.String()
		result.Passed = err == nil
	case CheckCmdOutput:
		checkCtx, checkCancel := context.WithTimeout(ctx, defaultCheckTimeout)
		defer checkCancel()
		command := exec.CommandContext(checkCtx, "bash", "-c", check.Command)
		command.Dir = projectDir

		var stdout, stderr cappedBuffer
		command.Stdout = &stdout
		command.Stderr = &stderr

		err := command.Run()
		result.Output = stdout.String()
		if result.Output == "" && stderr.Len() > 0 {
			result.Output = stderr.String()
		}
		if err != nil {
			result.Passed = false
			return result
		}

		// Trim leading/trailing whitespace from each line before matching.
		// This prevents platform-specific formatting (e.g., macOS wc -w
		// producing "  42" instead of "42") from causing false negatives.
		trimmed := trimOutputLines(stdout.String())
		grep := exec.CommandContext(checkCtx, "bash", "-c", fmt.Sprintf("grep -qE -- %s", textutil.ShellQuote(check.Pattern)))
		grep.Stdin = strings.NewReader(trimmed)
		result.Passed = grep.Run() == nil
	}

	return result
}

// trimOutputLines trims leading and trailing whitespace from each line of
// output. This normalizes platform differences (e.g., macOS wc produces
// leading spaces) so that anchored patterns like ^[0-9]+$ match reliably.
func trimOutputLines(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimSpace(line)
	}
	return strings.Join(lines, "\n")
}

// maxCheckOutput caps the amount of output captured from verification commands
// to prevent unbounded memory growth on pathologically verbose checks.
const maxCheckOutput = 10 * 1024 * 1024 // 10 MB

// cappedBuffer is a bytes.Buffer that stops accepting writes after maxCheckOutput.
type cappedBuffer struct {
	buf bytes.Buffer
}

func (c *cappedBuffer) Write(p []byte) (int, error) {
	remaining := maxCheckOutput - c.buf.Len()
	if remaining <= 0 {
		return len(p), nil // discard silently
	}
	if len(p) > remaining {
		p = p[:remaining]
	}
	return c.buf.Write(p)
}

func (c *cappedBuffer) String() string { return c.buf.String() }
func (c *cappedBuffer) Len() int       { return c.buf.Len() }
