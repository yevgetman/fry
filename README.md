# fry 🍳

![fry](bestfry-1.jpg)

< The following is written by a human >

Fry is an autonomous document engine built for long-run coding tasks but capable of so much more. Fry is designed for generating a large corpus of documents -- one-shot run on large complex codebases that actually work, yes, but also research documents, strategic analyses, business plans, it'll even write you an complete book. And the thing is, it actaully doesnt suck. 

</ end of stuff written by human >

It breaks your plan into sprints, runs each one through an AI agent loop, then verifies the output with machine-executable checks. If a check fails, it re-runs the sprint with a targeted fix prompt. An effort system sizes the run to match the task — a small fix gets one sprint, a large project gets phased execution with mid-build review and dynamic replanning. After each sprint, a separate AI agent audits the work and blocks the build on critical issues. Once all sprints finish, a final holistic audit reviews the full output. Every sprint is git-checkpointed automatically.


> This is the Go rewrite of [Fry](https://github.com/yevgetman/fry). Same capabilities, single static binary, no bash 4.0+ dependency.

## How It Works

```
plans/plan.md          You write this -- what to build        (at least one of
  OR                                                           these three is
plans/executive.md     You write this -- why to build it       required)
  OR
--user-prompt "..."    You describe it -- Fry generates the rest
  OR
--user-prompt-file f   Same, but reads the prompt from a local file
media/                 Optional binary assets (images, PDFs, fonts) referenced in plans
assets/                Optional text documents (specs, schemas) read during plan generation
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
  (or plans/output/)     Planning docs use ordered filenames:
                           1--research--market-landscape.md
                           2--analysis--positioning-options.md
```

Each sprint runs as an iterative loop where the AI agent gets a prompt, does work, and logs progress. The next iteration reads what the previous one accomplished and continues. When the agent signals completion (via a promise token), the sprint ends and the next one begins.

**Key mechanisms:**

- **Effort-level triage** -- `--effort low|medium|high|max` controls sprint count, density, and rigor. Auto-detects when unspecified. See [Effort Levels](docs/effort-levels.md).
- **Media assets** -- optional `media/` directory for images, PDFs, fonts, and other files referenced in plans and copied into builds
- **Supplementary assets** -- optional `assets/` directory for text reference documents (specs, schemas, requirements) whose full contents are read during plan and epic generation
- **Layered prompts** -- assembled per sprint with executive context, media manifest, user directives, plan references, sprint tasks, iteration memory, and completion signals
- **Two-file progress tracking** -- per-sprint iteration log + cross-sprint compacted summary for bounded context
- **Promise tokens** -- `===PROMISE: TOKEN===` signals sprint completion
- **Independent verification** -- machine-executable checks run after each sprint
- **Self-healing** -- automatic re-runs with targeted fix prompts on verification failure
- **Sprint audit** -- post-sprint semantic review by a separate AI agent, with automatic fix loop (CRITICAL/HIGH block the build; MODERATE is advisory)
- **Build audit** -- final holistic codebase audit after the entire epic completes, with iterative remediation (up to 10 passes)
- **Build summary** -- comprehensive `build-summary.md` generated after all sprints, covering what was built, events, audit findings, and advisories
- **Git checkpoints** -- automatic commits after each sprint
- **Dynamic sprint review** -- optional mid-build review with replanning

## Quick Start

```bash
# Install
git clone https://github.com/yevgetman/fry.git && cd fry
make install

# Option A: Start from just a prompt (no files needed)
fry --user-prompt "build a REST API for a todo app with PostgreSQL" --engine claude

# Option B: Create a plan file first
mkdir -p plans
cat > plans/plan.md << 'EOF'
# My Project -- Build Plan
**Stack:** Node 20, Express, PostgreSQL 16, TypeScript strict mode.
...
EOF
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
fry                                    # Run all sprints (prepare: claude, build: codex)
fry --engine claude                    # Use Claude Code for build stage
fry --effort low                       # Simple task: 1-2 sprints, minimal overhead
fry --effort max --engine claude       # Maximum rigor: extended prompts, thorough reviews
fry run epic.md 3 5                    # Run sprints 3-5
fry --planning                         # Planning mode (documents, not code) — claude for both stages
fry --user-prompt "no ORMs, raw SQL"   # Inject a directive
fry --user-prompt "build a todo app"  # Start from just a prompt (no plan files needed)
fry --user-prompt-file ./prompt.txt   # Load a longer prompt from a file
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
| [Sprint Audit](docs/sprint-audit.md) | Post-sprint semantic code review by AI, audit/fix loop, severity classification |
| [Build Audit](docs/build-audit.md) | Final holistic codebase audit after epic completion, iterative remediation |
| [Sprint Review](docs/sprint-review.md) | Dynamic mid-build review, replanning, deviation specs, safeguards |
| [Docker Support](docs/docker.md) | Docker Compose lifecycle, health checks, sprint scoping |
| [Preflight Checks](docs/preflight.md) | Pre-build validation, required tools, custom commands |
| [Planning Mode](docs/planning-mode.md) | Non-code project support: documents, analyses, strategies |
| [Media Assets](docs/media-assets.md) | Optional `media/` directory for images, PDFs, fonts, and other build assets |
| [Supplementary Assets](docs/supplementary-assets.md) | Optional `assets/` directory for text reference documents read during plan generation |
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
