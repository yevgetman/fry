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
       └─ Exhausted iterations
               │
               ├─ CRITICAL or HIGH → sprint FAILS, epic stops
               └─ MODERATE → advisory warning, build continues
```

The audit runs **after** verification passes but **before** the git checkpoint, so that the checkpoint commits both the sprint's work and any audit fixes in one clean commit.

## Two-Agent Design

The audit uses two separate agent sessions:

1. **Audit agent** -- reads the sprint's code changes and writes findings to `.fry/sprint-audit.txt`. This agent does not modify source code.
2. **Fix agent** -- reads the audit findings and makes minimal code changes to address CRITICAL, HIGH, and MODERATE issues. LOW issues are left alone. The fix agent receives the sprint goals (`@prompt` block) for context, and on iteration 2+ it also receives the **previous iteration's audit findings** so it can avoid repeating failed approaches.

This separation mirrors the existing verify→heal pattern and keeps the audit agent's context focused on review.

## Blocking vs Advisory

After the audit loop exhausts its iterations, the outcome depends on the highest remaining severity:

- **CRITICAL or HIGH** -- The sprint **fails** and the epic stops. The build summary shows `FAIL (audit: CRITICAL)` or `FAIL (audit: HIGH)`. You can resume with `fry run <epic> <sprint>` after fixing the issues.
- **MODERATE** -- The sprint **continues** with an advisory warning logged to the build output and build summary. This prevents moderate semantic disagreements between two AI agents from stalling the entire build.
- **LOW or none** -- The audit passes cleanly.

```
# Blocking (CRITICAL/HIGH) — exact severity counts:
[2026-03-10 12:20:00]   AUDIT: FAILED — 1 CRITICAL, 1 HIGH remain after 3 passes

# Advisory (MODERATE) — exact severity counts:
[2026-03-10 12:20:00]   AUDIT: 2 MODERATE remain after 3 audit passes (advisory)
```

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
| `--no-audit` | Disable sprint and build audits for this run |

## Severity Classification

The audit agent classifies each finding with a severity level. Descriptions are mode-specific; the table below shows software-mode descriptions. In writing mode, CRITICAL maps to factual errors/contradictions, HIGH to structural gaps, MODERATE to voice/transition issues, and LOW to formatting/word choice. The blocking behavior is identical across modes.

| Level | Description (software) | Action | If unresolved |
|---|---|---|---|
| CRITICAL | Data loss, security breach, or crash under normal use | Fix agent remediates | **Blocks** sprint |
| HIGH | Significant bug; affects core functionality | Fix agent remediates | **Blocks** sprint |
| MODERATE | Edge case gaps, poor error handling, quality concerns | Fix agent remediates | Advisory warning |
| LOW | Style, naming, cosmetic | Ignored by fix agent | Ignored |

Severity is parsed from structured `**Severity:**` lines in the audit output, preventing false positives from severity keywords appearing in code diffs or prose.

## Audit Criteria

The audit agent evaluates the sprint's work against six criteria. The criteria vary by mode.

### Software and planning modes (default)

1. **Correctness** -- Does the code do what the sprint goals require?
2. **Usability** -- Are APIs, CLIs, and interfaces intuitive and consistent?
3. **Edge Cases** -- Are boundary conditions and error paths handled?
4. **Security** -- Are there injection, auth, or data-exposure risks?
5. **Performance** -- Are there obvious bottlenecks or resource leaks?
6. **Code Quality** -- Is the code readable, well-structured, and idiomatic?

### Writing mode (`--mode writing`)

1. **Coherence** -- Logical flow between sections; consistent narrative thread
2. **Accuracy** -- Factual correctness of claims, examples, and references
3. **Completeness** -- All topics from the sprint prompt are covered at appropriate depth
4. **Tone & Voice** -- Consistent register, audience-appropriate language, no jarring shifts
5. **Structure** -- Clear headings, logical section ordering, effective use of lists and examples
6. **Depth** -- Sufficient detail and analysis; not superficial or padded

See [Writing Mode](writing-mode.md) for the full writing-mode reference.

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

## Context Provided to Fix Agent

The fix prompt includes:

| Context | Source | When |
|---|---|---|
| Sprint goals | `@prompt` block from the epic | Always |
| Previous audit findings | Prior iteration's findings (retained in memory) | Iteration 2+ only |
| Current audit findings | This iteration's `.fry/sprint-audit.txt` | Always |
| Context pointers | References to `sprint-progress.txt` and `plans/plan.md` | Always |

On the first iteration, only the current findings are included. On subsequent iterations, the fix agent sees both the previous and current findings, allowing it to recognize recurring issues and try different remediation strategies. Note: previous findings are held in memory between iterations — the `.fry/sprint-audit.txt` file is deleted after each pass to prevent stale results from contaminating the next audit, then recreated by the next audit agent invocation.

## Effort Level Interaction

- **`low`** -- Sprint audits are skipped entirely, regardless of audit settings. This matches the behavior of sprint reviews at low effort.
- **`medium`** -- Bounded audit: runs up to `@max_audit_iterations` (default: 3) audit→fix cycles. Stops when iterations are exhausted.
- **`high`** -- Progress-based audit: continues as long as the fix agent is making progress (resolving findings or uncovering new ones). Safety cap at 50 iterations. Stops early if 3 consecutive audit passes show no progress (same findings repeating).
- **`max`** -- Same progress-based behavior as `high`, but with a safety cap of 150 iterations, allowing more thorough remediation for mission-critical builds.

When `@max_audit_iterations` is explicitly set in the epic, it is always respected as a hard cap regardless of effort level, and progress detection is disabled. Progress-based behavior only activates when the iteration count is not explicitly configured.

### Progress detection

At `high` and `max` effort, Fry tracks audit findings across iterations by extracting structured `**Description:**` lines from each audit pass. After each audit:

1. If no findings remain above LOW → pass (exit).
2. If findings are identical to the previous pass or a superset (no issues resolved, no new issues) → increment stale counter.
3. If any previous findings were resolved or new findings appeared → reset stale counter (progress made).
4. After 3 consecutive stale passes → stop and run final audit.

## Terminal Output

### Clean audit (no issues):
```
[2026-03-10 12:10:36] ▶ AUDIT  sprint 3/8 "Auth & Permissions"  pass 1/3  engine=claude
[2026-03-10 12:12:00]   AUDIT: pass (none)
```

### Issues found and fixed (bounded) — severity counts shown:
```
[2026-03-10 12:10:36] ▶ AUDIT  sprint 3/8 "Auth & Permissions"  pass 1/3  engine=claude
[2026-03-10 12:12:00]   AUDIT: 1 HIGH, 2 MODERATE — running fix agent...
[2026-03-10 12:14:30] ▶ AUDIT  sprint 3/8 "Auth & Permissions"  pass 2/3  engine=claude
[2026-03-10 12:16:00]   AUDIT: pass (1 LOW)
```

### Progress-based audit (high/max effort):
```
[2026-03-10 12:10:36] ▶ AUDIT  sprint 3/8 "Auth & Permissions"  pass 1 (progress-based, cap 50)  engine=claude
[2026-03-10 12:12:00]   AUDIT: 1 HIGH, 1 MODERATE — running fix agent...
[2026-03-10 12:14:30] ▶ AUDIT  sprint 3/8 "Auth & Permissions"  pass 2 (progress-based, cap 50)  engine=claude
[2026-03-10 12:16:00]   AUDIT: 2 MODERATE — running fix agent...
[2026-03-10 12:18:00] ▶ AUDIT  sprint 3/8 "Auth & Permissions"  pass 3 (progress-based, cap 50)  engine=claude
[2026-03-10 12:20:00]   AUDIT: pass (1 LOW)
```

### Progress stall (high/max effort):
```
[2026-03-10 12:18:00] ▶ AUDIT  sprint 3/8 "Auth & Permissions"  pass 4 (progress-based, cap 50)  engine=claude
[2026-03-10 12:20:00]   AUDIT: no progress detected (1/3 stale iterations)
[2026-03-10 12:20:00]   AUDIT: 1 HIGH — running fix agent...
[2026-03-10 12:22:00] ▶ AUDIT  sprint 3/8 "Auth & Permissions"  pass 5 (progress-based, cap 50)  engine=claude
[2026-03-10 12:24:00]   AUDIT: no progress detected (2/3 stale iterations)
[2026-03-10 12:24:00]   AUDIT: 1 HIGH — running fix agent...
[2026-03-10 12:26:00] ▶ AUDIT  sprint 3/8 "Auth & Permissions"  pass 6 (progress-based, cap 50)  engine=claude
[2026-03-10 12:28:00]   AUDIT: no progress detected (3/3 stale iterations)
[2026-03-10 12:28:00]   AUDIT: stopping — no progress after 6 iterations
```

### CRITICAL/HIGH issues persist (blocking) — shows exact counts:
```
[2026-03-10 12:20:00]   AUDIT: FAILED — 1 HIGH remain after 3 passes
Retry:  fry run --retry --sprint 3
Resume: fry run --sprint 3
```

### MODERATE issues persist (advisory) — shows exact counts:
```
[2026-03-10 12:20:00]   AUDIT: 2 MODERATE remain after 3 audit passes (advisory)
```

## Build Logs

Audit sessions are logged to `.fry/build-logs/`:

```
sprint1_audit1_20060102_150405.log       # Audit pass 1
sprint1_auditfix_1_20060102_150405.log   # Fix agent pass 1
sprint1_audit_final_20060102_150405.log  # Final audit-only pass
```

## Cleanup

After the audit completes (pass or fail), Fry removes `.fry/sprint-audit.txt` and `.fry/audit-prompt.md` before the git checkpoint. These are transient files that should not be committed.

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

## See Also

- [Build Audit](build-audit.md) -- final holistic codebase audit that runs after the entire epic completes successfully
