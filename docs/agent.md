# Agent Foundation

Fry includes an agent foundation layer (`internal/agent/`) that provides the building blocks for conversational interfaces to the Fry build engine. The agent package is consumed by:

1. **The OpenClaw extension** (`openclaw-extension/`) — via CLI commands
2. **Future native agent** — via direct Go imports (planned)

## Connecting OpenClaw to Fry

### Prerequisites

- Fry binary installed and in your PATH (verify: `fry version`)
- OpenClaw gateway running on your machine (verify: `openclaw channels status`)

### Step 1: Install the extension

Add the Fry extension to your OpenClaw config. Edit `~/.openclaw/config.yaml` (or your OpenClaw config file):

```yaml
plugins:
  entries:
    fry:
      enabled: true
      source: "/path/to/fry/openclaw-extension"
      config:
        fry_binary: "fry"           # path to fry binary (default: from PATH)
        default_effort: "auto"      # auto | low | medium | high | max
        default_engine: "claude"    # claude | codex | ollama
        notifications: "milestones" # all | milestones | errors
```

Replace `/path/to/fry` with the actual path to your Fry repository (e.g., `~/Sites/fry`).

### Step 2: Enable the tools on your agent

In your OpenClaw agent configuration, allow the Fry tools:

```yaml
agents:
  - id: main
    tools:
      allow:
        # Monitoring (Layer 0)
        - fry_build_start
        - fry_build_status
        - fry_build_restart
        - fry_build_logs
        - fry_read_progress
        - fry_consciousness_stats
        # Build steering (Layer 1)
        - fry_build_directive
        - fry_build_hold
        - fry_build_respond
        - fry_build_pause
```

### Step 3: Restart the gateway

Restart your OpenClaw gateway to pick up the new extension:

```bash
openclaw gateway restart
# or restart via the OpenClaw Mac app
```

### Step 4: Verify

From any OpenClaw channel (Telegram, Discord, iMessage, web chat, etc.), send:

```
hey fry, what's your status?
```

The agent should respond using the Fry tools. You can also verify the tools loaded:

```bash
# Check the extension registered
openclaw plugins list

# Test the CLI commands directly
fry status --json
fry agent prompt | head -20
fry events
```

### Step 5: Start a build

From any messaging channel connected to OpenClaw:

```
start a build on ~/Sites/myproject with medium effort
```

The agent will call `fry_build_start`, spawn the build process, and start the watcher for proactive notifications. You'll receive milestone updates as the build progresses.

### Notification Levels

Configure how much the agent reports proactively:

| Level | What's reported |
|-------|----------------|
| `all` | Every sprint start/complete, heal attempt, audit finding, build event |
| `milestones` (default) | Sprint completions, build end, errors |
| `errors` | Only failures and build end |

Set in the plugin config: `notifications: "milestones"`

### Troubleshooting

**"Could not load Fry system prompt"** — The `fry` binary isn't found. Check `fry_binary` in your plugin config, or ensure `fry` is in your PATH.

**No proactive notifications** — Verify the build watcher started. Check OpenClaw logs for `[fry] Build watcher ready`. If using `notifications: "errors"`, you'll only see failures.

**"No build events found"** — No `.fry/observer/events.jsonl` exists in the project directory. Start a build first, or check `--project-dir` points to the right location.

**Extension not loading** — Verify the `source` path in your plugin config points to the `openclaw-extension/` directory inside the Fry repo, not the Fry repo root.

---

## CLI Commands

### `fry status --json`

Returns structured JSON build state. Used by the OpenClaw extension to answer "how's the build going?" without parsing text output.

```bash
$ fry status --json --project-dir ~/Sites/myproject
{
  "active": true,
  "project_dir": "/Users/yev/Sites/myproject",
  "epic": "REST API Build",
  "effort": "medium",
  "engine": "claude",
  "total_sprints": 4,
  "current_sprint": 2,
  "current_sprint_name": "API Endpoints",
  "status": "running",
  "last_event": {
    "type": "sprint_complete",
    "ts": "2026-03-27T10:31:45Z",
    "sprint": 1,
    "data": {"status": "PASS", "heal_attempts": "0"}
  },
  "git_branch": "fry/rest-api",
  "started_at": "2026-03-27T10:00:00Z",
  "pid": 12345
}
```

### `fry events`

List or stream build events from the observer event log.

```bash
# List all events
$ fry events --project-dir ~/Sites/myproject

# Stream events in real-time (follows events.jsonl)
$ fry events --follow --json --project-dir ~/Sites/myproject
{"ts":"2026-03-27T10:31:45Z","type":"sprint_complete","sprint":1,"data":{"status":"PASS"}}
{"ts":"2026-03-27T10:35:00Z","type":"sprint_start","sprint":2,"data":{"name":"API Endpoints"}}
```

### `fry agent prompt`

Print the agent system prompt. This is the canonical prompt that makes any LLM "speak Fry" — it includes the artifact schema, build lifecycle, event types, identity, and conversation patterns.

```bash
$ fry agent prompt
# Identity
...
# Role
...
# Build Lifecycle
...
```

---

## Go Packages

### `internal/agent/` — Agent Foundation

Types:
- **`BuildState`** — Structured representation of a running or completed build
- **`BuildEvent`** — A single event from the observer event stream
- **`ArtifactInfo`** — Description of a Fry build artifact (for prompt generation)

Functions:
- **`ReadBuildState(projectDir)`** — Assembles state from `.fry/` artifacts
- **`ReadProgress(projectDir, scope)`** — Reads sprint or epic progress
- **`ReadLatestLog(projectDir, logType, lines)`** — Reads recent build logs
- **`TailEvents(ctx, projectDir)`** — Follows events.jsonl, returns Go channel
- **`ReadAllEvents(projectDir)`** — Reads all events at once
- **`ArtifactSchema()`** — Returns complete artifact manifest
- **`BuildAgentSystemPrompt()`** — Generates the Fry agent system prompt

### `internal/steering/` — Build Steering (Layer 1)

File-based IPC for mid-build human intervention. The extension writes files; the sprint loop reads them. All operations are atomic (rename-based) to avoid TOCTOU races.

Functions:
- **`ConsumeDirective(projectDir)`** — Atomically read and delete the directive file (rename + read + remove)
- **`ReadDirective(projectDir)`** — Read directive without consuming (for inspection)
- **`ClearDirective(projectDir)`** — Delete the directive file
- **`IsHoldRequested(projectDir)`** — Check if the hold sentinel exists
- **`ClearHold(projectDir)`** — Remove the hold sentinel
- **`WriteDecisionNeeded(projectDir, prompt)`** — Create the decision-needed file (atomic write)
- **`ClearDecisionNeeded(projectDir)`** — Remove the decision-needed file
- **`WaitForDecision(ctx, projectDir)`** — Block until a directive appears (polls every 2s via ticker)
- **`IsPaused(projectDir)`** — Check if the pause sentinel exists
- **`ClearPause(projectDir)`** — Remove the pause sentinel
- **`CleanupAll(projectDir)`** — Remove all steering files (called at build completion)

---

## OpenClaw Extension Tools

### Monitoring (Layer 0)

| Tool | Description |
|------|-------------|
| `fry_build_start` | Start a new Fry build. Spawns `fry run` as a subprocess and starts the event watcher. |
| `fry_build_status` | Get structured JSON build state (calls `fry status --json`). |
| `fry_build_restart` | Restart a failed/stopped build with `--continue`, `--resume`, or `--simple-continue`. Accepts `user_prompt` for new direction. |
| `fry_build_logs` | Read recent sprint, heal, or audit logs from `.fry/build-logs/`. |
| `fry_read_progress` | Read sprint-progress.txt or epic-progress.txt. |
| `fry_consciousness_stats` | Query consciousness pipeline status (memory counts, reflection, identity version). |

### Build Steering (Layer 1)

| Tool | Tier | Description |
|------|------|-------------|
| `fry_build_directive` | A (whisper) | Inject a directive into the next iteration's prompt. Build doesn't stop. |
| `fry_build_hold` | B (hold) | Request the build to pause after the current sprint completes. |
| `fry_build_respond` | B (hold) | Respond to a held build: "continue", directive for next sprint, or "replan: instructions". |
| `fry_build_pause` | C (abort) | Stop the build gracefully after the current iteration. Work is checkpointed. |

### Steering Tiers

**Tier A — Whisper**: Inject a note into the next iteration via `fry_build_directive`. The sprint agent sees it as an extra prompt section. Safe, no structural change.

**Tier B — Hold at Sprint Boundary**: Call `fry_build_hold` to pause after the current sprint. You'll get a notification with a summary. Respond via `fry_build_respond` with:
- `"continue"` — proceed as planned
- A directive — injected as context for the next sprint
- `"replan: <instructions>"` — replan remaining sprints using Fry's review/replan system

**Tier C — Abort**: Call `fry_build_pause` to stop after the current iteration finishes. Work is git-checkpointed. Resume with `fry_build_restart` and optionally pass `user_prompt` for new direction.

## Build Steering Artifacts

These files are created by the steering system:

| File | Purpose | Written By | Read By |
|------|---------|-----------|---------|
| `.fry/agent-directive.md` | User directive for next iteration | Extension | Sprint loop |
| `.fry/agent-hold-after-sprint` | Hold flag (sentinel) | Extension | Inter-sprint loop |
| `.fry/agent-pause` | Pause flag (sentinel) | Extension | Sprint loop |
| `.fry/decision-needed.md` | Build waiting for human input | Sprint loop | Extension |

## Build Steering Events

| Event | When | Key Data |
|-------|------|----------|
| `directive_received` | Sprint loop read a directive | `preview` |
| `decision_needed` | Build holding for user decision | `reason`, `completed_sprint`, `remaining_sprints` |
| `decision_received` | User responded to hold | `preview` |
| `build_paused` | Build stopped after iteration | `sprint`, `iteration` |

---

## Future: Native Agent

The `internal/agent/` and `internal/steering/` packages are designed as the foundation for a native `fry agent` command that does not require OpenClaw. The path:

1. **Done (Layer 0)**: Domain types, state readers, event streaming, artifact schema, prompt generation
2. **Done (Layer 1)**: Build steering — file-based IPC for directives, holds, pauses, and replans
3. **Future**: LLM client (`internal/agent/llm/`), channel adapters (`internal/agent/channels/`), `fry agent` command
