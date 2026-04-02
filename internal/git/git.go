package git

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	frylog "github.com/yevgetman/fry/internal/log"
)

// InitGit initializes a git repository with local identity and .gitignore entries.
func InitGit(ctx context.Context, projectDir string) error {
	return InitGitWith(ctx, projectDir, DefaultExecutor)
}

// InitGitWith is like InitGit but uses the provided Executor.
func InitGitWith(ctx context.Context, projectDir string, ex Executor) error {
	if _, err := os.Stat(filepath.Join(projectDir, ".git")); os.IsNotExist(err) {
		if err := ex.Init(ctx, projectDir); err != nil {
			return fmt.Errorf("init git: %w", err)
		}
	}

	if err := ensureLocalIdentityWith(ctx, projectDir, ex); err != nil {
		return err
	}
	if err := ensureGitignoreEntries(projectDir, []string{".fry/", ".fry-archive/", ".env", ".DS_Store", ".fry-worktrees/"}); err != nil {
		return err
	}

	if ex.HasHead(ctx, projectDir) {
		return nil
	}

	if err := ex.AddAll(ctx, projectDir); err != nil {
		return fmt.Errorf("initial commit: add: %w", err)
	}
	if err := ex.CommitAllowEmpty(ctx, projectDir, "Initial commit [automated]"); err != nil {
		return fmt.Errorf("initial commit: %w", err)
	}
	return nil
}

// GitCheckpoint creates a git commit capturing all current changes.
func GitCheckpoint(ctx context.Context, projectDir, epicName string, sprintNum int, sprintName, label string) error {
	return GitCheckpointWith(ctx, projectDir, epicName, sprintNum, sprintName, label, DefaultExecutor)
}

// GitCheckpointWith is like GitCheckpoint but uses the provided Executor.
func GitCheckpointWith(ctx context.Context, projectDir, epicName string, sprintNum int, sprintName, label string, ex Executor) error {
	var message string
	if sprintName != "" {
		message = fmt.Sprintf("%s — %s: Sprint %d %s [automated]", epicName, sprintName, sprintNum, label)
	} else {
		message = fmt.Sprintf("%s: Sprint %d %s [automated]", epicName, sprintNum, label)
	}
	if err := ex.AddAll(ctx, projectDir); err != nil {
		return fmt.Errorf("git checkpoint: add: %w", err)
	}
	if err := ex.CommitAllowEmpty(ctx, projectDir, message); err != nil {
		return fmt.Errorf("git checkpoint: %w", err)
	}
	return nil
}

// CommitPartialWork commits partial work from a failed sprint.
func CommitPartialWork(ctx context.Context, projectDir, epicName string, sprintNum int, sprintName string) error {
	return GitCheckpoint(ctx, projectDir, epicName, sprintNum, sprintName, "failed-partial")
}

// CommitPartialWorkWith is like CommitPartialWork but uses the provided Executor.
func CommitPartialWorkWith(ctx context.Context, projectDir, epicName string, sprintNum int, sprintName string, ex Executor) error {
	return GitCheckpointWith(ctx, projectDir, epicName, sprintNum, sprintName, "failed-partial", ex)
}

// GitDiffForAudit returns a full diff including untracked files, excluding .fry/.
func GitDiffForAudit(ctx context.Context, projectDir string) (string, error) {
	return GitDiffForAuditWith(ctx, projectDir, DefaultExecutor)
}

// GitDiffForAuditWith is like GitDiffForAudit but uses the provided Executor.
func GitDiffForAuditWith(ctx context.Context, projectDir string, ex Executor) (string, error) {
	untrackedPaths, err := ex.ListUntracked(ctx, projectDir, []string{":(exclude).fry/"})
	if err != nil {
		untrackedPaths = nil
	}

	if len(untrackedPaths) > 0 {
		if addErr := ex.AddIntent(ctx, projectDir, untrackedPaths); addErr != nil {
			untrackedPaths = nil
		}
	}

	diff, diffErr := ex.DiffHead(ctx, projectDir, []string{":(exclude).fry/"})

	// Undo only the temporary intent-to-add entries we created for untracked files.
	if len(untrackedPaths) > 0 {
		if resetErr := ex.Reset(ctx, projectDir, untrackedPaths); resetErr != nil {
			fmt.Fprintf(os.Stderr, "fry: warning: git reset after diff failed: %v\n", resetErr)
		}
	}

	if diffErr != nil {
		return "", fmt.Errorf("git diff for audit: %w", diffErr)
	}
	return diff, nil
}

// DiffStatForNoopDetection returns git diff --stat output excluding progress files,
// for use in no-op detection. Returns a unique error string on failure to prevent
// false-positive no-op matching.
func DiffStatForNoopDetection(ctx context.Context, projectDir string) string {
	return DiffStatForNoopDetectionWith(ctx, projectDir, DefaultExecutor)
}

// DiffStatForNoopDetectionWith is like DiffStatForNoopDetection but uses the provided Executor.
func DiffStatForNoopDetectionWith(ctx context.Context, projectDir string, ex Executor) string {
	out, err := ex.DiffStat(ctx, projectDir, []string{
		":(exclude).fry/sprint-progress.txt",
		":(exclude).fry/epic-progress.txt",
	})
	if err != nil {
		frylog.Log("WARNING: git diff --stat failed: %v", err)
		return fmt.Sprintf("__git_error_%d__", time.Now().UnixNano())
	}
	return out
}

// WorktreeFingerprintForNoopDetection returns a fingerprint of the working tree
// suitable for no-op detection. It combines diff-stat output with filtered
// porcelain status so untracked-file changes count as real work while progress
// file writes remain excluded.
func WorktreeFingerprintForNoopDetection(ctx context.Context, projectDir string) string {
	return WorktreeFingerprintForNoopDetectionWith(ctx, projectDir, DefaultExecutor)
}

// WorktreeFingerprintForNoopDetectionWith is like WorktreeFingerprintForNoopDetection
// but uses the provided Executor.
func WorktreeFingerprintForNoopDetectionWith(ctx context.Context, projectDir string, ex Executor) string {
	diff := DiffStatForNoopDetectionWith(ctx, projectDir, ex)
	status, err := ex.StatusPorcelain(ctx, projectDir)
	if err != nil {
		frylog.Log("WARNING: git status --porcelain failed: %v", err)
		return fmt.Sprintf("__git_status_error_%d__", time.Now().UnixNano())
	}

	filteredStatus := filterStatusForNoopDetection(status)
	diff = strings.TrimSpace(diff)
	filteredStatus = strings.TrimSpace(filteredStatus)

	switch {
	case diff == "" && filteredStatus == "":
		return ""
	case diff == "":
		return filteredStatus
	case filteredStatus == "":
		return diff
	default:
		return diff + "\n--status--\n" + filteredStatus
	}
}

// CollectState returns git working tree state for build resumption reporting.
func CollectState(ctx context.Context, projectDir string) (clean bool, branch string, lastAutoCommit string) {
	return CollectStateWith(ctx, projectDir, DefaultExecutor)
}

// CollectStateWith is like CollectState but uses the provided Executor.
func CollectStateWith(ctx context.Context, projectDir string, ex Executor) (bool, string, string) {
	clean := true
	status, err := ex.StatusPorcelain(ctx, projectDir)
	if err == nil {
		clean = strings.TrimSpace(status) == ""
	} else {
		frylog.Log("WARNING: git status --porcelain failed: %v", err)
	}

	branch := ex.CurrentBranch(ctx, projectDir)

	lastCommit := ""
	out, err := ex.LogGrep(ctx, projectDir, `\[automated\]`, 1, "%s")
	if err == nil {
		lastCommit = strings.TrimSpace(out)
	}

	return clean, branch, lastCommit
}

func filterStatusForNoopDetection(status string) string {
	var kept []string
	for _, line := range strings.Split(status, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.Contains(line, ".fry/sprint-progress.txt") || strings.Contains(line, ".fry/epic-progress.txt") {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}

// gitConfigValue is an unexported convenience that uses DefaultExecutor.
// Used by tests in the same package.
func gitConfigValue(ctx context.Context, projectDir, key string) (string, error) {
	return DefaultExecutor.ConfigGet(ctx, projectDir, key)
}

// hasHead is an unexported convenience that uses DefaultExecutor.
func hasHead(ctx context.Context, projectDir string) bool {
	return DefaultExecutor.HasHead(ctx, projectDir)
}

// ensureLocalIdentity uses DefaultExecutor. Called by tests in the same package.
func ensureLocalIdentity(ctx context.Context, projectDir string) error {
	return ensureLocalIdentityWith(ctx, projectDir, DefaultExecutor)
}

func ensureLocalIdentityWith(ctx context.Context, projectDir string, ex Executor) error {
	name, err := ex.ConfigGet(ctx, projectDir, "user.name")
	if err != nil {
		return fmt.Errorf("get git user.name: %w", err)
	}
	if strings.TrimSpace(name) == "" {
		if err := ex.ConfigSet(ctx, projectDir, "user.name", "fry"); err != nil {
			return fmt.Errorf("set git user.name: %w", err)
		}
	}
	email, err := ex.ConfigGet(ctx, projectDir, "user.email")
	if err != nil {
		return fmt.Errorf("get git user.email: %w", err)
	}
	if strings.TrimSpace(email) == "" {
		if err := ex.ConfigSet(ctx, projectDir, "user.email", "fry@automated"); err != nil {
			return fmt.Errorf("set git user.email: %w", err)
		}
	}
	return nil
}

func ensureGitignoreEntries(projectDir string, entries []string) error {
	path := filepath.Join(projectDir, ".gitignore")
	existing := map[string]bool{}

	if data, err := os.ReadFile(path); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			existing[strings.TrimSpace(line)] = true
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("read .gitignore: %w", err)
	}

	var toAppend []string
	for _, entry := range entries {
		if !existing[entry] {
			toAppend = append(toAppend, entry)
		}
	}
	if len(toAppend) == 0 {
		return nil
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open .gitignore: %w", err)
	}
	defer file.Close()

	if info, err := file.Stat(); err == nil && info.Size() > 0 {
		if _, err := file.WriteString("\n"); err != nil {
			return fmt.Errorf("separate .gitignore entries: %w", err)
		}
	}
	if _, err := file.WriteString(strings.Join(toAppend, "\n") + "\n"); err != nil {
		return fmt.Errorf("write .gitignore entries: %w", err)
	}
	return nil
}
