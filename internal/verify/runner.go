package verify

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

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
		cmd := exec.CommandContext(ctx, "bash", "-c", fmt.Sprintf("grep -qE -- %s %s", shellQuote(check.Pattern), shellQuote(targetPath)))
		result.Passed = cmd.Run() == nil
	case CheckCmd:
		cmd := exec.CommandContext(ctx, "bash", "-c", check.Command)
		cmd.Dir = projectDir
		output, err := cmd.CombinedOutput()
		result.Output = string(output)
		result.Passed = err == nil
	case CheckCmdOutput:
		command := exec.CommandContext(ctx, "bash", "-c", check.Command)
		command.Dir = projectDir

		var stdout bytes.Buffer
		command.Stdout = &stdout
		var stderr bytes.Buffer
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

		grep := exec.CommandContext(ctx, "bash", "-c", fmt.Sprintf("grep -qE -- %s", shellQuote(check.Pattern)))
		grep.Stdin = strings.NewReader(stdout.String())
		result.Passed = grep.Run() == nil
	}

	return result
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
