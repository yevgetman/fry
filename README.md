# fry 🍳

![fry](trustmepro.png)

## What is Fry?

Fry is a self-improving agent orchestration tool designed for long-run coding, planning, and writing tasks. You provide some input — as little as a simple prompt or as much as a comprehensive build plan with an extensive corpus of supporting documents — and it will apply a layered system of planning, building, and checking its own work to produce a result with the level of effort of your choosing. To put it simply, **you give it as much or as little you want, and it will do as much or as little as you want it to do.**

### What does it actually do?

Fry can code, write [planning documents](docs/planning-mode.md), or write human-language content like essays, technical writing, and can even [write a complete book](docs/writing-mode.md)!

### Input

Fry takes one or all of the following as input:

- **A [user prompt](docs/user-prompt.md)** — provided as text on the command line or a path to a text file.
- **An executive.md file** — a high-level description of the project. Think of this as the "What and Why" for the project.
- **A plan.md file** — a detailed plan to build/write the output. Think of this as the "How" for the project.

Any of the above can be something you write yourself or have an LLM write for you.

### Plan Resolution

- If only a user prompt is provided, Fry generates a `plan.md` from the user prompt.
- If an `executive.md` is provided as well, Fry uses it to inform the generated `plan.md`.
- If a `plan.md` is provided, Fry uses it directly.

In short, Fry will do its best to use whatever info you provide to generate a `plan.md` file, if one was not explicitly provided.

### Triage

Before doing any of the heavy lifting, Fry runs a [triage gate](docs/triage.md) — a single cheap LLM call that classifies the task as **simple**, **moderate**, or **complex** and suggests an effort level. This determines how much preparation is needed:

- **Simple** tasks (fix a typo, add a config flag) skip preparation entirely — Fry builds a 1-sprint epic programmatically and gets straight to work. Zero LLM calls wasted on planning.
- **Moderate** tasks (add an endpoint with tests, build a small tool) also skip LLM-based preparation — Fry builds a programmatic 1-2 sprint epic with auto-generated sanity checks. Zero LLM calls for planning.
- **Complex** tasks (multi-subsystem features, architectural changes) get the full preparation pipeline described below.

After classification, Fry shows you the triage decision (difficulty, effort, reason) and asks you to confirm, decline, or adjust before the build begins. You can override both difficulty and effort at this step. Use `--no-project-overview` to skip the confirmation prompt.

Both simple and moderate tasks respect the effort level (suggested by triage or overridden with `--effort`), which controls iteration budgets, alignment, and audit depth. Max effort is reserved for complex tasks. The classifier is intentionally biased toward over-classification — it's better to over-prepare a simple task than to under-prepare a complex one. Use `--full-prepare` to bypass triage and force the full pipeline.

### Preparation

For complex tasks (or when `--full-prepare` is used), Fry will:

1. Generate an `AGENTS.md` file (if one was not provided) establishing best practices for the agents
2. Decompose `plan.md` into an [epic](docs/epic-format.md), delimited by sprints with each sprint broken up by specific tasks
3. Generate a `verification.md` for [sanity checks](docs/sanity-checks.md) to run after each sprint (deep semantic checks are done as part of a separate [audit system](docs/sprint-audit.md))

### The Build

Fry deploys agents using [OpenAI Codex, Claude Code, or Ollama](docs/engines.md) — the specific models used vary by task and user-defined [effort level](docs/effort-levels.md).

A single agent carries out the work to complete a sprint (although there is a parallel mode to run multiple agents at once).

Once a sprint is complete, [sanity checks](docs/sanity-checks.md) run to verify the deliverables. If any checks fail, an [alignment system](docs/alignment.md) deploys to fix the issues.

If/when all sanity checks pass, an [audit process](docs/sprint-audit.md) is deployed to ensure the work has been completed with no bugs, edge cases covered, etc. — basically that it was done well. The audit process is layered and iterative, making multiple passes (based on effort level) to ensure issues are fixed on a first-in-first-out basis before a follow-up audit is run to verify and/or surface new issues. The process repeats until the exit condition is met.

The build continues in this manner until complete.

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
  fry run               Triage gate: cheap LLM classifies complexity + effort
                           ↓ Interactive confirmation [Y/n/a] (adjust difficulty/effort)
                           SIMPLE   → programmatic epic (0 prep calls)
                           MODERATE → programmatic epic + auto-sanity-checks (0 prep calls)
                           COMPLEX  → full prepare (below)
                         (--full-prepare skips triage, --no-project-overview skips confirmation)
        |
        v
  fry prepare           Step 0 (if needed): AI generates plans/plan.md from executive.md
  (complex tasks)        Steps 1-3: AI generates .fry/AGENTS.md + .fry/epic.md + .fry/verification.md
  (pass --mode planning for documents, --mode writing for books/guides)
        |
        v
     fry run             Executes sprints via AI agent loop
        |                + runs independent sanity checks
        |                + auto-aligns on sanity check failure
        v
  Working software       Git-checkpointed after each sprint
  (or output/)           Planning: 1--research--market-landscape.md
                         Writing:  01--introduction.md → manuscript.md
```

Each sprint runs as an iterative loop where the AI agent gets a prompt, does work, and logs progress. The next iteration reads what the previous one accomplished and continues. When the agent signals completion (via a promise token), the sprint ends and the next one begins.

**Key mechanisms:**

- **Triage gate** -- before running the full prepare pipeline, a single cheap LLM call classifies task complexity as `simple`, `moderate`, or `complex` and suggests an effort level. After classification, an interactive confirmation prompt lets you review, accept, or adjust the difficulty and effort before the build starts (`--no-project-overview` skips this). Simple and moderate tasks skip prepare entirely (zero LLM calls for planning) with effort-aware iteration budgets, alignment, and audit depth. Complex tasks get the full pipeline. Max effort is reserved for complex tasks. Biased toward over-classification to avoid wasting tokens. See [Triage](docs/triage.md). Use `--full-prepare` to bypass.
- **Project overview** -- after `plan.md` exists, Fry shows an AI-generated project summary and asks for confirmation before generating build artifacts (skippable with `--no-project-overview`)
- **Effort-level triage** -- `--effort low|medium|high|max` controls sprint count, density, and rigor. Auto-detects when unspecified. See [Effort Levels](docs/effort-levels.md).
- **Media assets** -- optional `media/` directory for images, PDFs, fonts, and other files referenced in plans and copied into builds
- **Supplementary assets** -- optional `assets/` directory for text reference documents (specs, schemas, requirements) whose full contents are read during plan and epic generation
- **Layered prompts** -- assembled per sprint with executive context, media manifest, user directives, operational disposition, plan references, sprint tasks, iteration memory, and completion signals
- **Two-file progress tracking** -- per-sprint iteration log + cross-sprint compacted summary for bounded context
- **Promise tokens** -- `===PROMISE: TOKEN===` signals sprint completion
- **Sanity checks** -- machine-executable checks run after each sprint with a configurable failure threshold (`@max_fail_percent`, default 20%) — minor failures are deferred rather than blocking the build
- **Alignment** -- automatic re-runs with targeted fix prompts on sanity check failure; `--resume` picks up where a failed build left off with boosted alignment attempts; `--continue` uses an LLM agent to analyze build state and auto-resume (automatically restores the build mode from the previous run)
- **Sprint audit** -- post-sprint semantic review by a separate AI agent, with automatic fix loop (CRITICAL/HIGH block the build; MODERATE is advisory)
- **Build audit** -- final holistic codebase audit after the entire epic completes, with iterative remediation
- **Build summary** -- comprehensive `build-summary.md` generated after all sprints, covering what was built, events, audit findings, and advisories
- **Build archiving** -- on successful full builds, `.fry/` and root-level outputs are auto-archived to `.fry-archive/`; run `fry clean` to archive manually
- **Git strategy** -- `--git-strategy auto|current|branch|worktree` controls build isolation. Auto mode lets triage decide: complex tasks get an isolated worktree, simpler tasks get a new branch. Use `current` for the previous behavior (work on the current branch). See [Git Strategy](docs/git-strategy.md).
- **Git checkpoints** -- automatic commits after each sprint
- **Dynamic sprint review** -- optional mid-build review with replanning
- **Observer** -- metacognitive layer that watches builds, notices patterns, and collects build experience records for the consciousness pipeline. Identity is compiled into the binary and read-only during builds. Non-fatal; effort-level gated. See [Observer](docs/observer.md).
- **Experience upload** -- opt-in telemetry sends anonymized build experience summaries to the central consciousness API. Offline-resilient with local caching and automatic retry. Control via `--telemetry` / `--no-telemetry`, `FRY_TELEMETRY` env var, or `~/.fry/settings.json`. See [Consciousness](docs/consciousness.md).
- **Writing mode** -- `--mode writing` re-orients the pipeline for books, guides, and reports with content-oriented audit criteria and a final `manuscript.md`
- **Colored output** -- terminal output is colorized for readability (phase banners in cyan, PASS in green, FAIL in red, warnings in yellow). Respects `NO_COLOR`, `TERM=dumb`, and `--no-color`. Log files are always plain text.

## Self-Improving Codebase

Fry improves itself. An automated pipeline runs daily, scanning the Fry source code for bugs, testing gaps, feature opportunities, and other improvements. It selects 2-3 items from a roadmap, implements them, runs the full test suite, and either merges directly to master or opens a pull request for human review.

The loop uses Fry's own features — planning mode for discovery, `--always-verify` for quality gates, worktrees for isolation, and the triage gate for complexity-appropriate effort levels. A bash orchestrator (`.self-improve/orchestrate.sh`) drives the cycle, and a macOS launchd agent triggers it daily.

The canonical roadmap lives at `.self-improve/roadmap.json`. To run the loop manually:

```bash
fry-improve                  # full loop (planning if needed + build + PR)
fry-improve --auto-merge     # merge directly to master
fry-improve --skip-planning  # build only
```

See [Self-Improvement Pipeline](docs/self-improvement.md) for the full architecture, configuration, and operational guide.

## Requirements

- **Go 1.22+** — to build fry from source
- **git** — for automatic sprint checkpointing
- **bash** — used by AI engine CLIs and sanity check commands
- At least one AI engine CLI:
  - [Claude Code](https://www.npmjs.com/package/@anthropic-ai/claude-code): `npm i -g @anthropic-ai/claude-code` (default engine)
  - [OpenAI Codex CLI](https://www.npmjs.com/package/@openai/codex): `npm i -g @openai/codex`
  - [Ollama](https://ollama.com): `brew install ollama` — local models, no API key required (required only when using `--engine ollama`)
- **Docker** (optional) — only needed if your project uses `@docker_from_sprint`

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

# Option C: Write a book or guide
fry --mode writing --user-prompt "Write a comprehensive guide to Go concurrency"

# Validate without running
fry --dry-run
```

See [Getting Started](docs/getting-started.md) for full setup instructions.

## Commands

| Command | Description |
|---|---|
| `fry run` | Execute sprints from an epic file (default command) |
| `fry init` | Scaffold the fry project structure (`plans/`, `assets/`, `media/`, git, `.gitignore`) |
| `fry prepare` | Generate `.fry/AGENTS.md`, `.fry/epic.md`, and `.fry/verification.md` from your plan |
| `fry replan` | Replan an epic after a deviation |
| `fry identity` | Print Fry's compiled-in identity (core + disposition) |
| `fry reflect` | Trigger identity reflection from accumulated memories |
| `fry status` | Show current build state, or archived/worktree build history if no active build |
| `fry clean` | Archive `.fry/` and build outputs to `.fry-archive/` |
| `fry version` | Print fry version |

```bash
fry                                    # Run all sprints (prepare: claude, build: claude)
fry --engine codex                     # Use OpenAI Codex for build stage
fry --engine ollama                    # Use local Ollama models (no API key)
fry --effort low                       # Simple task: 1-2 sprints, minimal overhead
fry --effort max                       # Maximum rigor: extended prompts, thorough reviews
fry run epic.md 3 5                    # Run sprints 3-5
fry run --resume --sprint 4             # Resume failed sprint 4 (skip iterations, align only)
fry run --continue                     # Auto-detect and resume from where you left off
fry clean                              # Archive .fry/ and build outputs (interactive)
fry clean --force                      # Archive without confirmation prompt
fry --mode planning                    # Planning mode (documents, not code) — claude for both stages
fry --mode writing --user-prompt "..."  # Writing mode (books, guides) — claude for both stages
fry --user-prompt "no ORMs, raw SQL"   # Inject a directive
fry --user-prompt "build a todo app"  # Start from just a prompt (no plan files needed)
fry --user-prompt-file ./prompt.txt   # Load a longer prompt from a file
fry --git-strategy worktree            # Force worktree isolation for the build
fry --git-strategy branch --branch-name feat/api  # Build on a named branch
fry --always-verify                    # Force sanity checks+audit on all tasks
fry --no-observer                      # Disable the observer metacognitive layer
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
| [AI Engines](docs/engines.md) | Codex, Claude, and Ollama engine configuration, mixing engines, model overrides |
| [Sprint Execution](docs/sprint-execution.md) | Agent iteration loop, prompt assembly, progress tracking, promise tokens |
| [Sanity Checks](docs/sanity-checks.md) | Check primitives, file format, outcome matrix, graceful degradation |
| [Alignment](docs/alignment.md) | Alignment loop mechanics, configuration, diagnostics |
| [Sprint Audit](docs/sprint-audit.md) | Post-sprint semantic code review by AI, audit/fix loop, severity classification |
| [Build Audit](docs/build-audit.md) | Final holistic codebase audit after epic completion, iterative remediation |
| [Sprint Review](docs/sprint-review.md) | Dynamic mid-build review, replanning, deviation specs, safeguards |
| [Docker Support](docs/docker.md) | Docker Compose lifecycle, health checks, sprint scoping |
| [Preflight Checks](docs/preflight.md) | Pre-build validation, required tools, custom commands |
| [Planning Mode](docs/planning-mode.md) | Non-code project support: documents, analyses, strategies |
| [Writing Mode](docs/writing-mode.md) | Human-language content: books, guides, reports, documentation |
| [Media Assets](docs/media-assets.md) | Optional `media/` directory for images, PDFs, fonts, and other build assets |
| [Supplementary Assets](docs/supplementary-assets.md) | Optional `assets/` directory for text reference documents read during plan generation |
| [User Prompt](docs/user-prompt.md) | Injecting directives, prompt hierarchy, persistence |
| [Project Structure](docs/project-structure.md) | Directory layout, generated artifacts, file reference |
| [Terminal Output](docs/terminal-output.md) | Status banners, verbose mode, log format |
| [Triage](docs/triage.md) | Complexity classification with interactive confirmation — controls whether full prepare runs |
| [Git Strategy](docs/git-strategy.md) | Branch and worktree isolation strategies for builds |
| [Self-Improvement](docs/self-improvement.md) | Automated self-improvement pipeline: roadmap, orchestrator, planning, build, alignment |
| [Observer](docs/observer.md) | Metacognitive layer: event stream, identity, wake-ups, effort-level gating |
| [Consciousness](docs/consciousness.md) | Experience synthesis and identity pipeline |
| [Architecture](docs/architecture.md) | Internal package structure, data flow, build system |

## License

See repository for license information.
