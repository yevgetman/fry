package cli

import (
	"bufio"
	"fmt"
	"os"
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
		projectPath, err := resolveProjectDir(projectDir)
		if err != nil {
			return err
		}

		if lock.IsLocked(projectPath) {
			fmt.Fprintln(os.Stderr, "fry: warning: a build appears to be running (lock file active)")
		}

		if !cleanForce {
			fmt.Fprint(os.Stdout, "Archive .fry/ and build outputs? [y/N] ")
			reader := bufio.NewReader(os.Stdin)
			answer, _ := reader.ReadString('\n')
			if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(answer)), "y") {
				fmt.Fprintln(os.Stdout, "Aborted.")
				return nil
			}
		}

		archivePath, err := archive.Archive(projectPath)
		if err != nil {
			return fmt.Errorf("clean: %w", err)
		}

		fmt.Fprintf(os.Stdout, "Archived to %s\n", archivePath)
		return nil
	},
}

func init() {
	cleanCmd.Flags().BoolVar(&cleanForce, "force", false, "Skip confirmation prompt")
}
