package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/yevgetman/fry/internal/config"
	"github.com/yevgetman/fry/internal/engine"
	"github.com/yevgetman/fry/internal/git"
	frylog "github.com/yevgetman/fry/internal/log"
	"github.com/yevgetman/fry/internal/scan"
)

var (
	initEngine string
	initHeuristicOnly bool
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Scaffold the fry project structure in the current directory",
	Long:  "Scaffold the fry project structure. In an empty directory, creates plans/, assets/, and media/ with a plan template. In an existing project (detected via git history, project markers, or file count), runs a structural scan and a semantic LLM scan to generate .fry/codebase.md. Use --heuristic-only to skip the semantic scan and only run structural heuristics.",
	RunE: func(cmd *cobra.Command, args []string) error {
		projectPath, err := resolveProjectDir(projectDir)
		if err != nil {
			return err
		}

		ctx := context.Background()

		// Detect existing project BEFORE scaffolding so we can report it.
		existing := scan.IsExistingProject(ctx, projectPath)

		created, err := scaffoldProject(projectPath)
		if err != nil {
			return fmt.Errorf("init: %w", err)
		}

		if err := git.InitGit(ctx, projectPath); err != nil {
			return fmt.Errorf("init: %w", err)
		}

		for _, path := range created {
			rel, _ := filepath.Rel(projectPath, path)
			if rel == "" {
				rel = path
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  created %s\n", rel)
		}

		if existing {
			fmt.Fprintln(cmd.OutOrStdout())
			fmt.Fprintln(cmd.OutOrStdout(), "Existing project detected. Scanning codebase...")

			snap, scanErr := scan.RunStructuralScan(ctx, projectPath)
			if scanErr != nil {
				return fmt.Errorf("init: codebase scan: %w", scanErr)
			}

			indexPath := filepath.Join(projectPath, config.FileIndexFile)
			if writeErr := scan.WriteFileIndex(snap, indexPath); writeErr != nil {
				return fmt.Errorf("init: write file index: %w", writeErr)
			}

			printScanSummary(cmd, snap)

			// Semantic scan: generate .fry/codebase.md (default behavior).
			if initHeuristicOnly {
				fmt.Fprintln(cmd.OutOrStdout(), "  Semantic scan skipped (--heuristic-only)")
			} else if err := runSemanticScan(ctx, cmd, projectPath, snap); err != nil {
				// Non-fatal: structural scan succeeded, semantic failed gracefully.
				fmt.Fprintf(cmd.ErrOrStderr(), "fry: warning: semantic scan failed: %v\n", err)
			}

			fmt.Fprintln(cmd.OutOrStdout())
			fmt.Fprintln(cmd.OutOrStdout(), "Codebase indexed. Next steps:")
			fmt.Fprintln(cmd.OutOrStdout(), "  1. Run: fry run --user-prompt \"describe what you want to change\"")
			fmt.Fprintln(cmd.OutOrStdout(), "     Or write plans/plan.md and run: fry run")
		} else {
			fmt.Fprintln(cmd.OutOrStdout())
			fmt.Fprintln(cmd.OutOrStdout(), "Project initialized. Next steps:")
			fmt.Fprintln(cmd.OutOrStdout(), "  1. Write plans/plan.md (see plans/plan.example.md for reference)")
			fmt.Fprintln(cmd.OutOrStdout(), "     Or provide a --user-prompt and fry will generate one for you")
			fmt.Fprintln(cmd.OutOrStdout(), "  2. Run: fry prepare")
			fmt.Fprintln(cmd.OutOrStdout(), "  3. Run: fry run")
		}

		return nil
	},
}

func init() {
	initCmd.Flags().StringVar(&initEngine, "engine", "", "Engine for semantic codebase scan (default: auto-resolved)")
	initCmd.Flags().BoolVar(&initHeuristicOnly, "heuristic-only", false, "Skip semantic LLM scan; only run structural heuristics")
}

// runSemanticScan resolves an engine and runs the semantic scan.
// Returns an error (treated as non-fatal by the caller) on failure.
func runSemanticScan(ctx context.Context, cmd *cobra.Command, projectDir string, snap *scan.StructuralSnapshot) error {
	engineName, err := engine.ResolveEngine(initEngine, "", "", config.DefaultEngine)
	if err != nil {
		return fmt.Errorf("resolve engine: %w", err)
	}

	eng, err := newResilientEngine(engineName)
	if err != nil {
		return fmt.Errorf("create engine: %w", err)
	}

	model := engine.ResolveModelForSession(engineName, "", engine.SessionCodebaseScan)

	frylog.Log("▶ SCAN     generating codebase.md with %s (%s)", engineName, model)
	fmt.Fprintln(cmd.OutOrStdout(), "  Generating codebase understanding (this may take a moment)...")

	return scan.RunSemanticScan(ctx, scan.SemanticScanOpts{
		ProjectDir: projectDir,
		Snapshot:   snap,
		Engine:     eng,
		Model:      model,
	})
}

// scaffoldProject creates the fry directory structure and returns paths of created items.
func scaffoldProject(projectDir string) ([]string, error) {
	var created []string

	// Create plans/, assets/, media/ directories
	dirs := []string{
		filepath.Join(projectDir, config.PlansDir),
		filepath.Join(projectDir, config.AssetsDir),
		filepath.Join(projectDir, config.MediaDir),
	}
	for _, dir := range dirs {
		if !fileExists(dir) {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return nil, fmt.Errorf("create directory %s: %w", filepath.Base(dir), err)
			}
			created = append(created, dir)
		}
	}

	// Write plan.example.md (always, so the template stays up to date)
	examplePath := filepath.Join(projectDir, config.PlansDir, "plan.example.md")
	if err := os.WriteFile(examplePath, []byte(starterPlan), 0o644); err != nil {
		return nil, fmt.Errorf("write plan.example.md: %w", err)
	}
	created = append(created, examplePath)

	return created, nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// printScanSummary outputs a human-readable summary of the structural scan.
func printScanSummary(cmd *cobra.Command, snap *scan.StructuralSnapshot) {
	w := cmd.OutOrStdout()

	fmt.Fprintf(w, "  Indexed %d files across %d directories (%s)\n",
		snap.FileStats.TotalFiles, snap.FileStats.TotalDirs,
		scan.FormatSize(snap.FileStats.TotalSize))

	if len(snap.Languages) > 0 {
		names := make([]string, 0, len(snap.Languages))
		for _, lang := range snap.Languages {
			names = append(names, lang.Name)
		}
		fmt.Fprintf(w, "  Languages: %s\n", joinMax(names, 5))
	}

	if len(snap.Frameworks) > 0 {
		fmt.Fprintf(w, "  Frameworks: %s\n", joinMax(snap.Frameworks, 5))
	}

	if len(snap.EntryPoints) > 0 {
		fmt.Fprintf(w, "  Entry points: %s\n", joinMax(snap.EntryPoints, 3))
	}

	if len(snap.Dependencies) > 0 {
		fmt.Fprintf(w, "  Dependencies: %d\n", len(snap.Dependencies))
	}

	if snap.GitHistory != nil {
		fmt.Fprintf(w, "  Git commits: %d\n", snap.GitHistory.TotalCommits)
	}
}

// joinMax joins up to max strings with ", " and appends "..." if truncated.
func joinMax(items []string, max int) string {
	if len(items) <= max {
		return strings.Join(items, ", ")
	}
	return strings.Join(items[:max], ", ") + ", ..."
}

const starterPlan = `# Build Plan

## Overview

Describe what you want to build. Fry will use this plan to generate an epic
with sprint definitions, then execute each sprint using an AI agent.

## Goals

- Goal 1
- Goal 2

## Technical Details

Add implementation details, architecture decisions, technology choices,
file structure expectations, and any constraints the AI agent should follow.

## Deliverables

- [ ] Deliverable 1
- [ ] Deliverable 2
`
