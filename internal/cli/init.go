package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/yevgetman/fry/internal/config"
	"github.com/yevgetman/fry/internal/git"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Scaffold the fry project structure in the current directory",
	Long:  "Create the plans/, assets/, and media/ directories with a plan.example.md template, initialize git, and configure .gitignore for fry.",
	RunE: func(cmd *cobra.Command, args []string) error {
		projectPath, err := resolveProjectDir(projectDir)
		if err != nil {
			return err
		}

		created, err := scaffoldProject(projectPath)
		if err != nil {
			return fmt.Errorf("init: %w", err)
		}

		ctx := context.Background()
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

		fmt.Fprintln(cmd.OutOrStdout())
		fmt.Fprintln(cmd.OutOrStdout(), "Project initialized. Next steps:")
		fmt.Fprintln(cmd.OutOrStdout(), "  1. Write plans/plan.md (see plans/plan.example.md for reference)")
		fmt.Fprintln(cmd.OutOrStdout(), "     Or provide a --user-prompt and fry will generate one for you")
		fmt.Fprintln(cmd.OutOrStdout(), "  2. Run: fry prepare")
		fmt.Fprintln(cmd.OutOrStdout(), "  3. Run: fry run")

		return nil
	},
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
