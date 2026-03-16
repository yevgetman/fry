package continuerun

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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
	state.GitClean, state.GitBranch, state.LastAutoCommit = collectGitState(projectDir)

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
		num := 0
		fmt.Sscanf(m[1], "%d", &num)
		if num > 0 {
			completed = append(completed, CompletedSprint{
				Number: num,
				Name:   strings.TrimSpace(m[2]),
				Status: strings.TrimSpace(m[3]),
			})
		}
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
func collectGitState(projectDir string) (bool, string, string) {
	clean := true
	branch := ""
	lastCommit := ""

	// Check if working tree is clean
	cmd := exec.Command("bash", "-c", "git status --porcelain")
	cmd.Dir = projectDir
	var out bytes.Buffer
	cmd.Stdout = &out
	if cmd.Run() == nil {
		clean = strings.TrimSpace(out.String()) == ""
	}

	// Get current branch
	out.Reset()
	cmd = exec.Command("bash", "-c", "git branch --show-current")
	cmd.Dir = projectDir
	cmd.Stdout = &out
	if cmd.Run() == nil {
		branch = strings.TrimSpace(out.String())
	}

	// Get last automated commit
	out.Reset()
	cmd = exec.Command("bash", "-c", `git log --oneline --grep="\[automated\]" -1 --format="%s"`)
	cmd.Dir = projectDir
	cmd.Stdout = &out
	if cmd.Run() == nil {
		lastCommit = strings.TrimSpace(out.String())
	}

	return clean, branch, lastCommit
}

// countDeviations counts the number of DEVIATE entries in the deviation log.
func countDeviations(projectDir string) int {
	path := filepath.Join(projectDir, config.DeviationLogFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	return strings.Count(string(data), "DEVIATE")
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

var severityRe = regexp.MustCompile(`\b(CRITICAL|HIGH|MODERATE|LOW)\b`)

// extractMaxSeverity returns the highest severity found in audit content.
func extractMaxSeverity(content string) string {
	maxRank := 0
	maxSev := ""
	for _, m := range severityRe.FindAllString(strings.ToUpper(content), -1) {
		r := sevRank(m)
		if r > maxRank {
			maxRank = r
			maxSev = m
		}
	}
	return maxSev
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
