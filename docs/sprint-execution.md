# Sprint Execution

Sprints are the core execution unit of Fry. Each sprint runs as an iterative agent loop where the AI gets a prompt, does work, and logs progress. The next iteration reads what the previous one accomplished and continues.

## Execution Flow

```
FOR each sprint in range:
  ├─ Ensure Docker up (if configured)
  ├─ Run @pre_sprint hook
  ├─ Initialize sprint progress file
  ├─ Assemble layered prompt
  ├─ Agent iteration loop:
  │   ├─ Run @pre_iteration hook
  │   ├─ Invoke engine with prompt
  │   ├─ Log output to iteration log
  │   ├─ Check for promise token
  │   ├─ No-op detection (2 consecutive iterations with no changes
  │   │   AND all verification checks pass = early exit; 3 at max effort)
  │   └─ Continue until: promise found OR no-op+verified OR max iterations
  ├─ Run verification checks
  ├─ If checks fail: enter heal loop
  ├─ Sprint audit: audit→fix loop (CRITICAL/HIGH block; MODERATE advisory)
  ├─ Git checkpoint
  ├─ Compact progress
  └─ Sprint review (if enabled)

AFTER all sprints:
  ├─ Print build summary table
  ├─ Append review summary (if reviews enabled)
  ├─ Generate build-summary.md (agent session)
  └─ Build audit (if enabled and full epic completed)
```

## Prompt Assembly

Each sprint prompt is assembled in layers, giving the AI agent structured context:

| Layer | Content | Source |
|---|---|---|
| 1 | Executive context | `plans/executive.md` (if exists) |
| 1.25 | Media assets | Manifest of files in `media/` (if directory exists) |
| 1.5 | User directive | `--user-prompt`, `--user-prompt-file`, or `.fry/user-prompt.txt` |
| 1.75 | Quality directive | Injected at `max` effort only — instructs agent to handle all edge cases, write defensive code, validate assumptions |
| 2 | Strategic plan reference | Pointer to `plans/plan.md` |
| 3 | Sprint instructions | `@prompt` block from epic |
| 4 | Iteration memory | `.fry/sprint-progress.txt` + `.fry/epic-progress.txt` |
| 5 | Completion signal | Promise token pattern |

The assembled prompt is written to `.fry/prompt.md` before each agent invocation.

**Note:** Supplementary assets (`assets/` directory) are **not** included in sprint prompts. Their contents are injected only during `fry prepare` and baked into `plans/plan.md` and `.fry/epic.md`. See [Supplementary Assets](supplementary-assets.md).

## Promise Tokens

Each sprint defines a completion signal via `@promise TOKEN`. The agent loop ends when the AI outputs the string `===PROMISE: TOKEN===` in its response.

Example in an epic:
```
@promise SPRINT1_DONE
```

The agent should output:
```
===PROMISE: SPRINT1_DONE===
```

### Completion Conditions

The agent loop exits when any of these occur:
1. **Promise found** — the agent output contains the promise token string
2. **No-op detection** — 2 consecutive iterations with no meaningful changes and verification passes (3 consecutive at `max` effort, to allow more rumination)
3. **Max iterations reached** — the `@max_iterations` limit is hit

## Progress Tracking

Fry uses a two-file progress system that provides bounded context without unbounded growth:

### `.fry/sprint-progress.txt`
- **Per-sprint iteration log** — overwritten at the start of each new sprint
- Records what each iteration accomplished
- Grows with each iteration within a sprint
- The agent reads this to know what was done in prior iterations

### `.fry/epic-progress.txt`
- **Cross-sprint compacted summary** — append-only within a run
- Reset on full rebuild from sprint 1
- Contains condensed summaries of completed sprints
- Gives the agent context about the broader project state

### Progress Compaction

After each sprint completes, progress is compacted:

- **Mechanical extraction** (default): Summarizes iteration logs programmatically
- **AI compaction** (`@compact_with_agent`): Uses the AI engine to produce a richer summary

## Sprint Status Values

| Status | Meaning |
|---|---|
| `PASS` | Completed with promise token and verification passed |
| `PASS (healed)` | Verification initially failed, then fixed via healing |
| `PASS (verification passed, no promise)` | No promise token found, but verification passed |
| `PASS (healed, no promise)` | Healed without promise token |
| `FAIL (verification failed, heal exhausted)` | All heal attempts exhausted |
| `FAIL (no promise, verification failed, heal exhausted)` | No promise + healing failed |
| `FAIL (no promise after N iters)` | No promise token found and no verification checks exist |
| `FAIL (no prompt)` | Sprint had no prompt text |
| `SKIPPED` | Sprint was not in the run range |

## Build Logs

Build logs are written to `.fry/build-logs/`:
```
.fry/build-logs/
  sprint1_20060102_150405.log              # Per-sprint aggregate log
  sprint1_iter1_20060102_150405.log        # Per-iteration log
  sprint1_iter2_20060102_150405.log        # Per-iteration log
  sprint1_heal1_20060102_150405.log        # Heal attempt log
  sprint1_audit1_20060102_150405.log       # Audit pass log
  sprint1_auditfix_1_20060102_150405.log   # Audit fix agent log
  sprint1_audit_final_20060102_150405.log  # Final audit pass log
  summary_20060102_150405.log              # Build summary agent log
  build_audit_20060102_150405.log          # Build audit agent log
```

## Shell Hooks

Two hooks can be configured to run shell commands at sprint boundaries:

| Directive | When it runs |
|---|---|
| `@pre_sprint <cmd>` | Before each sprint starts (e.g., `npm install`) |
| `@pre_iteration <cmd>` | Before each agent invocation (e.g., `npm run lint:fix`) |

Hooks execute via `bash -c` in the project directory.

## Resuming Failed Builds

When a sprint fails (after exhausting heal attempts), Fry commits partial work and prints two recovery commands:

```
Retry:  fry run --retry --sprint 4
Resume: fry run --sprint 4
```

### `--retry` (recommended for verification failures)

The `--retry` flag skips the iteration loop entirely and goes straight to verification + healing with a boosted heal budget (2x normal attempts, minimum 6). It preserves the existing `.fry/sprint-progress.txt` so the agent retains full context from the previous failed attempt — including prior iteration logs and heal failure reports.

Use `--retry` when:
- The sprint's code was largely written correctly but verification checks are failing
- The heal loop was exhausted but more attempts might fix remaining issues
- You don't want to re-run iterations that would overwrite existing work

After the retried sprint passes, subsequent sprints in the range run normally.

### Resume (full re-run)

`fry run --sprint 4` re-runs the sprint from scratch — fresh iterations, fresh progress file. Use this when the sprint's approach was fundamentally wrong and needs a clean start.

Progress is preserved in `.fry/sprint-progress.txt`, `.fry/epic-progress.txt`, and git history. The agent picks up where it left off.

## Signal Handling

Fry catches `SIGINT` and `SIGTERM` signals. On interrupt, it commits partial work to git before exiting, ensuring no progress is lost.
