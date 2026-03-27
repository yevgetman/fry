package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/yevgetman/fry/internal/archive"
	"github.com/yevgetman/fry/internal/config"
	"github.com/yevgetman/fry/internal/continuerun"
	"github.com/yevgetman/fry/internal/epic"
	"github.com/yevgetman/fry/internal/git"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current build state without making an LLM call",
	RunE: func(cmd *cobra.Command, args []string) error {
		projectDir, _ := cmd.Flags().GetString("project-dir")

		// If a worktree strategy was used, resolve to the worktree directory
		// where the actual build artifacts live.
		buildDir := projectDir
		if setup, err := git.ReadPersistedStrategy(projectDir); err == nil && setup != nil && setup.WorkDir != "" {
			buildDir = setup.WorkDir
		}

		epicPath := filepath.Join(buildDir, config.FryDir, "epic.md")
		ep, err := epic.ParseEpic(epicPath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				if buildDir != projectDir {
					fmt.Fprintf(cmd.OutOrStdout(), "Build worktree not found at %s\n", buildDir)
					fmt.Fprintf(cmd.OutOrStdout(), "The worktree may have been removed. Run 'fry run --continue' to resume.\n")
				} else {
					archives, _ := archive.ScanArchives(projectDir)
					worktrees, _ := git.ScanWorktreeBuilds(projectDir)
					summary := continuerun.FormatInactiveSummary(projectDir, archives, worktrees)
					fmt.Fprint(cmd.OutOrStdout(), summary)
				}
				return nil
			}
			return err
		}
		state, err := continuerun.CollectBuildState(cmd.Context(), buildDir, ep)
		if err != nil {
			return err
		}
		report := continuerun.FormatReport(state)
		fmt.Fprint(cmd.OutOrStdout(), report)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
