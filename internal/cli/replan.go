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
)

var replanCmd = &cobra.Command{
	Use:   "replan <deviation_spec>",
	Short: "Replan an epic after deviation",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		projectPath, err := resolveProjectDir(projectDir)
		if err != nil {
			return err
		}

		epicPath := replanEpic
		if epicPath == "" {
			epicPath = filepath.Join(config.FryDir, "epic.md")
		}

		var replanner engine.Engine
		if !replanDryRun {
			engineName, err := engine.ResolveEngine(replanEngine, "", "", config.DefaultEngine)
			if err != nil {
				return err
			}
			replanner, err = engine.NewEngine(engineName)
			if err != nil {
				return err
			}
		}

		if err := review.RunReplan(cmd.Context(), review.ReplanOpts{
			ProjectDir:        projectPath,
			EpicPath:          epicPath,
			DeviationSpecPath: args[0],
			CompletedSprint:   replanCompleted,
			MaxScope:          replanMaxScope,
			Engine:            replanner,
			Model:             replanModel,
			DryRun:            replanDryRun,
		}); err != nil {
			return err
		}

		if replanDryRun {
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
}
