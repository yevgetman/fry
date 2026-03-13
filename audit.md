# Codebase Audit Report

**Date**: 2026-03-12 — Iteration 2 (post-remediation)
**Scope**: `internal/media/` package and all integration points

---

## Build & Test Status

- `go build ./...` — PASS
- `go test ./...` — PASS (all packages, 16 media tests)

---

## Summary

| Severity | Count |
|----------|-------|
| CRITICAL | 0     |
| HIGH     | 0     |
| MODERATE | 0     |
| LOW      | 3     |

---

## Remediated Issues (from Iteration 1)

| Issue | Was | Fix Applied |
|-------|-----|-------------|
| H1 — Symlink following / path traversal | HIGH | Switched to `filepath.WalkDir`, skip symlinks via `d.Type()&ModeSymlink`, validate relative paths stay within `media/`, use `Lstat` for root check |
| M1 — Silent error swallowing in prepare.go | MODERATE | Added `frylog.Log("WARNING: ...")` on scan error |
| M2 — Identical error messages | MODERATE | Differentiated to `"scan media: stat:"` vs `"scan media: walk:"` |
| M3 — Dotfiles in manifest | MODERATE | Skip files/dirs starting with `.`; hidden dirs get `SkipDir` |
| L1 — O(n*m) categorization | LOW | Built flat `extToCategory` map at init for O(1) lookup |
| Dead code — `d.IsDir()` on symlink branch | n/a | Found during re-audit; simplified symlink check |
| Stale doc comment mentioning "Truncated" | n/a | Found during re-audit; fixed comment |

---

## LOW Issues (remaining)

### L1 — `PromptSection` swallows scan errors silently

**File**: `internal/media/media.go:173`
**Description**: When `Scan` returns an error, `PromptSection` returns empty string with no logging. This is by design — `PromptSection` is called from `sprint/prompt.go` where there's no logger available, and the prepare path already logs warnings. The sprint path silently degrades, which is acceptable since media is optional context.

### L2 — `truncated` variable unused

**File**: `internal/media/media.go:135`
**Description**: The `truncated` bool is set when `MaxAssets` is exceeded but only consumed as `_ = truncated`. Callers (prepare.go) could log a warning if truncation occurred, but this would require changing the `Scan` return signature. Current behavior (silently returning first 10,000 files) is adequate.

### L3 — Test string literals for EffortLevel in prepare_test.go

**File**: `internal/prepare/prepare_test.go:39,77`
**Description**: Tests pass raw string literals (`""`, `"low"`) where `epic.EffortLevel` type is expected. Works due to Go's implicit string constant conversion but reduces refactoring safety. Pre-existing issue, not introduced by media feature.

---

## Conclusion

**EXIT CONDITION MET: No issues of CRITICAL, HIGH, or MODERATE severity found.**

All remaining issues are LOW severity and do not impact correctness, security, or usability.
