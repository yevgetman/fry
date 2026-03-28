package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/yevgetman/fry/internal/agent"
	"github.com/yevgetman/fry/internal/archive"
	"github.com/yevgetman/fry/internal/config"
	"github.com/yevgetman/fry/internal/consciousness"
	"github.com/yevgetman/fry/internal/continuerun"
	"github.com/yevgetman/fry/internal/epic"
	"github.com/yevgetman/fry/internal/git"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current build state without making an LLM call",
	RunE: func(cmd *cobra.Command, args []string) error {
		jsonOutput, _ := cmd.Flags().GetBool("json")
		if jsonOutput {
			projectDir, _ := cmd.Flags().GetString("project-dir")
			state, err := agent.ReadBuildState(projectDir)
			if err != nil {
				return fmt.Errorf("read build state: %w", err)
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(state)
		}

		showConsciousness, _ := cmd.Flags().GetBool("consciousness")
		if showConsciousness {
			ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
			defer cancel()

			stats, err := consciousness.FetchPipelineStats(ctx, config.ConsciousnessAPIURL)
			if err != nil {
				return fmt.Errorf("consciousness status: %w", err)
			}
			fmt.Fprint(cmd.OutOrStdout(), consciousness.FormatPipelineStats(stats))
			return nil
		}

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
		// alwaysVerify is hardcoded to false because it is a runtime-only flag with
		// no persistent state. For low-effort builds originally run with --always-verify,
		// the sentinel status may show "N/A" even though the audit ran. This is a known
		// limitation; persisting the flag would require a .fry/build-flags.txt artifact.
		state, err := continuerun.CollectBuildState(cmd.Context(), buildDir, ep, false)
		if err != nil {
			return err
		}
		report := continuerun.FormatReport(state)
		fmt.Fprint(cmd.OutOrStdout(), report)
		return nil
	},
}

func init() {
	statusCmd.Flags().Bool("consciousness", false, "Show consciousness pipeline status")
	statusCmd.Flags().Bool("json", false, "Output build state as JSON (for agent consumption)")
	rootCmd.AddCommand(statusCmd)
}
