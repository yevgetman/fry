package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/yevgetman/fry/internal/agent"
	"github.com/yevgetman/fry/internal/archive"
	"github.com/yevgetman/fry/internal/config"
	"github.com/yevgetman/fry/internal/consciousness"
	"github.com/yevgetman/fry/internal/continuerun"
	"github.com/yevgetman/fry/internal/epic"
	"github.com/yevgetman/fry/internal/git"
	"github.com/yevgetman/fry/internal/lock"
	"github.com/yevgetman/fry/internal/team"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current build state without making an LLM call",
	RunE: func(cmd *cobra.Command, args []string) error {
		projectDir, _ := cmd.Flags().GetString("project-dir")
		buildDir, _ := resolveBuildDir(projectDir)

		// --runs: list all run snapshots
		showRuns, _ := cmd.Flags().GetBool("runs")
		if showRuns {
			return runStatusRuns(cmd, projectDir)
		}

		// --run <id>: show a specific run
		runID, _ := cmd.Flags().GetString("run")
		if runID != "" {
			return runStatusByRunID(cmd, projectDir, runID)
		}

		jsonOutput, _ := cmd.Flags().GetBool("json")
		if jsonOutput {
			return runStatusJSON(cmd, projectDir)
		}

		showConsciousness, _ := cmd.Flags().GetBool("consciousness")
		showConsciousnessRemote, _ := cmd.Flags().GetBool("consciousness-remote")
		if showConsciousnessRemote {
			ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
			defer cancel()

			stats, err := consciousness.FetchPipelineStats(ctx, config.ConsciousnessAPIURL)
			if err != nil {
				return fmt.Errorf("consciousness status: %w", err)
			}
			fmt.Fprint(cmd.OutOrStdout(), consciousness.FormatPipelineStats(stats))
			return nil
		}
		if showConsciousness {
			status, err := consciousness.ReadLocalStatus(buildDir)
			if err != nil {
				return fmt.Errorf("consciousness status: %w", err)
			}
			fmt.Fprint(cmd.OutOrStdout(), consciousness.FormatLocalStatus(status))
			return nil
		}

		return runStatusHumanReadable(cmd, projectDir)
	},
}

func init() {
	statusCmd.Flags().Bool("consciousness", false, "Show local consciousness session health")
	statusCmd.Flags().Bool("consciousness-remote", false, "Show remote consciousness pipeline stats")
	statusCmd.Flags().Bool("json", false, "Output build state as JSON (for agent consumption)")
	statusCmd.Flags().Bool("runs", false, "List all run snapshots")
	statusCmd.Flags().String("run", "", "Show status for a specific run ID")
	rootCmd.AddCommand(statusCmd)
}

// resolveBuildDir returns the directory where build artifacts live, which may
// be a worktree directory if the build used the worktree strategy.
func resolveBuildDir(projectDir string) (buildDir string, worktreeDir string) {
	buildDir = projectDir
	if setup, err := git.ReadPersistedStrategy(projectDir); err == nil && setup != nil && setup.WorkDir != "" && setup.WorkDir != projectDir {
		// Verify the worktree directory actually exists
		if _, statErr := os.Stat(setup.WorkDir); statErr == nil {
			buildDir = setup.WorkDir
			worktreeDir = setup.WorkDir
		}
	}
	return buildDir, worktreeDir
}

// readPhase reads build-phase.txt from one or more directories, returning
// the first non-empty phase found.
func readPhase(dirs ...string) string {
	for _, dir := range dirs {
		if data, err := os.ReadFile(filepath.Join(dir, config.BuildPhaseFile)); err == nil {
			if phase := strings.TrimSpace(string(data)); phase != "" {
				return phase
			}
		}
	}
	return ""
}

// runStatusRuns lists all run snapshots in .fry/runs/.
func runStatusRuns(cmd *cobra.Command, projectDir string) error {
	buildDir, _ := resolveBuildDir(projectDir)

	runs, err := agent.ScanRuns(buildDir)
	if err != nil {
		return fmt.Errorf("scan runs: %w", err)
	}
	if len(runs) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No run snapshots found.")
		return nil
	}

	jsonOutput, _ := cmd.Flags().GetBool("json")
	if jsonOutput {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(runs)
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "RUN ID\tTYPE\tSTATUS\tSPRINTS\tSTARTED\tPARENT")
	for _, r := range runs {
		parent := r.ParentID
		if parent == "" {
			parent = "-"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\t%s\n",
			r.RunID,
			r.RunType,
			r.Status,
			r.Sprints,
			r.StartedAt.Format("2006-01-02 15:04"),
			parent,
		)
	}
	return w.Flush()
}

// runStatusByRunID shows the build status for a specific run ID.
func runStatusByRunID(cmd *cobra.Command, projectDir, runID string) error {
	buildDir, _ := resolveBuildDir(projectDir)

	// Allow prefix match: if the user gives a short prefix, find the first match.
	if !strings.HasPrefix(runID, config.RunPrefix) {
		runID = config.RunPrefix + runID
	}

	// Reject path traversal in run IDs.
	if strings.ContainsAny(runID, "/\\") || strings.Contains(runID, "..") {
		return fmt.Errorf("invalid run ID %q", runID)
	}

	status, err := agent.ReadRunStatus(buildDir, runID)
	if err != nil {
		return fmt.Errorf("read run %s: %w", runID, err)
	}
	if status == nil {
		return fmt.Errorf("run %s not found", runID)
	}

	jsonOutput, _ := cmd.Flags().GetBool("json")
	if jsonOutput {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(status)
	}

	// Human-readable output for a specific run
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Run:     %s\n", runID)
	if status.Run != nil {
		fmt.Fprintf(out, "Type:    %s\n", status.Run.RunType)
		if status.Run.ParentRunID != "" {
			fmt.Fprintf(out, "Parent:  %s\n", status.Run.ParentRunID)
		}
	}
	fmt.Fprintf(out, "Epic:    %s\n", status.Build.Epic)
	fmt.Fprintf(out, "Status:  %s\n", status.Build.Status)
	if status.Build.Phase != "" {
		fmt.Fprintf(out, "Phase:   %s\n", status.Build.Phase)
	}
	fmt.Fprintf(out, "Engine:  %s\n", status.Build.Engine)
	fmt.Fprintf(out, "Effort:  %s\n", status.Build.Effort)
	fmt.Fprintf(out, "Started: %s\n", status.Build.StartedAt.Format(time.RFC3339))
	fmt.Fprintf(out, "Updated: %s\n", status.UpdatedAt.Format(time.RFC3339))
	fmt.Fprintf(out, "\nSprints (%d):\n", len(status.Sprints))
	for _, sp := range status.Sprints {
		fmt.Fprintf(out, "  %2d. %-40s %s\n", sp.Number, sp.Name, sp.Status)
	}
	return nil
}

func runStatusJSON(cmd *cobra.Command, projectDir string) error {
	projectDir, _ = filepath.Abs(projectDir)
	buildDir, worktreeDir := resolveBuildDir(projectDir)

	state, err := agent.ReadBuildState(buildDir)
	if err != nil {
		return fmt.Errorf("read build state: %w", err)
	}

	// Always show the original project dir, not the worktree
	state.ProjectDir = projectDir
	if worktreeDir != "" {
		state.WorktreeDir = worktreeDir
	}

	// For worktree builds, the lock lives in the original project dir.
	// ReadBuildState checked the worktree dir for a lock, which won't exist there.
	if worktreeDir != "" && state.PID == 0 && state.Status != "completed" && state.Status != "failed" {
		if lock.IsLocked(projectDir) {
			state.PID = lock.ReadPID(projectDir)
			state.Active = true
			if state.Status == "idle" || state.Status == "stopped" || state.Status == "unknown" {
				state.Status = "running"
			}
		}
	}

	// Check original dir for early-phase signals (triage/prepare happen before worktree exists)
	if state.Status == "idle" {
		phase := readPhase(projectDir, buildDir)
		if phase != "" && phase != "complete" && phase != "failed" {
			state.Phase = phase
			if lock.IsLocked(projectDir) {
				state.Active = true
				state.PID = lock.ReadPID(projectDir)
				switch {
				case phase == "triage":
					state.Status = "triaging"
				case strings.HasPrefix(phase, "prepare"):
					state.Status = "preparing"
				case strings.HasPrefix(phase, "sprint"):
					state.Status = "running"
				default:
					state.Status = "running"
				}
			} else {
				state.Status = "stopped"
				state.Active = false
			}
		}
	}

	if snap, teamErr := team.ActiveSnapshot(cmd.Context(), projectDir); teamErr == nil {
		state.Team = &agent.TeamSummary{
			ID:             snap.Config.TeamID,
			Status:         string(snap.Config.Status),
			WorkerCount:    len(snap.Workers),
			IdleWorkers:    snap.IdleWorkers,
			RunningWorkers: snap.RunningWorkers,
			StalledWorkers: snap.StalledWorkers,
			PendingTasks:   snap.Pending,
			ActiveTasks:    snap.InProgress,
			CompletedTasks: snap.Completed,
			FailedTasks:    snap.Failed,
			IntegratedDir:  snap.IntegratedDir,
		}
		if !state.Active && snap.Active {
			state.Active = true
		}
	}

	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(state)
}

func runStatusHumanReadable(cmd *cobra.Command, projectDir string) error {
	buildDir, _ := resolveBuildDir(projectDir)
	teamSnap, _ := team.ActiveSnapshot(cmd.Context(), projectDir)

	// Check for early-phase build (triage/prepare) before requiring epic
	phase := readPhase(projectDir, buildDir)
	if phase != "" && phase != "complete" && phase != "failed" {
		pid := lock.ReadPID(projectDir)
		alive := pid > 0 && lock.IsLocked(projectDir)
		if alive && (phase == "triage" || strings.HasPrefix(phase, "prepare")) {
			phaseLabel := phase
			if strings.HasPrefix(phase, "prepare") {
				phaseLabel = "prepare"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Build in progress: %s (PID %d)\n", phaseLabel, pid)
			return nil
		}
		// If phase says sprint:worktree but worktree is gone, fall through
		// to the normal flow which will show archives/worktrees.
	}

	// Check if fry has ever been used in this project.
	fryDir := filepath.Join(buildDir, config.FryDir)
	fryDirExists := false
	if _, statErr := os.Stat(fryDir); statErr == nil {
		fryDirExists = true
	}
	if !fryDirExists {
		// No .fry/ — check for archives or worktrees before declaring no project.
		hasArchives := fileExists(filepath.Join(projectDir, config.ArchiveDir))
		hasWorktrees := fileExists(filepath.Join(projectDir, config.GitWorktreeDir))
		if !hasArchives && !hasWorktrees {
			if teamSnap == nil {
				fmt.Fprintf(cmd.OutOrStdout(), "No fry project found in %s\n", projectDir)
				fmt.Fprintln(cmd.OutOrStdout(), "Run 'fry init' to get started.")
				return nil
			}
		}
	}

	epicPath := filepath.Join(fryDir, "epic.md")
	ep, err := epic.ParseEpic(epicPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// Check if process is running but no epic yet (early phase, died mid-triage)
			if phase != "" && phase != "complete" && phase != "failed" {
				fmt.Fprintf(cmd.OutOrStdout(), "Build phase: %s (process not running)\n", phase)
				return nil
			}

			archives, _ := archive.ScanArchives(projectDir)
			worktrees, _ := git.ScanWorktreeBuilds(projectDir)
			summary := continuerun.FormatInactiveSummary(projectDir, archives, worktrees)
			fmt.Fprint(cmd.OutOrStdout(), summary)
			if teamSnap != nil {
				fmt.Fprintln(cmd.OutOrStdout())
				fmt.Fprint(cmd.OutOrStdout(), formatTeamSummary(teamSnap))
			}
			return nil
		}
		return err
	}

	state, err := continuerun.CollectBuildState(cmd.Context(), buildDir, ep, false)
	if err != nil {
		return err
	}
	report := continuerun.FormatReport(state)
	fmt.Fprint(cmd.OutOrStdout(), report)
	if teamSnap != nil {
		fmt.Fprintln(cmd.OutOrStdout())
		fmt.Fprint(cmd.OutOrStdout(), formatTeamSummary(teamSnap))
	}
	return nil
}

func formatTeamSummary(snap *team.Snapshot) string {
	if snap == nil {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Active Team Runtime: %s (%s)\n", snap.Config.TeamID, snap.Config.Status)
	fmt.Fprintf(&b, "  Workers: idle=%d running=%d stalled=%d total=%d\n",
		snap.IdleWorkers, snap.RunningWorkers, snap.StalledWorkers, len(snap.Workers))
	fmt.Fprintf(&b, "  Tasks: pending=%d in-progress=%d completed=%d failed=%d blocked=%d\n",
		snap.Pending, snap.InProgress, snap.Completed, snap.Failed, snap.Blocked)
	if snap.IntegratedDir != "" {
		fmt.Fprintf(&b, "  Integrated output: %s\n", snap.IntegratedDir)
	}
	return b.String()
}
