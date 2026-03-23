package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/yevgetman/fry/internal/config"
	"github.com/yevgetman/fry/internal/continuerun"
	"github.com/yevgetman/fry/internal/epic"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current build state without making an LLM call",
	RunE: func(cmd *cobra.Command, args []string) error {
		projectDir, _ := cmd.Flags().GetString("project-dir")
		epicPath := filepath.Join(projectDir, config.FryDir, "epic.md")
		ep, err := epic.ParseEpic(epicPath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				fmt.Fprintf(cmd.OutOrStdout(), "No active build found in %s\n", projectDir)
				fmt.Fprintf(cmd.OutOrStdout(), "Run 'fry run' to start a build.\n")
				return nil
			}
			return err
		}
		state, err := continuerun.CollectBuildState(cmd.Context(), projectDir, ep)
		if err != nil {
			if errors.Is(err, continuerun.ErrNoPreviousBuild) {
				fmt.Fprintf(cmd.OutOrStdout(), "No active build found in %s\n", projectDir)
				fmt.Fprintf(cmd.OutOrStdout(), "Run 'fry run' to start a build.\n")
				return nil
			}
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
