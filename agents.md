# AGENTS.md — Operating Doctrine

> Read this on every wake before you start work. These are the rules of engagement. `prompt.md` is the mission. `claude.md` is your memory. This file is your **how**.

---

## 1. Per-Wake Loop (do this every wake, in this order)

1. **Read `prompt.md`** — the mission north star. Every wake, no exceptions.
2. **Read `product-spec.md`** — what the tool IS. Every wake.
3. **Read `build-plan.md`** — how to build it. Every wake.
4. **Read `claude.md` + `agents.md`** — your memory + doctrine. Pay attention to `Current Phase`, `Current Wake Number`, `Current Focus`, `Current Milestone`, and `Next Wake Should`.
5. **Run `tail -10 logs/wake_log.jsonl`** — recent history of what your prior selves did.
6. **Compute `elapsed_hours`** from `CREATED_AT` in prompt.md. Determine phase (building / overtime / complete).
7. **Shutdown decision tree** per `prompt.md` step 8. If conditions are met for shutdown, execute protocol and exit.
8. **Execute one wake's worth of work** — meaningful, visible progress. Target 3–7 minutes of real work. 540 s hard cap set by the runner.
9. **Update `claude.md`** — bump `Current Wake Number`, update `Elapsed Hours`, refresh `Current Focus` and `Next Wake Should`, append to `Key Decisions Made` if binding decision made, update `Open Questions`.
10. **Append one JSON line to `logs/wake_log.jsonl`** per schema in `prompt.md` §Log schema. Never skip.
11. **Exit.** Do not loop. Do not start the next wake's work.

---

## 2. Milestone Schedule (approximate wake targets)

| Milestone | Target complete by wake | Notes |
|---|---|---|
| M1 — Filesystem + state | Wake 5 | `fry new`/`list`/`status` working |
| M2 — Scheduler + LaunchAgent | Wake 10 | `fry start`/`stop` real |
| M3 — Wake: claude invocation | Wake 16 | One real LLM wake end-to-end |
| M4 — Prompt assembly + notes round-trip | Wake 21 | Cross-wake memory works |
| M5 — Chat session | Wake 25 | `fry chat` spawns supervised Claude |
| M6 — Deadlines + no-op + shutdown | Wake 30 | Mission auto-terminates |
| M7 — Polish + README + ship | Wake 34 | All §15 gates green |

These are **targets**, not hard gates (except M7 must be done before elapsed ≥ 12h for on-time delivery). If a milestone slips by 2+ wakes, log it in claude.md Open Questions and assess whether scope needs cutting.

---

## 3. Logging Schema (strict)

One JSON object per wake, appended to `logs/wake_log.jsonl`. No pretty-printing — one line per record. Per schema in `prompt.md`:

```json
{
  "wake_number": N,
  "timestamp_utc": "ISO8601Z",
  "elapsed_hours": F,
  "phase": "bootstrap|building|building (overtime)|complete",
  "current_milestone": "M1|M2|...|M7|(bootstrap)|(done)",
  "wake_goal": "one sentence",
  "actions_taken": ["..."],
  "artifacts_touched": ["path/to/file"],
  "blockers": [],
  "next_wake_should": "one sentence",
  "self_assessment": "one sentence",
  "budget_usd_spent_est": F
}
```

- `timestamp_utc` — at time of writing the log, ISO-8601 Z-suffixed.
- `elapsed_hours` — float, 3 decimals.
- `artifacts_touched` — paths relative to `/Users/julie/code/fry/`.
- `blockers` — empty array if none. If anything, describe + the workaround you took.
- `self_assessment` — one honest sentence. "Behind on M2 due to launchctl TCC issue" > "On track."

---

## 4. Decision Principles (hard rules)

1. **M-exit-gates are binary — no partial credit.** A milestone is either done or it isn't. No "mostly done". No "code written but not verified".
2. **Every wake produces a visible artifact.** Committed code, new file, passing test, updated milestone checkbox. "I thought about it" is not a wake output.
3. **Cut scope, don't pivot.** If behind, reduce scope within the active milestone. Do NOT jump ahead to M<N+1> to avoid a blocker in M<N>.
4. **NEVER leave code in a non-compiling state at wake boundary.** If mid-refactor, commit WIP or stash before exit.
5. **Commit + push at end of every wake that produced code.** Even WIP commits. Descriptive messages. Branch: main.
6. **Non-goals are hard.** No optional flags, no mode systems, no plugin registries, no provider abstractions. If it's in build-plan.md §12 Non-Goals, reject it.
7. **No CGO.** No more than 3 external Go deps (cobra, testify, maybe uuid). Every dep must be justified.
8. **Scheduler is safety-critical.** Agent signals completion (`FRY_STATUS_TRANSITION=complete` on stdout). Tool parses + calls `scheduler.Uninstall`. Agent must NEVER call `launchctl` or `systemctl` directly.
9. **`make install` after every code change.** The memory system says so. Always run it to verify the binary builds.
10. **Prompt caching matters.** Keep L0–L2 (stable prefix) stable wake-to-wake. Changing content (L3–L5) at the end.

---

## 5. Recovery Checklist (if state is inconsistent)

You wake up and something is wrong. In order:

1. `cat /Users/julie/code/fry/claude.md` — what does your memory say?
2. `tail -20 logs/wake_log.jsonl` — what actually happened recently?
3. `go build ./...` from `/Users/julie/code/fry/` — does it compile?
4. `git status` — what's uncommitted?
5. Check for stale lock: `ls -la ~/missions/*/lock/ 2>/dev/null` — if mtime > 2× interval, remove it and log the cleanup.
6. Compute elapsed; verify you're in the right phase.
7. From `claude.md → Current Milestone`, find the first unchecked exit gate item in `build-plan.md` for that milestone.
8. Do one concrete thing toward it. Update claude.md. Log. Exit.

**If a prior wake crashed mid-edit:**
- Check `git diff` for partial changes.
- If the code doesn't compile: finish the minimum viable change to make it compile, commit as "WIP: complete partial M<N> edit from crashed wake".
- If the code compiles but is inconsistent: commit the clean state, note the inconsistency in claude.md Open Questions.

**If the lockfile is stale:**
- Check `ls -la ~/missions/<name>/lock` — if older than `interval_seconds * 2`, it's stale.
- `rmdir ~/missions/<name>/lock` and log a warning to `supervisor_log.jsonl` if one exists.

---

## 6. What NOT to Do

- Do NOT ask the user questions. The user is absent/offline.
- Do NOT run `launchctl unload` for the fry wake scheduler except in the Shutdown Protocol (claude.md §Shutdown Protocol).
- Do NOT modify `prompt.md`, `product-spec.md`, or `build-plan.md` without writing a `Key Decisions Made` entry explaining why.
- Do NOT modify `wake.sh` or the LaunchAgent plist unless the runner is demonstrably broken (evidence in `logs/cron.log`).
- Do NOT start M<N+1> work to avoid a blocker in M<N>.
- Do NOT leave the code non-compiling at wake boundary.
- Do NOT commit secrets, `.env*`, `*.pem`, `*.key`.
- Do NOT implement anything in §12 Non-Goals of product-spec.md.
- Do NOT skip the wake log entry. A missing log is worse than a boring one.

---

## 7. LOC Budget

Target: ≤2000 LOC of Go across `internal/` + `cmd/`. Check periodically:

```bash
find /Users/julie/code/fry/internal /Users/julie/code/fry/cmd -name '*.go' | xargs wc -l | tail -1
```

If approaching 2000, cut features from lower-priority milestones (M5 chat is on non-critical path; M7 polish is reducible). Never cut M1–M4 or M6 — those are the core loop.

---

## 8. Final Sanity

You are one actor operating discretely every 10 minutes, across roughly 72 chances before the soft deadline. Each wake is a discrete, atomic unit of progress. The mission ends when M7 exit gate passes and SHIPPED.md exists, OR when the clock says so.

The thesis: **structured builder orchestration with a flexible LLM-first UI layer.** Every line of code either enables that thesis or it doesn't belong.

Ship a working tool.
