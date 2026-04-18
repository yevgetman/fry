# fry

**Structured builder orchestration with an LLM-first UI layer.** `fry` runs a Claude agent on a repeating schedule — each "wake" gets a layered prompt (your mission goal + cross-wake notes + recent history) and produces artifacts. When the mission is done, the scheduler unloads itself. You can intervene at any time with `fry chat`.

## Install

```bash
go install github.com/yevgetman/fry/cmd/fry@latest
```

Requires Go 1.22+, macOS (Linux stub present; LaunchAgent scheduler is macOS-only in v0.1), and the [Claude CLI](https://docs.anthropic.com/claude/claude-code) authenticated and in your PATH.

> **PATH note:** if you have an older `fry` binary at `~/.local/bin/fry`, ensure `~/go/bin` comes first in your PATH, or remove the old binary.

## Quickstart

```bash
# 1. Create a mission (30-minute window, fires every 5 minutes)
fry new demo --prompt /path/to/prompt.md --duration 30m --interval 5m --effort fast

# 2. Start the scheduler (installs a macOS LaunchAgent)
fry start demo

# 3. Watch progress
fry logs demo

# 4. Talk to the agent while it's running
fry chat demo
```

The mission self-terminates when the duration elapses or the agent emits `FRY_STATUS_TRANSITION=complete` on stdout.

## Commands

| Command | Description |
|---|---|
| `fry new <name>` | Scaffold a new mission directory |
| `fry start <name>` | Install the LaunchAgent scheduler |
| `fry stop <name>` | Unload the scheduler (preserves artifacts) |
| `fry status <name>` | Print mission snapshot |
| `fry list` | List all missions |
| `fry wake <name>` | Fire one wake immediately |
| `fry logs <name>` | Tail `wake_log.jsonl` |
| `fry chat <name>` | Open an interactive Claude session with mission context |
| `fry version` | Print version |

### Key flags for `fry new`

| Flag | Default | Description |
|---|---|---|
| `--prompt <file>` | — | Prompt file (required unless `--plan` or `--spec-dir`) |
| `--plan <file>` | — | Plan file (alternative to prompt) |
| `--spec-dir <dir>` | — | Directory of spec files |
| `--effort` | `standard` | `fast` / `standard` / `max` (maps to claude model + budget) |
| `--duration` | `12h` | How long the mission runs |
| `--interval` | `10m` | How often the agent wakes |
| `--overtime` | `0h` | Grace window after duration |
| `--base-dir` | `~/missions/` | Where mission directories live |

## File layout

```
~/missions/<name>/
├── state.json            # machine state (status, wake count, deadlines)
├── prompt.md             # your original input (immutable)
├── plan.md               # optional plan file
├── notes.md              # cross-wake narrative memory (agent edits this)
├── wake_log.jsonl        # per-wake structured log
├── supervisor_log.jsonl  # chat-session audit trail
├── artifacts/            # agent's working directory
├── scheduler.plist       # LaunchAgent plist (installed to ~/Library/LaunchAgents/)
└── runner.sh             # shell script invoked by launchd
```

## How it works

Each wake:
1. Acquires an `os.Mkdir`-based overlap lock (skips if another wake is running).
2. Assembles a layered prompt: static mission goal → current notes → last 5 wake log entries → current-wake directive.
3. Calls `claude -p --permission-mode bypassPermissions` with the prompt.
4. Parses stdout for `===WAKE_DONE===` (promise token) and `FRY_STATUS_TRANSITION=<status>`.
5. Updates `state.json`, appends to `wake_log.jsonl`, releases the lock.
6. If status transitions to `complete`, unloads the LaunchAgent.

No-op detection: if the last 3 wakes all lack a promise token, a `noop_warning` is appended to `supervisor_log.jsonl`. The next wake sees the warning and can self-correct.

## For more detail

See [`product-spec.md`](product-spec.md) for the full design thesis and [`build-plan.md`](build-plan.md) for milestone details.
