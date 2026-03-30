package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/yevgetman/fry/internal/config"
	"github.com/yevgetman/fry/internal/lock"
)

var (
	destroyForce bool
	destroyYes   bool
)

var destroyCmd = &cobra.Command{
	Use:   "destroy",
	Short: "Remove all fry artifacts as if fry was never run",
	Long:  "Completely remove all fry-generated directories and files (.fry/, .fry-archive/, .fry-worktrees/, plans/, assets/, media/, and root-level build outputs). Unlike clean, which archives build artifacts and preserves the codebase index, destroy wipes everything.",
	RunE: func(cmd *cobra.Command, args []string) error {
		pDir, _ := cmd.Flags().GetString("project-dir")
		projectPath, err := resolveProjectDir(pDir)
		if err != nil {
			return err
		}

		if lock.IsLocked(projectPath) {
			fmt.Fprintln(cmd.ErrOrStderr(), "fry: warning: a build appears to be running (lock file active)")
		}

		// Collect what exists so we can show the user what will be removed.
		targets := destroyTargets(projectPath)
		if len(targets) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "Nothing to destroy — no fry artifacts found.")
			return nil
		}

		fmt.Fprintln(cmd.OutOrStdout(), "The following will be permanently deleted:")
		for _, t := range targets {
			rel, _ := filepath.Rel(projectPath, t)
			if rel == "" {
				rel = t
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", rel)
		}

		force, _ := cmd.Flags().GetBool("force")
		yes, _ := cmd.Flags().GetBool("yes")
		if force || yes {
			fmt.Fprintln(cmd.OutOrStdout(), "Proceed? [y/N] y (auto-accepted)")
		} else {
			fmt.Fprint(cmd.OutOrStdout(), "Proceed? [y/N] ")
			reader := bufio.NewReader(cmd.InOrStdin())
			answer, _ := reader.ReadString('\n')
			if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(answer)), "y") {
				fmt.Fprintln(cmd.OutOrStdout(), "Aborted.")
				return nil
			}
		}

		var removed int
		for _, t := range targets {
			if err := os.RemoveAll(t); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "fry: warning: failed to remove %s: %v\n", t, err)
				continue
			}
			removed++
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Destroyed %d fry artifact(s).\n", removed)
		return nil
	},
}

func init() {
	destroyCmd.Flags().BoolVar(&destroyForce, "force", false, "Skip confirmation prompt")
	destroyCmd.Flags().BoolVarP(&destroyYes, "yes", "y", false, "Auto-accept confirmation prompts")
}

// destroyTargets returns absolute paths of fry-generated directories and files
// that exist in the project directory.
func destroyTargets(projectDir string) []string {
	candidates := []string{
		filepath.Join(projectDir, config.FryDir),          // .fry/
		filepath.Join(projectDir, config.ArchiveDir),      // .fry-archive/
		filepath.Join(projectDir, config.GitWorktreeDir),  // .fry-worktrees/
		filepath.Join(projectDir, config.PlansDir),        // plans/
		filepath.Join(projectDir, config.AssetsDir),        // assets/
		filepath.Join(projectDir, config.MediaDir),         // media/
		filepath.Join(projectDir, config.BuildAuditFile),   // build-audit.md
		filepath.Join(projectDir, config.SummaryFile),      // build-summary.md
		filepath.Join(projectDir, config.BuildAuditSARIFFile), // build-audit.sarif
	}

	var existing []string
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			existing = append(existing, c)
		}
	}
	return existing
}
