---
name: fry
description: "Fry build orchestration via CLI: start, monitor, steer, and resume multi-sprint AI builds. Use when: (1) starting or setting up a build, (2) checking build status or progress, (3) reading build logs or audit findings, (4) steering a running build with directives, holds, or pauses, (5) resuming a stopped build, (6) preparing epics or plans, (7) running planning-mode or writing-mode builds. NOT for: editing code directly (Fry's agents do that), running tests manually, or git operations."
metadata:
  {
    "openclaw":
      {
        "emoji": "🍳",
        "requires": { "bins": ["fry"] },
      },
  }
---

# Fry Build Orchestration Skill

Fry is a Go CLI that orchestrates AI agents through multi-sprint build loops.
It decomposes tasks into sprints, runs agents, verifies output with sanity
checks, aligns failures, audits code, and reviews cross-sprint coherence.

You are the conversational interface to Fry. You help the user set up projects,
start builds, monitor progress, interpret results, and steer builds mid-flight.

## Quick Reference

| Action | Command |
|--------|---------|
| Initialize project | `fry init --project-dir <dir>` |
| Prepare artifacts only | `fry prepare --project-dir <dir>` |
| Validate prepare | `fry prepare --validate-only --project-dir <dir>` |
| Start a build | `fry run -y --project-dir <dir> --json-report --telemetry` |
| Check status | `fry status --json --project-dir <dir>` |
| Resume (LLM-driven) | `fry run -y --continue --project-dir <dir> --json-report --telemetry` |
| Resume (lightweight) | `fry run -y --simple-continue --project-dir <dir> --json-report --telemetry` |
| Resume (skip to healing) | `fry run -y --resume --project-dir <dir> --json-report --telemetry` |
| Replan after deviation | `fry replan --project-dir <dir>` |
| Stream events | `fry events --follow --json --project-dir <dir>` |
| Consciousness stats | `fry status --consciousness --project-dir <dir>` |
| Trigger reflection | `fry reflect` |
| Print identity | `fry identity` (or `fry identity --full`) |
| Clean/archive build | `fry clean --project-dir <dir>` |
| Triage only | `fry run --triage-only --project-dir <dir>` |
| Dry run | `fry run --dry-run --project-dir <dir>` |

## How to Pass Tasks to Fry

**Let Fry handle its own planning.** Do not generate `plans/plan.md` or
`plans/executive.md` unless the user explicitly asks you to. Fry has its own
triage, prepare, and epic generation pipeline — your job is to pass the task
description and let Fry do the rest.

### Small tasks: use `--user-prompt`

For straightforward requests, pass the task description directly:

```bash
fry run -y --project-dir /path/to/project \
  --user-prompt "Add rate limiting to the API endpoints" \
  --json-report --telemetry
```

Fry will triage, prepare, and run from the prompt alone. No plan files needed.

### Larger tasks: use `--user-prompt-file`

When the task description is longer or multi-part, write it to a temp file
and pass it via `--user-prompt-file`:

```bash
cat <<'PROMPT' > /tmp/fry-task.md
## Task
Refactor the authentication system to support OAuth2.

## Requirements
- Replace session-based auth with JWT tokens
- Add Google and GitHub OAuth providers
- Migrate existing user sessions
- Update all API middleware
PROMPT

fry run -y --project-dir /path/to/project \
  --user-prompt-file /tmp/fry-task.md \
  --json-report --telemetry
```

### Complex builds: pre-scaffolded artifacts

For complex builds, the user will have already set up `plans/plan.md`,
`plans/executive.md`, `assets/`, and `media/` in the project, or will
explicitly ask you to scaffold them. In this case, just run Fry without
a user prompt — it picks up the plan files automatically:

```bash
fry run -y --project-dir /path/to/project --json-report --telemetry
```

### Decision guide

| Scenario | What to do |
|----------|-----------|
| User says "build X" or "fix Y" | Use `--user-prompt` with their request |
| User gives a detailed multi-part task | Write to temp file, use `--user-prompt-file` |
| `plans/plan.md` already exists in project | Run without `--user-prompt` — Fry uses the plan |
| User says "create a plan for..." | Then and only then write `plans/plan.md` |
| User says "set up the executive summary" | Then and only then write `plans/executive.md` |

**Never generate plan.md or executive.md on your own initiative.** These are
user-owned artifacts. If the user hasn't asked for them and they don't exist,
use `--user-prompt` or `--user-prompt-file` instead.

### Supplementary inputs

These directories are user-managed. Only populate them if the user asks:

| Directory | Purpose | What goes in |
|-----------|---------|--------------|
| `assets/` | Text references read in full during prepare | Specs, schemas, requirements, config files (max 512KB/file, 2MB total) |
| `media/` | Binary assets referenced by path | Images, PDFs, fonts, data files (manifest injected, not content) |

### Scaffolding with `fry init`

```bash
fry init --project-dir /path/to/project
```

Creates `plans/`, `assets/`, `media/` directories with a template plan file,
initializes git, and configures `.gitignore` for Fry artifacts. Only run this
when the user asks to initialize a new Fry project.

## Prepare Phase

Generate build artifacts without running a build:

```bash
fry prepare --project-dir /path/to/project
```

This runs the triage gate and generates `.fry/epic.md`, `.fry/AGENTS.md`, and
`.fry/verification.md`. Use `--validate-only` to check existing artifacts
without regenerating.

Prepare respects `--effort`, `--mode`, `--user-prompt`, and `--engine` flags.
Use `--full-prepare` to skip triage and run the full 3-step LLM pipeline
regardless of complexity.

## Triage Gate

Before preparing or running, Fry classifies task complexity:

| Classification | LLM calls | Sprints | Git strategy |
|----------------|-----------|---------|--------------|
| SIMPLE | 0 (template) | 1 | branch |
| MODERATE | 0 (template) | 1-2 | branch |
| COMPLEX | 3-4 (full prepare) | Based on plan | worktree |

To inspect triage without building:

```bash
fry run --triage-only --project-dir /path/to/project
```

## Starting Builds

**Before starting**, always check if a build is already running:

```bash
fry status --json --project-dir /path/to/project
```

### How to launch builds

**Always use a subagent to run Fry builds.** Do not use `nohup ... &` or plain
`cmd &` from a non-interactive shell — the exec environment's ephemeral shell
will clean up backgrounded processes unpredictably.

**Always pass `-y`** to auto-accept all interactive prompts (triage confirmation,
project overview, executive context bootstrap). Without it, the process will
block on stdin and die silently in a non-interactive shell.

**Use sessions_spawn:**

```
sessions_spawn({
  task: "Run this command and report the exit code and last 30 lines of output when done:\n\nfry run -y --project-dir /path/to/project --json-report --telemetry 2>&1 | tee /tmp/fry-out.log",
  runtime: "subagent",
  mode: "run",
  runTimeoutSeconds: 600
})
```

### Key flags

| Flag | Values | Default | Purpose |
|------|--------|---------|---------|
| `-y` / `--yes` | (flag) | off | Auto-accept all interactive prompts. Always use this. |
| `--effort` | low, medium, high, max, auto | auto | Sprint count and rigor |
| `--engine` | claude, codex, ollama | claude | Which LLM engine to use |
| `--mode` | software, planning, writing | software | Build mode |
| `--model` | opus[1m], sonnet, haiku | (engine default) | Override agent model |
| `--git-strategy` | auto, current, branch, worktree | auto | Git branching |
| `--review` | (flag) | off | Enable sprint review between sprints |
| `--no-audit` | (flag) | audit on | Disable sprint and build audits |
| `--always-verify` | (flag) | off | Force all quality gates |
| `--user-prompt` | string | (none) | Additional directive for the build |
| `--user-prompt-file` | path | (none) | Load user prompt from a file |
| `--mcp-config` | path | (none) | MCP server config file (Claude engine only) |
| `--dry-run` | (flag) | off | Preview without executing |
| `--sarif` | (flag) | off | Write SARIF 2.1.0 audit output |
| `--show-tokens` | (flag) | off | Print per-sprint token usage |
| `--verbose` | (flag) | off | Verbose logging |

### Effort levels

| Level | Sprints | Alignment | Review | Audit | Use case |
|-------|---------|-----------|--------|-------|----------|
| low | 1-2 | Skip | No | No | Quick fixes, one-file changes |
| medium | 2-4 | 3 attempts | No | Sprint only | Standard features |
| high | 4-10 | 10 + progress detection | Yes | Both | Complex features |
| max | Max rigor | Unlimited + progress | Yes | Both + deep | Critical/large work |
| auto | Triage decides | Based on triage | Based on triage | Based on triage | Let Fry decide |

### Engine differences

| Engine | CLI | Model tiers | Notes |
|--------|-----|-------------|-------|
| claude | Claude Code | opus[1m] / sonnet / haiku | Default. Supports MCP via `--mcp-config`. |
| codex | Codex CLI | gpt-5.4 / gpt-5.3-codex / gpt-5.4-mini | Alternative. |
| ollama | Ollama | llama3 variants | Local, no API key, no rate-limit detection. |

Override with `--engine <name>`, `@engine` in the epic, or `FRY_ENGINE` env var.

### Git strategies

| Strategy | Behavior | When to use |
|----------|----------|-------------|
| auto | Triage decides: complex → worktree, simple/moderate → branch | Default. Let Fry choose. |
| branch | Creates `fry/<slug>` branch | Standard isolation |
| worktree | Creates isolated checkout at `.fry-worktrees/<slug>/` | Maximum isolation for complex work |
| current | Works on current branch | When you want changes in-place |

## Build Modes

### Software mode (default)

Standard code generation. Sprints produce code changes committed via git.

### Planning mode

Generate documents instead of code. Output goes to `output/` directory.

```bash
fry run --project-dir /path/to/project --mode planning --json-report --telemetry
```

Output files named `{seq}--{category}--{name}.md`. Sanity checks verify
document existence, required sections, and word count minimums. Audit criteria
focus on domain boundaries, analytical frameworks, and document quality.

### Writing mode

Generate long-form content (books, guides). Output goes to `output/` with
final consolidation to `manuscript.md`.

```bash
fry run --project-dir /path/to/project --mode writing --json-report --telemetry
```

Output files named `{seq}--{name}.md`. Audit criteria: coherence, accuracy,
completeness, tone/voice, structure, depth. Resume with `--continue` auto-
detects writing mode from `.fry/build-mode.txt`.

## Checking Status

```bash
fry status --json --project-dir /path/to/project
```

Returns structured JSON with sprint number, iteration, status, outcome, and
timing. Use this as the primary way to monitor builds.

### Error handling

- Non-zero exit code: `.fry/` may not exist or project path is wrong.
- Completed/failed state when expected running: build process died. Check
  `/tmp/fry-out.log` and `.fry/build-logs/` for details.

## Reading Build Logs

Build logs are in `.fry/build-logs/`. Read the most recent:

```bash
ls -t "/path/to/project/.fry/build-logs/"*.log 2>/dev/null | head -1 | xargs -I{} tail -50 "{}"
```

Filter by type:

```bash
# Sprint logs only (exclude iteration and heal sub-logs)
ls "/path/to/project/.fry/build-logs/"sprint*.log 2>/dev/null | grep -v _iter | grep -v _heal | sort | tail -1 | xargs -I{} tail -50 "{}"

# Heal/alignment logs
ls "/path/to/project/.fry/build-logs/"*_heal*.log 2>/dev/null | sort | tail -1 | xargs -I{} tail -50 "{}"

# Audit logs
ls "/path/to/project/.fry/build-logs/"*audit*.log 2>/dev/null | sort | tail -1 | xargs -I{} tail -50 "{}"
```

## Reading Progress

```bash
# Current sprint progress (iteration-level detail)
cat "/path/to/project/.fry/sprint-progress.txt"

# Epic progress (compacted summaries of completed sprints)
cat "/path/to/project/.fry/epic-progress.txt"
```

If these files don't exist, the build hasn't produced progress output yet.

## Understanding Build Outputs

### Sanity checks

Fry verifies sprint output with five check types defined in `.fry/verification.md`:

| Check | Syntax | What it verifies |
|-------|--------|-----------------|
| File exists | `@check_file <path>` | File exists and is non-empty |
| File contains | `@check_file_contains <path> <pattern>` | Grep -E pattern matches |
| Command succeeds | `@check_cmd <command>` | Exit code 0 |
| Command output | `@check_cmd_output <cmd> \| <pattern>` | Stdout matches pattern |
| Tests pass | `@check_test <command>` | Exit 0 and zero test failures |

When checks fail, Fry runs an alignment loop to auto-fix. If alignment stalls,
the sprint may pass with deferred failures (below `@max_fail_percent` threshold).

### Sprint audit

After each sprint (medium effort and above), Fry runs a semantic audit:

- **CRITICAL/HIGH** findings block the sprint — Fry attempts auto-fix.
- **MODERATE** findings get one fix attempt.
- **LOW** findings are advisory (non-blocking except at high/max effort).

Read audit findings:

```bash
cat "/path/to/project/.fry/sprint-audit.txt"
```

### Build audit

After all sprints complete, Fry runs a holistic audit of the entire codebase.
Output is committed to the project:

```bash
cat "/path/to/project/build-audit.md"
```

Use `--sarif` to also generate `build-audit.sarif` in SARIF 2.1.0 format.

### Sprint review

When `--review` is enabled, Fry reviews cross-sprint coherence between sprints.
The reviewer issues one of:

- **CONTINUE** — proceed as planned.
- **DEVIATE** — replan remaining sprints. Fry auto-replans the epic.

Deviation history is logged in `.fry/deviation-log.md`. You can also manually
trigger replanning:

```bash
fry replan --project-dir /path/to/project
```

### Build summary

After a build completes, Fry generates `build-summary.md` at the project root
with a sprint status table, key findings, and overall outcome.

## Epic Format

Fry uses epic files (`.fry/epic.md`) to define sprint structure. These are
auto-generated by `fry prepare` from your plan, but you can also write or edit
them manually.

### Structure

```markdown
@epic My Feature
@engine claude
@effort high
@verification .fry/verification.md

@sprint 1
@name Scaffolding
@max_iterations 20
@promise All foundation files exist and compile
@prompt
Build the project scaffolding:
- Initialize module
- Create config package
- Set up database schema
@end

@sprint 2
@name Core Logic
@max_iterations 25
@prompt
Implement the business logic layer...
@end
```

### Key global directives

| Directive | Purpose |
|-----------|---------|
| `@epic <name>` | Epic title |
| `@engine <claude\|codex\|ollama>` | LLM engine |
| `@effort <low\|medium\|high\|max>` | Effort level |
| `@verification <path>` | Sanity checks file |
| `@model <name>` | Override agent model |
| `@mcp_config <path>` | MCP server config (Claude only) |
| `@review_between_sprints` | Enable sprint review |
| `@audit_after_sprint` | Enable sprint audit (default) |
| `@no_audit` | Disable all audits |
| `@max_heal_attempts <N>` | Alignment attempt limit |
| `@max_fail_percent <N>` | Deferred failure threshold (default 20%) |
| `@docker_from_sprint <N>` | Start Docker from sprint N |
| `@require_tool <name>` | Preflight tool check |
| `@pre_sprint <cmd>` | Run command before each sprint |
| `@pre_iteration <cmd>` | Run command before each iteration |

### Sprint directives

| Directive | Purpose |
|-----------|---------|
| `@sprint <N>` | Sprint number (sequential) |
| `@name <text>` | Sprint name |
| `@max_iterations <N>` | Iteration limit |
| `@promise <text>` | Success criteria (checked by agent) |
| `@prompt` | Start of sprint prompt (multi-line) |
| `@end` | End of sprint block |

## Build Steering (Layer 1)

Fry supports mid-flight steering through file-based IPC in `.fry/`.

### Tier A: Whisper (non-stopping directive)

Inject guidance into the next iteration without stopping:

```bash
cat <<'DIRECTIVE' > "/path/to/project/.fry/agent-directive.md.tmp"
Focus on error handling in the auth module.
Make sure all database calls have proper timeout handling.
DIRECTIVE
mv "/path/to/project/.fry/agent-directive.md.tmp" "/path/to/project/.fry/agent-directive.md"
```

Picked up on the next iteration. Keep under 10KB. Atomic write prevents
partial reads.

### Tier B: Hold (pause after sprint for decision)

```bash
touch "/path/to/project/.fry/agent-hold-after-sprint"
```

When the build holds, it writes `.fry/decision-needed.md`. Read it, then
respond:

```bash
cat "/path/to/project/.fry/decision-needed.md"

# Continue as planned
cat <<'DIRECTIVE' > "/path/to/project/.fry/agent-directive.md.tmp"
continue
DIRECTIVE
mv "/path/to/project/.fry/agent-directive.md.tmp" "/path/to/project/.fry/agent-directive.md"
rm -f "/path/to/project/.fry/decision-needed.md"

# Or provide new direction
cat <<'DIRECTIVE' > "/path/to/project/.fry/agent-directive.md.tmp"
Refactor the database layer before adding the API endpoints.
DIRECTIVE
mv "/path/to/project/.fry/agent-directive.md.tmp" "/path/to/project/.fry/agent-directive.md"
rm -f "/path/to/project/.fry/decision-needed.md"

# Or replan remaining sprints
cat <<'DIRECTIVE' > "/path/to/project/.fry/agent-directive.md.tmp"
replan: Split the remaining work into smaller sprints.
DIRECTIVE
mv "/path/to/project/.fry/agent-directive.md.tmp" "/path/to/project/.fry/agent-directive.md"
rm -f "/path/to/project/.fry/decision-needed.md"
```

**Always verify `.fry/decision-needed.md` exists before responding.** If it
doesn't, the build is not holding — use Tier A directive instead.

### Tier C: Pause (graceful stop)

```bash
touch "/path/to/project/.fry/agent-pause"
```

Work is checkpointed via git. Resume with `fry run --continue`.

## Resuming Builds

| Mode | Flag | When to use |
|------|------|-------------|
| LLM-driven | `--continue` | Default. Analyzes build state, decides where to resume. |
| Lightweight | `--simple-continue` | Skip LLM analysis, resume from first incomplete sprint. |
| Heal-only | `--resume` | Skip iterations, go straight to sanity checks + alignment. |
| From sprint N | `--sprint N` | Fresh start from specific sprint number. |

Use `sessions_spawn` to resume, same as starting a build:

```
sessions_spawn({
  task: "Run this command and report the exit code and last 30 lines of output when done:\n\nfry run -y --continue --project-dir /path/to/project --json-report --telemetry 2>&1 | tee /tmp/fry-out.log",
  runtime: "subagent",
  mode: "run",
  runTimeoutSeconds: 600
})
```

Resume auto-detects the build mode (software/planning/writing) and git
strategy (branch/worktree) from `.fry/build-mode.txt` and `.fry/git-strategy.txt`.

## Consciousness Pipeline

Fry has an introspective system that synthesizes build experiences:

```bash
# View consciousness status
fry status --consciousness --project-dir /path/to/project

# Print Fry's compiled identity
fry identity
fry identity --full

# Trigger reflection from accumulated experiences
fry reflect
```

Experiences are stored in `~/.fry/experiences/`. The `--telemetry` flag
enables experience upload. Reflection runs weekly and updates Fry's identity.

## Build Events

Stream live events from a running build:

```bash
fry events --follow --json --project-dir /path/to/project
```

Key event types: `sprint_start`, `sprint_complete`, `alignment_complete`,
`audit_complete`, `review_complete`, `build_end`, `directive_received`,
`decision_needed`, `build_paused`.

## Artifact Paths

| Path | Content |
|------|---------|
| `plans/plan.md` | Build plan (user input) |
| `plans/executive.md` | Strategic context (user input, optional) |
| `assets/` | Text reference documents (user input, optional) |
| `media/` | Binary assets (user input, optional) |
| `.fry/epic.md` | Generated epic with sprint definitions |
| `.fry/AGENTS.md` | Generated agent instructions |
| `.fry/verification.md` | Sanity check definitions |
| `.fry/sprint-progress.txt` | Current sprint iteration log |
| `.fry/epic-progress.txt` | Compacted summaries of completed sprints |
| `.fry/build-logs/` | Sprint, heal, and audit log files |
| `.fry/build-report.json` | Structured build report (when `--json-report`) |
| `.fry/sprint-audit.txt` | Current sprint audit findings (transient) |
| `.fry/observer/events.jsonl` | Full event stream |
| `.fry/deviation-log.md` | Sprint review deviation history |
| `.fry/build-mode.txt` | Active build mode (software/planning/writing) |
| `.fry/git-strategy.txt` | Active git strategy |
| `build-summary.md` | Build summary (project root, committed) |
| `build-audit.md` | Build audit findings (project root, committed) |
| `output/` | Planning/writing mode output directory |
| `.fry/agent-directive.md` | Active directive (Layer 1 steering) |
| `.fry/agent-hold-after-sprint` | Hold flag (Layer 1 steering) |
| `.fry/agent-pause` | Pause flag (Layer 1 steering) |
| `.fry/decision-needed.md` | Decision request from held build |

## Cleaning Up

Archive completed build artifacts to `.fry-archive/`:

```bash
fry clean --project-dir /path/to/project
```

Creates a timestamped snapshot at `.fry-archive/.fry--build--YYYYMMDD-HHMMSS/`
and removes the `.fry/` directory. The project-root outputs (`build-summary.md`,
`build-audit.md`) are preserved.

## Behavior Guidelines

- **Always use `sessions_spawn`** to run Fry builds — never `nohup` or `&`.
- **Always pass `-y`** to auto-accept all interactive prompts.
- **Check status before starting** — verify no build is active first.
- **Use `fry status --json`** as the primary monitoring tool.
- **Atomic file writes** for directives: write to `.tmp` then `mv`.
- **Quote all paths** to handle directories with spaces.
- **Don't modify `.fry/` files** other than the steering files.
- **One build per project directory** at a time.
- **Report build outcomes**: sprint pass/fail, alignment attempts, audit
  findings, review verdicts, total duration.
- When a build finishes, read `build-summary.md` and `build-audit.md` and
  summarize the key findings for the user.
