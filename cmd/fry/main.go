package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/yevgetman/fry/internal/mission"
	"github.com/yevgetman/fry/internal/state"
)

var version = "dev"

func main() {
	if err := rootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

func rootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "fry",
		Short: "Structured builder orchestration with an LLM-first UI layer",
		SilenceUsage: true,
	}

	root.AddCommand(
		versionCmd(),
		newCmd(),
		listCmd(),
		statusCmd(),
	)

	return root
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print fry version",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println(version)
			return nil
		},
	}
}

func newCmd() *cobra.Command {
	var (
		promptFile string
		planFile   string
		specDir    string
		effort     string
		interval   string
		duration   string
		overtime   string
		baseDir    string
	)

	cmd := &cobra.Command{
		Use:   "new <name>",
		Short: "Scaffold a new mission directory",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			iv, err := time.ParseDuration(interval)
			if err != nil {
				return fmt.Errorf("invalid --interval %q: %w", interval, err)
			}
			dur, err := parseDurationHours(duration)
			if err != nil {
				return fmt.Errorf("invalid --duration %q: %w", duration, err)
			}
			ov, err := parseDurationHours(overtime)
			if err != nil {
				return fmt.Errorf("invalid --overtime %q: %w", overtime, err)
			}

			opts := mission.NewOptions{
				Name:       name,
				BaseDir:    baseDir,
				PromptFile: promptFile,
				PlanFile:   planFile,
				SpecDir:    specDir,
				Effort:     effort,
				Interval:   iv,
				Duration:   dur,
				Overtime:   ov,
			}

			dir, err := mission.Scaffold(opts)
			if err != nil {
				return err
			}
			fmt.Printf("Mission %q created at %s\n", name, dir)
			fmt.Printf("Run `fry start %s` to install the scheduler.\n", name)
			return nil
		},
	}

	cmd.Flags().StringVar(&promptFile, "prompt", "", "Prompt file path")
	cmd.Flags().StringVar(&planFile, "plan", "", "Plan file path")
	cmd.Flags().StringVar(&specDir, "spec-dir", "", "Spec directory path")
	cmd.Flags().StringVar(&effort, "effort", "standard", "Effort level: fast, standard, max")
	cmd.Flags().StringVar(&interval, "interval", "10m", "Wake interval (Go duration: 2m, 10m, 1h)")
	cmd.Flags().StringVar(&duration, "duration", "12h", "Mission duration (hours, e.g. 12h)")
	cmd.Flags().StringVar(&overtime, "overtime", "0h", "Overtime window after duration (hours)")
	cmd.Flags().StringVar(&baseDir, "base-dir", "", "Base directory for missions (default: ~/missions/)")
	return cmd
}

func listCmd() *cobra.Command {
	var baseDir string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all missions",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := baseDir
			if dir == "" {
				home, err := os.UserHomeDir()
				if err != nil {
					return err
				}
				dir = filepath.Join(home, "missions")
			}

			entries, err := os.ReadDir(dir)
			if os.IsNotExist(err) {
				fmt.Println("No missions found (missions directory does not exist yet).")
				return nil
			}
			if err != nil {
				return fmt.Errorf("read missions dir: %w", err)
			}

			found := false
			for _, e := range entries {
				if !e.IsDir() {
					continue
				}
				missionDir := filepath.Join(dir, e.Name())
				m, err := state.Load(missionDir)
				if err != nil {
					fmt.Printf("  %-20s  (error reading state: %v)\n", e.Name(), err)
					continue
				}
				found = true
				fmt.Printf("  %-20s  status=%-10s  wake=%d  elapsed=%.1fh\n",
					m.MissionID, m.Status, m.CurrentWake,
					m.ElapsedHours(time.Now().UTC()))
			}
			if !found {
				fmt.Println("No missions found.")
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&baseDir, "base-dir", "", "Base directory for missions (default: ~/missions/)")
	return cmd
}

func statusCmd() *cobra.Command {
	var baseDir string

	cmd := &cobra.Command{
		Use:   "status <name>",
		Short: "Print mission snapshot",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			dir := baseDir
			if dir == "" {
				home, err := os.UserHomeDir()
				if err != nil {
					return err
				}
				dir = filepath.Join(home, "missions")
			}
			missionDir := filepath.Join(dir, name)

			m, err := state.Load(missionDir)
			if err != nil {
				return fmt.Errorf("cannot load mission %q: %w", name, err)
			}

			now := time.Now().UTC()
			elapsed := m.ElapsedHours(now)

			fmt.Printf("Mission:      %s\n", m.MissionID)
			fmt.Printf("Status:       %s\n", m.Status)
			fmt.Printf("Wake:         %d\n", m.CurrentWake)
			fmt.Printf("Effort:       %s\n", m.Effort)
			fmt.Printf("Elapsed:      %.2fh\n", elapsed)
			fmt.Printf("Soft deadline:%s\n", m.SoftDeadline().Format(time.RFC3339))
			fmt.Printf("Hard deadline:%s\n", m.HardDeadlineUTC.Format(time.RFC3339))
			fmt.Printf("Input mode:   %s\n", m.InputMode)
			fmt.Printf("Interval:     %ds\n", m.IntervalSeconds)

			if m.CurrentWake == 0 {
				fmt.Println("\nNot yet started. Run `fry start " + name + "` to install the scheduler.")
			}

			// Show recent wake log
			wakeLog := filepath.Join(missionDir, "wake_log.jsonl")
			entries := tailJSON(wakeLog, 3)
			if len(entries) > 0 {
				fmt.Printf("\nLast %d wake(s):\n", len(entries))
				for _, e := range entries {
					wn, _ := e["wake_number"]
					goal, _ := e["wake_goal"]
					fmt.Printf("  wake %v: %v\n", wn, goal)
				}
			}

			// Artifact count
			artifactsDir := filepath.Join(missionDir, "artifacts")
			aEntries, err := os.ReadDir(artifactsDir)
			if err == nil {
				fmt.Printf("\nArtifacts: %d file(s)\n", len(aEntries))
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&baseDir, "base-dir", "", "Base directory for missions (default: ~/missions/)")
	return cmd
}

// parseDurationHours parses a duration string that may be hours like "12h" or minutes.
func parseDurationHours(s string) (time.Duration, error) {
	if s == "" || s == "0" || s == "0h" {
		return 0, nil
	}
	return time.ParseDuration(s)
}

// tailJSON reads the last n JSON objects from a .jsonl file.
func tailJSON(path string, n int) []map[string]interface{} {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	lines := splitLines(string(data))
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	var result []map[string]interface{}
	for _, line := range lines {
		if line == "" {
			continue
		}
		var m map[string]interface{}
		if json.Unmarshal([]byte(line), &m) == nil {
			result = append(result, m)
		}
	}
	return result
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
