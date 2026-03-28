package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/yevgetman/fry/internal/consciousness"
)

var identityCmd = &cobra.Command{
	Use:   "identity",
	Short: "Print Fry's current identity",
	RunE: func(cmd *cobra.Command, args []string) error {
		full, _ := cmd.Flags().GetBool("full")
		var content string
		var err error
		if full {
			content, err = consciousness.LoadFullIdentity()
		} else {
			content, err = consciousness.LoadCoreIdentity()
		}
		if err != nil {
			return fmt.Errorf("load identity: %w", err)
		}
		fmt.Fprint(cmd.OutOrStdout(), content)
		return nil
	},
}

func init() {
	identityCmd.Flags().Bool("full", false, "Print all identity layers including domains")
}
