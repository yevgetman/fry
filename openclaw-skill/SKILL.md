---
name: fry
description: "Fry build orchestration via CLI: start, monitor, steer, and resume multi-sprint AI builds. Use when: (1) starting a new build, (2) checking build status or progress, (3) reading build logs, (4) steering a running build with directives, holds, or pauses, (5) resuming a stopped build. NOT for: editing code directly (Fry's agents do that), running tests manually, or git operations."
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

You are the conversational interface to Fry. You help the user start builds,
monitor progress, understand what's happening, and steer builds mid-flight.

## Quick Reference

| Action | Command |
|--------|---------|
| Start a build | `fry run --project-dir <dir> --json-report --telemetry` |
| Check status | `fry status --json --project-dir <dir>` |
| Resume (LLM-driven) | `fry run --continue --project-dir <dir> --json-report --telemetry` |
| Resume (lightweight) | `fry run --simple-continue --project-dir <dir> --json-report --telemetry` |
| Resume (skip to healing) | `fry run --resume --project-dir <dir> --json-report --telemetry` |
| Stream events | `fry events --follow --json --project-dir <dir>` |
| Read latest events | `fry events --json --project-dir <dir>` |
| Consciousness stats | `fry status --consciousness --project-dir <dir>` |
| Clean/archive artifacts | `fry clean --project-dir <dir>` |
| Initialize project | `fry init --project-dir <dir>` |
| Prepare without running | `fry prepare --project-dir <dir>` |

## Starting Builds

**Before starting**, always check if a build is already running:

```bash
fry status --json --project-dir /path/to/project
```

If no build is active (status returns error or shows completed/failed state),
start the build as a **detached background process**. Redirect stderr to a log
file so you can check for startup errors.

```bash
nohup fry run --project-dir /path/to/project \
  --json-report --telemetry \
  > /dev/null 2>/tmp/fry-start.log &
echo "PID: $!"
```

After starting, verify it launched successfully:

```bash
sleep 2 && cat /tmp/fry-start.log
fry status --json --project-dir /path/to/project
```

### With options

```bash
nohup fry run --project-dir /path/to/project \
  --effort high \
  --engine claude \
  --mode software \
  --user-prompt "Focus on the auth module" \
  --json-report --telemetry \
  > /dev/null 2>/tmp/fry-start.log &
```

### Key flags

| Flag | Values | Default | Purpose |
|------|--------|---------|---------|
| `--effort` | low, medium, high, max, auto | auto | Sprint count and rigor |
| `--engine` | claude, codex, ollama | claude | Which LLM engine to use |
| `--mode` | software, planning, writing | software | Build mode |
| `--model` | opus[1m], sonnet, haiku | (engine default) | Override agent model |
| `--git-strategy` | auto, current, branch, worktree | auto | Git branching |
| `--review` | (flag) | off | Enable sprint review |
| `--no-audit` | (flag) | audit on | Disable audits |
| `--always-verify` | (flag) | off | Force all quality gates |
| `--verbose` | (flag) | off | Verbose logging |
| `--planning` | (flag) | off | Planning mode shortcut |
| `--dry-run` | (flag) | off | Preview without executing |
| `--user-prompt` | string | (none) | Additional directive for the build |
| `--user-prompt-file` | path | (none) | Load user prompt from a file |
| `--sarif` | (flag) | off | Write SARIF 2.1.0 audit output |
| `--show-tokens` | (flag) | off | Print per-sprint token usage |

### Effort levels

| Level | Sprints | Alignment budget | Review | Audit | Use case |
|-------|---------|-----------------|--------|-------|----------|
| low | 1-2 | Minimal | No | No | Quick fixes, one-file changes |
| medium | 2-4 | Standard | No | Sprint only | Standard features |
| high | 4-10 | Extended | Yes | Both | Complex features |
| max | Max rigor | Maximum | Yes | Both + deep | Critical/large work |
| auto | Triage decides | Based on triage | Based on triage | Based on triage | Let Fry decide |

## Checking Status

```bash
fry status --json --project-dir /path/to/project
```

Returns structured JSON with sprint number, iteration, status, outcome, and
timing. Use this as the primary way to monitor builds.

### Error handling

- If `fry status` returns a non-zero exit code, the `.fry/` directory may not
  exist (no build has been run) or the project path is wrong.
- If the JSON shows a completed or failed state but you expected it to be
  running, the build process may have died. Check the stderr log from the start
  command and `.fry/build-logs/` for details.

## Reading Build Logs

Build logs are in `.fry/build-logs/`. Read the most recent one:

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

If these files don't exist, no build has started or the build hasn't produced
progress output yet.

## Build Steering (Layer 1)

Fry supports mid-flight steering through file-based IPC. These files live in
the project's `.fry/` directory and are read by the running build process.

### Tier A: Whisper (non-stopping directive)

Inject guidance into the next iteration without stopping the build.
Use `cat <<'EOF'` for safe multi-line content:

```bash
cat <<'DIRECTIVE' > "/path/to/project/.fry/agent-directive.md.tmp"
Focus on error handling in the auth module.
Make sure all database calls have proper timeout handling.
DIRECTIVE
mv "/path/to/project/.fry/agent-directive.md.tmp" "/path/to/project/.fry/agent-directive.md"
```

The directive is picked up on the next iteration and injected into the agent's
prompt. Keep directives under 10KB. The atomic write (`.tmp` then `mv`) prevents
the build from reading a partially written file.

### Tier B: Hold (pause after sprint for decision)

Request the build to checkpoint after the current sprint and wait:

```bash
touch "/path/to/project/.fry/agent-hold-after-sprint"
```

When the build holds, it writes `.fry/decision-needed.md` describing the state.
Read it, then respond with one of:

```bash
# First, read what the build is asking
cat "/path/to/project/.fry/decision-needed.md"

# Option 1: Continue as planned
cat <<'DIRECTIVE' > "/path/to/project/.fry/agent-directive.md.tmp"
continue
DIRECTIVE
mv "/path/to/project/.fry/agent-directive.md.tmp" "/path/to/project/.fry/agent-directive.md"
rm -f "/path/to/project/.fry/decision-needed.md"

# Option 2: Provide direction for next sprint
cat <<'DIRECTIVE' > "/path/to/project/.fry/agent-directive.md.tmp"
Refactor the database layer before adding the API endpoints.
DIRECTIVE
mv "/path/to/project/.fry/agent-directive.md.tmp" "/path/to/project/.fry/agent-directive.md"
rm -f "/path/to/project/.fry/decision-needed.md"

# Option 3: Replan remaining sprints
cat <<'DIRECTIVE' > "/path/to/project/.fry/agent-directive.md.tmp"
replan: Split the remaining work into smaller, more focused sprints.
DIRECTIVE
mv "/path/to/project/.fry/agent-directive.md.tmp" "/path/to/project/.fry/agent-directive.md"
rm -f "/path/to/project/.fry/decision-needed.md"
```

**Important**: Always verify `.fry/decision-needed.md` exists before responding.
If it doesn't, the build is not holding — use Tier A directive instead.

### Tier C: Pause (graceful stop)

Stop the build after the current iteration finishes:

```bash
touch "/path/to/project/.fry/agent-pause"
```

Work is checkpointed via git. Resume with `fry run --continue`.

## Resuming Builds

| Mode | Flag | When to use |
|------|------|-------------|
| LLM-driven | `--continue` | Default. Analyzes build state with an LLM, decides where to resume. |
| Lightweight | `--simple-continue` | Skip LLM analysis, resume from first incomplete sprint. |
| Heal-only | `--resume` | Skip iterations, go straight to sanity checks + alignment. |

```bash
nohup fry run --continue --project-dir /path/to/project \
  --json-report --telemetry \
  > /dev/null 2>/tmp/fry-resume.log &
```

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
| `.fry/sprint-progress.txt` | Current sprint iteration log |
| `.fry/epic-progress.txt` | Compacted summaries of completed sprints |
| `.fry/build-logs/` | Sprint, heal, and audit log files |
| `.fry/observer/events.jsonl` | Full event stream |
| `.fry/verification.md` | Sanity check definitions |
| `.fry/build-report.json` | Structured build report (when `--json-report`) |
| `.fry/agent-directive.md` | Active directive (Layer 1 steering) |
| `.fry/agent-hold-after-sprint` | Hold flag (Layer 1 steering) |
| `.fry/agent-pause` | Pause flag (Layer 1 steering) |
| `.fry/decision-needed.md` | Decision request from held build |

## Behavior Guidelines

- **Always run builds detached** (`nohup ... &`) so they don't block the conversation.
- **Always redirect stderr** to a temp file when starting builds, so startup errors are visible.
- **Check status before starting** — run `fry status --json` to verify no build is active.
- **Use `fry status --json`** to check on builds rather than reading log files first.
- **Atomic file writes** for directives: write to `.tmp` then `mv` to prevent partial reads.
- **Quote all paths** in commands to handle directories with spaces.
- **Don't modify `.fry/` files** other than the steering files documented above.
- **One build per project directory** at a time.
- **Report build outcomes** to the user: sprint pass/fail, alignment attempts, audit findings.
- When a build finishes, summarize: total sprints, pass rate, key audit findings, duration.
