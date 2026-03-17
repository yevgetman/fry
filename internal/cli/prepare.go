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
	prepareNoSanityCheck  bool
)

var prepareCmd = &cobra.Command{
	Use:   "prepare [epic_filename]",
	Short: "Prepare a project for fry",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
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

		return prepare.RunPrepare(cmd.Context(), prepare.PrepareOpts{
			ProjectDir:      projectPath,
			EpicFilename:    epicArg,
			Engine:          prepareEngine,
			UserPrompt:      userPrompt,
			ValidateOnly:    false,
			SkipSanityCheck: prepareNoSanityCheck,
			Mode:            mode,
			EffortLevel:     effortLevel,
			Stdin:           os.Stdin,
			Stdout:          cmd.OutOrStdout(),
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
	prepareCmd.Flags().BoolVar(&prepareNoSanityCheck, "no-sanity-check", false, "Skip the interactive project summary confirmation")
}
