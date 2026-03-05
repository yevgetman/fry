# fry

An automated build orchestration engine that uses AI coding agents to build software projects from a plan document. Write a plan, and fry decomposes it into sprints and executes them autonomously -- one iteration at a time, fresh context each pass -- until the project is built.

Supports **OpenAI Codex** (default) and **Claude Code** as interchangeable AI engines.

## How It Works

```
plans/plan.md          You write this -- what to build
        |
        v
  fry-prepare.sh       AI generates AGENTS.md (rules) + epic.md (sprints)
        |
        v
     fry.sh             Executes sprints sequentially via AI agent loop
        |
        v
  Working software      Git-checkpointed after each sprint
```

fry adopts the "Ralph Wiggum Loop" pattern: each sprint runs as an iterative loop where the AI agent gets a prompt, does work, and logs progress. The next iteration reads what the previous one accomplished and continues. When the agent signals completion (via a promise token), the sprint ends and the next one begins.

**Key mechanisms:**

- **prompt.md** -- Assembled per sprint with layered context: executive overview, strategic plan pointer, sprint tasks, iteration memory, and completion signal
- **progress.txt** -- Append-only memory file that persists across iterations so the agent knows what prior passes accomplished
- **Promise tokens** -- Each sprint defines a `<promise>TOKEN</promise>` string. The loop ends when the agent outputs it, or fails after max iterations
- **Git checkpoints** -- Automatic commits after each sprint completes or fails

## Quick Start

### Prerequisites

- **bash 4.0+** (macOS ships 3.2 -- install via `brew install bash`)
- **git**
- One of:
  - **OpenAI Codex CLI**: `npm i -g @openai/codex`
  - **Claude Code CLI**: `npm i -g @anthropic-ai/claude-code`

### 1. Write your plan

The only required input is `plans/plan.md`. Write it in any format -- prose, bullets, tables -- as long as it has enough detail for an AI to decompose into implementation sprints.

```bash
mkdir -p plans
cat > plans/plan.md << 'EOF'
# My Project -- Build Plan

**Stack:** Node 20, Express, PostgreSQL 16, TypeScript strict mode.

**Database tables:**
- users -- id (UUID PK), email (UNIQUE), name, created_at
- posts -- id (UUID PK), user_id (FK), title, body, published (bool), created_at

**API endpoints:**
- POST /users -- create user
- GET /posts -- list published posts, cursor pagination
- POST /posts -- create post (authenticated)
...

**Tests:** Jest for unit tests, supertest for integration tests.
EOF
```

### 2. Run

```bash
# Uses Codex (default) -- generates AGENTS.md + epic.md automatically, then builds
./fry.sh

# Or use Claude Code instead
./fry.sh --engine claude
```

That's it. fry will:
1. Detect that `epic.md` doesn't exist and call `fry-prepare.sh`
2. Generate `AGENTS.md` (operational rules for the AI)
3. Generate `epic.md` (sprint definitions)
4. Execute each sprint, iterating until completion or max iterations

If `AGENTS.md` or `epic.md` already exist, they are **overwritten** by default. Use `--keep-agents` or `--keep-epic` flags on `fry-prepare.sh` to preserve existing files.

### 3. Validate before running (recommended)

```bash
./fry.sh --dry-run
```

This parses the epic, checks prerequisites, and shows the sprint plan without executing anything.

## Usage

```
./fry.sh [epic.md] [start_sprint] [end_sprint] [options]
```

### Arguments

| Argument | Description |
|---|---|
| `epic.md` | Path to the epic definition file (default: `epic.md`, auto-generated if missing) |
| `start_sprint` | First sprint to run (default: 1) |
| `end_sprint` | Last sprint to run (default: last in epic) |

### Options

| Flag | Description |
|---|---|
| `--engine <codex\|claude>` | AI engine to use (default: codex) |
| `--prepare-engine <codex\|claude>` | Engine for auto-generating the epic via fry-prepare.sh |
| `--dry-run` | Parse epic and show plan without running anything |
| `--help` | Show usage information |

### Examples

```bash
./fry.sh                                      # Run all sprints (epic.md default)
./fry.sh --dry-run                            # Validate and preview
./fry.sh --engine claude                      # Run all sprints with Claude Code
./fry.sh epic.md                              # Run all sprints with Codex
./fry.sh epic-phase2.md                       # Use a custom epic filename
./fry.sh epic.md 4                            # Resume from sprint 4
./fry.sh epic.md 4 4                          # Run only sprint 4
./fry.sh epic.md 3 5                          # Run sprints 3 through 5
./fry.sh --prepare-engine claude              # Use Claude for generation, Codex for build
FRY_ENGINE=claude ./fry.sh                    # Set engine via environment variable
```

### Engine Selection

The AI engine is resolved with this precedence (highest wins):

1. `--engine` CLI flag
2. `@engine` directive in the epic file
3. `FRY_ENGINE` environment variable
4. Default: `codex`

This lets you mix engines -- for example, generate the epic with Claude and run the build with Codex:

```bash
./fry.sh --prepare-engine claude --engine codex
```

## Project Structure

### What you create

```
project-root/
  plans/
    plan.md              # Required -- your build plan (any format)
    executive.md         # Optional -- project vision, goals, scope
```

### What fry ships with

```
fry.sh                   # Main build runner
fry-prepare.sh           # Generates AGENTS.md + epic.md from plan.md
epic-example.md          # Epic format template/reference
GENERATE_EPIC.md         # Prompt for manually generating epics with any LLM
```

### What gets generated at runtime

```
AGENTS.md                # Operational rules for the AI agent (auto-generated)
epic.md                  # Sprint definitions (auto-generated or hand-authored)
prompt.md                # Assembled per sprint (gitignored)
progress.txt             # Append-only iteration memory (committed)
build-logs/              # Per-iteration logs (gitignored)
```

## Epic File Format

An epic file defines global configuration and a sequence of sprint blocks. It can be auto-generated from your plan or hand-authored using `epic-example.md` as a template.

### Global Directives

Placed before any `@sprint` block:

```
@epic My Project Phase 1
@engine codex
@docker_from_sprint 2
@docker_ready_cmd docker compose exec -T postgres pg_isready -U myapp
@docker_ready_timeout 30
@require_tool node
@require_tool docker
@preflight_cmd npm --version
@pre_sprint npm install
@pre_iteration npm run lint:fix
@model gpt-4.1
@engine_flags --full-auto
```

| Directive | Description |
|---|---|
| `@epic <name>` | Display name for logs and summaries |
| `@engine <codex\|claude>` | AI engine (default: codex) |
| `@docker_from_sprint <N>` | Start docker-compose from sprint N |
| `@docker_ready_cmd <cmd>` | Custom health check after docker-compose up |
| `@docker_ready_timeout <s>` | Health check timeout in seconds (default: 30) |
| `@require_tool <name>` | CLI tool that must be on PATH (repeatable) |
| `@preflight_cmd <cmd>` | Custom check before build starts (repeatable) |
| `@pre_sprint <cmd>` | Run before each sprint starts |
| `@pre_iteration <cmd>` | Run before each agent invocation |
| `@model <model>` | Override the agent model (alias: `@codex_model`) |
| `@engine_flags <flags>` | Extra CLI flags for the agent (alias: `@codex_flags`) |

### Sprint Blocks

```
@sprint 1
@name Scaffolding & Infrastructure
@max_iterations 20
@promise SPRINT1_DONE
@prompt
Sprint 1: Scaffolding for My Project.

Read AGENTS.md first, then plans/plan.md.

Build:
1. package.json with TypeScript, Express, pg dependencies
2. tsconfig.json with strict mode
3. src/index.ts -- minimal Express server
4. docker-compose.yml -- PostgreSQL 16
...

Verify:
- npm run build succeeds
- npm test passes
- Docker services start and become healthy

CRITICAL: This is a TypeScript project. No JavaScript files.

If stuck after 10 iterations: check import paths and tsconfig paths.

Output <promise>SPRINT1_DONE</promise> when all verifications pass.
```

Each sprint prompt follows a 7-part structure:

1. **Opener** -- "Sprint N: [What] for [Project]."
2. **References** -- "Read AGENTS.md, then plans/plan.md [relevant sections]."
3. **Build list** -- Numbered, specific: exact filenames, function signatures, SQL DDL
4. **Constraints** -- "CRITICAL: [thing that will go wrong if ignored]."
5. **Verification** -- Bulleted checklist of concrete commands/outcomes
6. **Stuck hint** -- "If stuck after N iterations: [most likely cause + fix]."
7. **Promise** -- "Output `<promise>TOKEN</promise>` when [exit criteria]."

## fry-prepare.sh

Generates `AGENTS.md` and `epic.md` from your plan. Called automatically by `fry.sh`, or run standalone:

```bash
./fry-prepare.sh                           # Generate with Codex (default)
./fry-prepare.sh --engine claude           # Generate with Claude Code
./fry-prepare.sh epic-phase1.md            # Custom output filename
./fry-prepare.sh epic.md --engine claude   # Custom filename + engine
./fry-prepare.sh --validate-only           # Check prerequisites without generating
./fry-prepare.sh --keep-agents             # Skip AGENTS.md if it already exists
./fry-prepare.sh --keep-epic               # Skip epic.md if it already exists
```

Both `AGENTS.md` and `epic.md` are **overwritten by default** on each run. Use `--keep-agents` and/or `--keep-epic` to preserve existing files.

**Step 1** generates `AGENTS.md` -- an operational rules file with 15-40 numbered rules derived from your plan (technology constraints, architecture patterns, testing rules, prohibitions).

**Step 2** generates the epic -- a sequenced set of 4-10 sprints decomposed from your plan, each with specific build instructions, verification checklists, and completion tokens.

## Manual Epic Generation

If you prefer to generate the epic with a different LLM (ChatGPT, Claude web, etc.), use the prompt in `GENERATE_EPIC.md`:

1. Attach `plans/plan.md` and `epic-example.md` as context
2. Copy the prompt from the `## The Prompt` section in `GENERATE_EPIC.md`
3. Save the output as `epic.md`
4. Validate: `./fry.sh --dry-run`

## Sprint Sizing Guidelines

| Layer | Typical Sprints | Max Iterations | Files |
|---|---|---|---|
| Scaffolding/config | 1 (always first) | 15-20 | 5-12 |
| Schema/migrations | 1-2 | 15-20 | 5-15 |
| Domain models/types | 1-2 | 20-25 | 5-20 |
| Business logic | 1-2 | 25-30 | 10-20 |
| API handlers/routes | 1-2 | 20-25 | 5-15 |
| Wiring/integration/E2E | 1 (always last) | 30-35 | 5-15 |

Aim for 4-10 total sprints. If a sprint would produce 30+ files, split it. If it would produce 1-3 files, merge it with an adjacent sprint.

## Docker Support

fry manages Docker automatically when configured:

- `@docker_from_sprint N` -- starts docker-compose from sprint N onward
- Detects Docker Compose V1 (`docker-compose`) and V2 (`docker compose`)
- Waits for services to be healthy before starting the sprint
- Custom health checks via `@docker_ready_cmd`

Docker is only required when running sprints at or after the configured sprint number. Earlier sprints can run without Docker installed.

## Resuming Failed Builds

If a sprint fails (promise not found after max iterations), fry commits partial work and stops:

```
Build stopped at Sprint 4.
Fix the issue, then resume: ./fry.sh epic.md 4
```

Progress is preserved in `progress.txt` and git history. The agent picks up where it left off.

## File Reference

| File | Purpose | Created by |
|---|---|---|
| `fry.sh` | Main build runner -- parser, preflight, sprint loop, Docker, git | Ships with fry |
| `fry-prepare.sh` | Generates AGENTS.md + epic.md from plan.md | Ships with fry |
| `epic-example.md` | Epic format template with full documentation | Ships with fry |
| `GENERATE_EPIC.md` | Prompt for manual epic generation with any LLM | Ships with fry |
| `plans/plan.md` | Your build plan (required input) | You |
| `plans/executive.md` | Executive context -- project vision/goals (optional) | You |
| `AGENTS.md` | Operational rules for the AI agent | Auto-generated |
| `epic.md` | Sprint definitions | Auto-generated or hand-authored |
| `prompt.md` | Assembled per-sprint prompt (gitignored) | fry.sh at runtime |
| `progress.txt` | Append-only iteration memory | fry.sh at runtime |
| `build-logs/` | Per-iteration and per-sprint logs (gitignored) | fry.sh at runtime |

## Environment Variables

| Variable | Description |
|---|---|
| `FRY_ENGINE` | Default engine (`codex` or `claude`). Overridden by `--engine` flag and `@engine` directive. |

## Requirements

- **bash 4.0+** -- macOS ships bash 3.2; install a newer version: `brew install bash`
- **git** -- for automatic checkpointing
- **OpenAI Codex CLI** (`npm i -g @openai/codex`) -- if using the codex engine
- **Claude Code CLI** (`npm i -g @anthropic-ai/claude-code`) -- if using the claude engine
- **Docker** (optional) -- only needed if your epic uses `@docker_from_sprint`

## License

See repository for license information.
