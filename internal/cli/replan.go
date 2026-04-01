package cli

import (
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/yevgetman/fry/internal/config"
	"github.com/yevgetman/fry/internal/engine"
	"github.com/yevgetman/fry/internal/review"
)

var (
	replanEpic      string
	replanCompleted int
	replanMaxScope  int
	replanEngine    string
	replanModel     string
	replanDryRun    bool
	replanMCPConfig string
)

var replanCmd = &cobra.Command{
	Use:   "replan <deviation_spec>",
	Short: "Replan an epic after deviation",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		pDir, _ := cmd.Flags().GetString("project-dir")
		projectPath, err := resolveProjectDir(pDir)
		if err != nil {
			return err
		}

		epicPath, _ := cmd.Flags().GetString("epic")
		if epicPath == "" {
			epicPath = filepath.Join(config.FryDir, "epic.md")
		}

		dryRun, _ := cmd.Flags().GetBool("dry-run")
		var replanner engine.Engine
		if !dryRun {
			engineVal, _ := cmd.Flags().GetString("engine")
			engineName, err := engine.ResolveEngine(engineVal, "", "", config.DefaultEngine)
			if err != nil {
				return err
			}
			planner := newEnginePlanner(engineName)
			var mcpOpts []engine.EngineOpt
			if replanMCPConfig != "" {
				mcpPath := replanMCPConfig
				if abs, err := filepath.Abs(replanMCPConfig); err == nil {
					mcpPath = abs
				}
				mcpOpts = append(mcpOpts, engine.WithMCPConfig(mcpPath))
			}
			replanner, err = planner.Build(engineName, mcpOpts...)
			if err != nil {
				return err
			}
		}

		completedSprint, _ := cmd.Flags().GetInt("completed")
		maxScope, _ := cmd.Flags().GetInt("max-scope")
		model, _ := cmd.Flags().GetString("model")

		if err := review.RunReplan(cmd.Context(), review.ReplanOpts{
			ProjectDir:        projectPath,
			EpicPath:          epicPath,
			DeviationSpecPath: args[0],
			CompletedSprint:   completedSprint,
			MaxScope:          maxScope,
			Engine:            replanner,
			Model:             model,
			EffortLevel:       "",
			DryRun:            dryRun,
		}); err != nil {
			return err
		}

		if dryRun {
			data, err := os.ReadFile(filepath.Join(projectPath, config.FryDir, "replan-prompt.md"))
			if err != nil {
				return err
			}
			_, err = cmd.OutOrStdout().Write(data)
			return err
		}
		return nil
	},
}

func init() {
	replanCmd.Flags().StringVar(&replanEpic, "epic", filepath.Join(config.FryDir, "epic.md"), "Epic file to update")
	replanCmd.Flags().IntVar(&replanCompleted, "completed", 0, "Completed sprint count")
	replanCmd.Flags().IntVar(&replanMaxScope, "max-scope", config.DefaultMaxDeviationScope, "Maximum deviation scope")
	replanCmd.Flags().StringVar(&replanEngine, "engine", "", "Replanning engine")
	replanCmd.Flags().StringVar(&replanModel, "model", "", "Model override")
	replanCmd.Flags().BoolVar(&replanDryRun, "dry-run", false, "Preview replanning changes")
	replanCmd.Flags().StringVar(&replanMCPConfig, "mcp-config", "", "Path to MCP server configuration file (Claude engine only)")
}
