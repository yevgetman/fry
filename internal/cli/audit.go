package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/yevgetman/fry/internal/audit"
	"github.com/yevgetman/fry/internal/config"
	"github.com/yevgetman/fry/internal/engine"
	"github.com/yevgetman/fry/internal/epic"
	"github.com/yevgetman/fry/internal/git"
	frylog "github.com/yevgetman/fry/internal/log"
	"github.com/yevgetman/fry/internal/prepare"
)

var (
	auditEngine    string
	auditEffort    string
	auditModel     string
	auditSARIF     bool
	auditMode      string
	auditMCPConfig string
)

var auditCmd = &cobra.Command{
	Use:   "audit",
	Short: "Run a standalone build-level audit on the codebase",
	Long: `Run a holistic AI-powered code audit on the current project.

Works on any codebase — a completed Fry build, a partially built project,
or code that was never built with Fry. Finds issues, attempts fixes, and
re-audits until clean or the cycle limit is reached.

Results are written to build-audit.md in the project root.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		projectPath, err := resolveProjectDir(projectDir)
		if err != nil {
			return err
		}

		effortLevel, err := epic.ParseEffortLevel(auditEffort)
		if err != nil {
			return err
		}
		if effortLevel == "" {
			effortLevel = epic.EffortHigh
		}

		mode, err := resolveMode(auditMode, false)
		if err != nil {
			return err
		}
		modeStr := string(mode)
		if modeStr == "" {
			modeStr = string(prepare.ModeSoftware)
		}

		engineName, err := engine.ResolveEngine(auditEngine, "", "", config.DefaultEngine)
		if err != nil {
			return fmt.Errorf("audit: resolve engine: %w", err)
		}

		var engineOpts []engine.EngineOpt
		if auditMCPConfig != "" {
			mcpPath := auditMCPConfig
			if abs, absErr := filepath.Abs(auditMCPConfig); absErr == nil {
				mcpPath = abs
			}
			engineOpts = append(engineOpts, engine.WithMCPConfig(mcpPath))
		}

		eng, err := newResilientEngine(engineName, engineOpts...)
		if err != nil {
			return fmt.Errorf("audit: create engine: %w", err)
		}

		// Try to load existing epic; fall back to a synthetic one for standalone audits.
		ep, epicErr := epic.ParseEpic(filepath.Join(projectPath, config.FryDir, "epic.md"))
		if epicErr != nil {
			if !errors.Is(epicErr, os.ErrNotExist) {
				return fmt.Errorf("audit: parse epic: %w", epicErr)
			}
			ep = &epic.Epic{
				Name:             "standalone-audit",
				EffortLevel:      effortLevel,
				TotalSprints:     1,
				AuditAfterSprint: true,
			}
		}
		if ep.EffortLevel == "" {
			ep.EffortLevel = effortLevel
		}

		buildAuditModel := auditModel
		if buildAuditModel == "" {
			buildAuditModel = engine.ResolveModel(ep.AuditModel, engineName, string(ep.EffortLevel), engine.SessionBuildAudit)
		}

		ctx := cmd.Context()

		frylog.Log("▶ BUILD AUDIT  standalone audit...  engine=%s  model=%s  effort=%s", engineName, buildAuditModel, ep.EffortLevel)

		result, err := audit.RunBuildAudit(ctx, audit.BuildAuditOpts{
			ProjectDir: projectPath,
			Epic:       ep,
			Engine:     eng,
			Verbose:    frylog.Verbose,
			Model:      buildAuditModel,
			Mode:       modeStr,
		})
		if err != nil {
			return fmt.Errorf("audit: %w", err)
		}

		if result.Passed {
			frylog.Log("  BUILD AUDIT: PASS (%s)", audit.FormatCounts(result.SeverityCounts))
		} else if result.Blocking {
			frylog.Log("  BUILD AUDIT: FAILED — %s remain", audit.FormatCounts(result.SeverityCounts))
		} else {
			frylog.Log("  BUILD AUDIT: %s remain (advisory)", audit.FormatCounts(result.SeverityCounts))
		}

		if auditSARIF {
			sarifData, sarifErr := audit.ConvertToSARIF(result.UnresolvedFindings)
			if sarifErr != nil {
				frylog.Log("WARNING: could not generate SARIF report: %v", sarifErr)
			} else {
				sarifPath := filepath.Join(projectPath, config.BuildAuditSARIFFile)
				if writeErr := os.WriteFile(sarifPath, sarifData, 0o644); writeErr != nil {
					frylog.Log("WARNING: could not write SARIF report: %v", writeErr)
				} else {
					frylog.Log("  SARIF: %s written (%d findings)", config.BuildAuditSARIFFile, len(result.UnresolvedFindings))
				}
			}
		}

		frylog.Log("  GIT: checkpoint — build-audit")
		if gitErr := git.GitCheckpoint(ctx, projectPath, ep.Name, ep.TotalSprints, "", "build-audit"); gitErr != nil {
			frylog.Log("WARNING: git checkpoint after build audit failed: %v", gitErr)
		}

		if result.Blocking {
			return fmt.Errorf("audit failed: %s unresolved", audit.FormatCounts(result.SeverityCounts))
		}
		return nil
	},
}

func init() {
	auditCmd.Flags().StringVar(&auditEngine, "engine", "", "Execution engine (default: claude)")
	auditCmd.Flags().StringVar(&auditEffort, "effort", "high", "Effort level: low, medium, high, max")
	auditCmd.Flags().StringVar(&auditModel, "model", "", "Override agent model")
	auditCmd.Flags().BoolVar(&auditSARIF, "sarif", false, "Write build-audit.sarif in SARIF 2.1.0 format")
	auditCmd.Flags().StringVar(&auditMode, "mode", "", "Audit mode: software, planning, writing")
	auditCmd.Flags().StringVar(&auditMCPConfig, "mcp-config", "", "Path to MCP server configuration file (Claude engine only)")
}
