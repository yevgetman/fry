# Sprint Execution

Sprints are the core execution unit of fry. Each sprint runs as an iterative agent loop where the AI gets a prompt, does work, and logs progress. The next iteration reads what the previous one accomplished and continues.

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
  │   ├─ Check for no-op (2 consecutive = done)
  │   └─ Continue until: promise found OR no-op OR max iterations
  ├─ Run verification checks
  ├─ If checks fail: enter heal loop
  ├─ Compact progress
  ├─ Git checkpoint
  └─ Sprint review (if enabled)
```

## Prompt Assembly

Each sprint prompt is assembled in layers, giving the AI agent structured context:

| Layer | Content | Source |
|---|---|---|
| 1 | Executive context | `plans/executive.md` (if exists) |
| 1.5 | User directive | `--user-prompt` or `.fry/user-prompt.txt` |
| 1.75 | Quality directive | Injected at `max` effort only — instructs agent to handle all edge cases, write defensive code, validate assumptions |
| 2 | Strategic plan reference | Pointer to `plans/plan.md` |
| 3 | Sprint instructions | `@prompt` block from epic |
| 4 | Iteration memory | `.fry/sprint-progress.txt` + `.fry/epic-progress.txt` |
| 5 | Completion signal | Promise token pattern |

The assembled prompt is written to `.fry/prompt.md` before each agent invocation.

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

fry uses a two-file progress system that provides bounded context without unbounded growth:

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
| `FAIL (no prompt)` | Sprint had no prompt text |
| `SKIPPED` | Sprint was not in the run range |

## Build Logs

Per-iteration logs are written to `.fry/build-logs/`:
```
.fry/build-logs/
  sprint1_20060102_150405.log         # Iteration log
  sprint1_heal1_20060102_150405.log   # Heal attempt log
```

## Shell Hooks

Two hooks can be configured to run shell commands at sprint boundaries:

| Directive | When it runs |
|---|---|
| `@pre_sprint <cmd>` | Before each sprint starts (e.g., `npm install`) |
| `@pre_iteration <cmd>` | Before each agent invocation (e.g., `npm run lint:fix`) |

Hooks execute via `bash -c` in the project directory.

## Resuming Failed Builds

When a sprint fails (after exhausting heal attempts), fry commits partial work and prints a resume command:

```
Resume: fry run .fry/epic.md 4
```

Progress is preserved in `.fry/sprint-progress.txt`, `.fry/epic-progress.txt`, and git history. The agent picks up where it left off.

## Signal Handling

fry catches `SIGINT` and `SIGTERM` signals. On interrupt, it commits partial work to git before exiting, ensuring no progress is lost.
