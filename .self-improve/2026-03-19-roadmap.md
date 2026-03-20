# Fry Self-Improvement Roadmap

> **Purpose:** Actionable improvements to fry, organized by category. Execute one at a time in future sessions.
> **Generated:** 2026-03-19

---

## Status Key

- [ ] Not started
- [x] Complete

---

## A. Bug Fixes

### A1. Signal handler calls `os.Exit(130)` directly

- **File:** `internal/cli/run.go:297`
- **Severity:** High
- **Description:** The signal handler goroutine calls `os.Exit(130)` instead of returning an error through the normal path. This bypasses all deferred cleanup — lock release errors are silently swallowed (line 296), open file handles aren't flushed, and the sprint log can be left truncated mid-write.
- **Fix:** Have the signal handler send an error through a channel that the main loop reads, so all deferred cleanup runs normally. Replace `os.Exit(130)` with a cancellation signal that propagates through the existing `context.Context`.
- **Status:** [x] Completed 2026-03-19. Replaced `os.Exit(130)` with channel-based interruption. Signal handler closes `interrupted` channel and cancels context. Main goroutine detects interruption, commits partial work, and falls through to normal summary/cleanup via deferred `releaseLock()`.

### A2. Missing `fsync` on dual-log writes

- **Files:** `internal/sprint/runner.go:468`, `internal/heal/heal.go:403`
- **Severity:** Medium
- **Description:** Both `runAgentWithDualLogs()` functions write iteration bytes to `sprintLog` without calling `Sync()`. If the process crashes between `Write()` and the deferred `Close()`, the sprint log will be truncated or corrupted. The code comments claim writes are unbuffered, but Go's `os.File.Write()` does not guarantee durability without an explicit `fsync`.
- **Fix:** Add `sprintLog.Sync()` after each `sprintLog.Write(iterBytes)` call. Also verify that `len(written) == len(iterBytes)` to catch partial writes.
- **Status:** [x] Completed 2026-03-19. Added partial-write detection (`n != len(iterBytes)`) and `sprintLog.Sync()` after each Write in both `internal/sprint/runner.go` and `internal/heal/heal.go`. Also removed the inaccurate "Go file writes are unbuffered" comment.

### A3. Unchecked error on lock release in signal handler

- **File:** `internal/cli/run.go:296`
- **Severity:** Medium
- **Description:** The signal handler calls `releaseLock()` (which wraps `lock.Release()` via `sync.Once`) and silently discards any error. If the lock file can't be removed (permissions, already deleted by race), subsequent `fry run` invocations will think the lock is held and refuse to start.
- **Fix:** Log the error to stderr if `lock.Release()` fails in the signal handler. Even though we're about to exit, the user needs to know the lock file may be stale.
- **Status:** [ ]

### A4. Partial write not detected on sprint log append

- **Files:** `internal/sprint/runner.go:468`, `internal/heal/heal.go:403`
- **Severity:** Low
- **Description:** `sprintLog.Write(iterBytes)` checks `err != nil` but does not verify that the number of bytes written equals `len(iterBytes)`. A partial write returns `nil` error but fewer bytes, causing silent log truncation.
- **Fix:** Check `n, err := sprintLog.Write(iterBytes)` and verify `n == len(iterBytes)`.
- **Status:** [x] Completed 2026-03-19 (fixed as part of A2).

### A5. Sprint index bounds validation gap

- **File:** `internal/cli/run.go:335-339`
- **Severity:** Low
- **Description:** The sprint loop trusts `resolveSprintRange()` to return valid bounds, but there's no redundant check immediately before the array access `ep.Sprints[sprintNum-1]`. If `resolveSprintRange()` has a logic error or sprints are mutated between validation and access, this panics.
- **Fix:** Add a defensive bounds check immediately before the array access, or add an assertion in `resolveSprintRange()` that guarantees `endSprint <= len(sprints)`.
- **Status:** [ ]

---

## B. Testing Improvements

### B1. Fix 13 tests missing `t.Parallel()`

- **Severity:** High (violates project convention in CLAUDE.md)
- **Description:** 13 test functions skip `t.Parallel()` because they mutate package-level variables. This violates CLAUDE.md and masks potential race conditions.
- **Affected files and root causes:**

| File | Tests Missing `t.Parallel()` | Root Cause |
|------|------------------------------|------------|
| `internal/docker/docker_test.go` | `TestDetectComposeCommand` (line 18), `TestDetectComposeCommand_FallbackToDockerCompose` (line 45), `TestDetectComposeCommand_NeitherAvailable` (line 79) | Mutates package-level `lookPath` and `execCommandContext` function pointers |
| `internal/prepare/prepare_test.go` | `TestBootstrapExecutive_UserApproves` (line 126), `TestBootstrapExecutive_UserDeclines` (line 276), `TestBootstrapExecutive_LogsAssetsAndMedia` (line 303), `TestBootstrapExecutive_LogsPromptOnlyWhenNoExtras` (line 335), `TestRunPrepare_SanityCheckSkipped` (line 582), `TestRunPrepare_PlanExistsWithoutExecutive` (line 610), `TestRunPrepare_LogsExistingFiles` (line 643), `TestRunPrepare_LogsUserPromptAndAssets` (line 674) | Mutates package-level `newEngine` variable |
| `internal/cli/integration_test.go` | `TestEngineResolution` (line 81) | Uses `t.Setenv()` which is incompatible with `t.Parallel()` |
| `internal/log/log_test.go` | `TestSetLogFile` (line 14), `TestLog_NilLogFile` (line 26), `TestAgentBanner_DefaultModel` (line 34), `TestAgentBanner_CustomModel` (line 50) | Mutates package-level `logFile` variable |

- **Fix:** Refactor these packages to accept dependencies as function parameters (dependency injection) instead of swapping package-level globals. This eliminates the race condition root cause and allows `t.Parallel()`. For example, `docker.go` should accept a `lookPathFunc` parameter rather than overwriting a package variable in tests.
- **Status:** [x] Completed 2026-03-19. All 13 tests now call `t.Parallel()`. Refactored:
  - **docker**: Extracted `dockerDeps` struct; public functions delegate to unexported versions accepting deps; tests use their own deps instances.
  - **prepare**: Removed package-level `newEngine` var; added `EngineFactory` and `LogFunc` fields to `PrepareOpts`; `RunPrepare` and `bootstrapExecutive` use injected functions; tests pass their own factories and log capture functions.
  - **log**: Created `Logger` struct with `SetLogFile`, `Log`, `AgentBanner` methods; package-level functions delegate to `defaultLogger`; tests create own `Logger` instances.
  - **cli/integration_test.go**: Rewrote `TestEngineResolution` to pass env value through `ResolveEngine`'s `envVar` parameter instead of using `t.Setenv`.

### B2. Add unit tests for untested helper functions in `continuerun`

- **Severity:** Medium
- **File:** `internal/continuerun/collector.go`
- **Description:** Several helper functions used in build state analysis have zero dedicated unit tests:

| Function | Lines | Purpose |
|----------|-------|---------|
| `checkDockerAvailable` | ~5 | Environment prerequisite check |
| `checkRequiredTools` | ~7 | Environment prerequisite check |
| `countDeviations` | ~7 | Build state string counting |
| `sevRank` | ~12 | Severity ranking logic |
| `extractMaxSeverity` | ~21 | Regex-based severity extraction |
| `sprintProgressMentionsSprint` | ~9 | File content scanning |

- **Fix:** Add a `collector_helpers_test.go` with table-driven tests for each function, covering happy paths, edge cases (empty input, malformed strings), and error paths.
- **Status:** [x] Completed 2026-03-19. Added `collector_helpers_test.go` with table-driven tests for `sevRank` (9 cases), `extractMaxSeverity` edge cases (9 cases), `countDeviations` (4 cases), `sprintProgressMentionsSprint` (5 cases), and `checkRequiredTools` (4 cases). Skipped `checkDockerAvailable` since it calls `docker info` and would be flaky in environments without Docker.

### B3. Expand error path coverage in engine tests

- **Severity:** Medium
- **File:** `internal/engine/engine_test.go`
- **Description:** Engine tests cover model resolution and basic invocation but don't test: process timeout handling, missing binary errors, exit code extraction from different error types, or stderr capture on failure.
- **Fix:** Add test cases for: engine binary not found on PATH, process killed by signal, non-zero exit with meaningful stderr, context cancellation mid-execution.
- **Status:** [ ]

### B4. Add verification runner edge case tests

- **Severity:** Low
- **File:** `internal/verify/runner_test.go`
- **Description:** Current tests (28 functions) cover standard check types well but miss: command timeout behavior (120s limit), large output truncation (10MB cap), regex pattern edge cases (special characters, empty patterns), and interaction between check types.
- **Fix:** Add test cases for timeout expiry, output exceeding 10MB, pathological regex patterns, and multi-check verification runs with mixed pass/fail.
- **Status:** [ ]

---

## C. New Features

### C1. Add Ollama engine (local model support)

- **Impact:** High — unlocks local/private model usage with zero API costs
- **Effort:** Medium (~2 hours)
- **Description:** Fry currently only supports Claude and Codex. Adding Ollama would allow users to run builds with local open-source models (Llama, Mistral, CodeLlama, etc.) without any API keys or cloud dependencies.
- **Implementation:**
  1. Create `internal/engine/ollama.go` implementing the `Engine` interface
  2. Shell out to `ollama run <model>` with the prompt via stdin or temp file
  3. Add `"ollama"` case to `NewEngine()` switch in `engine.go:54-63`
  4. Add `"ollama"` to `ResolveEngine()` validation in `engine.go:46-51`
  5. Add Ollama model tier mappings in `models.go`
  6. Update `docs/engines.md` with Ollama section
  7. Update `README.md` engines table
  8. Update `README.LLM.md` engine list
  9. Add tests in `internal/engine/ollama_test.go`
- **Status:** [ ]

### C2. Structured JSON build reports

- **Impact:** High — enables CI/CD integration, dashboards, trend analysis
- **Effort:** Low-Medium (~1.5 hours)
- **Description:** All build output is currently unstructured terminal text. A `--json-report` flag would emit a `build-report.json` containing sprint results, verification outcomes, heal attempt counts, audit findings, and timing data. The data already exists scattered across `.fry/` files.
- **Implementation:**
  1. Create `internal/report/report.go` with `BuildReport` struct
  2. Define nested types: `SprintResult`, `VerificationResult`, `HealSummary`, `AuditSummary`
  3. Aggregate data at the end of `executeRun()` in `internal/cli/run.go`
  4. Marshal to JSON and write to `build-report.json`
  5. Add `--json-report` flag to `runCmd`
  6. Add `build-report.json` to `.gitignore`
  7. Update `docs/commands.md` with flag documentation
  8. Add tests in `internal/report/report_test.go`
- **Status:** [ ]

### C3. `@check_test` verification primitive

- **Impact:** High — native test framework integration
- **Effort:** Medium (~2 hours)
- **Description:** Currently users must use `@check_cmd` with raw shell commands to run tests. A dedicated `@check_test` primitive would understand test frameworks (Go, npm/jest, pytest), parse pass/fail/skip counts, and report structured results.
- **Implementation:**
  1. Add `CheckTest` to the check type enum in `internal/verify/types.go`
  2. Add parsing for `@check_test` in `internal/verify/parser.go`
  3. Add execution logic in `internal/verify/runner.go` — run command, parse output for test counts
  4. Detect framework from command prefix (`go test`, `npm test`, `pytest`)
  5. Report: tests run, passed, failed, skipped, coverage % (if available)
  6. Update `docs/verification.md` with `@check_test` documentation
  7. Update `templates/verification-example.md`
  8. Add tests in `internal/verify/runner_test.go`
- **Syntax:**
  ```
  @check_test go test ./... --timeout 30s
  @check_test npm test -- --coverage
  @check_test pytest tests/ --tb=short
  ```
- **Status:** [ ]

### C4. Token usage tracking

- **Impact:** High — critical for cost-conscious users
- **Effort:** Medium (~2 hours)
- **Description:** There's currently no visibility into how many tokens each sprint, heal attempt, or audit cycle consumes. Adding token tracking would help users optimize effort levels and model selection.
- **Implementation:**
  1. Create `internal/metrics/tokens.go` with `TokenUsage` struct (input, output, total)
  2. Parse token counts from engine output/logs (Claude and Codex both emit usage stats)
  3. Aggregate per-sprint, per-heal, per-audit
  4. Include in JSON build report (C2) and terminal summary
  5. Add `--show-tokens` flag to display per-sprint token usage
  6. Update `docs/commands.md` and `docs/terminal-output.md`
  7. Add tests in `internal/metrics/tokens_test.go`
- **Status:** [ ]

---

## D. Existing Feature Improvements

### D1. Structured JSON build reports (same as C2)

> See C2 above. This is both a new feature and an improvement to the existing terminal-only output.

### D2. Heuristic-only `--continue` mode (no LLM cost)

- **Impact:** Medium — faster and free resume decisions
- **Effort:** Low (~1 hour)
- **Description:** The current `--continue` mode invokes an LLM to analyze build state and decide where to resume. For simple cases (resume from first failed sprint), this is expensive overkill. A `--continue --heuristic` flag would use deterministic rules instead.
- **Implementation:**
  1. Add `--heuristic` flag to the continue flow in `internal/cli/run.go`
  2. Add `HeuristicAnalyze()` function in `internal/continuerun/analyzer.go`
  3. Rules: find first sprint without a completion marker, resume from there
  4. Skip LLM call entirely when `--heuristic` is set
  5. Update `docs/commands.md`
  6. Add tests in `internal/continuerun/analyzer_test.go`
- **Status:** [ ]

### D3. Targeted healing (fix specific check categories)

- **Impact:** Medium — reduces token waste in heal loops
- **Effort:** Medium (~2 hours)
- **Description:** Currently the heal loop sends ALL failed checks to the agent every iteration. When a sprint has 10 failed checks of different types, the agent tries to fix everything at once and often fails. Targeted healing would prioritize by severity and fix one category per iteration.
- **Implementation:**
  1. Group failed checks by type in `internal/heal/heal.go`
  2. Sort groups by severity (file existence > file contents > command output)
  3. In each heal iteration, include only the highest-priority unfixed group
  4. Track which groups are resolved across iterations
  5. Fall back to all-at-once if targeted approach stalls (2 iterations with no progress)
  6. Update `docs/self-healing.md`
  7. Add tests in `internal/heal/heal_test.go`
- **Status:** [ ]

### D4. Audit finding export in SARIF format

- **Impact:** Medium — integrates with GitHub, VS Code, SonarQube
- **Effort:** Low-Medium (~1.5 hours)
- **Description:** Sprint and build audit findings are currently only in markdown. Exporting to SARIF (Static Analysis Results Interchange Format) would allow findings to appear as GitHub code annotations, VS Code problems, and in security dashboards.
- **Implementation:**
  1. Create `internal/audit/sarif.go` with SARIF 2.1.0 schema types
  2. Convert audit findings to SARIF `Result` objects with location, severity, message
  3. Write `build-audit.sarif` alongside `build-audit.md`
  4. Add `--sarif` flag to control output
  5. Add `build-audit.sarif` to `.gitignore`
  6. Update `docs/build-audit.md`
  7. Add tests in `internal/audit/sarif_test.go`
- **Status:** [ ]

---

## Execution Order (Suggested)

Priority is based on impact, effort, and dependency chain:

| Order | Item | Category | Rationale |
|-------|------|----------|-----------|
| 1 | A1 | Bug fix | Signal handler bypass is the most impactful bug |
| 2 | A2 + A4 | Bug fix | Related file sync issues, fix together |
| 3 | B1 | Testing | Fixes convention violations, improves test safety |
| 4 | C1 | Feature | Ollama engine broadens user base significantly |
| 5 | C2 | Feature | JSON reports unlock CI/CD integration |
| 6 | A3 | Bug fix | Quick fix, prevents stale lock issues |
| 7 | B2 | Testing | Fills coverage gaps in decision logic |
| 8 | C3 | Feature | `@check_test` is high-value for verification |
| 9 | D2 | Improvement | Quick win, reduces LLM costs on resume |
| 10 | C4 | Feature | Token tracking depends on JSON report (C2) |
| 11 | D3 | Improvement | Targeted healing reduces waste |
| 12 | D4 | Improvement | SARIF export, nice-to-have |
| 13 | A5 | Bug fix | Low severity, defensive hardening |
| 14 | B3 + B4 | Testing | Incremental test coverage |
