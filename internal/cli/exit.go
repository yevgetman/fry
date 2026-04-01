package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/yevgetman/fry/internal/agent"
	"github.com/yevgetman/fry/internal/config"
	"github.com/yevgetman/fry/internal/lock"
	"github.com/yevgetman/fry/internal/steering"
)

var exitCmd = &cobra.Command{
	Use:   "exit",
	Short: "Request a graceful stop at the next safe build checkpoint",
	Long: `Request that a running Fry build stop at the next safe checkpoint.

Fry settles the current unit of work, writes a structured resume point, and
leaves the build in a paused state so 'fry run --continue' or
'fry run --resume --sprint N' can pick it up systematically.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		projectArg, _ := cmd.Flags().GetString("project-dir")
		projectPath, err := resolveProjectDir(projectArg)
		if err != nil {
			return err
		}

		buildDir, worktreeDir := resolveBuildDir(projectPath)
		state, err := agent.ReadBuildState(buildDir)
		if err != nil {
			return fmt.Errorf("exit: read build state: %w", err)
		}

		pid := activeBuildPID(projectPath, buildDir)
		if pid == 0 {
			if point, pointErr := steering.ReadResumePoint(buildDir); pointErr == nil && point != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "Build is already paused at %s.\n", point.Phase)
				if point.RecommendedCommand != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "Continue: %s\n", point.RecommendedCommand)
				} else {
					fmt.Fprintln(cmd.OutOrStdout(), "Continue: fry run --continue")
				}
				return nil
			}
			return fmt.Errorf("exit: no running build found in %s", projectPath)
		}

		created, err := steering.RequestExit(buildDir)
		if err != nil {
			return fmt.Errorf("exit: request graceful stop: %w", err)
		}

		phase := state.Phase
		if phase == "" {
			phase = "sprint"
		}

		if created {
			fmt.Fprintf(cmd.OutOrStdout(), "Graceful exit requested for PID %d.\n", pid)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "Graceful exit was already requested for PID %d.\n", pid)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Current phase: %s\n", phase)
		if state.CurrentSprint > 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "Current sprint: %d (%s)\n", state.CurrentSprint, state.CurrentSprintName)
		}
		if worktreeDir != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "Build directory: %s\n", worktreeDir)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Fry will stop at the next safe checkpoint and write %s.\n", config.ResumePointFile)
		fmt.Fprintln(cmd.OutOrStdout(), "Resume with: fry run --continue")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(exitCmd)
}

func activeBuildPID(projectPath, buildDir string) int {
	if lock.IsLocked(projectPath) {
		return lock.ReadPID(projectPath)
	}
	if buildDir != "" && buildDir != projectPath && lock.IsLocked(buildDir) {
		return lock.ReadPID(buildDir)
	}
	return 0
}
