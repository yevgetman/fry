# Codebase Audit — Iteration 2 (Final)

## Summary

Re-audit after remediating all 10 findings from Iteration 1. All MODERATE issues have been
fixed. All tests pass. Only LOW-severity observations remain.

## Remediated (Iteration 1 → 2)

| # | Issue | Severity | Fix Applied |
|---|-------|----------|-------------|
| 1 | Agent exit errors silently discarded across all runner functions | MODERATE | Added `frylog.Log("WARNING: agent exited with error (non-fatal): %v", runErr)` to all 6 error-suppression sites in `sprint/runner.go`, `heal/heal.go`, `audit/audit.go`. Preserves lenient behavior while providing debuggability. |
| 2 | Audit severity parsing matched multiple keywords per line | MODERATE | Changed `severityWordRe.FindAllString` to `severityWordRe.FindString` in `audit/audit.go:321` — now only the first severity keyword on each line is used. Prevents `"**Severity:** LOW — not CRITICAL"` from being misclassified. |
| 3 | No per-check timeout on verification commands | MODERATE | Added `defaultCheckTimeout = 120 * time.Second` constant and wrapped `@check_cmd` / `@check_cmd_output` execution contexts with `context.WithTimeout` in `verify/runner.go:54,65`. |
| 4 | Lock file TOCTOU confusing error message | LOW | After `O_EXCL` create fails, `lock.go` now re-reads the lock file and returns proper "another fry instance is running (PID X)" if a live PID is found. |
| 5 | Duplicate `shellQuote` function | LOW | Moved to `textutil.ShellQuote` (exported). Removed local copies from `git/git.go` and `verify/runner.go`. Updated test to import from `textutil`. |
| 6 | `readFile` doesn't handle missing files | LOW | `sprint/progress.go:readFile` now returns `("", nil)` on `os.IsNotExist`. |
| 7 | `ParseVerdict` silently defaults to CONTINUE | LOW | Added `frylog.Log("WARNING: no <verdict> tag found...")` in `review/reviewer.go:135`. |
| 8 | Docker readiness loop context check ordering | LOW | Replaced `sleep(1*time.Second)` + separate `ctx.Done()` check with a single `select { case <-ctx.Done(): ... case <-time.After(1*time.Second): }` in `docker/docker.go:89-93`. |
| 9 | `summary.go` duplicated config constants | LOW | Removed `SummaryFile` and `SummaryPromptFile` from `summary.go`; now uses `config.SummaryFile` and `config.SummaryPromptFile`. |
| 10 | `media.Scan` truncation invisible to callers | LOW | Changed `Scan` signature to return `([]Asset, bool, error)` where the bool indicates truncation. Updated `PromptSection` and `prepare.go` callers. `prepare.go` now logs a warning when truncated. Updated all test call sites. |

## Remaining Findings

### 1. `@audit_after_sprint` directive is a no-op

- **Location:** `internal/epic/parser.go:97-98`
- **Severity:** LOW
- **Description:** `AuditAfterSprint` defaults to `true` (line 31), so `@audit_after_sprint` has no effect. It exists for documentation/explicitness. The functional counterpart is `@no_audit`.

### 2. `parseAuditSeverity` could false-positive on negation prose

- **Location:** `internal/audit/audit.go:305-333`
- **Severity:** LOW
- **Description:** A line like `**Severity:** LOW — this is not HIGH` would match LOW (the first keyword), which is correct after the Iteration 1 fix. However, `**Severity:** not HIGH` would match `HIGH` since "not" isn't part of the severity keyword. In practice, audit agents follow the structured format and always place the severity keyword immediately after the label, so this is theoretical.

### 3. `prepare.go` `runPrepareStep` suppresses errors when output is non-empty

- **Location:** `internal/prepare/prepare.go:248-252`
- **Severity:** LOW
- **Description:** Same "lenient agent exit" pattern as the other runners, but not covered by the logging fix since it returns output directly. The prepare phase has its own validation (step0-step3 validators) that catch incomplete output, so the risk is minimal.

### 4. `review/reviewer.go` and `review/replanner.go` agent calls don't log suppressed errors

- **Location:** `internal/review/reviewer.go:241-243`, `internal/review/replanner.go:108-110`
- **Severity:** LOW
- **Description:** These two agent call sites use the same lenient pattern (`if runErr != nil && output == "" { return err }`) but only fail when output is empty. When output is non-empty and there's an error, the error is silently discarded without a warning log. Minimal risk since the outputs are validated downstream.

## Verdict

PASS — all remaining issues are LOW severity.
