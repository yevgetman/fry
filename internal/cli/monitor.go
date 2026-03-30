package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/yevgetman/fry/internal/color"
	"github.com/yevgetman/fry/internal/config"
	"github.com/yevgetman/fry/internal/git"
	"github.com/yevgetman/fry/internal/monitor"
)

var (
	monitorDashboard bool
	monitorLogs      bool
	monitorJSON      bool
	monitorNoWait    bool
	monitorInterval  string
)

var monitorCmd = &cobra.Command{
	Use:   "monitor [project-dir]",
	Short: "Real-time build monitoring with enriched event stream",
	Long: `Monitor an active build in real time. Composes data from events,
build status, sprint progress, build logs, and process liveness into
a unified view.

By default, waits for a build to start and streams enriched events.
Use --dashboard for a refreshing status view, or --logs to tail the
active build log.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, _ := cmd.Flags().GetString("project-dir")
		if len(args) > 0 {
			dir = args[0]
		}
		dir, err := filepath.Abs(dir)
		if err != nil {
			return fmt.Errorf("resolve path: %w", err)
		}

		// Resolve worktree build directory.
		buildDir := dir
		if setup, readErr := git.ReadPersistedStrategy(dir); readErr == nil && setup != nil && setup.WorkDir != "" && setup.WorkDir != dir {
			if _, statErr := os.Stat(setup.WorkDir); statErr == nil {
				buildDir = setup.WorkDir
			}
		}

		interval := time.Duration(0) // zero means use default
		if monitorInterval != "" {
			parsed, parseErr := time.ParseDuration(monitorInterval)
			if parseErr != nil {
				return fmt.Errorf("invalid interval %q: %w", monitorInterval, parseErr)
			}
			interval = parsed
		}

		cfg := monitor.Config{
			ProjectDir:   dir,
			WorktreeDir:  buildDir,
			Interval:     interval,
			Wait:         !monitorNoWait,
			LogTailLines: config.MonitorDefaultLogTailLines,
		}

		ctx, cancel := context.WithCancel(cmd.Context())
		defer cancel()

		// Handle Ctrl+C gracefully.
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		defer signal.Stop(sigCh)
		go func() {
			select {
			case <-sigCh:
				cancel()
			case <-ctx.Done():
			}
		}()

		mon := monitor.New(cfg)

		if monitorJSON {
			return runMonitorJSON(ctx, cmd, mon)
		}
		if monitorDashboard {
			return runMonitorDashboard(ctx, cmd, mon)
		}
		if monitorLogs {
			return runMonitorLogs(ctx, cmd, mon)
		}
		return runMonitorStream(ctx, cmd, mon)
	},
}

func init() {
	monitorCmd.Flags().BoolVar(&monitorDashboard, "dashboard", false, "Show a refreshing status dashboard")
	monitorCmd.Flags().BoolVar(&monitorLogs, "logs", false, "Tail the active build log")
	monitorCmd.Flags().BoolVar(&monitorJSON, "json", false, "Output snapshots as NDJSON")
	monitorCmd.Flags().BoolVar(&monitorNoWait, "no-wait", false, "Exit immediately if no active build")
	monitorCmd.Flags().StringVar(&monitorInterval, "interval", "", "Polling interval (e.g. 1s, 500ms)")
}

func runMonitorStream(ctx context.Context, cmd *cobra.Command, mon *monitor.Monitor) error {
	w := cmd.OutOrStdout()
	useColor := color.Enabled()
	ch := mon.Run(ctx)
	waiting := true

	for snap := range ch {
		if !snap.BuildActive && len(snap.Events) == 0 && !snap.BuildEnded {
			if waiting {
				monitor.RenderWaiting(w, snap.ProjectDir)
				waiting = false
			}
			continue
		}
		waiting = false

		for _, evt := range snap.NewEvents {
			monitor.RenderEvent(w, evt, useColor)
		}

		if snap.BuildEnded {
			monitor.RenderBuildEnded(w, snap, useColor)
			return nil
		}
	}
	return nil
}

func runMonitorDashboard(ctx context.Context, cmd *cobra.Command, mon *monitor.Monitor) error {
	w := cmd.OutOrStdout()
	useColor := color.Enabled()
	clearScreen := color.Enabled() // only clear on TTY
	ch := mon.Run(ctx)

	for snap := range ch {
		if !snap.BuildActive && len(snap.Events) == 0 && !snap.BuildEnded {
			monitor.RenderWaiting(w, snap.ProjectDir)
			continue
		}
		monitor.RenderDashboard(w, snap, useColor, clearScreen)
		if snap.BuildEnded {
			return nil
		}
	}
	return nil
}

func runMonitorLogs(ctx context.Context, cmd *cobra.Command, mon *monitor.Monitor) error {
	w := cmd.OutOrStdout()
	clearScreen := color.Enabled()
	ch := mon.Run(ctx)

	for snap := range ch {
		if snap.ActiveLogPath == "" && !snap.BuildEnded {
			if !snap.BuildActive && len(snap.Events) == 0 {
				monitor.RenderWaiting(w, snap.ProjectDir)
			} else {
				fmt.Fprintln(w, "Waiting for build log...")
			}
			continue
		}
		if clearScreen {
			fmt.Fprint(w, "\033[H\033[2J")
		}
		monitor.RenderLogTail(w, snap)
		if snap.BuildEnded {
			useColor := color.Enabled()
			monitor.RenderBuildEnded(w, snap, useColor)
			return nil
		}
	}
	return nil
}

func runMonitorJSON(ctx context.Context, cmd *cobra.Command, mon *monitor.Monitor) error {
	enc := json.NewEncoder(cmd.OutOrStdout())
	ch := mon.Run(ctx)

	for snap := range ch {
		if err := enc.Encode(snap); err != nil {
			return err
		}
		if snap.BuildEnded {
			return nil
		}
	}
	return nil
}
