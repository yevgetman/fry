package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/yevgetman/fry/internal/agent"
)

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Agent foundation commands for Fry's conversational interface",
}

var agentPromptCmd = &cobra.Command{
	Use:   "prompt",
	Short: "Print the agent system prompt (artifact schema, lifecycle, identity)",
	RunE: func(cmd *cobra.Command, args []string) error {
		prompt := agent.BuildAgentSystemPrompt()
		fmt.Fprint(cmd.OutOrStdout(), prompt)
		return nil
	},
}

// eventsCmd is registered at the root level (not under agentCmd) for ergonomics:
// `fry events --follow` is more natural than `fry agent events --follow`.
// The build-watcher service depends on this being `fry events`.
var eventsCmd = &cobra.Command{
	Use:   "events",
	Short: "Stream or list build events from the observer event log",
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, _ := cmd.Flags().GetString("project-dir")
		follow, _ := cmd.Flags().GetBool("follow")
		jsonFmt, _ := cmd.Flags().GetBool("json")

		if follow {
			return streamEvents(cmd, dir, jsonFmt)
		}
		return listEvents(cmd, dir, jsonFmt)
	},
}

func streamEvents(cmd *cobra.Command, projectDir string, jsonFmt bool) error {
	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel()

	ch, err := agent.TailEvents(ctx, projectDir)
	if err != nil {
		return fmt.Errorf("tail events: %w", err)
	}

	enc := json.NewEncoder(cmd.OutOrStdout())
	for evt := range ch {
		if jsonFmt {
			if err := enc.Encode(evt); err != nil {
				return err
			}
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "[%s] %s", evt.Timestamp.Format("15:04:05"), evt.Type)
			if evt.Sprint > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), " (sprint %d)", evt.Sprint)
			}
			for _, k := range sortedKeys(evt.Data) {
				fmt.Fprintf(cmd.OutOrStdout(), " %s=%s", k, evt.Data[k])
			}
			fmt.Fprintln(cmd.OutOrStdout())
		}
	}
	return nil
}

func listEvents(cmd *cobra.Command, projectDir string, jsonFmt bool) error {
	events, err := agent.ReadAllEvents(projectDir)
	if err != nil {
		return fmt.Errorf("read events: %w", err)
	}
	if len(events) == 0 {
		if !jsonFmt {
			fmt.Fprintln(cmd.OutOrStdout(), "No build events found.")
		}
		return nil
	}

	if jsonFmt {
		enc := json.NewEncoder(cmd.OutOrStdout())
		for _, evt := range events {
			if err := enc.Encode(evt); err != nil {
				return err
			}
		}
	} else {
		for _, evt := range events {
			fmt.Fprintf(cmd.OutOrStdout(), "[%s] %s", evt.Timestamp.Format("15:04:05"), evt.Type)
			if evt.Sprint > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), " (sprint %d)", evt.Sprint)
			}
			for _, k := range sortedKeys(evt.Data) {
				fmt.Fprintf(cmd.OutOrStdout(), " %s=%s", k, evt.Data[k])
			}
			fmt.Fprintln(cmd.OutOrStdout())
		}
	}
	return nil
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func init() {
	eventsCmd.Flags().Bool("follow", false, "Follow the event stream (tail -f style)")
	eventsCmd.Flags().Bool("json", false, "Output events as JSON lines")

	agentCmd.AddCommand(agentPromptCmd)
}
