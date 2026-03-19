package git

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/yevgetman/fry/internal/textutil"
)

var execCommand = exec.Command

func InitGit(projectDir string) error {
	if _, err := os.Stat(filepath.Join(projectDir, ".git")); os.IsNotExist(err) {
		if err := runBash(projectDir, "git init"); err != nil {
			return fmt.Errorf("init git: %w", err)
		}
	}

	if err := ensureLocalIdentity(projectDir); err != nil {
		return err
	}
	if err := ensureGitignoreEntries(projectDir, []string{".fry/", ".fry-archive/", ".env", ".DS_Store"}); err != nil {
		return err
	}

	if hasHead(projectDir) {
		return nil
	}

	if err := runBash(projectDir, "git add -A && git commit --allow-empty -m 'Initial commit [automated]'"); err != nil {
		return fmt.Errorf("initial commit: %w", err)
	}
	return nil
}

func GitCheckpoint(projectDir, epicName string, sprintNum int, label string) error {
	cmd := fmt.Sprintf(
		"git add -A && git commit --allow-empty -m %s",
		textutil.ShellQuote(fmt.Sprintf("%s: Sprint %d %s [automated]", epicName, sprintNum, label)),
	)
	if err := runBash(projectDir, cmd); err != nil {
		return fmt.Errorf("git checkpoint: %w", err)
	}
	return nil
}

func CommitPartialWork(projectDir, epicName string, sprintNum int) error {
	return GitCheckpoint(projectDir, epicName, sprintNum, "failed-partial")
}

func ensureLocalIdentity(projectDir string) error {
	if strings.TrimSpace(gitConfigValue(projectDir, "user.name")) == "" {
		if err := runBash(projectDir, "git config user.name fry"); err != nil {
			return fmt.Errorf("set git user.name: %w", err)
		}
	}
	if strings.TrimSpace(gitConfigValue(projectDir, "user.email")) == "" {
		if err := runBash(projectDir, "git config user.email fry@automated"); err != nil {
			return fmt.Errorf("set git user.email: %w", err)
		}
	}
	return nil
}

func gitConfigValue(projectDir, key string) string {
	cmd := execCommand("bash", "-c", "git config --get "+textutil.ShellQuote(key))
	cmd.Dir = projectDir
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	_ = cmd.Run()
	return stdout.String()
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

func hasHead(projectDir string) bool {
	cmd := execCommand("bash", "-c", "git rev-parse --verify HEAD")
	cmd.Dir = projectDir
	return cmd.Run() == nil
}

func runBash(projectDir, command string) error {
	cmd := execCommand("bash", "-c", command)
	cmd.Dir = projectDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", command, strings.TrimSpace(string(output)))
	}
	return nil
}

func GitDiffForAudit(projectDir string) (string, error) {
	untrackedPaths, err := listUntrackedPaths(projectDir)
	if err == nil && len(untrackedPaths) > 0 {
		if addErr := runGit(projectDir, append([]string{"add", "-N", "--"}, untrackedPaths...)...); addErr != nil {
			// Non-fatal: if this fails, we still try to get a diff of tracked files.
			untrackedPaths = nil
		}
	}

	cmd := execCommand("bash", "-c", "git diff HEAD -- . ':!.fry/'")
	cmd.Dir = projectDir
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = nil
	err = cmd.Run()

	// Undo only the temporary intent-to-add entries we created for untracked files.
	if len(untrackedPaths) > 0 {
		if resetErr := runGit(projectDir, append([]string{"reset", "--"}, untrackedPaths...)...); resetErr != nil {
			fmt.Fprintf(os.Stderr, "fry: warning: git reset after diff failed: %v\n", resetErr)
		}
	}

	if err != nil {
		return "", fmt.Errorf("git diff for audit: %w", err)
	}
	return stdout.String(), nil
}

func listUntrackedPaths(projectDir string) ([]string, error) {
	cmd := execCommand("bash", "-c", "git ls-files --others --exclude-standard -- . ':!.fry/'")
	cmd.Dir = projectDir
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("list untracked paths: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	var paths []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			paths = append(paths, line)
		}
	}
	return paths, nil
}

func runGit(projectDir string, args ...string) error {
	cmd := execCommand("git", args...)
	cmd.Dir = projectDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(string(output)))
	}
	return nil
}
