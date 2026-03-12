# Codebase Audit Report

**Date**: 2026-03-12 (Iteration 2 — post-remediation)
**Scope**: Full codebase audit of fry-go, including recent changes to planning output directory and ordered filenames.

---

## Build & Test Status

- `go build ./...` — PASS
- `go test ./...` — PASS (all packages)

---

## Summary

| Severity | Count |
|----------|-------|
| CRITICAL | 0 |
| HIGH | 0 |
| MODERATE | 0 |
| LOW | 4 |

---

## Remediated Issues

### 1. (was MODERATE) Silent effort level override when CLI flag conflicts with epic directive — FIXED

**File**: `internal/cli/run.go:96-100`
**Original Issue**: When the user passes `--effort low` but the epic already contains `@effort high`, the CLI flag was silently ignored.
**Fix Applied**: Added a warning log when the CLI effort flag conflicts with the epic's `@effort` directive, informing the user to re-run `fry prepare` with the desired effort level.

---

## LOW Issues

### 1. Test type safety — string literals instead of EffortLevel constants

**File**: `internal/prepare/prepare_test.go:39,46,53,61,69,77`
**Issue**: Tests pass raw string literals (`""`, `"low"`, `"max"`) where `epic.EffortLevel` type is expected. Works because Go allows implicit conversion of untyped string constants to named string types, but reduces refactoring safety.

### 2. Inconsistent error variable naming in verify/parser.go

**File**: `internal/verify/parser.go:43,61`
**Issue**: Uses `parseErr` instead of the conventional `err` used everywhere else in the codebase.

### 3. Deferred file.Close() ignores close error in appendFile

**File**: `internal/sprint/progress.go:63`
**Issue**: `defer file.Close()` does not capture the error return. For append-mode writes this is practically harmless since the write error is already checked, but technically a close failure could mean data loss on some filesystems.

### 4. No jitter on docker ready check polling

**File**: `internal/docker/docker.go:94`
**Issue**: The polling loop uses a fixed `time.Sleep(1 * time.Second)` without jitter. In single-instance use this is fine; in theoretical multi-instance scenarios it could cause thundering herd effects.

---

## Verified False Positives

The following issues were reported by automated scanning but confirmed as non-issues after manual code review:

| Claimed Issue | Verdict | Reason |
|---|---|---|
| Race condition on `results` slice (run.go:160 vs 232) | **Not a race** | Both accesses are protected by `mu.Lock()`/`mu.Unlock()` |
| File descriptor leak in heal.go and sprint/runner.go | **No leak** | `defer iterLog.Close()` is set before the error return path |
| Resource leak in parseEpicContent (replanner.go:313-326) | **No leak** | `defer os.Remove()` handles cleanup; explicit `Close()` on error path |
| Deferred prompt deletion race (replanner.go:89-92) | **Not a race** | `defer` fires after `RunReplan` returns, which is after engine finishes |
| Command injection in docker/preflight/shellhook | **By design** | Commands come from epic.md — a trusted user-authored configuration file |
| Missing error check in heal.go:67 | **False positive** | Error IS checked: `if err := os.WriteFile(...); err != nil { return }` |
| Error suppression in runAgentWithDualLogs | **Intentional** | Engine errors are suppressed only when context is not cancelled — allows partial output from LLM engines that exit non-zero |

---

## Conclusion

**EXIT CONDITION MET: No issues of CRITICAL, HIGH, or MODERATE severity found.**

All remaining issues are LOW severity and do not require remediation. The implementation is complete,
well-tested, backward compatible, and follows existing code patterns consistently.
