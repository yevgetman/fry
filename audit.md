# Codebase Audit — fry-go

**Date**: 2026-03-11
**Scope**: Full codebase audit covering correctness, security, usability, edge cases, performance, and code quality.

---

## ITERATION 1

### HIGH — Replanner has same artifact-overwrite bug as prepare

**File**: `internal/review/replanner.go:102-136`

The `RunReplan` function captures the full engine output (including Codex session headers, prompt echoes, tool calls, and diffs) and writes it to `epicPath`. This is the same class of bug fixed in `prepare.go` — the engine's session transcript overwrites a potentially clean file.

**Impact**: Corrupted epic file mid-build; downstream sprints receive garbage prompts.

**Status**: FIXED — `textutil.ResolveArtifact` pattern applied.

---

### MODERATE — Duplicate `stripMarkdownFences` across packages

**Files**: `internal/prepare/prepare.go` and `internal/review/replanner.go`

Two identical copies of `stripMarkdownFences`.

**Status**: FIXED — Consolidated into `internal/textutil/textutil.go`.

---

### MODERATE — Three duplicate `runShellHook` functions

**Files**: `internal/cli/run.go`, `internal/sprint/runner.go`, `internal/heal/heal.go`

Three identical implementations of `runShellHook` across three packages, all discarding stdout/stderr.

**Status**: FIXED — Consolidated into `internal/shellhook/shellhook.go` with output capture in errors.

---

### MODERATE — Shell hooks discard command output

**Files**: Same three `runShellHook` locations.

**Status**: FIXED — `shellhook.Run` now captures and includes command output in error messages.

---

### LOW — `parseSprintNumber` compiles regex per call

**File**: `internal/review/replanner.go:377`

**Status**: FIXED — Hoisted to package-level `var reSprintNumber`.

---

### LOW — `AssemblePrompt` ignores write errors

**File**: `internal/sprint/prompt.go:111-112`

**Status**: FIXED — `AssemblePrompt` now returns `(string, error)`.

---

### LOW — `errorAs` wrapper adds no value

**File**: `internal/engine/errors.go`

**Status**: FIXED — Replaced with direct `errors.As` call; `errors.go` deleted.

---

## ITERATION 2

Full re-audit after Iteration 1 remediation. All Iteration 1 fixes verified via `go build ./...` and `go test ./...` — all pass.

### MODERATE — `gitDiffStat` silent failure can trigger premature early exit

**File**: `internal/sprint/runner.go:305-317`

When git fails, both pre/post diffs return `""`, falsely incrementing `consecutiveNoop`, potentially triggering premature early exit.

**Status**: FIXED — Returns unique sentinel on error; logs warning.

---

### MODERATE — Lock file accepts PID ≤ 0

**File**: `internal/lock/lock.go:22-26`

Corrupted lock file with `0` causes `syscall.Kill(0, 0)` which signals the entire process group, falsely blocking fry.

**Status**: FIXED — Added `pid > 0` guard.

---

### MODERATE — `AssembleReviewPrompt` ignores write errors for audit file

**File**: `internal/review/reviewer.go:99-102`

Review prompt file write errors silently discarded.

**Status**: FIXED — Now returns `(string, error)` and propagates write failures.

---

### MODERATE — Unbounded output collection in verification `CheckCmd`

**File**: `internal/verify/runner.go:46-51`

`CombinedOutput()` loads all check output into memory with no size limit.

**Status**: FIXED — Introduced `cappedBuffer` type with 10 MB limit.

---

### LOW — Config test has no-op assertion

**File**: `internal/config/config_test.go:11`

**Status**: FIXED — Removed meaningless self-comparison assertion.

---

### LOW — Log write errors silently discarded

**File**: `internal/log/log.go:32-35`

**Status**: FIXED — Log file write failures now emit warning to stderr.

---

### LOW — `truncateLines` does not indicate truncation

**File**: `internal/verify/collector.go:32-41`

**Status**: FIXED — Now appends `"... (N more lines)"` when output is truncated.

---

## ITERATION 3

Full re-audit after Iteration 2 remediation. All fixes verified:
- `go build ./...` — clean
- `go test ./...` — all packages pass
- `go vet ./...` — clean

### Remaining Issues

No issues of CRITICAL, HIGH, or MODERATE severity remain.

All remaining observations are LOW severity or design-level considerations that do not warrant code changes:

- **LOW (by design)**: Epic files execute commands via bash (shellhook, docker readyCmd, verification checks). This is inherent to the tool's purpose — epic files are trusted project configuration, analogous to Makefiles or CI configs.
- **LOW (cosmetic)**: `defaultIfEmpty` in `reviewer.go` trims trailing newline from actual content but not from fallback. Minor formatting inconsistency in review prompts only.

**EXIT CONDITION MET**: No issues of severity greater than LOW remain.
