package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/yevgetman/fry/internal/config"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print fry version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Fprintf(cmd.OutOrStdout(), "fry version %s\n", config.Version)
	},
}
