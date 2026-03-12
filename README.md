# fry

An automated build orchestration engine that uses AI agents to execute complex plans autonomously. Write a plan, and fry decomposes it into sprints and executes them -- one iteration at a time, fresh context each pass -- until the project is done.

Works for **software projects** (code, tests, infrastructure) and **planning projects** (business plans, research reports, strategic analyses, trip planning -- any endeavor requiring rigorous phased document creation). See [Planning Mode](docs/planning-mode.md).

Supports **OpenAI Codex** (default) and **Claude Code** as interchangeable AI engines.

> This is the Go rewrite of [fry](https://github.com/yevgetman/fry). Same capabilities, single static binary, no bash 4.0+ dependency.

## How It Works

```
plans/plan.md          You write this -- what to build        (at least one
  OR                                                           of these two
plans/executive.md     You write this -- why to build it       is required)
        |
        v
  fry prepare           Step 0 (if needed): AI generates plans/plan.md from executive.md
                         Steps 1-3: AI generates .fry/AGENTS.md + .fry/epic.md + .fry/verification.md
  (pass --planning for non-code projects)
        |
        v
     fry run             Executes sprints via AI agent loop
        |                + runs independent verification checks
        |                + auto-heals on verification failure
        v
  Working software       Git-checkpointed after each sprint
  (or planning docs)
```

fry adopts the "Ralph Wiggum Loop" pattern: each sprint runs as an iterative loop where the AI agent gets a prompt, does work, and logs progress. The next iteration reads what the previous one accomplished and continues. When the agent signals completion (via a promise token), the sprint ends and the next one begins.

**Key mechanisms:**

- **Effort-level triage** -- `--effort low|medium|high|max` controls sprint count, density, and rigor. Auto-detects when unspecified. See [Effort Levels](docs/effort-levels.md).
- **Layered prompts** -- assembled per sprint with executive context, user directives, plan references, sprint tasks, iteration memory, and completion signals
- **Two-file progress tracking** -- per-sprint iteration log + cross-sprint compacted summary for bounded context
- **Promise tokens** -- `===PROMISE: TOKEN===` signals sprint completion
- **Independent verification** -- machine-executable checks run after each sprint
- **Self-healing** -- automatic re-runs with targeted fix prompts on verification failure
- **Git checkpoints** -- automatic commits after each sprint
- **Dynamic sprint review** -- optional mid-build review with replanning

## Quick Start

```bash
# Install
git clone https://github.com/yevgetman/fry.git && cd fry
make install

# Create a plan
mkdir -p plans
cat > plans/plan.md << 'EOF'
# My Project -- Build Plan
**Stack:** Node 20, Express, PostgreSQL 16, TypeScript strict mode.
...
EOF

# Run
fry --engine claude

# Validate without running
fry --dry-run
```

See [Getting Started](docs/getting-started.md) for full setup instructions.

## Commands

| Command | Description |
|---|---|
| `fry run` | Execute sprints from an epic file (default command) |
| `fry prepare` | Generate `.fry/AGENTS.md`, `.fry/epic.md`, and `.fry/verification.md` from your plan |
| `fry replan` | Replan an epic after a deviation |
| `fry version` | Print fry version |

```bash
fry                                    # Run all sprints
fry --engine claude                    # Use Claude Code
fry --effort low                       # Simple task: 1-2 sprints, minimal overhead
fry --effort max --engine claude       # Maximum rigor: extended prompts, thorough reviews
fry run epic.md 3 5                    # Run sprints 3-5
fry --planning --engine claude         # Planning mode (documents, not code)
fry --user-prompt "no ORMs, raw SQL"   # Inject a directive
fry prepare --effort medium            # Generate artifacts with medium effort sizing
```

See [Commands](docs/commands.md) for complete flag and argument reference.

## Documentation

| Document | Description |
|---|---|
| [Getting Started](docs/getting-started.md) | Prerequisites, installation, first build walkthrough |
| [Commands](docs/commands.md) | Full CLI reference: `run`, `prepare`, `replan`, `version` |
| [Effort Levels](docs/effort-levels.md) | Effort triage: `low`, `medium`, `high`, `max` -- controls sprint count, density, and review rigor |
| [Epic Format](docs/epic-format.md) | Epic file syntax: global directives, sprint blocks, validation rules, sizing guidelines |
| [AI Engines](docs/engines.md) | Codex and Claude engine configuration, mixing engines, model overrides |
| [Sprint Execution](docs/sprint-execution.md) | Agent iteration loop, prompt assembly, progress tracking, promise tokens |
| [Verification](docs/verification.md) | Check primitives, file format, outcome matrix, graceful degradation |
| [Self-Healing](docs/self-healing.md) | Heal loop mechanics, configuration, diagnostics |
| [Sprint Review](docs/sprint-review.md) | Dynamic mid-build review, replanning, deviation specs, safeguards |
| [Docker Support](docs/docker.md) | Docker Compose lifecycle, health checks, sprint scoping |
| [Preflight Checks](docs/preflight.md) | Pre-build validation, required tools, custom commands |
| [Planning Mode](docs/planning-mode.md) | Non-code project support: documents, analyses, strategies |
| [User Prompt](docs/user-prompt.md) | Injecting directives, prompt hierarchy, persistence |
| [Project Structure](docs/project-structure.md) | Directory layout, generated artifacts, file reference |
| [Terminal Output](docs/terminal-output.md) | Status banners, verbose mode, log format |
| [Architecture](docs/architecture.md) | Internal package structure, data flow, build system |

## Requirements

- **git** -- for automatic checkpointing
- **bash** -- required by AI engine CLIs and verification commands
- **Go 1.22+** -- to build from source
- **OpenAI Codex CLI** (`npm i -g @openai/codex`) -- if using the codex engine
- **Claude Code CLI** (`npm i -g @anthropic-ai/claude-code`) -- if using the claude engine
- **Docker** (optional) -- only needed if your epic uses `@docker_from_sprint`

## License

See repository for license information.
