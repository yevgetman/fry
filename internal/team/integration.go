package team

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func MaybeFinalize(ctx context.Context, projectDir, teamID string) error {
	return withFileLock(ctx, FinalizeLockPath(projectDir, teamID), func() error {
		cfg, err := LoadConfig(projectDir, teamID)
		if err != nil {
			return err
		}
		if cfg.Status == StatusComplete || cfg.Status == StatusFailed || cfg.Status == StatusShutdown {
			return nil
		}
		tasks, err := ListTasks(projectDir, teamID)
		if err != nil {
			return err
		}
		if len(tasks) == 0 {
			return nil
		}
		hasFailures := false
		for _, task := range tasks {
			switch task.Status {
			case TaskCompleted, TaskFailed:
				if task.Status == TaskFailed {
					hasFailures = true
				}
			default:
				return nil
			}
		}
		if hasFailures {
			cfg.Status = StatusFailed
			if err := SaveConfig(projectDir, cfg); err != nil {
				return err
			}
			return writeMergeReport(projectDir, teamID, "failed", "one or more tasks failed")
		}

		if cfg.GitIsolationMode == IsolationPerWorkerWorktree {
			if err := integrateWorktrees(ctx, cfg); err != nil {
				cfg.Status = StatusFailed
				if saveErr := SaveConfig(projectDir, cfg); saveErr != nil {
					return saveErr
				}
				_ = writeMergeReport(projectDir, teamID, "failed", err.Error())
				return err
			}
			if err := emitEvent(projectDir, teamID, Event{
				Type:   "team_merge_ready",
				TeamID: teamID,
				Data: map[string]string{
					"path": IntegratedOutputDir(projectDir, teamID),
				},
			}); err != nil {
				return err
			}
		} else if err := writeMergeReport(projectDir, teamID, "shared", cfg.ProjectDir); err != nil {
			return err
		}

		cfg.Status = StatusComplete
		if err := SaveConfig(projectDir, cfg); err != nil {
			return err
		}
		return emitEvent(projectDir, teamID, Event{
			Type:   "team_complete",
			TeamID: teamID,
		})
	})
}

func integrateWorktrees(ctx context.Context, cfg *Config) error {
	projectDir := cfg.ProjectDir
	teamID := cfg.TeamID
	integrationDir := IntegratedOutputDir(projectDir, teamID)
	_ = os.RemoveAll(integrationDir)
	_ = DefaultGitExecutor.WorktreePrune(ctx, projectDir)
	if err := os.MkdirAll(filepath.Dir(integrationDir), 0o755); err != nil {
		return err
	}

	branchName := fmt.Sprintf("fry/team-%s-integrated", sanitizeBranchComponent(teamID))
	create := !DefaultGitExecutor.BranchExists(ctx, projectDir, branchName)
	if err := DefaultGitExecutor.WorktreeAdd(ctx, projectDir, integrationDir, branchName, create); err != nil {
		return err
	}

	workerIDs, err := ListWorkerIDs(projectDir, teamID)
	if err != nil {
		return err
	}
	sort.Strings(workerIDs)
	var merged []string
	for _, workerID := range workerIDs {
		identity, err := LoadIdentity(projectDir, teamID, workerID)
		if err != nil {
			return err
		}
		if identity.WorktreeBranch == "" {
			continue
		}
		status, err := DefaultGitExecutor.StatusPorcelain(ctx, identity.WorkDir)
		if err == nil && strings.TrimSpace(status) != "" {
			return fmt.Errorf("worker %s has uncommitted changes in %s", workerID, identity.WorkDir)
		}
		if err := runGitMerge(ctx, integrationDir, identity.WorktreeBranch); err != nil {
			return err
		}
		merged = append(merged, identity.WorktreeBranch)
	}
	return writeMergeReport(projectDir, teamID, "merged", strings.Join(merged, "\n"))
}

func runGitMerge(ctx context.Context, dir, branch string) error {
	cmd := exec.CommandContext(ctx, "git", "merge", branch, "--no-edit")
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("merge %s: %s", branch, strings.TrimSpace(string(output)))
	}
	return nil
}

func writeMergeReport(projectDir, teamID, mode, details string) error {
	body := fmt.Sprintf("# Team Merge Report\n\n- status: %s\n- generated_at: %s\n\n%s\n",
		mode,
		time.Now().UTC().Format(time.RFC3339),
		details,
	)
	return os.WriteFile(MergeReportPath(projectDir, teamID), []byte(body), 0o644)
}

func sanitizeBranchComponent(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	return strings.Trim(b.String(), "-")
}
