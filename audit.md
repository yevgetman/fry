# Codebase Audit Report

**Date:** 2026-03-06
**Scope:** Full codebase (fry.sh, fry-prepare.sh, all documentation)
**Files reviewed:** fry.sh, fry-prepare.sh, README.md, GENERATE_EPIC.md, epic-example.md, verification-example.md, .gitignore

---

## Summary

Two iterations of the audit loop were performed.

- **Iteration 1** found 1 MEDIUM issue and 6 LOW issues. The MEDIUM issue and 1 LOW documentation gap were fixed.
- **Iteration 2** confirmed all fixes and found no remaining issues above LOW severity.

**EXIT CONDITION MET:** All remaining findings are LOW severity.

---

## MEDIUM (Resolved)

### M1: `print_summary()` icon showed "?" for healed PASS results

**File:** fry.sh, line 1388
**Status:** FIXED

The PASS check used exact string match (`= "PASS"`), which didn't match heal-feature result strings like `"PASS (healed)"`. Changed to glob match (`== PASS*`) to be consistent with the FAIL line.

---

## LOW Severity (Remaining)

### L1: README Global Directives table was missing `@max_heal_attempts`

**File:** README.md
**Status:** FIXED — row added to directive table.

### L2: Duplicated progress.txt header template

**File:** fry.sh, lines 1105-1114 and 1118-1127

The progress.txt heredoc header is identical in both the "first run" and "restart from sprint 1" branches. Changes to the header format must be applied in both places.

### L3: Agent prompt string duplicated across verbose/non-verbose branches

**File:** fry.sh, lines 1232-1234 and 1237-1239

The long agent prompt string is duplicated in the verbose and non-verbose code paths. Same concern — changes must be applied in both places.

### L4: `.gitignore` entries could accumulate on edge case

**File:** fry.sh, lines 991-1008

`init_git()` checks for `prompt.md` in `.gitignore` and appends the full block if missing. If `prompt.md` is manually removed while other entries remain, the block is re-appended, creating duplicate entries for `build-logs/`, `.env`, etc. No functional impact — git handles duplicate .gitignore entries gracefully.

### L5: Lockfile has a small race window

**File:** fry.sh, lines 1474-1486

Between checking the lockfile PID and writing the new PID, another process could pass the same check. Theoretical for a manual CLI tool.

### L6: `fry-prepare.sh` agent does not support `--model` or `--engine_flags`

**File:** fry-prepare.sh, lines 75-93

The `run_agent()` in fry-prepare.sh doesn't support model or flag overrides. Users must configure via engine-specific environment variables.

---

## Categories Checked

| Category | Result |
|---|---|
| Coherence with application aim | No issues found |
| Bugs | 1 found (M1 — fixed) |
| Usability | No issues found |
| Edge cases | Adequately handled (safe array access patterns, input validation, graceful degradation) |
| Security | No vulnerabilities (eval usage is within the user-authored trust boundary) |
| Performance | No bottlenecks (check iterations are O(n) on small datasets) |
| Code quality and style | Consistent patterns, clear structure, good separation of concerns |
