package cli

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/yevgetman/fry/internal/archive"
	"github.com/yevgetman/fry/internal/lock"
)

var cleanForce bool

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Archive build artifacts from .fry/ to .fry-archive/",
	Long:  "Move .fry/ and root-level build outputs (build-audit.md, build-summary.md) into a timestamped folder under .fry-archive/.",
	RunE: func(cmd *cobra.Command, args []string) error {
		pDir, _ := cmd.Flags().GetString("project-dir")
		projectPath, err := resolveProjectDir(pDir)
		if err != nil {
			return err
		}

		if lock.IsLocked(projectPath) {
			fmt.Fprintln(cmd.ErrOrStderr(), "fry: warning: a build appears to be running (lock file active)")
		}

		force, _ := cmd.Flags().GetBool("force")
		if !force {
			fmt.Fprint(cmd.OutOrStdout(), "Archive .fry/ and build outputs? [y/N] ")
			reader := bufio.NewReader(cmd.InOrStdin())
			answer, _ := reader.ReadString('\n')
			if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(answer)), "y") {
				fmt.Fprintln(cmd.OutOrStdout(), "Aborted.")
				return nil
			}
		}

		archivePath, err := archive.Archive(projectPath)
		if err != nil {
			return fmt.Errorf("clean: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Archived to %s\n", archivePath)
		return nil
	},
}

func init() {
	cleanCmd.Flags().BoolVar(&cleanForce, "force", false, "Skip confirmation prompt")
}
