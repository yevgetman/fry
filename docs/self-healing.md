# Self-Healing

When verification checks fail after a sprint completes, Fry enters a **heal loop** that automatically re-runs the AI agent with a targeted fix prompt, then re-verifies. This allows Fry to recover from partial failures without human intervention.

## How It Works

1. **Verification fails** — one or more checks return non-zero
2. **Diagnostics collected** — all checks re-run, capturing pass/fail status and stderr/stdout (truncated to 20 lines per check)
3. **Heal prompt assembled** — `.fry/prompt.md` is overwritten with a targeted heal prompt containing the specific failures, instructions for minimum changes, and pointers to context files
4. **Agent re-runs** — a fresh agent session executes with the heal prompt
5. **Pre-sprint hook re-runs** — if configured (e.g., `npm install`), runs again to pick up changes
6. **Re-verification** — the verification file is re-read from disk (so agent edits to checks take effect), then all checks for the sprint run again
7. **Repeat or exit** — if checks still fail, the failure report is appended to `.fry/sprint-progress.txt` and steps 2-6 repeat. Exits when all checks pass (**PASS (healed)**), max attempts exhausted, or the heal loop detects it is stuck (no progress for consecutive attempts — see effort-level behavior below)
8. **Threshold evaluation** — after heal attempts are exhausted (or skipped), if the failure percentage is within the threshold, the sprint passes with status **PASS (deferred failures)**. Deferred failures are documented in `.fry/deferred-failures.md` and passed to the final [Build Audit](build-audit.md) for remediation. If failures exceed the threshold, the sprint **FAIL**s

## Heal Prompt Structure

The heal prompt is specifically designed for targeted fixes:

- **What happened**: Sprint finished but failed verification
- **Failed checks**: Each failing check with its diagnostic output
- **Instructions**: Read progress files, fix minimum changes only, no refactoring
- **Context references**: Pointers to `.fry/sprint-progress.txt` and other context files

In [writing mode](writing-mode.md), the heal instructions use content-oriented language (e.g., "create missing content files, add missing sections, expand insufficient content") instead of code-oriented language (e.g., "fix build errors, correct config").

Each failed attempt's report is appended to `.fry/sprint-progress.txt`, giving subsequent heal attempts cumulative knowledge of what was tried.

## Effort-Level Behavior

The heal loop adapts to the effort level, controlling how many attempts are made, whether progress detection is used, and what failure threshold applies:

| Aspect | `low` | `medium` | `high` | `max` |
|---|---|---|---|---|
| Max heal attempts | 0 (no healing) | 3 (fixed) | 10 (with progress detection) | Unlimited (progress-based, safety cap 50) |
| Progress detection | No | No | Yes (stuck after 2 no-progress) | Yes (stuck after 3 no-progress) |
| Fail threshold | 20% | 20% | 20% | 10% |
| Mid-loop threshold exit | N/A | N/A | N/A | After ≥10 attempts, exits if ≤10% fail |

- **Low**: Skips healing entirely. If ≤20% of checks fail, the sprint passes with deferred failures; otherwise it fails immediately.
- **Medium**: Makes exactly 3 heal attempts. After exhaustion, evaluates the 20% threshold.
- **High**: Makes up to 10 heal attempts, but exits early if the agent is stuck (fail count did not decrease for 2 consecutive attempts). After exhaustion or stuck exit, evaluates the 20% threshold.
- **Max**: Makes unlimited heal attempts (safety cap: 50) while the agent makes progress. Exits when stuck (3 consecutive no-progress attempts). Can also exit early after ≥10 attempts if failures are within the 10% threshold.

## Configuration

| Directive | Scope | Default | Description |
|---|---|---|---|
| `@max_heal_attempts <N>` | Global | Effort-level default | Maximum heal attempts for all sprints (overrides effort default) |
| `@max_heal_attempts <N>` | Per-sprint | Inherits global | Override for a specific sprint |
| `@max_fail_percent <N>` | Global | Effort-level default | Maximum failure percentage before sprint fails |

When `@max_heal_attempts` is explicitly set, it overrides the effort-level default and disables progress detection — the loop runs a fixed number of attempts.

### Override Priority

1. `--retry` override (highest — used by retry mode)
2. Per-sprint `@max_heal_attempts` directive
3. Global `@max_heal_attempts` directive
4. Effort-level default
5. `config.DefaultMaxHealAttempts` (3, fallback for auto/unset effort)

### Examples

```
# Global: allow up to 5 heal attempts for all sprints (overrides effort default)
@max_heal_attempts 5

@sprint 3
@name Complex Integration
@max_heal_attempts 8       # This sprint gets more attempts
```

## Terminal Output

Self-healing progress is always visible in the terminal:

```
[2026-03-10 12:10:30] ▶ AGENT  sprint 3/8 "Auth & Permissions"  heal 1/3  engine=claude  model=default
[2026-03-10 12:12:00] Re-running verification after heal attempt 1...
[2026-03-10 12:12:05] Heal attempt 1 SUCCEEDED — all checks now pass.
```

When heal attempts are exhausted but failures are within the `@max_fail_percent` threshold:

```
[2026-03-10 12:10:30] ▶ AGENT  sprint 3/8 "Auth & Permissions"  heal 3/3  engine=claude  model=default
[2026-03-10 12:14:00] Re-running verification after heal attempt 3...
[2026-03-10 12:14:05] Heal attempt 3 — 1/10 checks still failing.
[2026-03-10 12:14:05] All 3 heal attempts exhausted.
[2026-03-10 12:14:05] Failure rate 10% is within 20% threshold — deferring 1 failures.
```

## Retrying After Heal Exhaustion

When all heal attempts are exhausted and the sprint fails, Fry prints two recovery commands:

```
Retry:  fry run --retry --sprint 4
Resume: fry run --sprint 4
```

### `--retry` (recommended)

The `--retry` flag skips the iteration loop entirely and goes straight to verification + healing with a **boosted heal budget** (2x the normal max, minimum 6 attempts). It preserves the existing `.fry/sprint-progress.txt`, giving the heal agent full context from the previous run — including all prior iteration logs and failed heal attempt reports.

Use `--retry` when:
- The sprint's code was largely correct but verification checks are failing
- The heal loop ran out of attempts but more effort could fix the remaining issues
- You don't want to re-run iterations that would overwrite existing work

After the retried sprint passes, subsequent sprints in the range run normally with full iterations.

### Resume (full re-run)

`fry run --sprint 4` re-runs the sprint from scratch — fresh iterations, fresh progress file. Use this when the sprint's approach was fundamentally wrong and needs a clean start.

### Retry heal budget

The retry heal budget is calculated as:

```
boosted = max(normal_max * 2, 6)
```

The `normal_max` is determined by the effort-level default (or explicit `@max_heal_attempts`):

| Effort | Normal max | Retry budget |
|---|---|---|
| `low` | 0 | 6 (minimum applies) |
| `medium` | 3 | 6 |
| `high` | 10 | 20 |
| `max` | unlimited | 6 (minimum applies) |
| Explicit `@max_heal_attempts 5` | 5 | 10 |

Retry mode always uses a hard cap (no unlimited mode) but preserves progress detection for effort levels that use it.

## Build Log Files

Heal attempt logs are stored alongside regular iteration logs:
```
.fry/build-logs/
  sprint3_heal1_20060102_150405.log
  sprint3_heal2_20060102_150405.log
  sprint3_retry_20060102_150405.log      # Retry mode aggregate log
```
