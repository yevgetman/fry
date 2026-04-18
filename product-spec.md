# Fry — Product Spec (v0.1 draft)

> **Thesis:** Structured builder orchestration with a flexible LLM-first UI layer.
>
> - **Go owns:** scheduler, state plumbing, overlap guards, process isolation, logging.
> - **Claude owns:** everything semantic — planning, decisions, code, phase interpretation, status narration, intervention.

---

## 1. What this is

A small Go CLI that runs an LLM-driven build in **scheduled wakes**. The scheduler fires a runner every N minutes; each wake is an independent process that reads mission state, does one unit of work, writes updated state, logs, and exits. A separate `fry chat` command opens a Claude session with read/write access to the mission so a human can monitor, query, and steer.

One command builds a thing. Another command lets you talk to it.

## 2. When to reach for fry

- Long-running builds (hours to days) where continuous babysitting is expensive.
- Tasks where human steering is useful but not required every few minutes.
- Any work that decomposes naturally into "wake, read state, do one chunk, exit" — small iterative progress with persistent memory between iterations.

Not for: one-shot code generation, interactive debugging sessions, anything where the build must finish in a single process lifetime.

## 3. Predecessor (legacy-fry)

The `~/code/fry-legacy/` project (previously `~/code/fry/`) was a prior attempt at a related problem as a *synchronous sprint runner*. It accumulated multi-agent audit loops, observer sub-agents, copilot coordinator, team runtime, writing/planning/software modes, skill plugins. The over-engineering sank it.

The new fry is a **rewrite from first principles** that borrows specific proven patterns from legacy-fry and explicitly rejects the rest. See §11 Vendor List and §12 Non-Goals.

---

## 4. Input model

`fry new` accepts user direction at any point on the specificity ladder. Exactly one of:

| Input flag | What happens |
|---|---|
| `--prompt <file>` | Vague direction. Wake 1 generates a `plan.md` from the prompt before executing. |
| `--plan <file>` | Full plan. No LLM prep; wake 1 executes directly. |
| `--prompt <file> --plan <file>` | Plan is blueprint; prompt is *active steering*, re-read every wake. |
| `--spec-dir <dir>` | User-provided bundle (executive + plan + whatever). Copied verbatim; wake 1 executes. |

This mirrors legacy-fry's input ladder (`--user-prompt` / `--user-prompt + executive.md` / `plan.md` provided), simplified to four explicit modes.

**Phases are markdown, not code.** If the user wants phase structure (research/planning/building or anything else), they write a `## Phases` section in the prompt/plan:

```markdown
## Phases
- research: 0h–3h
- planning: 3h–6h
- building: 6h–∞
```

The Go tool doesn't parse or model phases. It only tracks `elapsed_hours`. The wake reads the prompt each invocation and the agent interprets which phase it's in. Flexibility lives in markdown; the tool stays small.

---

## 5. Initialization interface

`fry new <name>` takes a small, orthogonal set of flags:

```
--prompt <file>         [input mode: see §4]
--plan <file>           [input mode]
--spec-dir <dir>        [input mode]
--effort {fast,standard,max}   default: standard
--interval <duration>   default: 10m  (any Go duration: 2m, 17m, 43m, 2h)
--duration <hours>      default: 12h  (soft stop)
--overtime <hours>      default: 0    (additional discretionary window before hard stop)
--base-dir <dir>        default: ~/missions/
```

**Effort** maps to per-wake claude invocation:

| Level | Model | Effort | Budget/wake |
|---|---|---|---|
| `fast` | sonnet | medium | $1 |
| `standard` | sonnet | high | $2 |
| `max` | opus | max | $5 |

**Interval** accepts any Go duration. Short intervals (≤2m) are legal but cost-scaling is linear with wake count. Overlap lock (§8) protects against "wake takes longer than interval" — the late fire skips rather than running in parallel.

**Duration + overtime** — `duration` is the target ship point; the agent is encouraged to finish there. `overtime` is a discretionary window between `duration` and `duration+overtime` where the agent may continue if (and only if) core deliverables are genuinely unfinished. Feature polish is not a valid overtime justification. Hard stop at `duration+overtime`, no discretion.

---

## 6. Mission lifecycle

```
$ fry new charterdeck --prompt prompt.md --duration 12h --overtime 12h
# Scaffolds ~/missions/charterdeck/ with prompt.md, state.json, notes.md, runner.sh, scheduler plist.

$ fry start charterdeck
# Installs LaunchAgent. First wake fires in <interval>.

$ fry chat charterdeck
# Opens a Claude session with full mission context. Monitor, query, steer.

$ fry stop charterdeck
# Unloads scheduler. Mission dir preserved.
```

Mission terminates naturally when the agent sets `state.status = complete` (typically on hitting soft deadline with deliverables in place) or hits the hard deadline.

---

## 7. File layout

```
~/missions/<name>/
├── prompt.md                 # user-provided input (copied at new; wake-1-generated plan.md appended if --prompt-only)
├── state.json                # machine-readable state (tool-owned)
├── notes.md                  # narrative state (wake-owned)
├── wake_log.jsonl            # one JSON per wake (structured, append-only)
├── supervisor_log.jsonl      # one JSON per chat intervention (audit trail)
├── runner.sh                 # generated; scheduler invokes this, which invokes `fry wake <name>`
├── scheduler.plist           # macOS LaunchAgent (or .service on Linux v0.2)
├── lock/                     # mkdir-based overlap lock
└── artifacts/                # agent's working directory (code, output files, anything the build produces)
```

**Invariants:**
- The tool never writes to `notes.md` or `artifacts/`. Those are the wake's output.
- The wake never writes to `state.json` except via a narrow update API (see §9).
- `supervisor_log.jsonl` is write-only during `fry chat` sessions; wakes may read but not write.
- `wake_log.jsonl` is append-only by wakes; the tool reads for status.

---

## 8. Wake contract

Each wake process is invoked by `runner.sh` (which the scheduler fires). The runner:

1. Acquires overlap lock (`mkdir ~/missions/<name>/lock`). If lock exists, logs "skipped — overlap" and exits.
2. Sets env: `FRY_MISSION_NAME`, `FRY_WAKE_NUMBER`, `FRY_ELAPSED_HOURS`, `FRY_DEADLINE_UTC`.
3. Invokes `fry wake <name>` with a wall-clock cap (default: `interval - 30s` or 540s, whichever is smaller).
4. Releases lock on exit.

`fry wake <name>` then:

1. Reads `prompt.md`, `state.json`, `notes.md`, `tail -N wake_log.jsonl`.
2. Assembles a **layered prompt** (borrowed from legacy-fry):
   - L0: wake contract reminder (stable)
   - L1: mission overview (stable across mission)
   - L2: input prompt + plan (stable unless regenerated)
   - L3: `notes.md` current focus + supervisor injections (changes every wake)
   - L4: last N wake_log entries (changes every wake)
   - L5: current wake directive
   - This ordering keeps prompt-cache hit rates high on the stable prefix.
3. Spawns `claude -p --permission-mode bypassPermissions --dangerously-skip-permissions --model <from effort> --effort <from effort> --max-budget-usd <from effort> --add-dir <mission-dir>` with the assembled prompt fed via stdin.
4. Streams output to `cron.log`.
5. Parses output for **promise token** `===WAKE_DONE===` as the final line. Absence = failed wake.
6. Validates that a new `wake_log.jsonl` line was appended.
7. Updates `state.json`: increments `current_wake`, updates `last_wake_at`, transitions `status` per deadline logic.
8. Performs **no-op detection**: if the last 3 wakes show zero file changes in `artifacts/` AND identical `notes.md` snapshots AND no new decisions, flag a warning in `cron.log` and append to supervisor_log.jsonl. Does not auto-stop; the agent / human decides.
9. Checks deadline:
   - `elapsed < duration` → normal; continue.
   - `duration ≤ elapsed < duration+overtime` AND `status != complete` → enter overtime; tag wake log entry as `overtime: true`.
   - `elapsed ≥ duration+overtime` → hard stop; set `status = complete`, unload scheduler.

The agent inside the wake can set `status = complete` itself when it believes the mission is done. The tool honors this.

---

## 9. State schema

### `state.json` (tool-owned)

```json
{
  "mission_id": "charterdeck",
  "created_at": "2026-04-18T02:35:34Z",
  "prompt_path": "~/missions/charterdeck/prompt.md",
  "input_mode": "prompt" | "plan" | "prompt+plan" | "spec-dir",
  "effort": "standard",
  "interval_seconds": 600,
  "duration_hours": 12.0,
  "overtime_hours": 12.0,
  "current_wake": 42,
  "last_wake_at": "2026-04-18T11:34:00Z",
  "status": "active",
  "hard_deadline_utc": "2026-04-19T02:35:34Z"
}
```

`status` values: `active` | `overtime` | `complete` | `stopped` | `failed`

Wakes can update `current_wake`, `last_wake_at`, and `status` (only to `complete`). They cannot change `created_at`, `duration_hours`, `overtime_hours`, or `interval_seconds`.

### `notes.md` (wake-owned)

Template written by `fry new`:

```markdown
# Mission Notes

## Current Focus
<one sentence — what this wake is for>

## Next Wake Should
<handoff directive from current wake to next>

## Decisions
- YYYY-MM-DDTHH:MMZ (wake N): <decision>

## Open Questions
- ...

## Supervisor Injections
<!-- Chat sessions append here; wakes read and honor. -->
```

### `wake_log.jsonl`

Strict JSON, one per line. Minimum fields:

```json
{
  "wake_number": 42,
  "timestamp_utc": "2026-04-18T11:34:00Z",
  "elapsed_hours": 8.95,
  "wake_goal": "...",
  "actions_taken": ["...", "..."],
  "files_changed": ["artifacts/foo.go", "notes.md"],
  "blockers": [],
  "next_wake_plan": "...",
  "self_assessment": "...",
  "promise_token_found": true,
  "exit_code": 0,
  "wall_clock_seconds": 287,
  "cost_usd": 0.42,
  "overtime": false
}
```

### `supervisor_log.jsonl`

Append-only during `fry chat` sessions. Schema:

```json
{
  "timestamp_utc": "2026-04-18T11:32:10Z",
  "type": "intervention" | "query" | "manual_wake" | "stop_mission",
  "summary": "Edited notes.md to pivot next wake toward SHIPPED.md + smoke tests.",
  "fields_changed": ["notes.md"],
  "operator": "chat"
}
```

---

## 10. CLI commands

### `fry new <name> [options]`

Scaffolds a mission directory. Writes all initial files. Does NOT start the scheduler.

Validates: exactly one input flag, valid effort, valid duration/interval, mission name not already taken.

### `fry start <name>`

Installs the scheduler (LaunchAgent on macOS). Verifies it loaded. First wake fires in `<interval>`.

Idempotent: running on an already-started mission is a no-op with a warning.

### `fry stop <name>`

Unloads the scheduler. Does NOT delete the mission directory. Sets `state.status = stopped`.

### `fry chat <name>`

Spawns a Claude session:

```
claude --add-dir ~/missions/<name>/ \
       --append-system-prompt <fry-supervisor-system-prompt>
```

The appended system prompt tells Claude:
- The mission file layout and purpose of each file.
- What it may modify: `notes.md`, append to `supervisor_log.jsonl`, run `fry wake/stop/status` via Bash.
- What it must NOT modify: `prompt.md` (unless user explicitly asks), `state.json` except through the narrow update API, `wake_log.jsonl`.
- The audit expectation: every change to mission state gets a corresponding `supervisor_log.jsonl` entry.
- How to read mission context efficiently: `state.json` + last 5 lines of `wake_log.jsonl` + `notes.md` gives you 90% of what you need.

The chat session inherits all Claude Code's native tools (Read, Edit, Write, Bash, WebFetch, etc.). No custom tool vocabulary. Keeping it stock.

### `fry wake <name>`

Internal — scheduler invokes this via `runner.sh`. Exposed as a user command so the human (or chat session) can trigger a manual wake immediately. Fails fast if lock held.

### `fry status <name>`

Non-LLM human-readable snapshot. Prints: current wake, elapsed, status, last wake goal, files in `artifacts/` (count + newest), scheduler health. For when you want info without burning tokens.

### `fry list`

Lists all missions in `~/missions/` with one-line status each.

### `fry logs <name> [-n 10] [-f]`

Tails `wake_log.jsonl`. Pretty-prints by default; `--json` for raw.

---

## 11. Vendor list (from legacy-fry)

When the refactor begins, physically copy + adapt these files. Do NOT import legacy-fry as a Go module dependency — fry-legacy should stay dead.

**Tentative (finalize during implementation):**

- `internal/prompt/layers.go` — layered prompt assembly with deterministic ordering for cache-hit rate.
- `internal/triage/classify.go` — cheap-LLM mission classifier. **Deferred to v0.2.** In v0.1 the human sets duration/overtime explicitly.
- Progress-file split pattern (`sprint-progress.txt` + `epic-progress.txt`) — conceptually borrowed as `wake_log.jsonl` + `notes.md`. Re-implement, don't copy.
- Promise-token idiom — re-implement as `===WAKE_DONE===` in wake stdout.
- No-op detection logic — re-implement in `fry wake` runner.
- Alignment (heal) loop pattern — studied for inspiration only; v0.1 doesn't have sanity-check infrastructure to justify it.

**Explicitly NOT borrowed:**

- Sprint runner (legacy-fry is synchronous in-process; wake runner is async cross-process — different primitive).
- Copilot / observer agents (our chat session subsumes this).
- Team / parallel runtime (v0.1 is single-wake, lock-guarded).
- Mode system (software/planning/writing).
- Skill plugin system (openclaw-skill; 1195 lines deleted in fry-legacy history).
- Multi-agent audit loops (single-session is cheaper and clearer).
- Auto-detect effort (human sets it upfront).

---

## 12. Non-Goals (v0.1)

These are **explicitly excluded.** A PR adding any of these is rejected:

- Multi-agent audit loops.
- Observer / copilot sub-agents as separate processes.
- Team / parallel wakes per mission.
- Writing / planning / software *mode system*. One mode: whatever-the-prompt-says.
- Skill / plugin architecture.
- Feature flags that ship off by default. If it's in, it's on.
- Provider abstraction. Claude CLI only.
- Web UI / TUI. CLI + chat session is the whole UX.
- Custom prompt DSL. Markdown is the prompt language.
- Auto-detected effort.
- Interactive confirmations (runs unattended by design).
- Multi-mission parallelism in a single wake fire.
- Rich dashboards. Use `fry status` for text, `fry chat` for conversation.
- Triage gate (auto-classify mission size). Deferred to v0.2 or later.

**If a v0.2+ adds any of these, they must ship with a kill-switch and a clear deprecation path for what they replace.**

---

## 13. Platform support

- **v0.1: macOS** via LaunchAgent. Tested on Apple Silicon, zsh. Uses `launchctl load/unload`, `launchctl kickstart`, `launchctl print` for probe.
- Interface scaffolded so Linux `systemd --user` backend drops in as a package-level swap.
- Cron is **not** a target. LaunchAgent/systemd are strictly better for headless user-session work (keychain ACLs, TCC, env).

The scheduler abstraction in Go:

```go
type Scheduler interface {
    Install(mission string, runner string, interval time.Duration) error
    Uninstall(mission string) error
    Status(mission string) (SchedulerStatus, error)
    Kickstart(mission string) error  // fire now
}
```

Two implementations: `scheduler_darwin.go` (LaunchAgent), `scheduler_linux.go` (systemd; v0.2).

---

## 14. Chat-session supervisor design

This is the LLM-first UI layer — the feature that distinguishes fry from a plain cron runner.

**What the user sees:**
```
$ fry chat charterdeck
[fry] Loaded charterdeck | wake 42 | elapsed 8.95h | status active
> what's happening?
```

Claude responds with a natural-language summary of current state, drawing from state.json + recent log entries + notes.md.

**What Claude can do in a chat session:**
- Summarize status.
- Query logs ("what did wake 31 change?").
- Read artifacts ("show me the current dashboard component").
- Propose interventions ("I think wake 42 should pivot to tests; shall I update notes.md?").
- Execute interventions on user confirmation (edit notes.md + append supervisor_log entry).
- Manually fire a wake (`fry wake charterdeck` via Bash).
- Stop the mission (`fry stop charterdeck` via Bash).

**What Claude cannot do:**
- Modify `prompt.md` (the original input) — that's immutable mid-mission unless user explicitly asks.
- Modify `state.json` directly — must go through `fry` subcommands that validate transitions.
- Modify `wake_log.jsonl` (append-only by wakes).
- Start new missions (user must run `fry new` themselves).

**Why this works:** Claude Code's native toolset already does 95% of what we need. The `--append-system-prompt` flag injects mission awareness and the access rules. Audit logging is handled by the session itself emitting `supervisor_log` entries when it makes state changes. No custom tools, no MCP server, no special sauce.

---

## 15. Shipping bar for v0.1

To declare v0.1 done, all must be true:

1. `fry new`, `fry start`, `fry stop`, `fry chat`, `fry wake`, `fry status`, `fry logs`, `fry list` all functional on macOS.
2. A dogfood mission runs end-to-end: `fry new demo --prompt sample.md --duration 30m --interval 5m --effort fast && fry start demo` produces:
   - ≥5 wake entries in `wake_log.jsonl`
   - `state.status` transitions to `complete` at or before 30m
   - `notes.md` is meaningfully updated each wake
   - No overlap-lock failures
3. `fry chat demo` opens a Claude session and the user can do `"what's happening"` → get a coherent summary in ≤1 turn.
4. README documenting all commands, a quickstart, and the file layout.
5. Non-goals list enforced: no optional features flags in the codebase, no ModeX files, no plugin registry, no provider abstraction.
6. Total codebase ≤2000 LOC Go (excluding vendored legacy-fry packages).

If any of these slip, v0.1 isn't shipped.

---

## 16. Open questions (deferred decisions)

- **Triage gate inclusion:** Kept out of v0.1. If v0.2 adds it, it should be opt-in via `--triage` flag on `fry new`, not default.
- **Chat session authentication:** Does the chat session need its own session ID separate from wakes, or can it reuse the same Claude auth? (Probably reuse; deferred until an actual problem surfaces.)
- **Recovery from scheduler death:** LaunchAgent auto-restarts on reboot; does the runner need to detect and clean up stale locks on startup? Likely yes — add a `--recover-stale-lock` path in `fry wake`.
- **Cost budget at mission level:** Should `fry new` accept `--max-total-cost` as a hard ceiling? Lean yes for v0.1, simple implementation — sum cost_usd in wake_log.jsonl, halt on exceed. Defer if ship pressure is high.
- **Multi-user / multi-machine:** Explicitly out of scope for v0.1. Missions are local to one machine.
- **Prompt cache strategy:** Layered prompt (§8 step 2) is designed for cache-friendly ordering. Concrete cache-key strategy (which fields are stable across wakes, which change) needs measurement during implementation.
- **Log rotation:** At 72 wakes of ~1KB each, `wake_log.jsonl` is ~72KB. Not a concern for v0.1. Revisit if missions run weeks.

---

## 17. What to build first

Implementation order once we start writing Go:

1. **State + file layout.** `fry new` scaffolds correctly. `fry status` reads state.json and prints. No scheduler, no wakes — just the filesystem contract.
2. **Scheduler abstraction + macOS LaunchAgent backend.** `fry start` / `fry stop` actually installs/removes LaunchAgent. Manual `fry wake` runs a noop that appends to wake_log.jsonl with a canned entry. Proves the loop.
3. **Real wake: claude invocation.** `fry wake` spawns `claude -p` with assembled prompt, parses output for promise token. One cycle end-to-end with a trivial prompt like "write hello world to artifacts/out.txt."
4. **Notes.md round-trip.** Wake reads notes.md, updates it, next wake reads the update. Cross-wake memory works.
5. **Chat session.** `fry chat` spawns Claude with the appended system prompt. User can query and intervene.
6. **No-op detection + deadline logic.** Soft/hard stops, overtime, status transitions.
7. **Dogfood:** run the 30-minute test mission from §15. Fix whatever breaks.
8. **README + shipping.**

That's the whole MVP. Anything not in this list is post-v0.1.

---

## 18. Naming + future

"fry" is the name, inherited from legacy-fry. The new codebase replaces the old in `~/code/fry/`; legacy is preserved at `~/code/fry-legacy/` for reference only.

Version numbers: v0.1 ships the above. v0.2 considers: triage gate, Linux systemd backend, cost-budget ceiling, `fry pause/resume`. v0.3+: only if real usage demands.

The discipline: **every release must make fry smaller in some dimension, not larger.** If we can't delete something to add something, the addition probably isn't earning its place.
