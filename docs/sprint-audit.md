# Sprint Audit

The sprint audit is a semantic quality gate that runs by default after each sprint passes verification. A separate AI agent session reviews the sprint's work holistically -- checking correctness, security, edge cases, and code quality -- then a fix agent remediates any issues found. This complements the syntactic verification system (`@check_file`, `@check_cmd`, etc.) with deeper, AI-driven review.

## How It Works

```
Sprint passes verification
       │
       ▼
  Audit agent reviews work (read-only)
       │
       ├─ PASS (no issues or all LOW) → continue to git checkpoint
       │
       ├─ FAIL (CRITICAL/HIGH/MODERATE found)
       │       │
       │       ▼
       │   Fix agent remediates issues
       │       │
       │       ▼
       │   Re-audit (loop up to @max_audit_iterations)
       │
       └─ Exhausted iterations → advisory warning, build continues
```

The audit runs **after** verification passes but **before** the git checkpoint, so that the checkpoint commits both the sprint's work and any audit fixes in one clean commit.

## Two-Agent Design

The audit uses two separate agent sessions:

1. **Audit agent** -- reads the sprint's code changes and writes findings to `.fry/sprint-audit.txt`. This agent does not modify source code.
2. **Fix agent** -- reads the audit findings and makes minimal code changes to address CRITICAL, HIGH, and MODERATE issues. LOW issues are left alone.

This separation mirrors the existing verify→heal pattern and keeps the audit agent's context focused on review.

## Advisory, Not Blocking

If the audit loop exhausts its iterations with issues remaining, the sprint still passes -- it already passed verification. The build continues with an advisory log message:

```
[2026-03-10 12:20:00]   AUDIT: MODERATE issues remain after 3 passes (advisory)
```

This prevents a semantic disagreement between two AI agents from stalling the entire build.

## Configuration

Sprint audits are **enabled by default**. No directive is needed -- audits will run automatically after each sprint passes verification.

To customize audit settings:

```
@max_audit_iterations 5
@audit_engine claude
@audit_model claude-sonnet-4-20250514
```

To disable audits for a specific epic, add `@no_audit`:

```
@epic Quick Prototype
@engine claude
@no_audit
```

## Epic Directives

| Directive | Description |
|---|---|
| `@audit_after_sprint` | Enable post-sprint audit (default: enabled) |
| `@no_audit` | Disable post-sprint audit |
| `@max_audit_iterations <N>` | Maximum audit→fix cycles per sprint (default: 3) |
| `@audit_engine <codex\|claude>` | AI engine for audit/fix sessions (default: same as `@engine`) |
| `@audit_model <model>` | Model override for audit/fix sessions |

## CLI Flags

| Flag | Description |
|---|---|
| `--no-audit` | Disable sprint audit for this run |

## Severity Classification

The audit agent classifies each finding with a severity level:

| Level | Description | Action |
|---|---|---|
| CRITICAL | Data loss, security breach, or crash under normal use | Fix agent remediates |
| HIGH | Significant bug; affects core functionality | Fix agent remediates |
| MODERATE | Edge case gaps, poor error handling, quality concerns | Fix agent remediates |
| LOW | Style, naming, cosmetic | Ignored by fix agent |

Severity is parsed from structured `**Severity:**` lines in the audit output, preventing false positives from severity keywords appearing in code diffs or prose.

## Audit Criteria

The audit agent evaluates the sprint's work against six criteria:

1. **Correctness** -- Does the code do what the sprint goals require?
2. **Usability** -- Are APIs, CLIs, and interfaces intuitive and consistent?
3. **Edge Cases** -- Are boundary conditions and error paths handled?
4. **Security** -- Are there injection, auth, or data-exposure risks?
5. **Performance** -- Are there obvious bottlenecks or resource leaks?
6. **Code Quality** -- Is the code readable, well-structured, and idiomatic?

## Audit Output Format

The audit agent writes findings to `.fry/sprint-audit.txt`:

```
## Summary
Brief overview of the audit results.

## Findings
- **Location:** src/handler.go:42
- **Description:** SQL query uses string concatenation instead of parameterized queries
- **Severity:** HIGH
- **Recommended Fix:** Use db.Query with $1 placeholders

- **Location:** src/auth.go:15
- **Description:** Variable name `x` is unclear
- **Severity:** LOW
- **Recommended Fix:** Rename to `tokenExpiry`

## Verdict
FAIL (HIGH issues found)
```

## Context Provided to Audit Agent

The audit prompt includes:

| Context | Source | Limit |
|---|---|---|
| Executive summary | `plans/executive.md` | First 2,000 characters |
| Sprint goals | `@prompt` block from the epic | Full content |
| What was done | `.fry/sprint-progress.txt` | First 50KB |
| Code changes | `git diff` of sprint work | First 100KB |

The git diff is refreshed before each audit pass (via a callback) so that fixes made by the fix agent are reflected in subsequent audits.

## Effort Level Interaction

- **`low`** -- Sprint audits are skipped entirely, regardless of audit settings. This matches the behavior of sprint reviews at low effort.
- **`medium`**, **`high`**, **`max`** -- Audit runs normally when enabled.

## Terminal Output

### Clean audit (no issues):
```
[2026-03-10 12:10:36] ▶ AUDIT  sprint 3/8 "Auth & Permissions"  pass 1/3  engine=claude
[2026-03-10 12:12:00]   AUDIT: pass (max severity: none)
```

### Issues found and fixed:
```
[2026-03-10 12:10:36] ▶ AUDIT  sprint 3/8 "Auth & Permissions"  pass 1/3  engine=claude
[2026-03-10 12:12:00]   AUDIT: HIGH issues found — running fix agent...
[2026-03-10 12:14:30] ▶ AUDIT  sprint 3/8 "Auth & Permissions"  pass 2/3  engine=claude
[2026-03-10 12:16:00]   AUDIT: pass (max severity: LOW)
```

### Issues persist (advisory):
```
[2026-03-10 12:20:00]   AUDIT: MODERATE issues remain after 3 passes (advisory)
```

## Build Logs

Audit sessions are logged to `.fry/build-logs/`:

```
sprint1_audit1_20060102_150405.log       # Audit pass 1
sprint1_auditfix_1_20060102_150405.log   # Fix agent pass 1
sprint1_audit_final_20060102_150405.log  # Final audit-only pass
```

## Cleanup

After the audit completes (pass or fail), fry removes `.fry/sprint-audit.txt` and `.fry/audit-prompt.md` before the git checkpoint. These are transient files that should not be committed.

## Examples

### Minimal setup

```
@epic My API
@engine claude
```

Audits run by default: 3 max iterations, same engine and model as the build agent.

### Custom audit configuration

```
@epic Payment System
@engine codex
@max_audit_iterations 5
@audit_engine claude
@audit_model claude-sonnet-4-20250514
```

Uses Codex for building but Claude for auditing, with up to 5 audit→fix cycles.

### Disable in epic

```
@epic Quick Prototype
@engine claude
@no_audit
```

### Disable at runtime

```bash
fry --no-audit --engine claude
```
