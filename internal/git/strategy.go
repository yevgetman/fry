package git

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/yevgetman/fry/internal/config"
	"github.com/yevgetman/fry/internal/textutil"
)

// StrategyOpts configures SetupStrategy.
type StrategyOpts struct {
	ProjectDir string
	Strategy   GitStrategy
	BranchName string // empty = auto-generate from EpicName
	EpicName   string // used for auto-generated branch names
	ForceReuse bool   // true when --continue/--resume (reuse existing branch/worktree)
}

// SetupStrategy configures the git branch/worktree based on the chosen strategy.
// It validates preconditions, creates branches/worktrees as needed, and returns
// a StrategySetup with the effective working directory.
func SetupStrategy(ctx context.Context, opts StrategyOpts) (*StrategySetup, error) {
	if opts.Strategy == StrategyAuto {
		return nil, fmt.Errorf("git strategy must be resolved before calling SetupStrategy (got auto)")
	}
	if opts.Strategy == StrategyCurrent {
		return &StrategySetup{
			WorkDir:     opts.ProjectDir,
			OriginalDir: opts.ProjectDir,
			Strategy:    StrategyCurrent,
		}, nil
	}

	if !IsInsideGitRepo(ctx, opts.ProjectDir) {
		return nil, fmt.Errorf("git strategy %q requires an existing git repository; run 'git init' first or use --git-strategy current", opts.Strategy)
	}

	branchName := opts.BranchName
	if branchName == "" {
		branchName = GenerateBranchName(opts.EpicName)
	}

	origBranch := CurrentBranch(ctx, opts.ProjectDir)

	switch opts.Strategy {
	case StrategyBranch:
		return setupBranch(ctx, opts.ProjectDir, branchName, origBranch, opts.ForceReuse)
	case StrategyWorktree:
		return setupWorktree(ctx, opts.ProjectDir, branchName, origBranch, opts.ForceReuse)
	default:
		return nil, fmt.Errorf("unexpected git strategy %q", opts.Strategy)
	}
}

func setupBranch(ctx context.Context, projectDir, branchName, origBranch string, forceReuse bool) (*StrategySetup, error) {
	exists := branchExists(ctx, projectDir, branchName)

	if exists && !forceReuse {
		return nil, fmt.Errorf("branch %q already exists; use --branch-name to specify a different name, or delete it with: git branch -d %s", branchName, branchName)
	}

	if exists {
		if err := runBash(ctx, projectDir, fmt.Sprintf("git checkout %s", textutil.ShellQuote(branchName))); err != nil {
			return nil, fmt.Errorf("checkout existing branch: %w", err)
		}
	} else {
		if err := runBash(ctx, projectDir, fmt.Sprintf("git checkout -b %s", textutil.ShellQuote(branchName))); err != nil {
			return nil, fmt.Errorf("create branch: %w", err)
		}
	}

	return &StrategySetup{
		WorkDir:        projectDir,
		OriginalDir:    projectDir,
		BranchName:     branchName,
		OriginalBranch: origBranch,
		Strategy:       StrategyBranch,
	}, nil
}

func setupWorktree(ctx context.Context, projectDir, branchName, origBranch string, forceReuse bool) (*StrategySetup, error) {
	slug := worktreeSlug(branchName)
	worktreeDir := filepath.Join(projectDir, config.GitWorktreeDir, slug)

	exists := worktreeExists(ctx, projectDir, worktreeDir)

	if exists && !forceReuse {
		return nil, fmt.Errorf("worktree at %q already exists; remove it with: git worktree remove %s", worktreeDir, worktreeDir)
	}

	if exists {
		// Validate it's still a valid worktree
		if !IsInsideGitRepo(ctx, worktreeDir) {
			// Worktree directory exists but is invalid; prune and recreate
			_ = runBash(ctx, projectDir, "git worktree prune")
			exists = false
		}
	}

	if !exists {
		if err := os.MkdirAll(filepath.Dir(worktreeDir), 0o755); err != nil {
			return nil, fmt.Errorf("create worktree parent dir: %w", err)
		}

		branchFlag := fmt.Sprintf("-b %s", textutil.ShellQuote(branchName))
		if branchExists(ctx, projectDir, branchName) {
			branchFlag = textutil.ShellQuote(branchName)
		}

		cmd := fmt.Sprintf("git worktree add %s %s", textutil.ShellQuote(worktreeDir), branchFlag)
		if err := runBash(ctx, projectDir, cmd); err != nil {
			return nil, fmt.Errorf("create worktree: %w", err)
		}

		// Copy .fry/ and plans/ into worktree so sprint runner finds artifacts.
		// Only on fresh creation — when reusing an existing worktree (ForceReuse),
		// the worktree's .fry/ has the actual build progress and must not be overwritten.
		if err := copyDirIfExists(filepath.Join(projectDir, config.FryDir), filepath.Join(worktreeDir, config.FryDir)); err != nil {
			return nil, fmt.Errorf("copy .fry/ to worktree: %w", err)
		}
		if err := copyDirIfExists(filepath.Join(projectDir, config.PlansDir), filepath.Join(worktreeDir, config.PlansDir)); err != nil {
			return nil, fmt.Errorf("copy plans/ to worktree: %w", err)
		}
	}

	return &StrategySetup{
		WorkDir:        worktreeDir,
		OriginalDir:    projectDir,
		BranchName:     branchName,
		OriginalBranch: origBranch,
		Strategy:       StrategyWorktree,
		IsWorktree:     true,
	}, nil
}

// ResolveAutoStrategy maps triage complexity to a concrete strategy.
// COMPLEX -> StrategyWorktree, SIMPLE/MODERATE -> StrategyBranch.
func ResolveAutoStrategy(complexity string) GitStrategy {
	switch strings.ToUpper(complexity) {
	case "COMPLEX":
		return StrategyWorktree
	default:
		return StrategyBranch
	}
}

// GenerateBranchName creates a branch name from an epic name.
// Format: "fry/<slugified-epic-name>" (lowercase, hyphens, max 54 chars).
func GenerateBranchName(epicName string) string {
	slug := slugify(epicName)
	if slug == "" {
		slug = "build"
	}
	return config.GitBranchPrefix + slug
}

// IsInsideGitRepo checks if projectDir is inside a git repository.
func IsInsideGitRepo(ctx context.Context, projectDir string) bool {
	cmd := execCommand(ctx, "bash", "-c", "git rev-parse --is-inside-work-tree")
	cmd.Dir = projectDir
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		return false
	}
	return strings.TrimSpace(stdout.String()) == "true"
}

// CurrentBranch returns the name of the current git branch, or "" on error.
func CurrentBranch(ctx context.Context, projectDir string) string {
	cmd := execCommand(ctx, "bash", "-c", "git rev-parse --abbrev-ref HEAD")
	cmd.Dir = projectDir
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		return ""
	}
	return strings.TrimSpace(stdout.String())
}

// CheckoutBranch switches to the specified branch.
func CheckoutBranch(ctx context.Context, projectDir, branchName string) error {
	return runBash(ctx, projectDir, fmt.Sprintf("git checkout %s", textutil.ShellQuote(branchName)))
}

// DetectExistingSetup checks if a prior fry branch or worktree exists
// for the given branch name. Returns nil if nothing found.
func DetectExistingSetup(ctx context.Context, projectDir, branchName string) (*StrategySetup, error) {
	slug := worktreeSlug(branchName)
	worktreeDir := filepath.Join(projectDir, config.GitWorktreeDir, slug)

	// Check for worktree first
	if worktreeExists(ctx, projectDir, worktreeDir) && IsInsideGitRepo(ctx, worktreeDir) {
		return &StrategySetup{
			WorkDir:        worktreeDir,
			OriginalDir:    projectDir,
			BranchName:     branchName,
			OriginalBranch: CurrentBranch(ctx, projectDir),
			Strategy:       StrategyWorktree,
			IsWorktree:     true,
		}, nil
	}

	// Check for branch
	if branchExists(ctx, projectDir, branchName) {
		return &StrategySetup{
			WorkDir:        projectDir,
			OriginalDir:    projectDir,
			BranchName:     branchName,
			OriginalBranch: CurrentBranch(ctx, projectDir),
			Strategy:       StrategyBranch,
		}, nil
	}

	return nil, nil
}

// PersistStrategy writes the strategy setup to a file for --continue detection.
func PersistStrategy(originalDir string, setup *StrategySetup) error {
	path := filepath.Join(originalDir, config.GitStrategyFile)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create dir for git strategy file: %w", err)
	}
	content := fmt.Sprintf("strategy=%s\nbranch=%s\nworkdir=%s\noriginaldir=%s\n",
		setup.Strategy, setup.BranchName, setup.WorkDir, setup.OriginalDir)
	return os.WriteFile(path, []byte(content), 0o644)
}

// ReadPersistedStrategy reads a previously persisted strategy setup.
// Returns nil, nil if the file does not exist.
func ReadPersistedStrategy(projectDir string) (*StrategySetup, error) {
	path := filepath.Join(projectDir, config.GitStrategyFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read git strategy file: %w", err)
	}

	setup := &StrategySetup{}
	for _, line := range strings.Split(string(data), "\n") {
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key, val := parts[0], parts[1]
		switch key {
		case "strategy":
			parsed, parseErr := ParseGitStrategy(val)
			if parseErr != nil {
				return nil, parseErr
			}
			setup.Strategy = parsed
		case "branch":
			setup.BranchName = val
		case "workdir":
			setup.WorkDir = val
		case "originaldir":
			setup.OriginalDir = val
		}
	}

	if setup.Strategy == "" || setup.WorkDir == "" {
		return nil, fmt.Errorf("invalid git strategy file at %s", path)
	}

	setup.IsWorktree = setup.Strategy == StrategyWorktree
	return setup, nil
}

// --- helpers ---

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(s string) string {
	slug := strings.ToLower(s)
	slug = slugRe.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	if len(slug) > 50 {
		slug = slug[:50]
		slug = strings.TrimRight(slug, "-")
	}
	return slug
}

// worktreeSlug strips the branch prefix (e.g. "fry/") before slugifying
// to avoid redundant directory names like .fry-worktrees/fry-my-epic.
func worktreeSlug(branchName string) string {
	name := strings.TrimPrefix(branchName, config.GitBranchPrefix)
	return slugify(name)
}

func branchExists(ctx context.Context, projectDir, name string) bool {
	cmd := execCommand(ctx, "bash", "-c", fmt.Sprintf("git rev-parse --verify %s", textutil.ShellQuote("refs/heads/"+name)))
	cmd.Dir = projectDir
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run() == nil
}

func worktreeExists(ctx context.Context, projectDir, worktreeDir string) bool {
	absWT, err := filepath.Abs(worktreeDir)
	if err != nil {
		return false
	}
	cmd := execCommand(ctx, "bash", "-c", "git worktree list --porcelain")
	cmd.Dir = projectDir
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		return false
	}
	for _, line := range strings.Split(stdout.String(), "\n") {
		if strings.HasPrefix(line, "worktree ") {
			wt := strings.TrimPrefix(line, "worktree ")
			if wt == absWT {
				return true
			}
		}
	}
	return false
}

func copyDirIfExists(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if !info.IsDir() {
		return nil
	}

	return filepath.WalkDir(src, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)

		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}
