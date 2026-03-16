# Self-Healing

When verification checks fail after a sprint completes, Fry enters a **heal loop** that automatically re-runs the AI agent with a targeted fix prompt, then re-verifies. This allows Fry to recover from partial failures without human intervention.

## How It Works

1. **Verification fails** — one or more checks return non-zero
2. **Diagnostics collected** — all checks re-run, capturing pass/fail status and stderr/stdout (truncated to 20 lines per check)
3. **Heal prompt assembled** — `.fry/prompt.md` is overwritten with a targeted heal prompt containing the specific failures, instructions for minimum changes, and pointers to context files
4. **Agent re-runs** — a fresh agent session executes with the heal prompt
5. **Pre-sprint hook re-runs** — if configured (e.g., `npm install`), runs again to pick up changes
6. **Re-verification** — the verification file is re-read from disk (so agent edits to checks take effect), then all checks for the sprint run again
7. **Repeat or exit** — if checks still fail, the failure report is appended to `.fry/sprint-progress.txt` and steps 2-6 repeat. Exits when all checks pass (**PASS (healed)**), or max attempts exhausted and remaining failures are evaluated against the `@max_fail_percent` threshold (default: 20%)
8. **Threshold evaluation** — after all heal attempts are exhausted, if the failure percentage is within the threshold, the sprint passes with status **PASS (deferred failures)**. Deferred failures are documented in `.fry/deferred-failures.md` and passed to the final [Build Audit](build-audit.md) for remediation. If failures exceed the threshold, the sprint **FAIL**s

## Heal Prompt Structure

The heal prompt is specifically designed for targeted fixes:

- **What happened**: Sprint finished but failed verification
- **Failed checks**: Each failing check with its diagnostic output
- **Instructions**: Read progress files, fix minimum changes only, no refactoring
- **Context references**: Pointers to `.fry/sprint-progress.txt` and other context files

Each failed attempt's report is appended to `.fry/sprint-progress.txt`, giving subsequent heal attempts cumulative knowledge of what was tried.

## Configuration

| Directive | Scope | Default | Description |
|---|---|---|---|
| `@max_heal_attempts <N>` | Global | 3 | Maximum heal attempts for all sprints |
| `@max_heal_attempts <N>` | Per-sprint | Inherits global | Override for a specific sprint |

### Examples

```
# Global: allow up to 5 heal attempts for all sprints
@max_heal_attempts 5

@sprint 3
@name Complex Integration
@max_heal_attempts 8       # This sprint gets more attempts
```

### Minimum Value

Setting `@max_heal_attempts` to 0 or a negative value causes it to fall back to the default (3). The minimum effective value is 1.

## Terminal Output

Self-healing progress is always visible in the terminal:

```
[2026-03-10 12:10:30] ▶ AGENT  sprint 3/8 "Auth & Permissions"  heal 1/3  engine=claude  model=default
[2026-03-10 12:12:00] Re-running verification after heal attempt 1...
[2026-03-10 12:12:05] Heal attempt 1 SUCCEEDED — all checks now pass.
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

| Normal max | Retry budget |
|---|---|
| 1 | 6 |
| 2 | 6 |
| 3 | 6 |
| 4 | 8 |
| 5 | 10 |

The normal max is determined by `@max_heal_attempts` (global or per-sprint), defaulting to 3.

## Build Log Files

Heal attempt logs are stored alongside regular iteration logs:
```
.fry/build-logs/
  sprint3_heal1_20060102_150405.log
  sprint3_heal2_20060102_150405.log
  sprint3_retry_20060102_150405.log      # Retry mode aggregate log
```
