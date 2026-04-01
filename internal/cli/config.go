package cli

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/yevgetman/fry/internal/config"
	"github.com/yevgetman/fry/internal/settings"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Read or write repo-local Fry settings",
}

var configGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Read a repo-local Fry setting",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		projectArg, _ := cmd.Flags().GetString("project-dir")
		projectPath, err := resolveProjectDir(projectArg)
		if err != nil {
			return err
		}

		switch args[0] {
		case "engine":
			engineName, err := settings.GetEngine(projectPath)
			if err != nil {
				return fmt.Errorf("config get engine: %w", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), engineName)
			return nil
		default:
			return fmt.Errorf("unknown config key %q", args[0])
		}
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Write a repo-local Fry setting",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		projectArg, _ := cmd.Flags().GetString("project-dir")
		projectPath, err := resolveProjectDir(projectArg)
		if err != nil {
			return err
		}

		switch args[0] {
		case "engine":
			if err := settings.SetEngine(projectPath, args[1]); err != nil {
				return fmt.Errorf("config set engine: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Set engine=%s in %s\n", args[1], filepath.Join(projectPath, config.ProjectConfigFile))
			return nil
		default:
			return fmt.Errorf("unknown config key %q", args[0])
		}
	},
}

func init() {
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configSetCmd)
}
