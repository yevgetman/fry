package cli

import (
	"github.com/spf13/cobra"
	frlog "github.com/yevgetman/fry/internal/log"
)

var (
	projectDir string

	rootCmd = &cobra.Command{
		Use:   "fry",
		Short: "Automated AI build orchestration engine",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCmd.RunE(cmd, args)
		},
	}
)

func init() {
	rootCmd.PersistentFlags().StringVar(&projectDir, "project-dir", ".", "Project directory")
	rootCmd.PersistentFlags().BoolVar(&frlog.Verbose, "verbose", false, "Enable verbose logging")
	rootCmd.Flags().StringVar(&runEngine, "engine", "", "Execution engine")
	rootCmd.Flags().BoolVar(&runDryRun, "dry-run", false, "Preview actions without executing")
	rootCmd.Flags().StringVar(&runUserPrompt, "user-prompt", "", "Additional user prompt")
	rootCmd.Flags().StringVar(&runUserPromptFile, "user-prompt-file", "", "Path to file containing user prompt")
	rootCmd.Flags().BoolVar(&runNoReview, "no-review", false, "Disable sprint review")
	rootCmd.Flags().StringVar(&runSimulateReview, "simulate-review", "", "Simulate review verdict")
	rootCmd.Flags().StringVar(&runPrepareEngine, "prepare-engine", "", "Engine for auto-prepare")
	rootCmd.Flags().BoolVar(&runPlanning, "planning", false, "Use planning mode")
	rootCmd.Flags().StringVar(&runEffort, "effort", "", "Effort level: low, medium, high, max (default: auto)")
	rootCmd.Flags().BoolVar(&runNoAudit, "no-audit", false, "Disable sprint and build audits")

	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(prepareCmd)
	rootCmd.AddCommand(replanCmd)
	rootCmd.AddCommand(versionCmd)
}

func Execute() error {
	return rootCmd.Execute()
}
