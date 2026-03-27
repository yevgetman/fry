package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/yevgetman/fry/internal/epic"
	"github.com/yevgetman/fry/internal/prepare"
)

var (
	prepareEngine         string
	prepareUserPrompt     string
	prepareUserPromptFile string
	prepareValidateOnly   bool
	preparePlanning       bool
	prepareMode           string
	prepareEffort         string
	prepareNoProjectOverview  bool
	prepareReview         bool
)

var prepareCmd = &cobra.Command{
	Use:   "prepare [epic_filename]",
	Short: "Prepare a project for fry",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := checkArgsForMissingDashes(cmd, args); err != nil {
			return err
		}

		projectPath, err := resolveProjectDir(projectDir)
		if err != nil {
			return err
		}

		userPrompt, err := resolveUserPrompt(projectPath, prepareUserPrompt, prepareUserPromptFile, true)
		if err != nil {
			return err
		}

		epicArg := "epic.md"
		if len(args) > 0 {
			epicArg = args[0]
		}

		effortLevel, err := epic.ParseEffortLevel(prepareEffort)
		if err != nil {
			return err
		}

		mode, err := resolveMode(prepareMode, preparePlanning)
		if err != nil {
			return err
		}

		if prepareValidateOnly {
			epicPath, _, err := resolveEpicPath(projectPath, epicArg)
			if err != nil {
				return err
			}
			parsed, err := epic.ParseEpic(epicPath)
			if err != nil {
				return err
			}
			if err := epic.ValidateEpic(parsed); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Epic validation passed: %s\n", epicPath)
			return nil
		}

		promptSource := userPromptSource(userPrompt, prepareUserPrompt, prepareUserPromptFile)

		return prepare.RunPrepare(cmd.Context(), prepare.PrepareOpts{
			ProjectDir:       projectPath,
			EpicFilename:     epicArg,
			Engine:           prepareEngine,
			UserPrompt:       userPrompt,
			UserPromptSource: promptSource,
			ValidateOnly:     false,
			SkipProjectOverview:  prepareNoProjectOverview,
			Mode:             mode,
			EffortLevel:      effortLevel,
			EnableReview:     prepareReview,
			Stdin:            os.Stdin,
			Stdout:           cmd.OutOrStdout(),
		})
	},
}

func init() {
	prepareCmd.Flags().StringVar(&prepareEngine, "engine", "", "Preparation engine")
	prepareCmd.Flags().StringVar(&prepareUserPrompt, "user-prompt", "", "Additional user prompt")
	prepareCmd.Flags().StringVar(&prepareUserPromptFile, "user-prompt-file", "", "Path to file containing user prompt")
	prepareCmd.Flags().BoolVar(&prepareValidateOnly, "validate-only", false, "Validate without generating files")
	prepareCmd.Flags().BoolVar(&preparePlanning, "planning", false, "Use planning prepare mode (alias for --mode planning)")
	prepareCmd.Flags().StringVar(&prepareMode, "mode", "", "Execution mode: software, planning, writing")
	prepareCmd.Flags().StringVar(&prepareEffort, "effort", "", "Effort level: low, medium, high, max (default: auto)")
	prepareCmd.Flags().BoolVar(&prepareNoProjectOverview, "no-project-overview", false, "Skip the interactive project overview confirmation")
	prepareCmd.Flags().BoolVar(&prepareNoProjectOverview, "no-sanity-check", false, "Deprecated alias for --no-project-overview")
	_ = prepareCmd.Flags().MarkHidden("no-sanity-check")
	prepareCmd.Flags().BoolVar(&prepareReview, "review", false, "Enable sprint review between sprints")
}
