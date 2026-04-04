package verify

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// HarnessIssue describes a single harness configuration problem found
// during self-validation.
type HarnessIssue struct {
	Sprint  int
	Type    string // "path_mismatch", "suspicious_glob", "missing_parent_dir"
	Target  string
	Message string
}

// HarnessCheckResult captures the outcome of the harness self-check.
type HarnessCheckResult struct {
	Issues []HarnessIssue
}

// HasIssues returns true if any harness problems were found.
func (r *HarnessCheckResult) HasIssues() bool {
	return len(r.Issues) > 0
}

// Summary returns a compact human-readable summary of harness issues.
func (r *HarnessCheckResult) Summary() string {
	if len(r.Issues) == 0 {
		return "harness self-check passed"
	}
	var parts []string
	for _, issue := range r.Issues {
		parts = append(parts, fmt.Sprintf("sprint %d: %s (%s: %s)", issue.Sprint, issue.Message, issue.Type, issue.Target))
	}
	return strings.Join(parts, "; ")
}

// ValidateHarness checks that sanity check targets make sense from the
// project directory. FILE and FILE_CONTAINS targets are validated for
// path syntax, parent directory existence, and suspicious patterns.
// CMD/TEST checks are validated for basic syntax.
// This runs before the main build loop to catch harness mismatches early.
func ValidateHarness(projectDir string, checks []Check) *HarnessCheckResult {
	result := &HarnessCheckResult{}

	for _, c := range checks {
		switch c.Type {
		case CheckFile, CheckFileContains:
			validateFileTarget(projectDir, c, result)
		case CheckCmd, CheckCmdOutput, CheckTest:
			validateCmdTarget(c, result)
		}
	}

	return result
}

func validateFileTarget(projectDir string, c Check, result *HarnessCheckResult) {
	target := strings.TrimSpace(c.Path)
	if target == "" {
		result.Issues = append(result.Issues, HarnessIssue{
			Sprint:  c.Sprint,
			Type:    "path_mismatch",
			Target:  c.Path,
			Message: "empty file target",
		})
		return
	}

	// Absolute paths are suspicious in a portable build
	if filepath.IsAbs(target) {
		result.Issues = append(result.Issues, HarnessIssue{
			Sprint:  c.Sprint,
			Type:    "path_mismatch",
			Target:  target,
			Message: "absolute path in file check — should be relative to project root",
		})
		return
	}

	// Check for path traversal outside project
	cleaned := filepath.Clean(target)
	if strings.HasPrefix(cleaned, "..") {
		result.Issues = append(result.Issues, HarnessIssue{
			Sprint:  c.Sprint,
			Type:    "path_mismatch",
			Target:  target,
			Message: "path traverses outside project directory",
		})
		return
	}

	// Check that the parent directory exists (the file itself may not
	// exist yet — the sprint is supposed to create it — but the parent
	// directory should at least be plausible).
	fullPath := filepath.Join(projectDir, cleaned)
	parentDir := filepath.Dir(fullPath)
	if _, err := os.Stat(parentDir); os.IsNotExist(err) {
		// Only flag this for paths with at least one directory component.
		// A bare filename like "main.go" resolves to the project root which always exists.
		if strings.Contains(cleaned, string(filepath.Separator)) {
			result.Issues = append(result.Issues, HarnessIssue{
				Sprint:  c.Sprint,
				Type:    "missing_parent_dir",
				Target:  target,
				Message: fmt.Sprintf("parent directory %s does not exist", filepath.Dir(cleaned)),
			})
		}
	}
}

func validateCmdTarget(c Check, result *HarnessCheckResult) {
	cmd := strings.TrimSpace(c.Command)
	if cmd == "" {
		result.Issues = append(result.Issues, HarnessIssue{
			Sprint:  c.Sprint,
			Type:    "path_mismatch",
			Target:  "(empty)",
			Message: "empty command in check",
		})
	}
}
