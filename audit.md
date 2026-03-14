# Documentation Audit Report

**Date:** 2026-03-13
**Scope:** All `.md` documentation files audited against all Go source files
**Type:** Documentation-only audit -- no Go source modifications

---

## Findings

### 1. `docs/effort-levels.md` -- Incorrect effort flag precedence

**Location:** `docs/effort-levels.md`, line 129
**What's wrong:** States "When both `--effort` flag and `@effort` directive are present, the CLI flag takes precedence."
**What the code says:** In `internal/cli/run.go` lines 100-105, when both are present and differ, the CLI flag is **ignored** and a warning is logged: `WARNING: --effort <flag> ignored; epic already specifies @effort <epic>.`
**Recommended fix:** Replace the sentence to state that the `@effort` directive in the epic takes precedence and the CLI flag is ignored with a warning.

---

### 2. `docs/user-prompt.md` -- Missing prompt layers 1.25 and 1.75

**Location:** `docs/user-prompt.md`, lines 39-48 (Prompt Hierarchy table)
**What's wrong:** The prompt hierarchy table lists layers 1, 1.5, 2, 3, 4, 5 but omits Layer 1.25 (media assets) and Layer 1.75 (quality directive at max effort).
**What the code says:** `internal/sprint/prompt.go` assembles layers: 1 (executive), 1.25 (media), 1.5 (user directive), 1.75 (quality directive, max effort only), 2 (plan reference), 3 (sprint instructions), 4 (iteration memory), 5 (completion signal).
**Recommended fix:** Add layers 1.25 and 1.75 to the table.

---

### 3. `docs/verification.md` -- Incomplete outcome matrix

**Location:** `docs/verification.md`, lines 39-44 (Outcome Matrix)
**What's wrong:** The outcome matrix covers 4 scenarios but misses the case where the promise token is NOT found and there are NO verification checks. The source code returns `FAIL (no promise after N iters)` in this case.
**What the code says:** `internal/sprint/runner.go` line 220: `case !promiseFound && !hasChecks: return fmt.Sprintf("FAIL (no promise after %d iters)", cfg.Sprint.MaxIterations), nil`
**Recommended fix:** Add the missing row to the matrix.

---

### 4. `docs/project-structure.md` -- Missing `build-summary.md` and `.fry/summary-prompt.md`

**Location:** `docs/project-structure.md` (directory tree and generated artifacts table)
**What's wrong:** The project structure does not list `build-summary.md` (generated in project root) or `.fry/summary-prompt.md` (transient prompt file for build summary generation).
**What the code says:** `internal/config/config.go` defines `SummaryFile = "build-summary.md"` and `SummaryPromptFile = ".fry/summary-prompt.md"`. The `internal/summary/summary.go` package generates these.
**Recommended fix:** Add both files to the directory tree and generated artifacts table.

---

### 5. `docs/sprint-execution.md` -- Missing build summary step in flow

**Location:** `docs/sprint-execution.md`, lines 7-26 (Execution Flow)
**What's wrong:** The execution flow does not mention the build summary generation step that runs after all sprints complete and before the build audit.
**What the code says:** `internal/cli/run.go` lines 418-433: After the sprint loop, `summary.GenerateBuildSummary()` is called before the build audit.
**Recommended fix:** Add "Generate build summary" to the execution flow.

---

### 6. `docs/sprint-execution.md` -- Missing `FAIL (no promise after N iters)` status

**Location:** `docs/sprint-execution.md`, lines 89-101 (Sprint Status Values)
**What's wrong:** The status table does not include `FAIL (no promise after N iters)`, which occurs when no promise token is found and no verification checks exist.
**What the code says:** `internal/sprint/runner.go` line 220.
**Recommended fix:** Add the missing status.

---

### 7. `docs/self-healing.md` -- Incorrect behavior for `@max_heal_attempts 0`

**Location:** `docs/self-healing.md`, lines 44-46
**What's wrong:** States "Set `@max_heal_attempts 0` globally or per-sprint to disable healing entirely."
**What the code says:** `internal/heal/heal.go` lines 43-45: `if maxAttempts <= 0 { maxAttempts = config.DefaultMaxHealAttempts }` -- setting to 0 does NOT disable healing; it falls back to the default of 3.
**Recommended fix:** Remove the claim about setting to 0 disabling healing. Clarify that the minimum effective value is 1.

---

### 8. `docs/sprint-execution.md` -- No-op early exit requires verification pass

**Location:** `docs/sprint-execution.md`, line 19
**What's wrong:** States "Check for no-op (2 consecutive = done)" without mentioning that verification checks must also pass for early exit.
**What the code says:** `internal/sprint/runner.go` lines 159-164: no-op early exit only triggers when `passCount == totalCount`.
**Recommended fix:** Clarify that no-op early exit requires both consecutive no-change iterations AND passing verification.

---

### 9. `docs/architecture.md` -- Missing `summary/` package

**Location:** `docs/architecture.md`, lines 7-28 (Package Overview)
**What's wrong:** The `summary/` package is missing from the package list.
**What the code says:** `internal/summary/summary.go` exists, handles build summary generation, and is imported by `internal/cli/run.go`.
**Recommended fix:** Add `summary/` with a description.

---

### 10. `docs/sprint-execution.md` -- Build logs missing log types

**Location:** `docs/sprint-execution.md`, lines 103-112 (Build Logs)
**What's wrong:** Missing `summary_TIMESTAMP.log`, `build_audit_TIMESTAMP.log`, and per-iteration `sprint1_iter1_TIMESTAMP.log` patterns. Also, the pattern shown as "Iteration log" (`sprint1_20060102_150405.log`) is actually the per-sprint aggregate log.
**What the code says:** `runner.go` line 99: sprint log, line 136: iter log; `summary.go` line 60: summary log; `build_audit.go` line 64: build audit log.
**Recommended fix:** Add missing log types and correct naming.

---

### 11. `docs/verification.md` -- Per-check diagnostic truncation differs by type

**Location:** `docs/verification.md`, lines 63-65 (Safety Limits)
**What's wrong:** States "Per-check diagnostic output is truncated to 20 lines" but `CheckCmdOutput` failures are truncated to 10 lines.
**What the code says:** `internal/verify/collector.go`: `truncateLines(result.Output, 20)` for CheckCmd, `truncateLines(result.Output, 10)` for CheckCmdOutput.
**Recommended fix:** Clarify the different truncation limits.

---

### 12. `docs/verification.md` -- Missing per-check timeout

**Location:** `docs/verification.md` (Safety Limits section)
**What's wrong:** Does not mention the 120-second per-check timeout for command-based checks.
**What the code says:** `internal/verify/runner.go` line 18: `const defaultCheckTimeout = 120 * time.Second`
**Recommended fix:** Add the timeout to the Safety Limits section.

---

### 13. `README.md` -- Missing build summary in key mechanisms

**Location:** `README.md`, lines 42-56 (Key mechanisms)
**What's wrong:** Does not mention the build summary (`build-summary.md`) that is generated after all sprints complete.
**Recommended fix:** Add build summary to the key mechanisms list.

---

### 14. `docs/effort-levels.md` -- Auto-detect default iterations undocumented

**Location:** `docs/effort-levels.md`, lines 169-181 (Effects Summary)
**What's wrong:** Does not document the default max iterations when effort is unset/auto.
**What the code says:** `internal/epic/types.go` line 58: `default: return 25 // default = high`
**Recommended fix:** Add a note about auto-detect defaults.

---

### 15. Cross-doc consistency: `@effort` flag precedence

**Location:** `docs/effort-levels.md` vs `docs/commands.md`
**What's wrong:** `effort-levels.md` claims CLI flag takes precedence (incorrect). `commands.md` does not mention the precedence behavior at all.
**Recommended fix:** Fix both docs to reflect actual behavior.

---

## Summary of Changes Required

| # | File | Issue | Severity |
|---|---|---|---|
| 1 | `docs/effort-levels.md` | Wrong effort flag precedence | High |
| 2 | `docs/user-prompt.md` | Missing prompt layers 1.25, 1.75 | Medium |
| 3 | `docs/verification.md` | Missing outcome matrix row | Medium |
| 4 | `docs/project-structure.md` | Missing build-summary.md, summary-prompt.md | Medium |
| 5 | `docs/sprint-execution.md` | Missing build summary step | Medium |
| 6 | `docs/sprint-execution.md` | Missing FAIL status value | Medium |
| 7 | `docs/self-healing.md` | Wrong @max_heal_attempts 0 behavior | High |
| 8 | `docs/sprint-execution.md` | No-op requires verification pass | Low |
| 9 | `docs/architecture.md` | Missing summary/ package | Low |
| 10 | `docs/sprint-execution.md` | Missing/incorrect log types | Low |
| 11 | `docs/verification.md` | Incorrect truncation detail | Low |
| 12 | `docs/verification.md` | Missing per-check timeout | Medium |
| 13 | `README.md` | Missing build summary mention | Low |
| 14 | `docs/effort-levels.md` | Auto-detect default undocumented | Low |
| 15 | Cross-doc | Effort precedence inconsistency | High |
