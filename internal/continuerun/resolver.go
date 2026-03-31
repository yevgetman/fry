package continuerun

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/yevgetman/fry/internal/agent"
	"github.com/yevgetman/fry/internal/config"
	"github.com/yevgetman/fry/internal/git"
)

// ContinueTarget describes the canonical project directory to use for a
// continue/resume operation after reconciling original-project and persisted
// worktree state.
type ContinueTarget struct {
	ProjectDir string
	Strategy   *git.StrategySetup
	Reason     string
}

type stateDirInfo struct {
	Path          string
	Completed     int
	Latest        time.Time
	HasArtifacts  bool
	BuildUpdated  time.Time
	ProgressStamp time.Time
	ExitStamp     time.Time
}

// ResolveContinueTarget chooses the canonical state directory for continue-
// style commands. When a persisted worktree exists, it compares both the
// original project and worktree state and picks the more advanced/newer one.
func ResolveContinueTarget(ctx context.Context, projectDir string) (*ContinueTarget, error) {
	persisted, err := git.ReadPersistedStrategy(projectDir)
	if err != nil {
		return nil, err
	}
	if persisted == nil {
		return &ContinueTarget{
			ProjectDir: projectDir,
			Reason:     "no persisted git strategy; using project directory",
		}, nil
	}
	if !persisted.IsWorktree || persisted.WorkDir == "" || persisted.WorkDir == projectDir {
		return &ContinueTarget{
			ProjectDir: projectDir,
			Strategy:   persisted,
			Reason:     "persisted strategy is not a separate worktree; using project directory",
		}, nil
	}
	if !git.IsInsideGitRepo(ctx, persisted.WorkDir) {
		return &ContinueTarget{
			ProjectDir: projectDir,
			Reason:     "persisted worktree is unavailable; using project directory",
		}, nil
	}

	orig := collectStateDirInfo(projectDir)
	worktree := collectStateDirInfo(persisted.WorkDir)
	if preferStateDir(orig, worktree) {
		return &ContinueTarget{
			ProjectDir: projectDir,
			Reason:     "original project state is newer or more complete than persisted worktree state",
		}, nil
	}
	return &ContinueTarget{
		ProjectDir: persisted.WorkDir,
		Strategy:   persisted,
		Reason:     "persisted worktree state is canonical for continue/resume",
	}, nil
}

func collectStateDirInfo(dir string) stateDirInfo {
	info := stateDirInfo{Path: dir}

	epicProgressPath := filepath.Join(dir, config.EpicProgressFile)
	if data, err := os.ReadFile(epicProgressPath); err == nil {
		info.Completed = len(ParseCompletedSprints(string(data)))
		info.HasArtifacts = true
		if stat, statErr := os.Stat(epicProgressPath); statErr == nil {
			info.ProgressStamp = stat.ModTime()
			info.Latest = maxTime(info.Latest, stat.ModTime())
		}
	}

	buildStatusPath := filepath.Join(dir, config.BuildStatusFile)
	if st, err := agent.ReadBuildStatus(dir); err == nil && st != nil {
		info.HasArtifacts = true
		info.BuildUpdated = st.UpdatedAt
		info.Latest = maxTime(info.Latest, st.UpdatedAt)
	} else if stat, statErr := os.Stat(buildStatusPath); statErr == nil {
		info.HasArtifacts = true
		info.Latest = maxTime(info.Latest, stat.ModTime())
	}

	exitReasonPath := filepath.Join(dir, config.BuildExitReasonFile)
	if stat, err := os.Stat(exitReasonPath); err == nil {
		info.HasArtifacts = true
		info.ExitStamp = stat.ModTime()
		info.Latest = maxTime(info.Latest, stat.ModTime())
	}

	phasePath := filepath.Join(dir, config.BuildPhaseFile)
	if stat, err := os.Stat(phasePath); err == nil {
		info.HasArtifacts = true
		info.Latest = maxTime(info.Latest, stat.ModTime())
	}

	return info
}

func preferStateDir(a, b stateDirInfo) bool {
	if a.Completed != b.Completed {
		return a.Completed > b.Completed
	}
	if !a.Latest.Equal(b.Latest) {
		return a.Latest.After(b.Latest)
	}
	if a.HasArtifacts != b.HasArtifacts {
		return a.HasArtifacts
	}
	return true
}

func maxTime(a, b time.Time) time.Time {
	if b.After(a) {
		return b
	}
	return a
}
