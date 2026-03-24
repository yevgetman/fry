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
	"time"

	"github.com/spf13/cobra"

	"github.com/yevgetman/fry/internal/archive"
	"github.com/yevgetman/fry/internal/audit"
	"github.com/yevgetman/fry/internal/color"
	"github.com/yevgetman/fry/internal/config"
	"github.com/yevgetman/fry/internal/continuerun"
	"github.com/yevgetman/fry/internal/docker"
	"github.com/yevgetman/fry/internal/engine"
	"github.com/yevgetman/fry/internal/epic"
	"github.com/yevgetman/fry/internal/git"
	"github.com/yevgetman/fry/internal/lock"
	frlog "github.com/yevgetman/fry/internal/log"
	"github.com/yevgetman/fry/internal/metrics"
	"github.com/yevgetman/fry/internal/observer"
	"github.com/yevgetman/fry/internal/preflight"
	"github.com/yevgetman/fry/internal/prepare"
	"github.com/yevgetman/fry/internal/report"
	"github.com/yevgetman/fry/internal/review"
	"github.com/yevgetman/fry/internal/shellhook"
	"github.com/yevgetman/fry/internal/sprint"
	"github.com/yevgetman/fry/internal/summary"
	"github.com/yevgetman/fry/internal/triage"
	"github.com/yevgetman/fry/internal/verify"
)

var (
	runEngine         string
	runDryRun         bool
	runUserPrompt     string
	runUserPromptFile string
	runNoReview       bool
	runReview         bool
	runSimulateReview string
	runPrepareEngine  string
	runPlanning       bool
	runMode           string
	runEffort         string
	runNoAudit        bool
	runResume         bool
	runSprint         int
	runContinue       bool
	runNoSanityCheck  bool
	runFullPrepare    bool
	runGitStrategy    string
	runBranchName     string
	runAlwaysVerify   bool
	runSimpleContinue bool
	runSARIF          bool
	runJSONReport     bool
	runShowTokens     bool
	runNoObserver     bool
	runTriageOnly     bool
	runModel          string
)

type deferredEntry struct {
	SprintNumber int
	SprintName   string
	Failures     []verify.CheckResult
}

var runCmd = &cobra.Command{
	Use:   "run [epic.md] [start] [end]",
	Short: "Run fry against an epic",
	Args:  cobra.MaximumNArgs(3),
	RunE: func(cmd *cobra.Command, args []string) (retErr error) {
		if err := checkArgsForMissingDashes(cmd, args); err != nil {
			return err
		}

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

		gitStrategy, err := git.ParseGitStrategy(runGitStrategy)
		if err != nil {
			return err
		}
		if gitStrategy == git.StrategyCurrent && runBranchName != "" {
			return fmt.Errorf("--branch-name cannot be used with --git-strategy current")
		}
		if runTriageOnly && runFullPrepare {
			return fmt.Errorf("cannot use --triage-only with --full-prepare")
		}
		if runTriageOnly && (runResume || runContinue || runSimpleContinue) {
			return fmt.Errorf("cannot use --triage-only with --resume, --continue, or --simple-continue")
		}

		// When --continue is used and no explicit --mode was given,
		// auto-detect mode from the persisted build-mode.txt file.
		userSetMode := strings.TrimSpace(runMode) != "" || runPlanning
		if (runContinue || runSimpleContinue) && !userSetMode {
			if detected := continuerun.ReadBuildMode(projectPath); detected != "" {
				parsedMode, parseErr := prepare.ParseMode(detected)
				if parseErr == nil && parsedMode != mode {
					mode = parsedMode
					frlog.Log("▶ CONTINUE  auto-detected mode: %s", mode)
				}
			}
		}

		ctx, cancel := context.WithCancel(cmd.Context())
		defer cancel()

		// For --continue/--resume, detect if a prior run used a worktree and redirect
		// projectPath before any file reads (epic, user-prompt, etc.)
		var strategySetup *git.StrategySetup
		originalProjectPath := projectPath
		if runContinue || runSimpleContinue || runResume {
			if persisted, readErr := git.ReadPersistedStrategy(projectPath); readErr == nil && persisted != nil {
				if persisted.IsWorktree && git.IsInsideGitRepo(ctx, persisted.WorkDir) {
					projectPath = persisted.WorkDir
					strategySetup = persisted
					frlog.Log("▶ CONTINUE  reattaching to worktree: %s", persisted.WorkDir)
				} else if persisted.Strategy == git.StrategyBranch && persisted.BranchName != "" {
					// Checkout the branch if we're not already on it.
					if git.CurrentBranch(ctx, projectPath) != persisted.BranchName {
						if coErr := git.CheckoutBranch(ctx, projectPath, persisted.BranchName); coErr != nil {
							frlog.Log("WARNING: could not checkout branch %s: %v", persisted.BranchName, coErr)
						}
					}
					strategySetup = persisted
					frlog.Log("▶ CONTINUE  reattaching to branch: %s", persisted.BranchName)
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
		promptSource := userPromptSource(userPrompt, runUserPrompt, runUserPromptFile)

		epicPath, epicExists, err := resolveEpicPath(projectPath, epicArg)
		if err != nil {
			return err
		}

		printMigrationHintIfNeeded(cmd.OutOrStdout(), projectPath, epicArg)

		if runTriageOnly && epicExists {
			return fmt.Errorf("--triage-only: epic already exists at %s; triage only runs when no epic exists", epicArg)
		}

		var triageDecision *triage.TriageDecision
		if !epicExists {
			if runResume {
				return fmt.Errorf("--resume requires existing build artifacts; epic file not found at %s", epicArg)
			}
			if runContinue {
				return fmt.Errorf("--continue requires existing build artifacts; epic file not found at %s", epicArg)
			}
			if runSimpleContinue {
				return fmt.Errorf("--simple-continue requires existing build artifacts; epic file not found at %s", epicArg)
			}
			prepareEngineName := resolvePrepareEngine(runPrepareEngine, runEngine)
			if runFullPrepare {
				if err := prepare.RunPrepare(cmd.Context(), prepare.PrepareOpts{
					ProjectDir:       projectPath,
					EpicFilename:     filepath.Base(epicPath),
					Engine:           prepareEngineName,
					UserPrompt:       userPrompt,
					UserPromptSource: promptSource,
					SkipSanityCheck:  runNoSanityCheck || runDryRun,
					Mode:             mode,
					EffortLevel:      effortLevel,
					EnableReview:     runReview,
					Stdin:            os.Stdin,
					Stdout:           cmd.OutOrStdout(),
				}); err != nil {
					return err
				}
			} else {
				var err error
				triageDecision, err = runTriageGate(cmd.Context(), projectPath, epicPath, prepareEngineName, userPrompt, promptSource, effortLevel, mode, os.Stdin, cmd.OutOrStdout(), runNoSanityCheck || runDryRun, runTriageOnly)
				if err != nil {
					return err
				}
			}
		}

		if runTriageOnly {
			// When --no-sanity-check or --dry-run skipped the interactive
			// confirmation, the summary was never shown — display it now.
			// Otherwise ConfirmDecision already printed it.
			if triageDecision != nil && (runNoSanityCheck || runDryRun) {
				triage.DisplayTriageSummary(cmd.OutOrStdout(), triageDecision)
			}
			return nil
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
		// Apply model override — unconditional, takes precedence over @model in epic.
		if runModel != "" {
			ep.AgentModel = runModel
		}
		if err := epic.ValidateEpic(ep); err != nil {
			return err
		}

		// --always-verify: force verification, healing, and audit regardless of effort/complexity.
		if runAlwaysVerify {
			ep.AuditAfterSprint = true
			if ep.MaxHealAttempts == 0 {
				ep.MaxHealAttempts = config.DefaultMaxHealAttempts
			}
			ep.MaxHealAttemptsSet = true
			// Generate verification checks if none exist (simple tasks skip this).
			verifyPath := filepath.Join(projectPath, config.DefaultVerificationFile)
			if _, statErr := os.Stat(verifyPath); os.IsNotExist(statErr) {
				checks := triage.GenerateVerificationChecks(projectPath, ep.TotalSprints)
				if len(checks) > 0 {
					if writeErr := triage.WriteVerificationFile(verifyPath, checks); writeErr != nil {
						frlog.Log("WARNING: --always-verify: could not write verification file: %v", writeErr)
					} else {
						frlog.Log("  VERIFY: generated heuristic verification checks (--always-verify)")
					}
				} else {
					frlog.Log("WARNING: --always-verify: no recognized build system detected; skipping heuristic check generation")
				}
			}
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

		// Validate --continue and --simple-continue flag conflicts
		if runContinue && runSimpleContinue {
			return fmt.Errorf("cannot use --continue with --simple-continue; use one or the other")
		}
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
		if runSimpleContinue {
			if runSprint > 0 {
				return fmt.Errorf("cannot use --simple-continue with --sprint")
			}
			if len(args) > 1 {
				return fmt.Errorf("cannot use --simple-continue with positional sprint arguments")
			}
			if runResume {
				return fmt.Errorf("cannot use --simple-continue with --resume")
			}
		}

		var startSprint, endSprint int
		if runContinue || runSimpleContinue {
			frlog.Log("▶ CONTINUE  collecting build state...")
			state, collectErr := continuerun.CollectBuildState(cmd.Context(), projectPath, ep)
			if collectErr != nil {
				return fmt.Errorf("continue: %w", collectErr)
			}

			report := continuerun.FormatReport(state)
			fmt.Fprint(cmd.OutOrStdout(), report)
			fmt.Fprintln(cmd.OutOrStdout())

			var decision *continuerun.ContinueDecision
			if runSimpleContinue {
				frlog.Log("▶ CONTINUE  using heuristic continue (--simple-continue)...")
				decision = continuerun.HeuristicAnalyze(state)
			} else {
				continueOverride := ep.AuditModel
				if continueOverride == "" {
					continueOverride = ep.AgentModel
				}
				continueModel := engine.ResolveModel(continueOverride, engineName, string(ep.EffortLevel), engine.SessionContinue)
				var analyzeErr error
				decision, analyzeErr = continuerun.Analyze(cmd.Context(), continuerun.AnalyzeOpts{
					ProjectDir: projectPath,
					State:      state,
					Engine:     buildEngine,
					Model:      continueModel,
					Verbose:    frlog.Verbose,
				})
				if analyzeErr != nil {
					return fmt.Errorf("continue: %w", analyzeErr)
				}
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

		// Ensure git is initialised before strategy setup; branch/worktree
		// strategies require an existing repository.
		if err := git.InitGit(ctx, projectPath); err != nil {
			return err
		}

		// --- Git strategy setup ---
		if strategySetup == nil && gitStrategy != git.StrategyCurrent {
			// Resolve "auto" strategy
			if gitStrategy == git.StrategyAuto {
				if triageDecision != nil {
					if triageDecision.GitStrategyOverride != "" {
						// User explicitly chose a strategy during triage confirmation
						parsed, gsErr := git.ParseGitStrategy(triageDecision.GitStrategyOverride)
						if gsErr == nil {
							gitStrategy = parsed
						}
					}
					if gitStrategy == git.StrategyAuto {
						gitStrategy = git.ResolveAutoStrategy(string(triageDecision.Complexity))
					}
				} else {
					// No triage decision (epic already existed). Default to current for backwards compat.
					gitStrategy = git.StrategyCurrent
					if runBranchName != "" {
						frlog.Log("WARNING: --branch-name ignored because git strategy resolved to current (epic already exists; use --git-strategy branch to force)")
					}
				}
			}

			if gitStrategy != git.StrategyCurrent {
				branchName := runBranchName
				if branchName == "" {
					branchName = git.GenerateBranchName(ep.Name)
				}

				var setupErr error
				strategySetup, setupErr = git.SetupStrategy(ctx, git.StrategyOpts{
					ProjectDir: projectPath,
					Strategy:   gitStrategy,
					BranchName: branchName,
					EpicName:   ep.Name,
					ForceReuse: runContinue || runResume,
				})
				if setupErr != nil {
					return fmt.Errorf("git strategy setup: %w", setupErr)
				}

				projectPath = strategySetup.WorkDir
				originalProjectPath = strategySetup.OriginalDir

				// Re-resolve epicPath relative to redirected projectPath
				epicPath = filepath.Join(projectPath, epicArg)

				defer strategySetup.Cleanup()

				if persistErr := git.PersistStrategy(originalProjectPath, strategySetup); persistErr != nil {
					frlog.Log("WARNING: could not persist git strategy: %v", persistErr)
				}

				frlog.Log("  GIT: strategy=%s  branch=%s  workdir=%s", strategySetup.Strategy, strategySetup.BranchName, strategySetup.WorkDir)
			}
		} else if strategySetup != nil {
			// Already set up from --continue detection
			defer strategySetup.Cleanup()
		}

		if err := lock.AcquireIfNotDryRun(originalProjectPath, runDryRun); err != nil {
			return err
		}
		var lockOnce sync.Once
		releaseLock := func() {
			lockOnce.Do(func() {
				if err := lock.Release(originalProjectPath); err != nil {
					fmt.Fprintf(os.Stderr, "fry: warning: %v\n", err)
				}
			})
		}
		defer releaseLock()

		results := initializeSprintResults(ep, startSprint, endSprint)
		var mu sync.Mutex
		currentSprint := 0
		epicName := ep.Name // guarded by mu; updated after replan

		// Persist exit reason so `fry status` can show why the build stopped.
		defer func() {
			mu.Lock()
			lastSprint := currentSprint
			mu.Unlock()
			writeExitReason(projectPath, retErr, lastSprint)
		}()

		buildStart := time.Now()
		buildReport := report.BuildReport{
			EpicName:  ep.Name,
			StartTime: buildStart,
		}
		sprintReportResults := make([]report.SprintResult, 0, endSprint-startSprint+1)
		sprintTokens := make([]metrics.SprintTokens, 0, endSprint-startSprint+1)

		signalCh := make(chan os.Signal, 1)
		signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)
		defer signal.Stop(signalCh)

		interrupted := make(chan struct{})
		go func() {
			select {
			case <-ctx.Done():
				return
			case <-signalCh:
			}
			close(interrupted)
			cancel()
		}()

		wasInterrupted := func() bool {
			select {
			case <-interrupted:
				return true
			default:
				return false
			}
		}

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

		// Initialize observer metacognitive layer
		observerEnabled := !runNoObserver && !runDryRun && ep.EffortLevel != epic.EffortLow
		var observerEngine engine.Engine
		if observerEnabled {
			observerEngine, err = engine.NewEngine(engineName)
			if err != nil {
				frlog.Log("WARNING: observer: could not create engine: %v", err)
				observerEnabled = false
			}
		}
		if observerEnabled {
			if initErr := observer.InitBuild(projectPath, ep.Name, string(ep.EffortLevel), ep.TotalSprints); initErr != nil {
				frlog.Log("WARNING: observer: init failed: %v", initErr)
				observerEnabled = false
			}
		}

		exitErr := error(nil)
		for sprintNum := startSprint; sprintNum <= endSprint; sprintNum++ {
			if sprintNum < 1 || sprintNum > len(ep.Sprints) {
				return fmt.Errorf("sprint %d out of range [1, %d]", sprintNum, len(ep.Sprints))
			}
			spr := &ep.Sprints[sprintNum-1]

			mu.Lock()
			currentSprint = sprintNum
			mu.Unlock()

			if observerEnabled {
				_ = observer.EmitEvent(projectPath, observer.Event{
					Type:   observer.EventSprintStart,
					Sprint: spr.Number,
					Data:   map[string]string{"name": spr.Name},
				})
			}

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
			sprintStart := time.Now()
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
				if wasInterrupted() {
					mu.Lock()
					activeSprint := currentSprint
					name := epicName
					mu.Unlock()
					if activeSprint > 0 {
						_ = git.CommitPartialWork(ctx, projectPath, name, activeSprint)
					}
					exitErr = fmt.Errorf("interrupted by signal")
					break
				}
				return err
			}

			mu.Lock()
			results[sprintNum-startSprint] = *result
			mu.Unlock()

			if observerEnabled {
				_ = observer.EmitEvent(projectPath, observer.Event{
					Type:   observer.EventSprintComplete,
					Sprint: spr.Number,
					Data: map[string]string{
						"status":        result.Status,
						"duration":      result.Duration.Round(time.Second).String(),
						"heal_attempts": strconv.Itoa(result.HealAttempts),
					},
				})
				if result.HealAttempts > 0 {
					_ = observer.EmitEvent(projectPath, observer.Event{
						Type:   observer.EventHealComplete,
						Sprint: spr.Number,
						Data: map[string]string{
							"attempts": strconv.Itoa(result.HealAttempts),
							"status":   result.Status,
						},
					})
				}
			}

			// Collect per-sprint data for the build report and token summary.
			sprintEnd := time.Now()
			sprintPassed := isPassStatus(result.Status)
			sprintVerify := &report.VerificationResult{
				TotalChecks:  result.VerificationTotalCount,
				PassedChecks: result.VerificationPassCount,
				FailedChecks: result.VerificationTotalCount - result.VerificationPassCount,
			}
			for _, cr := range result.VerificationResults {
				sprintVerify.CheckResults = append(sprintVerify.CheckResults, report.CheckResult{
					Name:    cr.Check.Command + cr.Check.Path,
					Type:    cr.Check.Type.String(),
					Passed:  cr.Passed,
					Message: truncateString(cr.Output, 4096),
				})
			}
			if result.VerificationTotalCount == 0 {
				sprintVerify = nil
			}
			var sprintTokenUsage *metrics.TokenUsage
			if (runShowTokens || runJSONReport) && result.SprintLogPath != "" {
				if logData, readErr := os.ReadFile(result.SprintLogPath); readErr == nil {
					u := metrics.ParseTokens(engineName, string(logData))
					if u.Total > 0 {
						sprintTokenUsage = &u
					}
				}
			}
			reportSprint := report.SprintResult{
				SprintNum:    spr.Number,
				Name:         spr.Name,
				StartTime:    sprintStart,
				EndTime:      sprintEnd,
				Passed:       sprintPassed,
				HealAttempts: result.HealAttempts,
				Verification: sprintVerify,
				TokenUsage:   sprintTokenUsage,
			}
			sprintReportResults = append(sprintReportResults, reportSprint)
			if sprintTokenUsage != nil {
				sprintTokens = append(sprintTokens, metrics.SprintTokens{
					SprintNum: spr.Number,
					Usage:     *sprintTokenUsage,
				})
			}

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
				if ep.AuditAfterSprint && !runNoAudit && (ep.EffortLevel != epic.EffortLow || runAlwaysVerify) {
					auditEngine, err := resolveAuditEngine(engineName, ep.AuditEngine)
					if err != nil {
						return err
					}
					gitDiff, err := git.GitDiffForAudit(ctx, projectPath)
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
						DiffFn:     func() (string, error) { return git.GitDiffForAudit(ctx, projectPath) },
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
							failStatus := fmt.Sprintf("FAIL (audit: %s)", auditResult.MaxSeverity)
							mu.Lock()
							results[sprintNum-startSprint].Status = failStatus
							mu.Unlock()
							_ = sprint.AppendToEpicProgress(projectPath,
								fmt.Sprintf("## Sprint %d: %s \u2014 %s\n\n", spr.Number, spr.Name, failStatus))
							fmt.Fprintf(cmd.OutOrStdout(), "Resume:   fry run --resume --sprint %d\n", spr.Number)
							fmt.Fprintf(cmd.OutOrStdout(), "Restart:  fry run --sprint %d\n", spr.Number)
							fmt.Fprintf(cmd.OutOrStdout(), "Continue: fry run --continue\n")
							exitErr = fmt.Errorf("sprint %d audit failed: %s after %d cycles",
								spr.Number, failStatus, auditResult.Iterations)
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

					if observerEnabled {
						auditData := map[string]string{
							"passed":     strconv.FormatBool(auditResult.Passed),
							"iterations": strconv.Itoa(auditResult.Iterations),
						}
						if auditResult.MaxSeverity != "" {
							auditData["max_severity"] = auditResult.MaxSeverity
						}
						_ = observer.EmitEvent(projectPath, observer.Event{
							Type:   observer.EventAuditComplete,
							Sprint: spr.Number,
							Data:   auditData,
						})
					}
				}

				frlog.Log("  GIT: checkpoint — sprint %d complete", spr.Number)
				if err := git.GitCheckpoint(ctx, projectPath, ep.Name, spr.Number, "complete"); err != nil {
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
				if err := git.GitCheckpoint(ctx, projectPath, ep.Name, spr.Number, "compacted"); err != nil {
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

					if observerEnabled {
						_ = observer.EmitEvent(projectPath, observer.Event{
							Type:   observer.EventReviewComplete,
							Sprint: spr.Number,
							Data:   map[string]string{"verdict": string(reviewResult.Verdict)},
						})
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
						if runModel != "" {
							ep.AgentModel = runModel
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
						if err := git.GitCheckpoint(ctx, projectPath, ep.Name, spr.Number, "reviewed-deviate"); err != nil {
							return err
						}
					}
				}

				// Observer wake-up: after sprint
				if observerEnabled && observer.ShouldWakeUp(ep.EffortLevel, observer.WakeAfterSprint) {
					observerModel := engine.ResolveModel("", engineName, string(ep.EffortLevel), engine.SessionObserver)
					frlog.Log("  OBSERVER: wake-up after sprint %d...  model=%s", spr.Number, observerModel)
					if _, obsErr := observer.WakeUp(ctx, observer.ObserverOpts{
						ProjectDir:   projectPath,
						Engine:       observerEngine,
						Model:        observerModel,
						EpicName:     ep.Name,
						WakePoint:    observer.WakeAfterSprint,
						SprintNum:    spr.Number,
						TotalSprints: ep.TotalSprints,
						EffortLevel:  ep.EffortLevel,
						Verbose:      frlog.Verbose,
					}); obsErr != nil {
						frlog.Log("  OBSERVER: wake-up failed (non-fatal): %v", obsErr)
					}
				}

				continue
			}

			_ = sprint.AppendToEpicProgress(projectPath,
				fmt.Sprintf("## Sprint %d: %s \u2014 %s\n\n", spr.Number, spr.Name, result.Status))
			fmt.Fprintf(cmd.OutOrStdout(), "Resume:   fry run --resume --sprint %d\n", spr.Number)
			fmt.Fprintf(cmd.OutOrStdout(), "Restart:  fry run --sprint %d\n", spr.Number)
			fmt.Fprintf(cmd.OutOrStdout(), "Continue: fry run --continue\n")
			exitErr = fmt.Errorf("sprint %d failed: %s", spr.Number, result.Status)
			break
		}

		mu.Lock()
		summaryCopy := append([]sprint.SprintResult(nil), results...)
		mu.Unlock()
		if ep.EffortLevel != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n", color.CyanText("Effort level:"), ep.EffortLevel)
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

		// Run final build audit once the entire epic has completed successfully.
		// This runs BEFORE the build summary so that audit results can be included in the summary.
		var buildAuditResult *audit.AuditResult
		if exitErr == nil && startSprint == 1 && endSprint == ep.TotalSprints && ep.AuditAfterSprint && !runNoAudit && (ep.EffortLevel != epic.EffortLow || runAlwaysVerify) {
			deferredContent := readDeferredFailuresArtifact(projectPath)
			auditEngine, err := resolveAuditEngine(engineName, ep.AuditEngine)
			if err != nil {
				frlog.Log("WARNING: could not create engine for build audit: %v", err)
			} else {
				buildAuditModel := engine.ResolveModel(ep.AuditModel, auditEngine.Name(), string(ep.EffortLevel), engine.SessionBuildAudit)
				frlog.Log("▶ BUILD AUDIT  running holistic audit across all %d sprints...  engine=%s  model=%s", ep.TotalSprints, auditEngine.Name(), buildAuditModel)
				result, auditErr := audit.RunBuildAudit(ctx, audit.BuildAuditOpts{
					ProjectDir:       projectPath,
					Epic:             ep,
					Engine:           auditEngine,
					Results:          summaryCopy,
					Verbose:          frlog.Verbose,
					Model:            buildAuditModel,
					DeferredFailures: deferredContent,
					Mode:             string(mode),
				})
				if auditErr != nil {
					frlog.Log("WARNING: build audit failed: %v", auditErr)
				} else {
					buildAuditResult = result
					if result.Passed {
						frlog.Log("  BUILD AUDIT: PASS (%s)", audit.FormatCounts(result.SeverityCounts))
					} else if result.Blocking {
						frlog.Log("  BUILD AUDIT: FAILED — %s remain", audit.FormatCounts(result.SeverityCounts))
					} else {
						frlog.Log("  BUILD AUDIT: %s remain (advisory)", audit.FormatCounts(result.SeverityCounts))
					}
					frlog.Log("  GIT: checkpoint — build-audit")
					if gitErr := git.GitCheckpoint(ctx, projectPath, ep.Name, ep.TotalSprints, "build-audit"); gitErr != nil {
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

		// Run a single-pass build audit for triaged tasks that skipped the main
		// build audit gate (e.g. simple+low, moderate+low where AuditAfterSprint=false).
		if exitErr == nil && buildAuditResult == nil && !runNoAudit && triageDecision != nil {
			auditEngine, auditEngErr := engine.NewEngine(engineName)
			if auditEngErr != nil {
				frlog.Log("WARNING: could not create engine for triage build audit: %v", auditEngErr)
			} else {
				buildAuditModel := engine.ResolveModelForSession(engineName, string(ep.EffortLevel), engine.SessionBuildAudit)
				frlog.Log("▶ BUILD AUDIT  single-pass audit for triaged task...  engine=%s  model=%s", engineName, buildAuditModel)
				result, auditErr := audit.RunBuildAudit(ctx, audit.BuildAuditOpts{
					ProjectDir: projectPath,
					Epic:       ep,
					Engine:     auditEngine,
					Results:    summaryCopy,
					Verbose:    frlog.Verbose,
					Model:      buildAuditModel,
					Mode:       string(mode),
				})
				if auditErr != nil {
					frlog.Log("WARNING: triage build audit failed: %v", auditErr)
				} else {
					buildAuditResult = result
					if result.Passed {
						frlog.Log("  BUILD AUDIT: PASS (%s)", audit.FormatCounts(result.SeverityCounts))
					} else {
						frlog.Log("  BUILD AUDIT: %s (advisory)", audit.FormatCounts(result.SeverityCounts))
					}
					if gitErr := git.GitCheckpoint(ctx, projectPath, ep.Name, ep.TotalSprints, "build-audit"); gitErr != nil {
						frlog.Log("WARNING: git checkpoint after build audit failed: %v", gitErr)
					}
				}
			}
		}

		// Observer: build audit event and wake-up
		if observerEnabled {
			buildAuditData := map[string]string{"ran": strconv.FormatBool(buildAuditResult != nil)}
			if buildAuditResult != nil {
				buildAuditData["passed"] = strconv.FormatBool(buildAuditResult.Passed)
				if buildAuditResult.MaxSeverity != "" {
					buildAuditData["max_severity"] = buildAuditResult.MaxSeverity
				}
			}
			_ = observer.EmitEvent(projectPath, observer.Event{
				Type: observer.EventBuildAuditDone,
				Data: buildAuditData,
			})

			if observer.ShouldWakeUp(ep.EffortLevel, observer.WakeAfterBuildAudit) {
				observerModel := engine.ResolveModel("", engineName, string(ep.EffortLevel), engine.SessionObserver)
				frlog.Log("  OBSERVER: wake-up after build audit...  model=%s", observerModel)
				if _, obsErr := observer.WakeUp(ctx, observer.ObserverOpts{
					ProjectDir:   projectPath,
					Engine:       observerEngine,
					Model:        observerModel,
					EpicName:     ep.Name,
					WakePoint:    observer.WakeAfterBuildAudit,
					TotalSprints: ep.TotalSprints,
					EffortLevel:  ep.EffortLevel,
					Verbose:      frlog.Verbose,
					BuildData:    buildAuditData,
				}); obsErr != nil {
					frlog.Log("  OBSERVER: wake-up failed (non-fatal): %v", obsErr)
				}
			}
		}

		// Generate build summary document (after build audit so results can be included)
		summaryEngine, err := engine.NewEngine(engineName)
		if err != nil {
			frlog.Log("WARNING: could not create engine for build summary: %v", err)
		} else {
			summaryModel := engine.ResolveModel(ep.AgentModel, engineName, string(ep.EffortLevel), engine.SessionBuildSummary)
			frlog.Log("▶ BUILD SUMMARY  generating...  engine=%s  model=%s", engineName, summaryModel)
			if summaryErr := summary.GenerateBuildSummary(ctx, summary.SummaryOpts{
				ProjectDir:       projectPath,
				EpicName:         ep.Name,
				Engine:           summaryEngine,
				Results:          summaryCopy,
				Verbose:          frlog.Verbose,
				Model:            summaryModel,
				BuildAuditResult: buildAuditResult,
			}); summaryErr != nil {
				frlog.Log("WARNING: build summary generation failed: %v", summaryErr)
			} else {
				frlog.Log("  BUILD SUMMARY: complete")
			}
		}

		// Observer: final wake-up at build end
		if observerEnabled {
			buildOutcome := "success"
			if exitErr != nil {
				buildOutcome = "failure"
			}
			_ = observer.EmitEvent(projectPath, observer.Event{
				Type: observer.EventBuildEnd,
				Data: map[string]string{"outcome": buildOutcome},
			})

			if observer.ShouldWakeUp(ep.EffortLevel, observer.WakeBuildEnd) {
				observerModel := engine.ResolveModel("", engineName, string(ep.EffortLevel), engine.SessionObserver)
				frlog.Log("  OBSERVER: final wake-up...  model=%s", observerModel)
				if _, obsErr := observer.WakeUp(ctx, observer.ObserverOpts{
					ProjectDir:   projectPath,
					Engine:       observerEngine,
					Model:        observerModel,
					EpicName:     ep.Name,
					WakePoint:    observer.WakeBuildEnd,
					TotalSprints: ep.TotalSprints,
					EffortLevel:  ep.EffortLevel,
					Verbose:      frlog.Verbose,
					BuildData:    map[string]string{"outcome": buildOutcome},
				}); obsErr != nil {
					frlog.Log("  OBSERVER: final wake-up failed (non-fatal): %v", obsErr)
				}
			}
		}

		releaseLock()

		// Write JSON build report if --json-report flag is set.
		if runJSONReport {
			buildEnd := time.Now()
			buildReport.EndTime = buildEnd
			buildReport.Duration = buildEnd.Sub(buildStart)
			buildReport.Sprints = sprintReportResults
			reportPath := filepath.Join(projectPath, config.BuildReportFile)
			if writeErr := report.Write(reportPath, buildReport); writeErr != nil {
				frlog.Log("WARNING: could not write JSON build report: %v", writeErr)
			} else {
				frlog.Log("  BUILD REPORT: written to %s", config.BuildReportFile)
			}
		}

		// Write SARIF report if --sarif flag is set and a build audit was run.
		if runSARIF && buildAuditResult != nil {
			sarifData, sarifErr := audit.ConvertToSARIF(buildAuditResult.UnresolvedFindings)
			if sarifErr != nil {
				frlog.Log("WARNING: could not generate SARIF report: %v", sarifErr)
			} else {
				sarifPath := filepath.Join(projectPath, config.BuildAuditSARIFFile)
				if writeErr := os.WriteFile(sarifPath, sarifData, 0o644); writeErr != nil {
					frlog.Log("WARNING: could not write SARIF report: %v", writeErr)
				} else {
					frlog.Log("  SARIF: build-audit.sarif written (%d findings)", len(buildAuditResult.UnresolvedFindings))
				}
			}
		}

		// Print per-sprint token summary to stderr if --show-tokens is set.
		if runShowTokens && len(sprintTokens) > 0 {
			tw := tabwriter.NewWriter(os.Stderr, 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "Sprint\tInput Tokens\tOutput Tokens\tTotal")
			fmt.Fprintln(tw, "------\t------------\t-------------\t-----")
			var totalIn, totalOut int
			for _, st := range sprintTokens {
				if st.Usage.Total == 0 && strings.EqualFold(engineName, "ollama") {
					fmt.Fprintf(tw, "%d\t-\t-\t-\n", st.SprintNum)
				} else {
					fmt.Fprintf(tw, "%d\t%d\t%d\t%d\n", st.SprintNum, st.Usage.Input, st.Usage.Output, st.Usage.Total)
				}
				totalIn += st.Usage.Input
				totalOut += st.Usage.Output
			}
			if strings.EqualFold(engineName, "ollama") && totalIn == 0 && totalOut == 0 {
				fmt.Fprintln(tw, "TOTAL\t-\t-\t-")
			} else {
				fmt.Fprintf(tw, "TOTAL\t%d\t%d\t%d\n", totalIn, totalOut, totalIn+totalOut)
			}
			_ = tw.Flush()
		}

		if exitErr == nil && startSprint == 1 && endSprint == ep.TotalSprints {
			archivePath, archiveErr := archive.Archive(projectPath)
			if archiveErr != nil {
				fmt.Fprintf(os.Stderr, "fry: warning: auto-archive failed: %v\n", archiveErr)
			} else {
				frlog.Log("  ARCHIVE  build artifacts archived to %s", archivePath)
			}
		}

		return exitErr
	},
}

func init() {
	runCmd.Flags().StringVar(&runEngine, "engine", "", "Execution engine")
	runCmd.Flags().BoolVar(&runDryRun, "dry-run", false, "Preview actions without executing")
	runCmd.Flags().StringVar(&runUserPrompt, "user-prompt", "", "Additional user prompt")
	runCmd.Flags().StringVar(&runUserPromptFile, "user-prompt-file", "", "Path to file containing user prompt")
	runCmd.Flags().BoolVar(&runReview, "review", false, "Enable sprint review between sprints")
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
	runCmd.Flags().BoolVar(&runNoSanityCheck, "no-sanity-check", false, "Skip interactive confirmations (triage classification and project summary)")
	runCmd.Flags().BoolVar(&runFullPrepare, "full-prepare", false, "Skip triage and run full prepare pipeline when no epic exists")
	runCmd.Flags().StringVar(&runGitStrategy, "git-strategy", "", "Git branching strategy: auto, current, branch, worktree (default: auto)")
	runCmd.Flags().StringVar(&runBranchName, "branch-name", "", "Git branch name (auto-generated from epic name if not specified)")
	runCmd.Flags().BoolVar(&runAlwaysVerify, "always-verify", false, "Force verification, healing, and audit to run regardless of effort level or triage complexity")
	runCmd.Flags().BoolVar(&runSimpleContinue, "simple-continue", false, "Resume from first incomplete sprint without LLM analysis (lightweight alternative to --continue)")
	runCmd.Flags().BoolVar(&runSARIF, "sarif", false, "Write build-audit.sarif in SARIF 2.1.0 format alongside build-audit.md")
	runCmd.Flags().BoolVar(&runJSONReport, "json-report", false, "Write build-report.json with structured sprint results")
	runCmd.Flags().BoolVar(&runShowTokens, "show-tokens", false, "Print per-sprint token usage summary to stderr after the run")
	runCmd.Flags().BoolVar(&runNoObserver, "no-observer", false, "Disable the observer metacognitive layer")
	runCmd.Flags().BoolVar(&runTriageOnly, "triage-only", false, "Run triage classification and exit without generating artifacts")
	runCmd.Flags().StringVar(&runModel, "model", "", "Override agent model for sprints (e.g. opus[1m], sonnet, haiku)")
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

// userPromptSource returns a human-readable description of where the user prompt
// was loaded from, for use in log messages. Returns empty string if no prompt.
func userPromptSource(prompt, flagValue, fileFlagValue string) string {
	if strings.TrimSpace(prompt) == "" {
		return ""
	}
	if strings.TrimSpace(fileFlagValue) != "" {
		return "--user-prompt-file " + fileFlagValue
	}
	if strings.TrimSpace(flagValue) != "" {
		return "--user-prompt flag"
	}
	return config.UserPromptFile
}

// checkArgsForMissingDashes detects positional arguments that look like flag names
// without the leading "--". This catches common mistakes like
// "fry run user-prompt-file ./file.md" instead of "fry run --user-prompt-file ./file.md".
func checkArgsForMissingDashes(cmd *cobra.Command, args []string) error {
	for _, arg := range args {
		// Skip args that start with dashes (already a flag) or look like file paths.
		if strings.HasPrefix(arg, "-") || strings.HasPrefix(arg, "/") || strings.HasPrefix(arg, ".") {
			continue
		}
		// Check if this arg matches a known flag name on this command or its parent.
		if f := cmd.Flags().Lookup(arg); f != nil {
			return fmt.Errorf("%q looks like a flag — did you mean --%s?", arg, arg)
		}
		if cmd.HasParent() {
			if f := cmd.Parent().Flags().Lookup(arg); f != nil {
				return fmt.Errorf("%q looks like a flag — did you mean --%s?", arg, arg)
			}
		}
	}
	return nil
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
	fmt.Fprintf(w, "%s %s\n", color.CyanText("Epic:"), ep.Name)
	fmt.Fprintf(w, "%s %s\n", color.CyanText("Project dir:"), projectDir)
	fmt.Fprintf(w, "%s %s\n", color.CyanText("Epic file:"), epicPath)
	fmt.Fprintf(w, "%s %s\n", color.CyanText("Engine:"), engineName)
	fmt.Fprintf(w, "%s %s\n", color.CyanText("Effort:"), ep.EffortLevel)
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
		fmt.Fprintf(tw, "%d\t%s\t%s\t%s\n", result.Number, result.Name, colorizeStatus(result.Status), result.Duration.Round(1e9))
	}
	_ = tw.Flush()

	// Print warnings after the table
	for _, result := range results {
		if result.AuditWarning != "" {
			fmt.Fprintf(w, "  %s Sprint %d: %s\n", color.YellowText("⚠"), result.Number, result.AuditWarning)
		}
		if len(result.DeferredFailures) > 0 {
			fmt.Fprintf(w, "  %s Sprint %d: %d deferred verification failures\n", color.YellowText("⚠"), result.Number, len(result.DeferredFailures))
		}
	}
}

func colorizeStatus(status string) string {
	if strings.HasPrefix(status, "PASS") {
		return color.GreenText(status)
	}
	if strings.HasPrefix(status, "FAIL") {
		return color.RedText(status)
	}
	if status == sprint.StatusSkipped {
		return color.YellowText(status)
	}
	return status
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

// writeExitReason persists the reason the build stopped to .fry/build-exit-reason.txt.
// On success (nil error), the file is removed so status doesn't show stale reasons.
func writeExitReason(projectDir string, buildErr error, sprintNum int) {
	path := filepath.Join(projectDir, config.BuildExitReasonFile)
	if buildErr == nil {
		os.Remove(path)
		return
	}
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	reason := buildErr.Error()
	if sprintNum > 0 {
		reason = fmt.Sprintf("After sprint %d: %s", sprintNum, reason)
	}
	_ = os.WriteFile(path, []byte(reason), 0o644)
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

// runTriageGate classifies task complexity and takes the appropriate execution path.
// For SIMPLE tasks, it builds the epic programmatically. For MODERATE, it builds
// a programmatic epic with auto-generated verification. For COMPLEX, it falls
// through to full prepare.
func runTriageGate(ctx context.Context, projectPath, epicPath, prepareEngineName, userPrompt, promptSource string, effortLevel epic.EffortLevel, mode prepare.Mode, stdin io.Reader, stdout io.Writer, skipConfirm bool, triageOnly bool) (*triage.TriageDecision, error) {
	// Read available inputs.
	planPath := filepath.Join(projectPath, config.PlanFile)
	executivePath := filepath.Join(projectPath, config.ExecutiveFile)

	planContent, _ := readOptionalFile(planPath)
	execContent, _ := readOptionalFile(executivePath)

	// Validate that we have at least some input.
	if strings.TrimSpace(planContent) == "" && strings.TrimSpace(execContent) == "" && strings.TrimSpace(userPrompt) == "" {
		return nil, fmt.Errorf("prepare requires %s, %s, or --user-prompt", config.PlanFile, config.ExecutiveFile)
	}

	// Resolve engine for triage.
	engName, err := engine.ResolveEngine(prepareEngineName, "", "", config.DefaultPrepareEngine)
	if err != nil {
		return nil, fmt.Errorf("triage: resolve engine: %w", err)
	}
	eng, err := engine.NewEngine(engName)
	if err != nil {
		return nil, fmt.Errorf("triage: create engine: %w", err)
	}
	triageModel := engine.ResolveModelForSession(engName, string(effortLevel), engine.SessionTriage)

	decision := triage.Classify(ctx, triage.TriageOpts{
		ProjectDir:  projectPath,
		UserPrompt:  userPrompt,
		PlanContent: planContent,
		ExecContent: execContent,
		Engine:      eng,
		Model:       triageModel,
		Mode:        mode,
		Verbose:     frlog.Verbose,
	})

	// Interactive confirmation (unless skipped).
	if !skipConfirm {
		result, confirmErr := triage.ConfirmDecision(triage.ConfirmOpts{
			Decision: decision,
			Stdin:    stdin,
			Stdout:   stdout,
		})
		if confirmErr != nil {
			return nil, confirmErr
		}
		decision.Complexity = result.Complexity
		if result.EffortLevel != "" {
			decision.EffortLevel = result.EffortLevel
		}
		if result.GitStrategy != "" {
			decision.GitStrategyOverride = result.GitStrategy
		}
	}

	if triageOnly {
		return decision, nil
	}

	// Resolve effort: CLI flag > triage suggestion > default per difficulty.
	resolvedEffort := effortLevel
	if resolvedEffort == "" {
		resolvedEffort = decision.EffortLevel
	}

	// Non-software modes (planning, writing) always need the full prepare pipeline
	// because programmatic epic builders only produce software-mode epics.
	if mode != prepare.ModeSoftware && mode != "" {
		decision.Complexity = triage.ComplexityComplex
		frlog.Log("  TRIAGE: non-software mode (%s) — routing to full prepare pipeline", mode)
	}

	switch decision.Complexity {
	case triage.ComplexitySimple:
		// Cap max → high for simple tasks.
		if resolvedEffort == epic.EffortMax {
			frlog.Log("  TRIAGE: --effort max capped to high for simple task")
			resolvedEffort = epic.EffortHigh
		}
		if resolvedEffort == "" {
			resolvedEffort = epic.EffortLow
		}

		ep, buildErr := triage.BuildSimpleEpic(triage.SimpleEpicOpts{
			ProjectDir:  projectPath,
			UserPrompt:  userPrompt,
			PlanContent: planContent,
			ExecContent: execContent,
			EngineName:  engName,
			EffortLevel: resolvedEffort,
		})
		if buildErr != nil {
			return nil, fmt.Errorf("triage simple path: %w", buildErr)
		}
		if err := triage.WriteEpicFile(epicPath, ep); err != nil {
			return nil, fmt.Errorf("triage simple path: %w", err)
		}
		frlog.Log("  TRIAGE: built 1-sprint epic programmatically (no LLM prepare)  effort=%s", resolvedEffort)

	case triage.ComplexityModerate:
		// Cap max → high for moderate tasks.
		if resolvedEffort == epic.EffortMax {
			frlog.Log("  TRIAGE: --effort max capped to high for moderate task")
			resolvedEffort = epic.EffortHigh
		}
		if resolvedEffort == "" {
			resolvedEffort = epic.EffortMedium
		}

		ep, buildErr := triage.BuildModerateEpic(triage.ModerateEpicOpts{
			ProjectDir:  projectPath,
			UserPrompt:  userPrompt,
			PlanContent: planContent,
			ExecContent: execContent,
			EngineName:  engName,
			EffortLevel: resolvedEffort,
			SprintCount: decision.SprintCount,
		})
		if buildErr != nil {
			return nil, fmt.Errorf("triage moderate path: %w", buildErr)
		}
		if err := triage.WriteEpicFile(epicPath, ep); err != nil {
			return nil, fmt.Errorf("triage moderate path: %w", err)
		}

		// Generate heuristic verification checks.
		verifyPath := filepath.Join(projectPath, config.DefaultVerificationFile)
		checks := triage.GenerateVerificationChecks(projectPath, ep.TotalSprints)
		if err := triage.WriteVerificationFile(verifyPath, checks); err != nil {
			frlog.Log("WARNING: triage moderate path: could not write verification file: %v", err)
		}

		frlog.Log("  TRIAGE: built %d-sprint moderate epic programmatically (no LLM prepare)  effort=%s", ep.TotalSprints, resolvedEffort)

	case triage.ComplexityComplex:
		// Complex tasks default to high effort for thorough audit cycles.
		// Only auto-elevate when user didn't explicitly set --effort.
		complexEffort := resolvedEffort
		if effortLevel == "" && (complexEffort == "" || complexEffort == epic.EffortLow || complexEffort == epic.EffortMedium) {
			complexEffort = epic.EffortHigh
			frlog.Log("  TRIAGE: complex task auto-elevated to effort=high")
		}
		resolvedEffort = complexEffort

		frlog.Log("  TRIAGE: task classified as complex — running full prepare pipeline")
		if err := prepare.RunPrepare(ctx, prepare.PrepareOpts{
			ProjectDir:       projectPath,
			EpicFilename:     filepath.Base(epicPath),
			Engine:           prepareEngineName,
			UserPrompt:       userPrompt,
			UserPromptSource: promptSource,
			SkipSanityCheck:  runNoSanityCheck || runDryRun,
			Mode:             mode,
			EffortLevel:      complexEffort,
			EnableReview:     runReview,
			Stdin:            stdin,
			Stdout:           stdout,
		}); err != nil {
			return nil, err
		}
	}

	return decision, nil
}

// truncateString returns s truncated to maxBytes bytes, appending a notice if truncated.
func truncateString(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	return s[:maxBytes] + "... [truncated]"
}

func readOptionalFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
