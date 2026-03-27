package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/yevgetman/fry/internal/config"
	"github.com/yevgetman/fry/internal/consciousness"
)

var reflectCmd = &cobra.Command{
	Use:   "reflect",
	Short: "Trigger identity reflection from accumulated memories",
	Long: `Trigger the Reflection pipeline on the consciousness API server.
Reflection reads all memories, computes effective weights, synthesizes an
updated identity.json via Claude, prunes decayed memories, and commits
the result to the GitHub repository.

Requires at least 50 memories in the store before reflection can run.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()

		fmt.Println("Triggering reflection...")

		result, err := consciousness.TriggerReflection(ctx, config.ConsciousnessAPIURL, config.ConsciousnessWriteKey)
		if err != nil {
			return fmt.Errorf("reflect: %w", err)
		}

		if result.Error != "" {
			fmt.Printf("Reflection skipped: %s\n", result.Error)
			return nil
		}

		fmt.Printf("Reflection complete:\n")
		fmt.Printf("  Memories considered: %d\n", result.MemoriesConsidered)
		fmt.Printf("  Memories integrated: %d\n", result.MemoriesIntegrated)
		fmt.Printf("  Memories pruned:     %d\n", result.MemoriesPruned)
		fmt.Printf("  Identity version:    %d\n", result.IdentityVersion)
		if result.CommitSHA != "" {
			fmt.Printf("  Commit:              %s\n", result.CommitSHA)
		}
		if len(result.Changes) > 0 {
			fmt.Println("  Changes:")
			for _, c := range result.Changes {
				fmt.Printf("    - %s\n", c)
			}
		}
		return nil
	},
}
