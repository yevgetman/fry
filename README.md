# fry

An automated build orchestration engine that uses AI coding agents to build software projects from a plan document. Write a plan, and fry decomposes it into sprints and executes them autonomously -- one iteration at a time, fresh context each pass -- until the project is built.

Supports **OpenAI Codex** (default) and **Claude Code** as interchangeable AI engines.

## How It Works

```
plans/plan.md          You write this -- what to build
        |
        v
  fry-prepare.sh       AI generates AGENTS.md + epic.md + verification.md
        |
        v
     fry.sh             Executes sprints via AI agent loop
        |                + runs independent verification checks
        v
  Working software      Git-checkpointed after each sprint
```

fry adopts the "Ralph Wiggum Loop" pattern: each sprint runs as an iterative loop where the AI agent gets a prompt, does work, and logs progress. The next iteration reads what the previous one accomplished and continues. When the agent signals completion (via a promise token), the sprint ends and the next one begins.

**Key mechanisms:**

- **prompt.md** -- Assembled per sprint with layered context: executive overview, strategic plan pointer, sprint tasks, iteration memory, and completion signal
- **progress.txt** -- Append-only memory file that persists across iterations so the agent knows what prior passes accomplished
- **Promise tokens** -- Each sprint defines a `<promise>TOKEN</promise>` string. The loop ends when the agent outputs it, or fails after max iterations
- **verification.md** -- Machine-executable checks per sprint, run independently by fry.sh after the agent signals completion (or after max iterations as a fallback)
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
# Uses Codex (default) -- generates AGENTS.md + epic.md + verification.md, then builds
./fry.sh

# Or use Claude Code instead
./fry.sh --engine claude
```

That's it. fry will:
1. Detect that `epic.md` doesn't exist and call `fry-prepare.sh`
2. Generate `AGENTS.md` (operational rules for the AI)
3. Generate `epic.md` (sprint definitions)
4. Generate `verification.md` (independent checks per sprint)
5. Execute each sprint, iterating until completion or max iterations
6. Run independent verification checks after each sprint

If `AGENTS.md`, `epic.md`, or `verification.md` already exist, they are **overwritten** by default. Use `--keep-agents`, `--keep-epic`, or `--keep-verification` flags on `fry-prepare.sh` to preserve existing files.

### 3. Validate before running (recommended)

```bash
./fry.sh --dry-run
```

This parses the epic, checks prerequisites, and shows the sprint plan without executing anything.

## Usage

```
./fry.sh [epic_file] [start_sprint] [end_sprint] [options]
```

All arguments are optional. With no arguments, fry uses `epic.md` as the epic file and runs all sprints.

### Positional Arguments

Positional arguments are order-dependent: the epic file must come first, followed by sprint numbers.

| Position | Argument | Description |
|---|---|---|
| 1 | `epic_file` | Path to the epic definition file (default: `epic.md`). Auto-generated via `fry-prepare.sh` if the file doesn't exist. |
| 2 | `start_sprint` | First sprint to run (default: 1) |
| 3 | `end_sprint` | Last sprint to run (default: last sprint in epic) |

To specify a sprint range, you must also specify the epic file:

```bash
./fry.sh epic.md 3 5       # Correct: run sprints 3-5
./fry.sh 3 5               # Wrong: treats "3" as the epic filename
```

### Options

Options (flags) can appear anywhere in the command -- before, after, or between positional arguments.

| Flag | Description |
|---|---|
| `--engine <codex\|claude>` | AI engine to use (default: codex) |
| `--prepare-engine <codex\|claude>` | Engine for auto-generating via fry-prepare.sh (defaults to `--engine` or `FRY_ENGINE`) |
| `--dry-run` | Parse epic and show plan without running anything |
| `--help`, `-h` | Show usage information |

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
fry-prepare.sh           # Generates AGENTS.md + epic.md + verification.md from plan.md
epic-example.md          # Epic format template/reference
verification-example.md  # Verification format template/reference
GENERATE_EPIC.md         # Prompt for manually generating epics with any LLM
```

### What gets generated at runtime

```
AGENTS.md                # Operational rules for the AI agent (auto-generated)
epic.md                  # Sprint definitions (auto-generated or hand-authored)
verification.md          # Independent verification checks (auto-generated or hand-authored)
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
@verification verification.md
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
| `@verification <file>` | Verification checks file (default: `verification.md`) |

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
Verification checks are defined in verification.md (sprint 1).
- npm run build succeeds
- npm test passes
- Docker services start and become healthy

CRITICAL: This is a TypeScript project. No JavaScript files.

If stuck after 10 iterations: check import paths and tsconfig paths.

Output <promise>SPRINT1_DONE</promise> when all checks pass.
```

Each sprint prompt follows a 7-part structure:

1. **Opener** -- "Sprint N: [What] for [Project]."
2. **References** -- "Read AGENTS.md, then plans/plan.md [relevant sections]."
3. **Build list** -- Numbered, specific: exact filenames, function signatures, SQL DDL
4. **Constraints** -- "CRITICAL: [thing that will go wrong if ignored]."
5. **Verification** -- Reference to verification.md checks, plus prose summary of key outcomes
6. **Stuck hint** -- "If stuck after N iterations: [most likely cause + fix]."
7. **Promise** -- "Output `<promise>TOKEN</promise>` when [exit criteria]."

## fry-prepare.sh

Generates `AGENTS.md`, `epic.md`, and `verification.md` from your plan. Called automatically by `fry.sh` when the epic file doesn't exist, or run standalone:

```
./fry-prepare.sh [epic_filename] [options]
```

| Argument / Flag | Description |
|---|---|
| `epic_filename` | Output filename for the epic (default: `epic.md`) |
| `--engine <codex\|claude>` | AI engine for generation (default: codex, or `FRY_ENGINE` env var) |
| `--keep-agents` | Skip AGENTS.md generation if the file already exists |
| `--keep-epic` | Skip epic.md generation if the file already exists |
| `--keep-verification` | Skip verification.md generation if the file already exists |
| `--validate-only` | Check that all prerequisites are present, then exit |

`AGENTS.md`, `epic.md`, and `verification.md` are **overwritten by default** on each run.

```bash
./fry-prepare.sh                           # Generate all with Codex (default)
./fry-prepare.sh --engine claude           # Generate all with Claude Code
./fry-prepare.sh epic-phase1.md            # Custom epic filename
./fry-prepare.sh epic.md --engine claude   # Custom filename + engine
./fry-prepare.sh --validate-only           # Check prerequisites only
./fry-prepare.sh --keep-agents             # Preserve existing AGENTS.md
./fry-prepare.sh --keep-epic               # Preserve existing epic.md
./fry-prepare.sh --keep-verification       # Preserve existing verification.md
```

**Step 1** generates `AGENTS.md` -- an operational rules file with 15-40 numbered rules derived from your plan (technology constraints, architecture patterns, testing rules, prohibitions).

**Step 2** generates the epic -- a sequenced set of 4-10 sprints decomposed from your plan, each with specific build instructions, verification references, and completion tokens.

**Step 3** generates `verification.md` -- machine-executable checks per sprint derived from the plan and epic. These are run independently by fry.sh to verify the agent's work.

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

## Verification

fry supports independent verification of each sprint's deliverables. When a `verification.md` file is present, fry.sh runs machine-executable checks after the agent signals completion, closing the gap between the agent's self-reported promise and actual results.

### How It Works

1. `fry-prepare.sh` generates `verification.md` as Step 3 (from plan.md + epic.md)
2. Each sprint block in `verification.md` defines checks using four primitives:

| Primitive | Example | Passes when |
|---|---|---|
| `@check_file <path>` | `@check_file src/index.ts` | File exists and is non-empty |
| `@check_file_contains <path> <pattern>` | `@check_file_contains package.json "typescript"` | File contains pattern (grep -E) |
| `@check_cmd <command>` | `@check_cmd npm run build` | Command exits 0 |
| `@check_cmd_output <cmd> \| <pattern>` | `@check_cmd_output curl -s /health \| "ok"` | stdout matches pattern |

3. After the agent outputs its promise token, fry.sh runs all checks for that sprint
4. All checks run to completion (failures don't short-circuit) and results are reported as "N/M checks passed"
5. All checks must pass for the sprint to succeed

### Outcome Matrix

When a sprint defines a `@promise` token:

| Promise Token | Checks Pass | Result |
|---|---|---|
| Found | All pass | **PASS** |
| Found | Some fail | **FAIL** -- "promise found, verification failed" |
| Not found | All pass | **PASS** -- auto-recovery (work done, token forgotten) |
| Not found | Some fail | **FAIL** |

When a sprint has no `@promise` token but has verification checks, the checks determine the outcome after all iterations run. When neither is defined, the sprint always passes after running all iterations.

### Verification File Format

`verification.md` uses the same `@sprint N` block structure as the epic file. See `verification-example.md` for the full format reference. The file can be auto-generated, hand-authored, or a mix (generate first, then edit).

The `@verification` directive in the epic file can override the default filename:

```
@verification checks-phase2.md
```

### Graceful Degradation

- If `verification.md` does not exist, fry.sh falls back to promise-only behavior
- If `fry-prepare.sh` fails to generate `verification.md`, it logs a WARNING and continues (it does not abort the build)
- If a sprint has no checks defined in `verification.md`, it behaves as if no verification file exists for that sprint

## Resuming Failed Builds

A sprint can fail for several reasons:

- Promise token not found after max iterations (and no verification checks, or checks also fail)
- Promise token found but verification checks fail
- No promise defined and verification checks fail

In all cases, fry commits partial work and stops with a resume command:

```
Build stopped at Sprint 4.
Fix the issue, then resume: ./fry.sh epic.md 4
```

Progress is preserved in `progress.txt` and git history. The agent picks up where it left off.

## File Reference

| File | Purpose | Created by |
|---|---|---|
| `fry.sh` | Main build runner -- parser, preflight, sprint loop, Docker, git | Ships with fry |
| `fry-prepare.sh` | Generates AGENTS.md + epic.md + verification.md from plan.md | Ships with fry |
| `epic-example.md` | Epic format template with full documentation | Ships with fry |
| `verification-example.md` | Verification format template with check primitives | Ships with fry |
| `GENERATE_EPIC.md` | Prompt for manual epic generation with any LLM | Ships with fry |
| `plans/plan.md` | Your build plan (required input) | You |
| `plans/executive.md` | Executive context -- project vision/goals (optional) | You |
| `AGENTS.md` | Operational rules for the AI agent | Auto-generated |
| `epic.md` | Sprint definitions | Auto-generated or hand-authored |
| `verification.md` | Independent verification checks per sprint | Auto-generated or hand-authored |
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
