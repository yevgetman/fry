package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/yevgetman/fry/internal/engine"
	"github.com/yevgetman/fry/internal/epic"
	"github.com/yevgetman/fry/internal/prepare"
)

var (
	prepareEngine            string
	prepareUserPrompt        string
	prepareUserPromptFile    string
	prepareGitHubIssue       string
	prepareValidateOnly      bool
	preparePlanning          bool
	prepareMode              string
	prepareEffort            string
	prepareNoProjectOverview bool
	prepareReview            bool
	prepareYes               bool
	prepareMCPConfig         string
	prepareConfirmFile       bool
)

var prepareCmd = &cobra.Command{
	Use:   "prepare [epic_filename]",
	Short: "Prepare a project for fry",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := checkArgsForMissingDashes(cmd, args); err != nil {
			return err
		}

		pDir, _ := cmd.Flags().GetString("project-dir")
		projectPath, err := resolveProjectDir(pDir)
		if err != nil {
			return err
		}

		validateOnly, _ := cmd.Flags().GetBool("validate-only")
		epicArg := "epic.md"
		if len(args) > 0 {
			epicArg = args[0]
		}
		if validateOnly {
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

		userPromptVal, _ := cmd.Flags().GetString("user-prompt")
		userPromptFileVal, _ := cmd.Flags().GetString("user-prompt-file")
		userPrompt, promptSource, err := resolveTopLevelPrompt(cmd.Context(), projectPath, userPromptVal, userPromptFileVal, prepareGitHubIssue, true)
		if err != nil {
			return err
		}

		effortVal, _ := cmd.Flags().GetString("effort")
		effortLevel, err := epic.ParseEffortLevel(effortVal)
		if err != nil {
			return err
		}

		modeVal, _ := cmd.Flags().GetString("mode")
		planningVal, _ := cmd.Flags().GetBool("planning")
		mode, err := resolveMode(modeVal, planningVal)
		if err != nil {
			return err
		}

		engineVal, _ := cmd.Flags().GetString("engine")
		noProjectOverview, _ := cmd.Flags().GetBool("no-project-overview")
		noSanityCheck, _ := cmd.Flags().GetBool("no-sanity-check")
		reviewVal, _ := cmd.Flags().GetBool("review")

		var prepareEngineOpts []engine.EngineOpt
		if prepareMCPConfig != "" {
			mcpPath := prepareMCPConfig
			if abs, err := filepath.Abs(prepareMCPConfig); err == nil {
				mcpPath = abs
			}
			prepareEngineOpts = append(prepareEngineOpts, engine.WithMCPConfig(mcpPath))
		}
		planner := newEnginePlanner(engineVal)

		return prepare.RunPrepare(cmd.Context(), prepare.PrepareOpts{
			ProjectDir:          projectPath,
			EpicFilename:        epicArg,
			Engine:              engineVal,
			UserPrompt:          userPrompt,
			UserPromptSource:    promptSource,
			ValidateOnly:        false,
			SkipProjectOverview: noProjectOverview || noSanityCheck,
			AutoAccept:          prepareYes,
			ConfirmFile:         prepareConfirmFile,
			Mode:                mode,
			EffortLevel:         effortLevel,
			EnableReview:        reviewVal,
			Stdin:               os.Stdin,
			Stdout:              cmd.OutOrStdout(),
			EngineFactory: func(name string) (engine.Engine, error) {
				if len(prepareEngineOpts) == 0 {
					return planner.Build(name)
				}
				return planner.Build(name, prepareEngineOpts...)
			},
		})
	},
}

func init() {
	prepareCmd.Flags().StringVar(&prepareEngine, "engine", "", "Preparation engine")
	prepareCmd.Flags().StringVar(&prepareUserPrompt, "user-prompt", "", "Additional user prompt")
	prepareCmd.Flags().StringVar(&prepareUserPromptFile, "user-prompt-file", "", "Path to file containing user prompt")
	prepareCmd.Flags().StringVar(&prepareGitHubIssue, "gh-issue", "", "GitHub issue URL to use as the task definition (requires gh auth)")
	prepareCmd.Flags().BoolVar(&prepareValidateOnly, "validate-only", false, "Validate without generating files")
	prepareCmd.Flags().BoolVar(&preparePlanning, "planning", false, "Use planning prepare mode (alias for --mode planning)")
	prepareCmd.Flags().StringVar(&prepareMode, "mode", "", "Execution mode: software, planning, writing")
	prepareCmd.Flags().StringVar(&prepareEffort, "effort", "", "Effort level: fast, standard, high, max (default: auto)")
	prepareCmd.Flags().BoolVar(&prepareNoProjectOverview, "no-project-overview", false, "Skip the interactive project overview confirmation")
	prepareCmd.Flags().BoolVar(&prepareNoProjectOverview, "no-sanity-check", false, "Deprecated alias for --no-project-overview")
	_ = prepareCmd.Flags().MarkHidden("no-sanity-check")
	prepareCmd.Flags().BoolVar(&prepareReview, "review", false, "Enable sprint review between sprints")
	prepareCmd.Flags().StringVar(&prepareMCPConfig, "mcp-config", "", "Path to MCP server configuration file (Claude engine only)")
	prepareCmd.Flags().BoolVarP(&prepareYes, "yes", "y", false, "Auto-accept all interactive confirmation prompts")
	prepareCmd.Flags().BoolVar(&prepareConfirmFile, "confirm-file", false, "Use file-based interactive prompts (.fry/confirm-prompt.json) instead of stdin")
}
