# Fry v0.1 Build Mission

CREATED_AT: 2026-04-18T14:49:18Z

## Mission

Build **fry v0.1** — a Go CLI tool that codifies the wake-based autonomous-builder orchestration pattern demonstrated in the sibling `saas_build` mission. Thesis: *structured builder orchestration with a flexible LLM-first UI layer.*

**Source of truth (read both, top-to-bottom, every wake):**

- `product-spec.md` — what the tool IS (thesis, input ladder, initialization interface, wake contract, CLI surface, non-goals, shipping bar).
- `build-plan.md` — how to build it (milestones M1 → M7, each with Goal / Deliverables / Implementation sketch / Verification / Exit gate).

These two documents are LOCKED. Do not modify them without writing a `Key Decisions Made` entry in `claude.md` explaining why. Prefer cutting scope over swapping scope.

## Deliverable

A locally-runnable `fry` CLI binary that satisfies the M7 exit gate in `build-plan.md`:

- `fry init <dir>` initializes a mission (writes state.json, installs a macOS LaunchAgent).
- `fry status` prints mission snapshot.
- `fry stop` tears down the scheduler.
- `fry chat` opens the intervention session.
- End-to-end smoke: a trivial "hello world" mission runs one wake, logs one entry, and self-terminates on agent signal.

Plus: `SHIPPED.md` at repo root honestly documenting what works and what doesn't.

## Time budget (supervisor-locked)

- **Soft:** 12h from CREATED_AT — target for completion.
- **Hard:** 24h from CREATED_AT — hard stop regardless of state.
- **Soft deadline:** 2026-04-19T02:49:18Z
- **Hard deadline:** 2026-04-19T14:49:18Z

Overtime (12h–24h) only if deliverables are incomplete. Feature polish is never valid overtime justification.

## Phase model

Unlike the saas_build mission, there is no research or planning phase — the spec is locked and the plan is written. Phase is determined by milestone progress, not elapsed time:

- **Wake 1 (bootstrap):** Read spec + plan. Generate `claude.md` (persistent memory) and `agents.md` (operating doctrine). Write first `wake_log.jsonl` entry. Do NOT start coding yet.
- **Wake 2+ (build):** Execute milestones M1 → M7 from `build-plan.md` in order. Each wake should close ≥1 verification step or move a milestone's exit gate visibly closer. Log which milestone is active in `Current Focus`.
- **Shutdown:** When M7 exit gate passes OR elapsed ≥ 12h with deliverables in place → soft shutdown. If elapsed ≥ 24h → hard stop.

## Shutdown authority

The tool-under-construction is designed with Option B (tool-owned teardown via stdout marker) — see `build-plan.md` §M6 "Scheduler teardown authority". But during this *bootstrap* mission, the agent has direct authority to unload the LaunchAgent when shutdown conditions are met (per `WAKE_INSTRUCTION.md`). Dogfooding the marker protocol is a wake-after-M6 task.

## Authority

- `--permission-mode bypassPermissions` and `--dangerously-skip-permissions` are active.
- Full read/write over `/Users/julie/code/fry/`.
- May run any shell command, install Go deps, `git commit`/`push`, use `gh`.
- May NOT modify `wake.sh`, `WAKE_INSTRUCTION.md`, the LaunchAgent plist, or `CREATED_AT` in this file (unless the runner is demonstrably broken).
- May NOT ask the user anything. The user is offline.

## Contact of record (test accounts only, NEVER in public copy)

yevgetman@gmail.com

## Log schema (append to `logs/wake_log.jsonl`, one JSON line per wake)

```json
{
  "wake_number": 0,
  "timestamp_utc": "ISO8601",
  "elapsed_hours": 0.0,
  "phase": "bootstrap|building|building (overtime)|complete",
  "current_milestone": "M1|M2|...|M7|(bootstrap)|(done)",
  "wake_goal": "one sentence",
  "actions_taken": ["..."],
  "artifacts_touched": ["path/to/file"],
  "blockers": [],
  "next_wake_should": "one sentence",
  "self_assessment": "one sentence",
  "budget_usd_spent_est": 0.0
}
```

BEGIN.
