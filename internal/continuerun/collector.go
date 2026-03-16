package continuerun

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"sort"
	"strings"

	"github.com/yevgetman/fry/internal/config"
	"github.com/yevgetman/fry/internal/epic"
)

var completedSprintRe = regexp.MustCompile(`(?m)^## Sprint (\d+):\s*(.+?)\s*—\s*(PASS.*)$`)

// CollectBuildState gathers a snapshot of the current build state from .fry/ artifacts.
func CollectBuildState(ctx context.Context, projectDir string, ep *epic.Epic) (*BuildState, error) {
	fryDir := filepath.Join(projectDir, config.FryDir)
	if _, err := os.Stat(fryDir); os.IsNotExist(err) {
		return nil, ErrNoPreviousBuild
	}

	state := &BuildState{
		EpicName:     ep.Name,
		TotalSprints: ep.TotalSprints,
		Engine:       ep.Engine,
		EffortLevel:  ep.EffortLevel.String(),
	}

	// Sprint names list
	state.SprintNames = make([]string, ep.TotalSprints)
	for i, spr := range ep.Sprints {
		state.SprintNames[i] = spr.Name
	}

	// Parse completed sprints from epic-progress.txt
	state.CompletedSprints = parseCompletedSprints(projectDir)
	for _, cs := range state.CompletedSprints {
		if cs.Number > state.HighestCompleted {
			state.HighestCompleted = cs.Number
		}
	}

	// Determine next sprint and check for partial work
	nextSprint := findNextSprint(state.CompletedSprints, ep.TotalSprints)
	if nextSprint > 0 && nextSprint <= ep.TotalSprints {
		active := collectActiveSprintState(projectDir, nextSprint, ep)
		if active != nil {
			state.ActiveSprint = active
		}
	}

	// Environment checks
	state.DockerAvailable = checkDockerAvailable(ctx)
	state.DockerRequired = ep.DockerFromSprint > 0 && nextSprint >= ep.DockerFromSprint
	state.RequiredTools = checkRequiredTools(ep.RequiredTools)
	state.GitClean, state.GitBranch, state.LastAutoCommit = collectGitState(ctx, projectDir)

	// Build mode
	state.Mode = ReadBuildMode(projectDir)

	// Deviation history
	state.DeviationCount = countDeviations(projectDir)

	// Deferred failures
	state.DeferredFailures = collectDeferredFailures(projectDir)

	return state, nil
}

// parseCompletedSprints reads epic-progress.txt and extracts completed sprint entries.
func parseCompletedSprints(projectDir string) []CompletedSprint {
	path := filepath.Join(projectDir, config.EpicProgressFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	matches := completedSprintRe.FindAllStringSubmatch(string(data), -1)
	var completed []CompletedSprint
	for _, m := range matches {
		num, err := strconv.Atoi(m[1])
		if err != nil || num < 1 {
			continue
		}
		completed = append(completed, CompletedSprint{
			Number: num,
			Name:   strings.TrimSpace(m[2]),
			Status: strings.TrimSpace(m[3]),
		})
	}
	return completed
}

// findNextSprint returns the first sprint number not in the completed set.
func findNextSprint(completed []CompletedSprint, totalSprints int) int {
	done := make(map[int]bool, len(completed))
	for _, cs := range completed {
		done[cs.Number] = true
	}
	for i := 1; i <= totalSprints; i++ {
		if !done[i] {
			return i
		}
	}
	return 0 // all complete
}

// collectActiveSprintState checks for evidence of a started-but-not-passed sprint.
func collectActiveSprintState(projectDir string, sprintNum int, ep *epic.Epic) *ActiveSprintState {
	logsDir := filepath.Join(projectDir, config.BuildLogsDir)
	pattern := filepath.Join(logsDir, fmt.Sprintf("sprint%d_*", sprintNum))
	matches, _ := filepath.Glob(pattern)

	// Check sprint-progress.txt for reference to this sprint
	progressMentions := sprintProgressMentionsSprint(projectDir, sprintNum)

	if len(matches) == 0 && !progressMentions {
		return nil
	}

	name := ""
	if sprintNum >= 1 && sprintNum <= len(ep.Sprints) {
		name = ep.Sprints[sprintNum-1].Name
	}

	active := &ActiveSprintState{
		Number: sprintNum,
		Name:   name,
	}

	// Count log types
	for _, m := range matches {
		base := filepath.Base(m)
		switch {
		case strings.Contains(base, "_iter"):
			active.IterationCount++
		case strings.Contains(base, "_audit"):
			active.AuditCount++
		case strings.Contains(base, "_heal"):
			active.HealCount++
		case strings.Contains(base, "_retry"):
			active.HasRetryLog = true
		}
	}

	// Read tail of most recent log
	if len(matches) > 0 {
		sort.Strings(matches) // lexicographic = chronological due to timestamp format
		active.LastLogTail = readTail(matches[len(matches)-1], 100)
	}

	// Check for audit severity
	auditPath := filepath.Join(projectDir, config.SprintAuditFile)
	if data, err := os.ReadFile(auditPath); err == nil {
		active.AuditSeverity = extractMaxSeverity(string(data))
	}

	// Sprint progress excerpt
	active.ProgressExcerpt = readSprintProgressExcerpt(projectDir, 50)

	return active
}

// sprintProgressMentionsSprint checks if sprint-progress.txt references a specific sprint.
func sprintProgressMentionsSprint(projectDir string, sprintNum int) bool {
	path := filepath.Join(projectDir, config.SprintProgressFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	header := fmt.Sprintf("# Sprint %d:", sprintNum)
	return strings.Contains(string(data), header)
}

// checkDockerAvailable returns true if the Docker daemon is reachable.
func checkDockerAvailable(ctx context.Context) bool {
	cmd := exec.CommandContext(ctx, "docker", "info")
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run() == nil
}

// checkRequiredTools checks whether each required tool is available in PATH.
func checkRequiredTools(tools []string) []ToolStatus {
	statuses := make([]ToolStatus, len(tools))
	for i, tool := range tools {
		_, err := exec.LookPath(tool)
		statuses[i] = ToolStatus{Name: tool, Available: err == nil}
	}
	return statuses
}

// collectGitState returns (clean, branch, lastAutoCommit).
func collectGitState(ctx context.Context, projectDir string) (bool, string, string) {
	clean := true
	branch := ""
	lastCommit := ""

	// Check if working tree is clean
	var statusOut bytes.Buffer
	statusCmd := exec.CommandContext(ctx, "bash", "-c", "git status --porcelain")
	statusCmd.Dir = projectDir
	statusCmd.Stdout = &statusOut
	if statusCmd.Run() == nil {
		clean = strings.TrimSpace(statusOut.String()) == ""
	}

	// Get current branch
	var branchOut bytes.Buffer
	branchCmd := exec.CommandContext(ctx, "bash", "-c", "git branch --show-current")
	branchCmd.Dir = projectDir
	branchCmd.Stdout = &branchOut
	if branchCmd.Run() == nil {
		branch = strings.TrimSpace(branchOut.String())
	}

	// Get last automated commit
	var logOut bytes.Buffer
	logCmd := exec.CommandContext(ctx, "bash", "-c", `git log --oneline --grep="\[automated\]" -1 --format="%s"`)
	logCmd.Dir = projectDir
	logCmd.Stdout = &logOut
	if logCmd.Run() == nil {
		lastCommit = strings.TrimSpace(logOut.String())
	}

	return clean, branch, lastCommit
}

// countDeviations counts the number of DEVIATE verdict entries in the deviation log.
func countDeviations(projectDir string) int {
	path := filepath.Join(projectDir, config.DeviationLogFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	return strings.Count(string(data), "**Decision**: DEVIATE")
}

// collectDeferredFailures reads summary lines from deferred-failures.md.
func collectDeferredFailures(projectDir string) []string {
	path := filepath.Join(projectDir, config.DeferredFailuresFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	content := strings.TrimSpace(string(data))
	if content == "" {
		return nil
	}
	var lines []string
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- DEFERRED:") || strings.HasPrefix(trimmed, "## Sprint") {
			lines = append(lines, trimmed)
		}
	}
	return lines
}

// readTail reads the last n lines of a file.
func readTail(path string, n int) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	lines := strings.Split(string(data), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

// readSprintProgressExcerpt reads the last n lines of sprint-progress.txt.
func readSprintProgressExcerpt(projectDir string, n int) string {
	path := filepath.Join(projectDir, config.SprintProgressFile)
	return readTail(path, n)
}

var (
	severityLabelRe = regexp.MustCompile(`(?i)\bseverity\b`)
	severityWordRe  = regexp.MustCompile(`\b(CRITICAL|HIGH|MODERATE|LOW)\b`)
)

// extractMaxSeverity returns the highest severity found in audit content.
// Only matches severity keywords on lines containing a "Severity" label
// to avoid false positives from prose.
func extractMaxSeverity(content string) string {
	maxRank := 0
	maxSev := ""
	for _, line := range strings.Split(content, "\n") {
		if !severityLabelRe.MatchString(line) {
			continue
		}
		upper := strings.ToUpper(line)
		m := severityWordRe.FindString(upper)
		if m == "" {
			continue
		}
		if sevRank(m) > maxRank {
			maxRank = sevRank(m)
			maxSev = m
		}
		if maxSev == "CRITICAL" {
			return "CRITICAL"
		}
	}
	return maxSev
}

// ReadBuildMode reads the persisted build mode from .fry/build-mode.txt.
// Returns an empty string if the file does not exist or cannot be read.
func ReadBuildMode(projectDir string) string {
	path := filepath.Join(projectDir, config.BuildModeFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func sevRank(sev string) int {
	switch sev {
	case "CRITICAL":
		return 4
	case "HIGH":
		return 3
	case "MODERATE":
		return 2
	case "LOW":
		return 1
	default:
		return 0
	}
}
