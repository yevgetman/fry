package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/yevgetman/fry/internal/team"
)

var (
	teamIDFlag         string
	teamWorkers        int
	teamRoles          []string
	teamTaskFile       string
	teamJSON           bool
	teamGitIsolation   string
	teamExecutablePath string
	teamScaleAdd       int
	teamScaleRemove    string
	teamShutdownForce  bool
	teamWorkerID       string
)

var teamCmd = &cobra.Command{
	Use:   "team",
	Short: "Standalone tmux-backed team runtime",
}

var teamStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start a standalone team runtime",
	RunE: func(cmd *cobra.Command, args []string) error {
		projectDir, _ := cmd.Flags().GetString("project-dir")
		projectDir, err := filepath.Abs(projectDir)
		if err != nil {
			return err
		}
		mode := team.IsolationMode(teamGitIsolation)
		switch mode {
		case "", team.IsolationShared, team.IsolationPerWorkerWorktree:
		default:
			return fmt.Errorf("invalid --git-isolation %q", teamGitIsolation)
		}
		cfg, err := team.Start(cmd.Context(), team.StartOptions{
			ProjectDir:       projectDir,
			TeamID:           teamIDFlag,
			Workers:          teamWorkers,
			Roles:            teamRoles,
			GitIsolationMode: mode,
			TaskFile:         teamTaskFile,
			ExecutablePath:   teamExecutablePath,
		})
		if err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Started team %s (%s) with %d worker(s)\n", cfg.TeamID, cfg.Status, cfg.WorkerCount)
		fmt.Fprintf(cmd.OutOrStdout(), "tmux session: %s\n", cfg.TMuxSession)
		return nil
	},
}

var teamStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show team runtime status",
	RunE: func(cmd *cobra.Command, args []string) error {
		projectDir, _ := cmd.Flags().GetString("project-dir")
		projectDir, err := filepath.Abs(projectDir)
		if err != nil {
			return err
		}
		snap, err := team.ActiveSnapshot(cmd.Context(), projectDir)
		if teamIDFlag != "" {
			snap, err = team.SnapshotForTeam(cmd.Context(), projectDir, teamIDFlag)
		}
		if err != nil {
			if errors.Is(err, os.ErrNotExist) || errors.Is(err, team.ErrNoActiveTeam) {
				return fmt.Errorf("no team runtime found")
			}
			return err
		}
		if teamJSON {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(snap)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Team: %s\n", snap.Config.TeamID)
		fmt.Fprintf(cmd.OutOrStdout(), "Status: %s\n", snap.Config.Status)
		fmt.Fprintf(cmd.OutOrStdout(), "Session: %s\n", snap.Config.TMuxSession)
		fmt.Fprintf(cmd.OutOrStdout(), "Tasks: pending=%d in_progress=%d completed=%d failed=%d blocked=%d\n",
			snap.Pending, snap.InProgress, snap.Completed, snap.Failed, snap.Blocked)
		fmt.Fprintf(cmd.OutOrStdout(), "Workers: idle=%d running=%d stalled=%d\n",
			snap.IdleWorkers, snap.RunningWorkers, snap.StalledWorkers)
		if snap.IntegratedDir != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "Integrated output: %s\n", snap.IntegratedDir)
		}
		for _, worker := range snap.Workers {
			msg := ""
			if worker.Heartbeat != nil && worker.Heartbeat.Message != "" {
				msg = " - " + worker.Heartbeat.Message
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  %s [%s/%s]%s\n",
				worker.Identity.WorkerID,
				worker.Identity.Role,
				worker.Record.Status,
				msg,
			)
		}
		return nil
	},
}

var teamAssignCmd = &cobra.Command{
	Use:   "assign",
	Short: "Load tasks into the team runtime from a JSON file",
	RunE: func(cmd *cobra.Command, args []string) error {
		projectDir, _ := cmd.Flags().GetString("project-dir")
		projectDir, err := filepath.Abs(projectDir)
		if err != nil {
			return err
		}
		teamID, err := team.ResolveTeamID(projectDir, teamIDFlag)
		if err != nil {
			return err
		}
		if strings.TrimSpace(teamTaskFile) == "" {
			return fmt.Errorf("--task-file is required")
		}
		tasks, err := team.AssignTasksFromFile(projectDir, teamID, teamTaskFile)
		if err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Loaded %d task(s) into team %s\n", len(tasks), teamID)
		return nil
	},
}

var teamPauseCmd = &cobra.Command{
	Use:   "pause",
	Short: "Pause task claiming for the team",
	RunE: func(cmd *cobra.Command, args []string) error {
		projectDir, _ := cmd.Flags().GetString("project-dir")
		projectDir, _ = filepath.Abs(projectDir)
		teamID, err := team.ResolveTeamID(projectDir, teamIDFlag)
		if err != nil {
			return err
		}
		if err := team.Pause(projectDir, teamID); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Paused team %s\n", teamID)
		return nil
	},
}

var teamResumeCmd = &cobra.Command{
	Use:   "resume",
	Short: "Resume a paused team and restart dead workers",
	RunE: func(cmd *cobra.Command, args []string) error {
		projectDir, _ := cmd.Flags().GetString("project-dir")
		projectDir, _ = filepath.Abs(projectDir)
		teamID, err := team.ResolveTeamID(projectDir, teamIDFlag)
		if err != nil {
			return err
		}
		if err := team.Resume(cmd.Context(), projectDir, teamID, teamExecutablePath); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Resumed team %s\n", teamID)
		return nil
	},
}

var teamScaleCmd = &cobra.Command{
	Use:   "scale",
	Short: "Scale worker count up or down",
	RunE: func(cmd *cobra.Command, args []string) error {
		projectDir, _ := cmd.Flags().GetString("project-dir")
		projectDir, _ = filepath.Abs(projectDir)
		teamID, err := team.ResolveTeamID(projectDir, teamIDFlag)
		if err != nil {
			return err
		}
		if err := team.Scale(cmd.Context(), team.ScaleOptions{
			ProjectDir: projectDir,
			TeamID:     teamID,
			Add:        teamScaleAdd,
			Remove:     teamScaleRemove,
		}, teamExecutablePath); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Updated scale for team %s\n", teamID)
		return nil
	},
}

var teamShutdownCmd = &cobra.Command{
	Use:   "shutdown",
	Short: "Shut down the team runtime",
	RunE: func(cmd *cobra.Command, args []string) error {
		projectDir, _ := cmd.Flags().GetString("project-dir")
		projectDir, _ = filepath.Abs(projectDir)
		teamID, err := team.ResolveTeamID(projectDir, teamIDFlag)
		if err != nil {
			return err
		}
		if err := team.Shutdown(cmd.Context(), projectDir, teamID, teamShutdownForce); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Shutdown team %s\n", teamID)
		return nil
	},
}

var teamAttachCmd = &cobra.Command{
	Use:   "attach",
	Short: "Attach to the team's tmux session",
	RunE: func(cmd *cobra.Command, args []string) error {
		projectDir, _ := cmd.Flags().GetString("project-dir")
		projectDir, _ = filepath.Abs(projectDir)
		teamID, err := team.ResolveTeamID(projectDir, teamIDFlag)
		if err != nil {
			return err
		}
		return team.Attach(cmd.Context(), projectDir, teamID)
	},
}

var teamWorkerCmd = &cobra.Command{
	Use:    "worker",
	Short:  "Internal worker entrypoint used by tmux-hosted team workers",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		projectDir, _ := cmd.Flags().GetString("project-dir")
		projectDir, err := filepath.Abs(projectDir)
		if err != nil {
			return err
		}
		return team.RunWorker(cmd.Context(), team.WorkerRunOptions{
			ProjectDir: projectDir,
			TeamID:     teamIDFlag,
			WorkerID:   teamWorkerID,
		})
	},
}

func init() {
	teamStartCmd.Flags().StringVar(&teamIDFlag, "team", "", "Team identifier")
	teamStartCmd.Flags().IntVar(&teamWorkers, "workers", 1, "Number of workers")
	teamStartCmd.Flags().StringSliceVar(&teamRoles, "role", []string{"executor"}, "Worker roles (repeat or comma-separate)")
	teamStartCmd.Flags().StringVar(&teamTaskFile, "task-file", "", "Path to JSON task file")
	teamStartCmd.Flags().StringVar(&teamGitIsolation, "git-isolation", "", "Team git isolation mode: shared, per-worker-worktree")
	teamStartCmd.Flags().StringVar(&teamExecutablePath, "executable-path", "", "Override fry executable path for worker hosts")

	teamStatusCmd.Flags().StringVar(&teamIDFlag, "team", "", "Team identifier")
	teamStatusCmd.Flags().BoolVar(&teamJSON, "json", false, "Output JSON")

	teamAssignCmd.Flags().StringVar(&teamIDFlag, "team", "", "Team identifier")
	teamAssignCmd.Flags().StringVar(&teamTaskFile, "task-file", "", "Path to JSON task file")

	teamPauseCmd.Flags().StringVar(&teamIDFlag, "team", "", "Team identifier")
	teamResumeCmd.Flags().StringVar(&teamIDFlag, "team", "", "Team identifier")
	teamResumeCmd.Flags().StringVar(&teamExecutablePath, "executable-path", "", "Override fry executable path for worker hosts")

	teamScaleCmd.Flags().StringVar(&teamIDFlag, "team", "", "Team identifier")
	teamScaleCmd.Flags().IntVar(&teamScaleAdd, "add", 0, "Add workers")
	teamScaleCmd.Flags().StringVar(&teamScaleRemove, "remove", "", "Drain or remove a worker ID")
	teamScaleCmd.Flags().StringVar(&teamExecutablePath, "executable-path", "", "Override fry executable path for worker hosts")

	teamShutdownCmd.Flags().StringVar(&teamIDFlag, "team", "", "Team identifier")
	teamShutdownCmd.Flags().BoolVar(&teamShutdownForce, "force", false, "Forcefully kill the tmux session")

	teamAttachCmd.Flags().StringVar(&teamIDFlag, "team", "", "Team identifier")

	teamWorkerCmd.Flags().StringVar(&teamIDFlag, "team", "", "Team identifier")
	teamWorkerCmd.Flags().StringVar(&teamWorkerID, "worker", "", "Worker identifier")

	teamCmd.AddCommand(teamStartCmd)
	teamCmd.AddCommand(teamStatusCmd)
	teamCmd.AddCommand(teamAssignCmd)
	teamCmd.AddCommand(teamPauseCmd)
	teamCmd.AddCommand(teamResumeCmd)
	teamCmd.AddCommand(teamScaleCmd)
	teamCmd.AddCommand(teamShutdownCmd)
	teamCmd.AddCommand(teamAttachCmd)
	teamCmd.AddCommand(teamWorkerCmd)
}
