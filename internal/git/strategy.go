package git

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/yevgetman/fry/internal/config"
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
func SetupStrategy(ctx context.Context, opts StrategyOpts) (*StrategySetup, error) {
	return SetupStrategyWith(ctx, opts, DefaultExecutor)
}

// SetupStrategyWith is like SetupStrategy but uses the provided Executor.
func SetupStrategyWith(ctx context.Context, opts StrategyOpts, ex Executor) (*StrategySetup, error) {
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

	if !ex.IsRepo(ctx, opts.ProjectDir) {
		return nil, fmt.Errorf("git strategy %q requires an existing git repository; run 'git init' first or use --git-strategy current", opts.Strategy)
	}

	branchName := opts.BranchName
	if branchName == "" {
		branchName = GenerateBranchName(opts.EpicName)
	}

	origBranch := ex.CurrentBranch(ctx, opts.ProjectDir)

	switch opts.Strategy {
	case StrategyBranch:
		return setupBranch(ctx, opts.ProjectDir, branchName, origBranch, opts.ForceReuse, ex)
	case StrategyWorktree:
		return setupWorktree(ctx, opts.ProjectDir, branchName, origBranch, opts.ForceReuse, ex)
	default:
		return nil, fmt.Errorf("unexpected git strategy %q", opts.Strategy)
	}
}

func setupBranch(ctx context.Context, projectDir, branchName, origBranch string, forceReuse bool, ex Executor) (*StrategySetup, error) {
	exists := ex.BranchExists(ctx, projectDir, branchName)

	if exists && !forceReuse {
		return nil, fmt.Errorf("branch %q already exists; use --branch-name to specify a different name, or delete it with: git branch -d %s", branchName, branchName)
	}

	if exists {
		if err := ex.Checkout(ctx, projectDir, branchName); err != nil {
			return nil, fmt.Errorf("checkout existing branch: %w", err)
		}
	} else {
		if err := ex.CreateAndCheckout(ctx, projectDir, branchName); err != nil {
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

func setupWorktree(ctx context.Context, projectDir, branchName, origBranch string, forceReuse bool, ex Executor) (*StrategySetup, error) {
	slug := worktreeSlug(branchName)
	worktreeDir := filepath.Join(projectDir, config.GitWorktreeDir, slug)

	exists := worktreeExists(ctx, projectDir, worktreeDir, ex)

	if exists && !forceReuse {
		return nil, fmt.Errorf("worktree at %q already exists; remove it with: git worktree remove %s", worktreeDir, worktreeDir)
	}

	if exists {
		// Validate it's still a valid worktree
		if !ex.IsRepo(ctx, worktreeDir) {
			// Worktree directory exists but is invalid; prune and recreate
			_ = ex.WorktreePrune(ctx, projectDir)
			exists = false
		}
	}

	if !exists {
		if err := os.MkdirAll(filepath.Dir(worktreeDir), 0o755); err != nil {
			return nil, fmt.Errorf("create worktree parent dir: %w", err)
		}

		createBranch := !ex.BranchExists(ctx, projectDir, branchName)
		if err := ex.WorktreeAdd(ctx, projectDir, worktreeDir, branchName, createBranch); err != nil {
			return nil, fmt.Errorf("create worktree: %w", err)
		}

		// Copy .fry/ and plans/ into worktree so sprint runner finds artifacts.
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
	return DefaultExecutor.IsRepo(ctx, projectDir)
}

// IsInsideGitRepoWith is like IsInsideGitRepo but uses the provided Executor.
func IsInsideGitRepoWith(ctx context.Context, projectDir string, ex Executor) bool {
	return ex.IsRepo(ctx, projectDir)
}

// CurrentBranch returns the name of the current git branch, or "" on error.
func CurrentBranch(ctx context.Context, projectDir string) string {
	return DefaultExecutor.CurrentBranch(ctx, projectDir)
}

// CurrentBranchWith is like CurrentBranch but uses the provided Executor.
func CurrentBranchWith(ctx context.Context, projectDir string, ex Executor) string {
	return ex.CurrentBranch(ctx, projectDir)
}

// CheckoutBranch switches to the specified branch.
func CheckoutBranch(ctx context.Context, projectDir, branchName string) error {
	return CheckoutBranchWith(ctx, projectDir, branchName, DefaultExecutor)
}

// CheckoutBranchWith is like CheckoutBranch but uses the provided Executor.
func CheckoutBranchWith(ctx context.Context, projectDir, branchName string, ex Executor) error {
	return ex.Checkout(ctx, projectDir, branchName)
}

// DetectExistingSetup checks if a prior fry branch or worktree exists
// for the given branch name. Returns nil if nothing found.
func DetectExistingSetup(ctx context.Context, projectDir, branchName string) (*StrategySetup, error) {
	return DetectExistingSetupWith(ctx, projectDir, branchName, DefaultExecutor)
}

// DetectExistingSetupWith is like DetectExistingSetup but uses the provided Executor.
func DetectExistingSetupWith(ctx context.Context, projectDir, branchName string, ex Executor) (*StrategySetup, error) {
	slug := worktreeSlug(branchName)
	worktreeDir := filepath.Join(projectDir, config.GitWorktreeDir, slug)

	// Check for worktree first
	if worktreeExists(ctx, projectDir, worktreeDir, ex) && ex.IsRepo(ctx, worktreeDir) {
		return &StrategySetup{
			WorkDir:        worktreeDir,
			OriginalDir:    projectDir,
			BranchName:     branchName,
			OriginalBranch: ex.CurrentBranch(ctx, projectDir),
			Strategy:       StrategyWorktree,
			IsWorktree:     true,
		}, nil
	}

	// Check for branch
	if ex.BranchExists(ctx, projectDir, branchName) {
		return &StrategySetup{
			WorkDir:        projectDir,
			OriginalDir:    projectDir,
			BranchName:     branchName,
			OriginalBranch: ex.CurrentBranch(ctx, projectDir),
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

func worktreeExists(ctx context.Context, projectDir, worktreeDir string, ex Executor) bool {
	absWT, err := filepath.Abs(worktreeDir)
	if err != nil {
		return false
	}
	paths, err := ex.WorktreeList(ctx, projectDir)
	if err != nil {
		return false
	}
	for _, wt := range paths {
		if wt == absWT {
			return true
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
