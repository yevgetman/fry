# Agent Foundation

Fry includes an agent foundation layer (`internal/agent/`) that provides the building blocks for conversational interfaces to the Fry build engine. The agent package is consumed by:

1. **The OpenClaw skill** (`openclaw-skill/`) — a markdown-based skill that teaches an OpenClaw agent how to use the Fry CLI
2. **Future native agent** — via direct Go imports (planned)

## Connecting OpenClaw to Fry

OpenClaw integration uses a **skill** — a documentation file (`SKILL.md`) that gets injected into the agent's prompt when relevant. The skill teaches the agent every Fry CLI command, flag, artifact path, and steering mechanism. No gateway plugins or code runs in the OpenClaw process.

### Prerequisites

- Fry binary installed and in your PATH (verify: `fry version`)
- OpenClaw gateway running on your machine (verify: `openclaw health`)

### Step 1: Install the skill

Copy the skill into OpenClaw's managed skills directory:

```bash
mkdir -p ~/.openclaw/skills/fry
cp /path/to/fry/openclaw-skill/SKILL.md ~/.openclaw/skills/fry/SKILL.md
```

Or install into the workspace skills directory (per-agent):

```bash
mkdir -p ~/.openclaw/workspace/skills/fry
cp /path/to/fry/openclaw-skill/SKILL.md ~/.openclaw/workspace/skills/fry/SKILL.md
```

### Step 2: Restart the gateway

```bash
openclaw gateway restart
```

### Step 3: Verify

Check that the skill loaded:

```bash
openclaw skills list
```

You should see the fry skill listed as `ready` with the frying pan emoji.

### Step 4: Start a build

From any messaging channel connected to OpenClaw:

```
start a build on ~/code/myproject with standard effort
```

The agent will use the Fry CLI to start the build, detaching it as a background process, and use `fry status --json` to monitor progress.

### What the Skill Provides

The skill teaches the OpenClaw agent how to:

| Capability | How |
|------------|-----|
| **Set up projects** | `fry init`, create `plans/plan.md`, configure `assets/` and `media/` |
| **Prepare builds** | `fry prepare` to generate epics, agent instructions, and sanity checks |
| **Start builds** | `fry run` as a detached background process with appropriate flags |
| **Monitor builds** | `fry status --json`, read logs, read progress files, stream events |
| **Steer builds** | Write directives, hold after sprints, or stop gracefully with `fry exit` and structured resume points |
| **Resume builds** | `fry run --continue`, `--simple-continue`, `--resume`, or `--sprint N` |
| **Interpret results** | Read audit findings, review verdicts, build summaries |
| **Use all modes** | Software, planning, and writing mode workflows |
| **Replan** | `fry replan` after deviations or review verdicts |
| **Clean up** | `fry clean` to archive completed builds |
| **Consciousness** | `fry status --consciousness`, `fry reflect`, `fry identity` |

### Skill Coverage

The skill covers the full Fry CLI surface:

- **All commands**: `run`, `prepare`, `replan`, `clean`, `init`, `status`, `events`, `exit`, `identity`, `reflect`
- **All flags**: `--effort`, `--engine`, `--mode`, `--model`, `--git-strategy`, `--review`, `--no-audit`, `--continue`, `--resume`, `--simple-continue`, `--sprint`, `--user-prompt`, `--user-prompt-file`, `--mcp-config`, `--dry-run`, `--sarif`, `--json-report`, `--telemetry`, `--triage-only`, `--full-prepare`, `--always-verify`, `--verbose`, `--show-tokens`
- **All build stages**: Triage, prepare, preflight, sprint execution, sanity checks, alignment, audit, review, build summary
- **All steering tiers**: Whisper (directive), Hold (sprint boundary), Pause (graceful stop)
- **All build modes**: Software, planning, writing
- **All artifact paths**: Input files (`plans/`, `assets/`, `media/`), generated artifacts (`.fry/`), output files (`build-summary.md`, `build-audit.md`, `output/`)
- **Epic format**: Global directives, sprint directives, sanity check types

### Skill vs. Plugin

The Fry integration was originally built as a TypeScript plugin (`openclaw-extension/`) that registered native tools in the OpenClaw gateway. This was replaced with a skill-based approach because:

1. **Gateway stability**: External plugins have a known race condition with channel initialization in OpenClaw (see [openclaw/openclaw#56626](https://github.com/openclaw/openclaw/issues/56626)). Skills have zero gateway impact.
2. **Simplicity**: A skill is a single markdown file. No dependencies, no build step, no TypeScript, no `node_modules`.
3. **Equivalent capability**: The agent achieves the same results by running Fry CLI commands via its exec tool as it did with dedicated plugin tools. The CLI commands are identical.
4. **One trade-off**: Skills cannot push proactive notifications. The plugin's build watcher spawned a background `fry events --follow` process that sent updates unprompted. With the skill, the agent monitors builds via polling (`fry status --json`) or when asked. A cron job can fill this gap.

### Proactive Monitoring via Cron

To replicate the plugin's proactive notifications, set up an OpenClaw cron job that polls build status:

From any OpenClaw channel:

```
set up a cron job that runs every 60 seconds: check fry status --json
for all active builds and notify me of any sprint completions, failures,
or build completion
```

---

## CLI Commands

### `fry status --json`

Returns structured JSON build state. Used by the OpenClaw agent to answer "how's the build going?" without parsing text output.

```bash
$ fry status --json --project-dir ~/code/myproject
{
  "active": true,
  "project_dir": "/Users/yev/code/myproject",
  "epic": "REST API Build",
  "effort": "standard",
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
$ fry events --project-dir ~/code/myproject

# Stream events in real-time (follows events.jsonl)
$ fry events --follow --json --project-dir ~/code/myproject
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

File-based IPC for mid-build human intervention. The skill teaches the OpenClaw agent to write steering files; `fry exit` writes a structured graceful-exit request; the runtime reads those signals and settles deterministic resume points. Atomic writes prevent partial directives or checkpoints.

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
- **`RequestExit(projectDir)`** — Create `.fry/exit-request.json` for `fry exit`
- **`ReadStopRequest(projectDir)`** — Resolve the effective graceful-stop signal (`fry exit` request or legacy pause sentinel)
- **`WriteResumePoint(projectDir, point)`** — Persist `.fry/resume-point.json` for deterministic pickup
- **`ReadResumePoint(projectDir)`** — Read the settled resume checkpoint
- **`ClearStopRequest(projectDir)`** — Remove the graceful-stop request artifacts
- **`CleanupAll(projectDir)`** — Remove all steering files (called at build completion)

---

## Build Steering Artifacts

These files are created by the steering system:

| File | Purpose | Written By | Read By |
|------|---------|-----------|---------|
| `.fry/agent-directive.md` | User directive for next iteration | OpenClaw agent (via skill) | Sprint loop |
| `.fry/agent-hold-after-sprint` | Hold flag (sentinel) | OpenClaw agent (via skill) | Inter-sprint loop |
| `.fry/agent-pause` | Legacy pause flag (sentinel) | OpenClaw agent (via skill) | Sprint/alignment/audit control flow |
| `.fry/exit-request.json` | Structured graceful-exit request | `fry exit` | Runtime graceful-exit checkpoints |
| `.fry/resume-point.json` | Settled resume checkpoint | Fry runtime | `--continue`, `--simple-continue`, humans, agents |
| `.fry/decision-needed.md` | Build waiting for human input | Sprint loop | OpenClaw agent (via skill) |

## Build Steering Events

| Event | When | Key Data |
|-------|------|----------|
| `directive_received` | Sprint loop read a directive | `preview` |
| `decision_needed` | Build holding for user decision | `reason`, `completed_sprint`, `remaining_sprints` |
| `decision_received` | User responded to hold | `preview` |
| `build_paused` | Build stopped at a settled checkpoint | `sprint`, `phase`, `detail` |

---

## Future: Native Agent

The `internal/agent/` and `internal/steering/` packages are designed as the foundation for a native `fry agent` command that does not require OpenClaw. The path:

1. **Done (Layer 0)**: Domain types, state readers, event streaming, artifact schema, prompt generation
2. **Done (Layer 1)**: Build steering — file-based IPC for directives, holds, graceful exits, and structured resume points
3. **Future**: LLM client (`internal/agent/llm/`), channel adapters (`internal/agent/channels/`), `fry agent` command
