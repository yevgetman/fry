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

	// These flags are intentionally registered on both rootCmd and runCmd.
	// rootCmd delegates to runCmd when invoked without a subcommand (e.g.,
	// "fry --engine claude"), so rootCmd needs its own flag definitions.
	// Both bind to the same variables so they stay in sync.
	rootCmd.Flags().StringVar(&runEngine, "engine", "", "Execution engine")
	rootCmd.Flags().BoolVar(&runDryRun, "dry-run", false, "Preview actions without executing")
	rootCmd.Flags().StringVar(&runUserPrompt, "user-prompt", "", "Additional user prompt")
	rootCmd.Flags().StringVar(&runUserPromptFile, "user-prompt-file", "", "Path to file containing user prompt")
	rootCmd.Flags().BoolVar(&runNoReview, "no-review", false, "Disable sprint review")
	rootCmd.Flags().StringVar(&runSimulateReview, "simulate-review", "", "Simulate review verdict")
	rootCmd.Flags().StringVar(&runPrepareEngine, "prepare-engine", "", "Engine for auto-prepare")
	rootCmd.Flags().BoolVar(&runPlanning, "planning", false, "Use planning mode (alias for --mode planning)")
	rootCmd.Flags().StringVar(&runMode, "mode", "", "Execution mode: software, planning, writing")
	rootCmd.Flags().StringVar(&runEffort, "effort", "", "Effort level: low, medium, high, max (default: auto)")
	rootCmd.Flags().BoolVar(&runNoAudit, "no-audit", false, "Disable sprint and build audits")
	rootCmd.Flags().BoolVar(&runResume, "resume", false, "Resume failed sprint: skip iterations, go straight to verification + healing with boosted attempts")
	rootCmd.Flags().IntVar(&runSprint, "sprint", 0, "Start from sprint N (alternative to positional sprint argument)")
	rootCmd.Flags().BoolVar(&runContinue, "continue", false, "Use an LLM agent to analyze build state and resume from where it left off")
	rootCmd.Flags().BoolVar(&runNoSanityCheck, "no-sanity-check", false, "Skip the interactive project summary during auto-prepare")
	rootCmd.Flags().BoolVar(&runFullPrepare, "full-prepare", false, "Skip triage and run full prepare pipeline when no epic exists")

	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(prepareCmd)
	rootCmd.AddCommand(replanCmd)
	rootCmd.AddCommand(cleanCmd)
	rootCmd.AddCommand(versionCmd)
}

func Execute() error {
	return rootCmd.Execute()
}
