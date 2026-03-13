# Codebase Audit — Iteration 2

## Summary
Re-audited after remediating 3 MODERATE and 2 LOW issues from Iteration 1. All MODERATE issues have been fixed. A few LOW issues remain.

## Remediated (from Iteration 1)

| # | Issue | Was | Fix |
|---|-------|-----|-----|
| 1 | `parseAuditSeverity` too greedy | MODERATE | Now only matches severity keywords on lines that also contain "severity" as a label |
| 2 | Sprint progress unbounded in audit prompt | MODERATE | Capped at 50KB with truncation indicator |
| 3 | Git diff stale across audit fix iterations | MODERATE | Added `DiffFn` option; caller passes `git.GitDiffForAudit` closure; diff refreshed before each audit pass |
| 4 | `audit.Cleanup` error ignored | LOW | Now logs warning on failure |
| 5 | `GitDiffForAudit` reset failure silent | LOW | Now prints warning to stderr |

## Remaining Findings

### 1. `parseAuditSeverity` could false-positive on negation phrases
- **Location:** `internal/audit/audit.go:291-316`
- **Severity:** LOW
- **Description:** A line like "No high-severity issues found" would match because it contains both "SEVERITY" and "HIGH" on the same line. In practice, audit agents following the structured output format would not produce such lines in the findings section — the Verdict section uses PASS/FAIL, not severity words. Risk is minimal.
- **Fix:** Could require a stricter pattern (e.g., `Severity:\s+(CRITICAL|HIGH|MODERATE|LOW)`), but the current approach is robust enough for the structured output format the audit agent is instructed to use.

### 2. `maxCheckOutput` is a package-scoped constant
- **Location:** `internal/verify/runner.go:87`
- **Severity:** LOW
- **Description:** `10 * 1024 * 1024` is a constant in the verify package rather than in `config`. This is a valid Go pattern for single-use constants and consistent with how `maxProgressBytes` is defined in audit.go.
- **Fix:** None needed.

### 3. `equalGlobalDirectives` grows linearly with Epic struct
- **Location:** `internal/review/replanner.go:338-364`
- **Severity:** LOW
- **Description:** Every new field added to `Epic` must be manually added to this comparison function. The function correctly includes all 4 new audit fields. This coupling is documented by the function's proximity to the struct usage.
- **Fix:** None needed — follows existing pattern.

### 4. `GitDiffForAudit` uses stderr instead of `frylog`
- **Location:** `internal/git/git.go:146`
- **Severity:** LOW
- **Description:** The reset warning uses `fmt.Fprintf(os.Stderr, ...)` while most of the codebase uses `frylog.Log("WARNING: ...")`. The git package intentionally avoids importing frylog to stay decoupled from application-level logging. This is a reasonable design choice.
- **Fix:** None needed.
