package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/yevgetman/fry/internal/audit"
	"github.com/yevgetman/fry/internal/config"
	"github.com/yevgetman/fry/internal/continuerun"
	"github.com/yevgetman/fry/internal/docker"
	"github.com/yevgetman/fry/internal/engine"
	"github.com/yevgetman/fry/internal/epic"
	"github.com/yevgetman/fry/internal/git"
	"github.com/yevgetman/fry/internal/lock"
	"github.com/yevgetman/fry/internal/shellhook"
	frlog "github.com/yevgetman/fry/internal/log"
	"github.com/yevgetman/fry/internal/preflight"
	"github.com/yevgetman/fry/internal/prepare"
	"github.com/yevgetman/fry/internal/review"
	"github.com/yevgetman/fry/internal/sprint"
	"github.com/yevgetman/fry/internal/summary"
	"github.com/yevgetman/fry/internal/verify"
)

var (
	runEngine         string
	runDryRun         bool
	runUserPrompt     string
	runUserPromptFile string
	runNoReview       bool
	runSimulateReview string
	runPrepareEngine  string
	runPlanning       bool
	runMode           string
	runEffort         string
	runNoAudit        bool
	runResume          bool
	runSprint         int
	runContinue       bool
	runNoSanityCheck  bool
)

var errBuildFailed = fmt.Errorf("build failed")

type deferredEntry struct {
	SprintNumber int
	SprintName   string
	Failures     []verify.CheckResult
}

var runCmd = &cobra.Command{
	Use:   "run [epic.md] [start] [end]",
	Short: "Run fry against an epic",
	Args:  cobra.MaximumNArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		projectPath, err := resolveProjectDir(projectDir)
		if err != nil {
			return err
		}

		effortLevel, err := epic.ParseEffortLevel(runEffort)
		if err != nil {
			return err
		}

		mode, err := resolveMode(runMode, runPlanning)
		if err != nil {
			return err
		}

		// When --continue is used and no explicit --mode was given,
		// auto-detect mode from the persisted build-mode.txt file.
		userSetMode := strings.TrimSpace(runMode) != "" || runPlanning
		if runContinue && !userSetMode {
			if detected := continuerun.ReadBuildMode(projectPath); detected != "" {
				parsedMode, parseErr := prepare.ParseMode(detected)
				if parseErr == nil && parsedMode != mode {
					mode = parsedMode
					frlog.Log("▶ CONTINUE  auto-detected mode: %s", mode)
				}
			}
		}

		epicArg := filepath.Join(config.FryDir, "epic.md")
		if len(args) > 0 {
			epicArg = args[0]
		}

		userPrompt, err := resolveUserPrompt(projectPath, runUserPrompt, runUserPromptFile, !runDryRun)
		if err != nil {
			return err
		}

		epicPath, epicExists, err := resolveEpicPath(projectPath, epicArg)
		if err != nil {
			return err
		}

		printMigrationHintIfNeeded(cmd.OutOrStdout(), projectPath, epicArg)

		if !epicExists {
			if runResume {
				return fmt.Errorf("--resume requires existing build artifacts; epic file not found at %s", epicArg)
			}
			if runContinue {
				return fmt.Errorf("--continue requires existing build artifacts; epic file not found at %s", epicArg)
			}
			prepareEngineName := resolvePrepareEngine(runPrepareEngine, runEngine)
			if err := prepare.RunPrepare(cmd.Context(), prepare.PrepareOpts{
				ProjectDir:      projectPath,
				EpicFilename:    filepath.Base(epicPath),
				Engine:          prepareEngineName,
				UserPrompt:      userPrompt,
				SkipSanityCheck: runNoSanityCheck || runDryRun,
				Mode:            mode,
				EffortLevel:     effortLevel,
				Stdin:           os.Stdin,
				Stdout:          cmd.OutOrStdout(),
			}); err != nil {
				return err
			}
		}

		ep, err := epic.ParseEpic(epicPath)
		if err != nil {
			return err
		}
		// Apply effort override if user specified one and the epic doesn't have one
		if effortLevel != "" && ep.EffortLevel == "" {
			ep.EffortLevel = effortLevel
		} else if effortLevel != "" && ep.EffortLevel != "" && effortLevel != ep.EffortLevel {
			frlog.Log("WARNING: --effort %s ignored; epic already specifies @effort %s. To change effort level, re-run fry prepare with --effort %s.", effortLevel, ep.EffortLevel, effortLevel)
		}
		if err := epic.ValidateEpic(ep); err != nil {
			return err
		}

		buildDefault := config.DefaultEngine
		switch mode {
		case prepare.ModePlanning:
			buildDefault = config.DefaultPlanningEngine
		case prepare.ModeWriting:
			buildDefault = config.DefaultWritingEngine
		}
		engineName, err := engine.ResolveEngine(runEngine, ep.Engine, "", buildDefault)
		if err != nil {
			return err
		}
		buildEngine, err := engine.NewEngine(engineName)
		if err != nil {
			return err
		}

		// Validate --continue flag conflicts
		if runContinue {
			if runSprint > 0 {
				return fmt.Errorf("cannot use --continue with --sprint; --continue auto-detects the resume point")
			}
			if len(args) > 1 {
				return fmt.Errorf("cannot use --continue with positional sprint arguments")
			}
			if runResume {
				return fmt.Errorf("cannot use --continue with --resume; --continue auto-detects whether to resume")
			}
		}

		var startSprint, endSprint int
		if runContinue {
			frlog.Log("▶ CONTINUE  collecting build state...")
			state, collectErr := continuerun.CollectBuildState(cmd.Context(), projectPath, ep)
			if collectErr != nil {
				return fmt.Errorf("continue: %w", collectErr)
			}

			report := continuerun.FormatReport(state)
			fmt.Fprint(cmd.OutOrStdout(), report)
			fmt.Fprintln(cmd.OutOrStdout())

			continueOverride := ep.AuditModel
			if continueOverride == "" {
				continueOverride = ep.AgentModel
			}
			continueModel := engine.ResolveModel(continueOverride, engineName, string(ep.EffortLevel), engine.SessionContinue)
			decision, analyzeErr := continuerun.Analyze(cmd.Context(), continuerun.AnalyzeOpts{
				ProjectDir: projectPath,
				State:      state,
				Engine:     buildEngine,
				Model:      continueModel,
				Verbose:    frlog.Verbose,
			})
			if analyzeErr != nil {
				return fmt.Errorf("continue: %w", analyzeErr)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Decision: %s (sprint %d)\n", decision.Verdict, decision.StartSprint)
			fmt.Fprintf(cmd.OutOrStdout(), "Reason: %s\n", decision.Reason)
			if len(decision.Preconditions) > 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "Pre-conditions:")
				for _, pc := range decision.Preconditions {
					fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", pc)
				}
			}

			switch decision.Verdict {
			case continuerun.VerdictAllComplete:
				fmt.Fprintf(cmd.OutOrStdout(), "All %d sprints already complete. Nothing to do.\n", ep.TotalSprints)
				return nil
			case continuerun.VerdictBlocked:
				return fmt.Errorf("continue: blocked — %s", decision.Reason)
			case continuerun.VerdictResume:
				startSprint = decision.StartSprint
				endSprint = ep.TotalSprints
				runResume = true
			case continuerun.VerdictResumeFresh:
				startSprint = decision.StartSprint
				endSprint = ep.TotalSprints
			case continuerun.VerdictContinueNext:
				startSprint = decision.StartSprint
				endSprint = ep.TotalSprints
			default:
				return fmt.Errorf("continue: unexpected verdict %q", decision.Verdict)
			}

			if startSprint < 1 || startSprint > ep.TotalSprints {
				return fmt.Errorf("continue: agent returned invalid sprint %d (total: %d)", startSprint, ep.TotalSprints)
			}
		} else {
			rangeArgs := []string{}
			if len(args) > 1 {
				rangeArgs = args[1:]
			}
			if runSprint > 0 {
				if len(rangeArgs) > 0 {
					return fmt.Errorf("cannot use --sprint with positional sprint arguments")
				}
				rangeArgs = []string{strconv.Itoa(runSprint)}
			}
			startSprint, endSprint, err = resolveSprintRange(rangeArgs, ep.TotalSprints)
			if err != nil {
				return err
			}
		}

		if runDryRun {
			return printDryRunReport(cmd.OutOrStdout(), projectPath, epicPath, ep, engineName, startSprint, endSprint)
		}

		if err := lock.AcquireIfNotDryRun(projectPath, runDryRun); err != nil {
			return err
		}
		var lockOnce sync.Once
		releaseLock := func() {
			lockOnce.Do(func() { _ = lock.Release(projectPath) })
		}
		defer releaseLock()

		ctx, cancel := context.WithCancel(cmd.Context())
		defer cancel()

		results := initializeSprintResults(ep, startSprint, endSprint)
		var mu sync.Mutex
		currentSprint := 0
		epicName := ep.Name // guarded by mu; updated after replan

		signalCh := make(chan os.Signal, 1)
		signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)
		defer signal.Stop(signalCh)

		go func() {
			select {
			case <-ctx.Done():
				return
			case <-signalCh:
			}
			cancel()
			mu.Lock()
			activeSprint := currentSprint
			name := epicName
			summaryCopy := append([]sprint.SprintResult(nil), results...)
			mu.Unlock()
			if activeSprint > 0 {
				_ = git.CommitPartialWork(projectPath, name, activeSprint)
			}
			printBuildSummary(cmd.OutOrStdout(), summaryCopy)
			releaseLock()
			os.Exit(130)
		}()

		if err := preflight.RunPreflight(preflight.PreflightConfig{
			ProjectDir:       projectPath,
			Engine:           engineName,
			DockerFromSprint: ep.DockerFromSprint,
			CurrentSprint:    startSprint,
			RequiredTools:    ep.RequiredTools,
			PreflightCmds:    ep.PreflightCmds,
		}); err != nil {
			return err
		}
		if err := git.InitGit(projectPath); err != nil {
			return err
		}

		reviewSummary := review.DeviationSummary{
			TotalSprints: startSprintCount(startSprint, endSprint),
			AllLowRisk:   true,
		}

		var allDeferredFailures []deferredEntry
		modeStr := string(mode)

		// Persist mode for --continue auto-detection on subsequent runs
		modePath := filepath.Join(projectPath, config.BuildModeFile)
		if mkErr := os.MkdirAll(filepath.Dir(modePath), 0o755); mkErr != nil {
			frlog.Log("WARNING: could not create dir for build mode: %v", mkErr)
		} else if writeErr := os.WriteFile(modePath, []byte(modeStr+"\n"), 0o644); writeErr != nil {
			frlog.Log("WARNING: could not persist build mode: %v", writeErr)
		}

		exitErr := error(nil)
		for sprintNum := startSprint; sprintNum <= endSprint; sprintNum++ {
			if sprintNum < 1 || sprintNum > len(ep.Sprints) {
				return fmt.Errorf("sprint %d out of range (total: %d)", sprintNum, len(ep.Sprints))
			}
			spr := &ep.Sprints[sprintNum-1]

			mu.Lock()
			currentSprint = sprintNum
			mu.Unlock()

			if ep.DockerFromSprint > 0 && sprintNum >= ep.DockerFromSprint {
				if err := docker.EnsureDockerUp(ctx, projectPath, ep.DockerReadyCmd, ep.DockerReadyTimeout); err != nil {
					return err
				}
			}

			if err := shellhook.Run(ctx, projectPath, ep.PreSprintCmd); err != nil {
				return err
			}

			// Skip epic progress reset in resume mode to preserve prior context
			if !runResume && sprint.ShouldResetEpicProgress(startSprint, sprintNum, endSprint, ep.TotalSprints) {
				if err := sprint.InitEpicProgress(projectPath, ep.Name); err != nil {
					return err
				}
			}

			// In resume mode, the first sprint skips iterations and goes straight
			// to verification + healing with a boosted attempt budget.
			var result *sprint.SprintResult
			if runResume && sprintNum == startSprint {
				result, err = sprint.ResumeSprint(ctx, sprint.RunConfig{
					ProjectDir:  projectPath,
					Epic:        ep,
					Sprint:      spr,
					Engine:      buildEngine,
					Verbose:     frlog.Verbose,
					DryRun:      false,
					UserPrompt:  userPrompt,
					StartSprint: startSprint,
					EndSprint:   endSprint,
					Mode:        modeStr,
				})
			} else {
				result, err = sprint.RunSprint(ctx, sprint.RunConfig{
					ProjectDir:  projectPath,
					Epic:        ep,
					Sprint:      spr,
					Engine:      buildEngine,
					Verbose:     frlog.Verbose,
					DryRun:      false,
					UserPrompt:  userPrompt,
					StartSprint: startSprint,
					EndSprint:   endSprint,
					Mode:        modeStr,
				})
			}
			if err != nil {
				return err
			}

			mu.Lock()
			results[sprintNum-startSprint] = *result
			mu.Unlock()

			if len(result.DeferredFailures) > 0 {
				allDeferredFailures = append(allDeferredFailures, deferredEntry{
					SprintNumber: spr.Number,
					SprintName:   spr.Name,
					Failures:     result.DeferredFailures,
				})
				writeDeferredFailuresArtifact(projectPath, allDeferredFailures)
				frlog.Log("  WARNING: %d deferred verification failures (within %d%% threshold)",
					len(result.DeferredFailures), ep.MaxFailPercent)
			}

			if isPassStatus(result.Status) {
				// Sprint audit
				if ep.AuditAfterSprint && !runNoAudit && ep.EffortLevel != epic.EffortLow {
					auditEngine, err := resolveAuditEngine(engineName, ep.AuditEngine)
					if err != nil {
						return err
					}
					gitDiff, err := git.GitDiffForAudit(projectPath)
					if err != nil {
						frlog.Log("WARNING: could not capture git diff for audit: %v", err)
						gitDiff = "(git diff unavailable)"
					}
					auditResult, err := audit.RunAuditLoop(ctx, audit.AuditOpts{
						ProjectDir: projectPath,
						Sprint:     spr,
						Epic:       ep,
						Engine:     auditEngine,
						GitDiff:    gitDiff,
						DiffFn:     func() (string, error) { return git.GitDiffForAudit(projectPath) },
						Verbose:    frlog.Verbose,
						Mode:       modeStr,
					})
					if err != nil {
						return err
					}
					if !auditResult.Passed {
						if auditResult.Blocking {
							frlog.Log("  AUDIT: FAILED — %s remain after %d audit cycles",
								audit.FormatCounts(auditResult.SeverityCounts), auditResult.Iterations)
							if cleanupErr := audit.Cleanup(projectPath); cleanupErr != nil {
								frlog.Log("WARNING: audit cleanup failed: %v", cleanupErr)
							}
							mu.Lock()
							results[sprintNum-startSprint].Status = fmt.Sprintf("FAIL (audit: %s)", auditResult.MaxSeverity)
							mu.Unlock()
							fmt.Fprintf(cmd.OutOrStdout(), "Resume:   fry run --resume --sprint %d\n", spr.Number)
							fmt.Fprintf(cmd.OutOrStdout(), "Restart:  fry run --sprint %d\n", spr.Number)
							fmt.Fprintf(cmd.OutOrStdout(), "Continue: fry run --continue\n")
							exitErr = errBuildFailed
							break
						}
						warning := fmt.Sprintf("%s remain after %d audit cycles (advisory)",
							audit.FormatCounts(auditResult.SeverityCounts), auditResult.Iterations)
						frlog.Log("  AUDIT: %s", warning)
						mu.Lock()
						results[sprintNum-startSprint].AuditWarning = warning
						mu.Unlock()
					}
					// Note LOW remainders at high/max effort (audit passed but LOW items remain)
					if auditResult.Passed && auditResult.SeverityCounts["LOW"] > 0 &&
						(ep.EffortLevel == epic.EffortHigh || ep.EffortLevel == epic.EffortMax) {
						lowNote := fmt.Sprintf("%d LOW issues remain after %d audit cycles (non-blocking)",
							auditResult.SeverityCounts["LOW"], auditResult.Iterations)
						frlog.Log("  AUDIT: %s", lowNote)
						mu.Lock()
						if results[sprintNum-startSprint].AuditWarning == "" {
							results[sprintNum-startSprint].AuditWarning = lowNote
						} else {
							results[sprintNum-startSprint].AuditWarning += "; " + lowNote
						}
						mu.Unlock()
					}
					if cleanupErr := audit.Cleanup(projectPath); cleanupErr != nil {
						frlog.Log("WARNING: audit cleanup failed: %v", cleanupErr)
					}
				}

				frlog.Log("  GIT: checkpoint — sprint %d complete", spr.Number)
				if err := git.GitCheckpoint(projectPath, ep.Name, spr.Number, "complete"); err != nil {
					return err
				}

				compactEngine, err := engine.NewEngine(engineName)
				if err != nil {
					return err
				}
				compactModel := engine.ResolveModel(ep.AgentModel, engineName, string(ep.EffortLevel), engine.SessionCompaction)
			compacted, err := sprint.CompactSprintProgress(ctx, projectPath, spr.Number, spr.Name, result.Status, compactEngine, ep.CompactWithAgent, compactModel)
				if err != nil {
					return err
				}
				if err := sprint.AppendToEpicProgress(projectPath, compacted+"\n"); err != nil {
					return err
				}
				frlog.Log("  GIT: checkpoint — sprint %d compacted", spr.Number)
				if err := git.GitCheckpoint(projectPath, ep.Name, spr.Number, "compacted"); err != nil {
					return err
				}

				if ep.ReviewBetweenSprints && !runNoReview && spr.Number < ep.TotalSprints && ep.EffortLevel != epic.EffortLow {
					reviewSummary.ReviewsConducted++

					reviewEngine, err := resolveReviewEngine(engineName, ep.ReviewEngine)
					if err != nil {
						return err
					}
					reviewResult, err := review.RunSprintReview(ctx, review.RunReviewOpts{
						ProjectDir:      projectPath,
						SprintNum:       spr.Number,
						TotalSprints:    ep.TotalSprints,
						SprintName:      spr.Name,
						Epic:            ep,
						Engine:          reviewEngine,
						SimulateVerdict: runSimulateReview,
						Verbose:         frlog.Verbose,
						Mode:            modeStr,
					})
					if err != nil {
						return err
					}

					if reviewResult.Verdict == review.VerdictDeviate && reviewResult.Deviation != nil {
						frlog.Log("  REVIEW: verdict DEVIATE — replanning sprints %s (risk: %s)",
							formatAffectedSprints(reviewResult.Deviation.AffectedSprints),
							reviewResult.Deviation.RiskAssessment)
					} else {
						frlog.Log("  REVIEW: verdict %s", reviewResult.Verdict)
					}

					entry := review.DeviationLogEntry{
						SprintNum:  spr.Number,
						SprintName: spr.Name,
						Verdict:    reviewResult.Verdict,
						Impact:     strings.TrimSpace(reviewResult.RawOutput),
					}
					if reviewResult.Deviation != nil {
						entry.Trigger = reviewResult.Deviation.Trigger
						entry.RiskAssessment = reviewResult.Deviation.RiskAssessment
						entry.AffectedSprints = reviewResult.Deviation.AffectedSprints
						if strings.TrimSpace(reviewResult.Deviation.Details) != "" {
							entry.Impact = strings.TrimSpace(reviewResult.Deviation.Details)
						}
						if !strings.Contains(strings.ToLower(reviewResult.Deviation.RiskAssessment), "low") {
							reviewSummary.AllLowRisk = false
						}
					}
					if err := review.AppendDeviationLog(projectPath, entry); err != nil {
						return err
					}

					if reviewResult.Verdict == review.VerdictDeviate {
						reviewSummary.DeviationsApplied++
						if reviewResult.Deviation == nil {
							return fmt.Errorf("review requested deviation without a deviation spec")
						}
						replanModel := engine.ResolveModel(ep.ReviewModel, reviewEngine.Name(), string(ep.EffortLevel), engine.SessionReplan)
						if err := review.RunReplan(ctx, review.ReplanOpts{
							ProjectDir:      projectPath,
							EpicPath:        epicPath,
							DeviationSpec:   reviewResult.Deviation,
							CompletedSprint: spr.Number,
							MaxScope:        ep.MaxDeviationScope,
							Engine:          reviewEngine,
							Model:           replanModel,
							Verbose:         frlog.Verbose,
						}); err != nil {
							return err
						}
						ep, err = epic.ParseEpic(epicPath)
						if err != nil {
							return err
						}
						if err := epic.ValidateEpic(ep); err != nil {
							return err
						}
						mu.Lock()
						epicName = ep.Name
						mu.Unlock()
						verificationPath := ep.VerificationFile
						if !filepath.IsAbs(verificationPath) {
							verificationPath = filepath.Join(projectPath, verificationPath)
						}
						if _, err := verify.ParseVerification(verificationPath); err != nil && !os.IsNotExist(err) {
							return err
						}
						frlog.Log("  GIT: checkpoint — sprint %d reviewed-deviate", spr.Number)
						if err := git.GitCheckpoint(projectPath, ep.Name, spr.Number, "reviewed-deviate"); err != nil {
							return err
						}
					}
				}
				continue
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Resume:   fry run --resume --sprint %d\n", spr.Number)
			fmt.Fprintf(cmd.OutOrStdout(), "Restart:  fry run --sprint %d\n", spr.Number)
			fmt.Fprintf(cmd.OutOrStdout(), "Continue: fry run --continue\n")
			exitErr = errBuildFailed
			break
		}

		mu.Lock()
		summaryCopy := append([]sprint.SprintResult(nil), results...)
		mu.Unlock()
		if ep.EffortLevel != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "Effort level: %s\n", ep.EffortLevel)
		}
		printBuildSummary(cmd.OutOrStdout(), summaryCopy)

		if ep.ReviewBetweenSprints && !runNoReview {
			if reviewSummary.ReviewsConducted == 0 {
				reviewSummary.AllLowRisk = true
			}
			if err := review.AppendDeviationSummary(projectPath, reviewSummary); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Reviews: %d, deviations: %d\n", reviewSummary.ReviewsConducted, reviewSummary.DeviationsApplied)
		}

		// Generate build summary document
		summaryEngine, err := engine.NewEngine(engineName)
		if err != nil {
			frlog.Log("WARNING: could not create engine for build summary: %v", err)
		} else {
			summaryModel := engine.ResolveModel(ep.AgentModel, engineName, string(ep.EffortLevel), engine.SessionBuildSummary)
			frlog.Log("▶ BUILD SUMMARY  generating...  engine=%s  model=%s", engineName, summaryModel)
			if summaryErr := summary.GenerateBuildSummary(ctx, summary.SummaryOpts{
				ProjectDir: projectPath,
				EpicName:   ep.Name,
				Engine:     summaryEngine,
				Results:    summaryCopy,
				Verbose:    frlog.Verbose,
				Model:      summaryModel,
			}); summaryErr != nil {
				frlog.Log("WARNING: build summary generation failed: %v", summaryErr)
			} else {
				frlog.Log("  BUILD SUMMARY: complete")
			}
		}

		// Run final build audit once the entire epic has completed successfully
		if exitErr == nil && startSprint == 1 && endSprint == ep.TotalSprints && ep.AuditAfterSprint && !runNoAudit && ep.EffortLevel != epic.EffortLow {
			deferredContent := readDeferredFailuresArtifact(projectPath)
			auditEngine, err := resolveAuditEngine(engineName, ep.AuditEngine)
			if err != nil {
				frlog.Log("WARNING: could not create engine for build audit: %v", err)
			} else {
				buildAuditModel := engine.ResolveModel(ep.AuditModel, auditEngine.Name(), string(ep.EffortLevel), engine.SessionBuildAudit)
				frlog.Log("▶ BUILD AUDIT  running holistic audit across all %d sprints...  engine=%s  model=%s", ep.TotalSprints, auditEngine.Name(), buildAuditModel)
				if auditErr := audit.RunBuildAudit(ctx, audit.BuildAuditOpts{
					ProjectDir:       projectPath,
					Epic:             ep,
					Engine:           auditEngine,
					Results:          summaryCopy,
					Verbose:          frlog.Verbose,
					Model:            buildAuditModel,
					DeferredFailures: deferredContent,
					Mode:             string(mode),
				}); auditErr != nil {
					frlog.Log("WARNING: build audit failed: %v", auditErr)
				} else {
					frlog.Log("  BUILD AUDIT: complete — report written to %s", config.BuildAuditFile)
					frlog.Log("  GIT: checkpoint — build-audit")
					if gitErr := git.GitCheckpoint(projectPath, ep.Name, ep.TotalSprints, "build-audit"); gitErr != nil {
						frlog.Log("WARNING: git checkpoint after build audit failed: %v", gitErr)
					}
				}
			}

			// Re-run deferred verification checks to see if the build audit fixed them
			if len(allDeferredFailures) > 0 {
				frlog.Log("  Re-running deferred verification checks after build audit...")
				for _, entry := range allDeferredFailures {
					var deferredChecks []verify.Check
					for _, f := range entry.Failures {
						deferredChecks = append(deferredChecks, f.Check)
					}
					_, passCount, totalCount := verify.RunChecks(ctx, deferredChecks, entry.SprintNumber, projectPath)
					if passCount == totalCount {
						frlog.Log("  Sprint %d deferred failures: ALL FIXED by build audit", entry.SprintNumber)
					} else {
						frlog.Log("  Sprint %d deferred failures: %d/%d still failing", entry.SprintNumber, totalCount-passCount, totalCount)
					}
				}
			}
		}

		releaseLock()
		return exitErr
	},
}

func init() {
	runCmd.Flags().StringVar(&runEngine, "engine", "", "Execution engine")
	runCmd.Flags().BoolVar(&runDryRun, "dry-run", false, "Preview actions without executing")
	runCmd.Flags().StringVar(&runUserPrompt, "user-prompt", "", "Additional user prompt")
	runCmd.Flags().StringVar(&runUserPromptFile, "user-prompt-file", "", "Path to file containing user prompt")
	runCmd.Flags().BoolVar(&runNoReview, "no-review", false, "Disable sprint review")
	runCmd.Flags().StringVar(&runSimulateReview, "simulate-review", "", "Simulate review verdict")
	runCmd.Flags().StringVar(&runPrepareEngine, "prepare-engine", "", "Engine for auto-prepare")
	runCmd.Flags().BoolVar(&runPlanning, "planning", false, "Use planning mode (alias for --mode planning)")
	runCmd.Flags().StringVar(&runMode, "mode", "", "Execution mode: software, planning, writing")
	runCmd.Flags().StringVar(&runEffort, "effort", "", "Effort level: low, medium, high, max (default: auto)")
	runCmd.Flags().BoolVar(&runNoAudit, "no-audit", false, "Disable sprint and build audits")
	runCmd.Flags().BoolVar(&runResume, "resume", false, "Resume failed sprint: skip iterations, go straight to verification + healing with boosted attempts")
	runCmd.Flags().IntVar(&runSprint, "sprint", 0, "Start from sprint N (alternative to positional sprint argument)")
	runCmd.Flags().BoolVar(&runContinue, "continue", false, "Use an LLM agent to analyze build state and resume from where it left off")
	runCmd.Flags().BoolVar(&runNoSanityCheck, "no-sanity-check", false, "Skip the interactive project summary during auto-prepare")
}

func resolveProjectDir(dir string) (string, error) {
	if strings.TrimSpace(dir) == "" {
		dir = "."
	}
	return filepath.Abs(dir)
}

func resolveUserPrompt(projectDir, provided, promptFile string, persist bool) (string, error) {
	if strings.TrimSpace(provided) != "" && strings.TrimSpace(promptFile) != "" {
		return "", fmt.Errorf("cannot use both --user-prompt and --user-prompt-file")
	}

	if strings.TrimSpace(promptFile) != "" {
		data, err := os.ReadFile(promptFile)
		if err != nil {
			return "", fmt.Errorf("reading user prompt file: %w", err)
		}
		provided = string(data)
	}

	if strings.TrimSpace(provided) != "" {
		if persist {
			path := filepath.Join(projectDir, config.UserPromptFile)
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return "", err
			}
			if err := os.WriteFile(path, []byte(provided), 0o644); err != nil {
				return "", err
			}
		}
		return provided, nil
	}

	data, err := os.ReadFile(filepath.Join(projectDir, config.UserPromptFile))
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return string(data), nil
}

func resolveEpicPath(projectDir, requested string) (string, bool, error) {
	if strings.TrimSpace(requested) == "" {
		requested = filepath.Join(config.FryDir, "epic.md")
	}

	candidates := []string{requested}
	if !filepath.IsAbs(requested) {
		candidates = []string{filepath.Join(projectDir, requested), requested}
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			absPath, absErr := filepath.Abs(candidate)
			return absPath, true, absErr
		}
	}

	resolved := filepath.Join(projectDir, config.FryDir, filepath.Base(requested))
	return resolved, false, nil
}

func resolveSprintRange(args []string, totalSprints int) (int, int, error) {
	startSprint := 1
	endSprint := totalSprints

	if len(args) >= 1 {
		value, err := strconv.Atoi(args[0])
		if err != nil {
			return 0, 0, fmt.Errorf("invalid start sprint %q", args[0])
		}
		startSprint = value
		endSprint = totalSprints
	}
	if len(args) >= 2 {
		value, err := strconv.Atoi(args[1])
		if err != nil {
			return 0, 0, fmt.Errorf("invalid end sprint %q", args[1])
		}
		endSprint = value
	}
	if startSprint < 1 || endSprint < startSprint || endSprint > totalSprints {
		return 0, 0, fmt.Errorf("invalid sprint range %d-%d (total sprints: %d)", startSprint, endSprint, totalSprints)
	}
	return startSprint, endSprint, nil
}

func printMigrationHintIfNeeded(w io.Writer, projectDir, epicArg string) {
	base := filepath.Base(epicArg)
	rootPath := filepath.Join(projectDir, base)
	fryPath := filepath.Join(projectDir, config.FryDir, base)
	if _, err := os.Stat(rootPath); err != nil {
		return
	}
	if _, err := os.Stat(fryPath); err == nil {
		return
	}
	fmt.Fprintln(w, "NOTE: Found fry artifacts in project root (old layout).")
	fmt.Fprintln(w, "  fry now stores all generated artifacts in .fry/.")
	fmt.Fprintln(w, "  To migrate: mv epic.md AGENTS.md verification.md sprint-progress.txt epic-progress.txt .fry/")
}

func resolvePrepareEngine(prepareFlag, runFlag string) string {
	if strings.TrimSpace(prepareFlag) != "" {
		return prepareFlag
	}
	if strings.TrimSpace(runFlag) != "" {
		return runFlag
	}
	return os.Getenv("FRY_ENGINE")
}

func printDryRunReport(w io.Writer, projectDir, epicPath string, ep *epic.Epic, engineName string, startSprint, endSprint int) error {
	fmt.Fprintf(w, "Epic: %s\n", ep.Name)
	fmt.Fprintf(w, "Project dir: %s\n", projectDir)
	fmt.Fprintf(w, "Epic file: %s\n", epicPath)
	fmt.Fprintf(w, "Engine: %s\n", engineName)
	fmt.Fprintf(w, "Effort: %s\n", ep.EffortLevel)
	if runContinue {
		fmt.Fprintln(w, "Mode: continue (auto-detected resume point)")
	} else if runResume {
		fmt.Fprintln(w, "Mode: resume (skip iterations, verify + heal only)")
	}
	fmt.Fprintf(w, "Sprints: %d-%d of %d\n", startSprint, endSprint, ep.TotalSprints)
	fmt.Fprintln(w, "Verification checks:")
	verificationPath := ep.VerificationFile
	if !filepath.IsAbs(verificationPath) {
		verificationPath = filepath.Join(projectDir, verificationPath)
	}
	if _, err := os.Stat(verificationPath); err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintln(w, "  (none)")
			return nil
		}
		return err
	}
	checks, err := verify.ParseVerification(verificationPath)
	if err != nil {
		return err
	}
	if len(checks) == 0 {
		fmt.Fprintln(w, "  (none)")
		return nil
	}
	for _, check := range checks {
		if check.Sprint < startSprint || check.Sprint > endSprint {
			continue
		}
		fmt.Fprintf(w, "  Sprint %d: %s\n", check.Sprint, check.Type)
	}
	return nil
}

func initializeSprintResults(ep *epic.Epic, startSprint, endSprint int) []sprint.SprintResult {
	results := make([]sprint.SprintResult, 0, endSprint-startSprint+1)
	for sprintNum := startSprint; sprintNum <= endSprint; sprintNum++ {
		name := ""
		if sprintNum >= 1 && sprintNum <= len(ep.Sprints) {
			name = ep.Sprints[sprintNum-1].Name
		}
		results = append(results, sprint.SprintResult{
			Number: sprintNum,
			Name:   name,
			Status: sprint.StatusSkipped,
		})
	}
	return results
}

func printBuildSummary(w io.Writer, results []sprint.SprintResult) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "SPRINT\tNAME\tSTATUS\tDURATION")
	for _, result := range results {
		fmt.Fprintf(tw, "%d\t%s\t%s\t%s\n", result.Number, result.Name, result.Status, result.Duration.Round(1e9))
	}
	_ = tw.Flush()

	// Print warnings after the table
	for _, result := range results {
		if result.AuditWarning != "" {
			fmt.Fprintf(w, "  ⚠ Sprint %d: %s\n", result.Number, result.AuditWarning)
		}
		if len(result.DeferredFailures) > 0 {
			fmt.Fprintf(w, "  ⚠ Sprint %d: %d deferred verification failures\n", result.Number, len(result.DeferredFailures))
		}
	}
}

func isPassStatus(status string) bool {
	return strings.HasPrefix(status, "PASS")
}

func resolveAuditEngine(buildEngineName, auditEngineName string) (engine.Engine, error) {
	name := buildEngineName
	if strings.TrimSpace(auditEngineName) != "" {
		name = auditEngineName
	}
	return engine.NewEngine(name)
}

func resolveReviewEngine(buildEngineName, reviewEngineName string) (engine.Engine, error) {
	name := buildEngineName
	if strings.TrimSpace(reviewEngineName) != "" {
		name = reviewEngineName
	}
	return engine.NewEngine(name)
}

func startSprintCount(startSprint, endSprint int) int {
	return endSprint - startSprint + 1
}

func writeDeferredFailuresArtifact(projectDir string, entries []deferredEntry) {
	path := filepath.Join(projectDir, config.DeferredFailuresFile)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		frlog.Log("WARNING: could not create deferred failures dir: %v", err)
		return
	}
	var b strings.Builder
	b.WriteString("# Deferred Verification Failures\n\n")
	for _, entry := range entries {
		b.WriteString(fmt.Sprintf("## Sprint %d: %s\n\n", entry.SprintNumber, entry.SprintName))
		b.WriteString(verify.CollectDeferredSummary(entry.Failures))
		b.WriteString("\n")
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		frlog.Log("WARNING: could not write deferred failures artifact: %v", err)
	}
}

func readDeferredFailuresArtifact(projectDir string) string {
	path := filepath.Join(projectDir, config.DeferredFailuresFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

func formatAffectedSprints(sprints []int) string {
	if len(sprints) == 0 {
		return "unknown"
	}
	parts := make([]string, len(sprints))
	for i, s := range sprints {
		parts[i] = strconv.Itoa(s)
	}
	return strings.Join(parts, ", ")
}

// resolveMode reconciles the --mode flag with the legacy --planning bool flag.
// Returns an error if both --mode and --planning are set to conflicting values.
func resolveMode(modeFlag string, planningFlag bool) (prepare.Mode, error) {
	if strings.TrimSpace(modeFlag) != "" && planningFlag {
		return "", fmt.Errorf("cannot use both --mode and --planning; use --mode planning instead")
	}
	if planningFlag {
		return prepare.ModePlanning, nil
	}
	return prepare.ParseMode(modeFlag)
}
