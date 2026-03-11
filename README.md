# fry

An automated build orchestration engine that uses AI agents to execute complex plans autonomously. Write a plan, and fry decomposes it into sprints and executes them -- one iteration at a time, fresh context each pass -- until the project is done.

Works for **software projects** (code, tests, infrastructure) and **planning projects** (business plans, research reports, strategic analyses, trip planning -- any endeavor requiring rigorous phased document creation). See [Planning Mode](#planning-mode-non-code-projects).

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

- **`.fry/prompt.md`** -- Assembled per sprint with layered context: executive overview, user directive (if provided via `--user-prompt`), strategic plan pointer, sprint tasks, iteration memory, and completion signal
- **`.fry/sprint-progress.txt`** + **`.fry/epic-progress.txt`** -- Two-file progress tracking: `.fry/sprint-progress.txt` is the per-sprint iteration log (overwritten each sprint), and `.fry/epic-progress.txt` is the cross-sprint compacted summary (append-only within a run; reset on full rebuild from sprint 1). Together they give each agent session bounded context without unbounded growth
- **Promise tokens** -- Each sprint defines a `===PROMISE: TOKEN===` string. The loop ends when the agent outputs it, or after 2 consecutive no-op iterations with passing verification, or fails after max iterations
- **`.fry/verification.md`** -- Machine-executable checks per sprint, run independently by fry after the agent signals completion (or after max iterations as a fallback)
- **Self-healing** -- When verification checks fail, fry automatically re-runs the AI agent with a targeted fix prompt containing the specific failures and diagnostic output, then re-checks. Repeats up to `@max_heal_attempts` times (default: 3, set to 0 to disable)
- **Agent session banners** -- Every time a new AI agent session starts (sprint iteration, heal attempt, reviewer, replanner, compaction, or artifact generation), fry prints a `▶ AGENT` banner to the terminal showing the engine, model, and reason. Always on -- not gated by `--verbose`
- **Git checkpoints** -- Automatic commits after each sprint completes or fails

## Quick Start

### Prerequisites

- **git**
- **Go 1.22+** (to build from source)
- One of:
  - **OpenAI Codex CLI**: `npm i -g @openai/codex`
  - **Claude Code CLI**: `npm i -g @anthropic-ai/claude-code`

### Install

```bash
# From source
git clone https://github.com/yevgetman/fry.git
cd fry
make install    # builds and copies to /usr/local/bin/fry

# Or build without installing
make build      # outputs to bin/fry

# Or using go install
go install github.com/yevgetman/fry/cmd/fry@latest
```

### 1. Write your plan

The minimum required input is either `plans/plan.md` or `plans/executive.md` (or both).

**`plans/plan.md`** -- the technical build plan. Write it in any format -- prose, bullets, tables -- as long as it has enough detail for an AI to decompose into implementation sprints. This is the primary source material that fry uses to generate everything else.

**`plans/executive.md`** -- a higher-level document that describes the project's purpose, business goals, target users, and scope. When both files are present, fry feeds executive.md into every generation step so the AI understands *why* the project exists, not just *what* to build. This leads to better-aligned `.fry/AGENTS.md` rules, more coherent sprint decomposition, and smarter verification checks.

**If only `executive.md` is provided** (no `plan.md`), fry auto-generates `plan.md` using the LLM in a "Step 0" before the normal preparation flow. The LLM makes all design, architecture, and implementation decisions based on your executive context. The generated `plan.md` is written to `plans/` (committed to your repo) so you can review the LLM's choices before building.

```bash
mkdir -p plans

# Option A: Provide a detailed build plan (you make all design decisions)
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

# Option B: Provide executive context only (LLM makes all design decisions)
# If you provide only this file, plan.md is auto-generated in Step 0.

# Option C: Provide both (plan.md = build details, executive.md = alignment north star)
cat > plans/executive.md << 'EOF'
# My Project -- Executive Context

**Vision:** A lightweight blogging platform for small teams.
**Target users:** Non-technical content creators who need a simple API-driven CMS.
**Success criteria:** Sub-100ms API responses, zero-downtime deploys, <30 min onboarding.
**Scope boundaries:** No auth provider integration in phase 1. No rich text -- markdown only.
EOF
```

### 2. Run

```bash
# Uses Codex (default) -- generates .fry/AGENTS.md + .fry/epic.md + .fry/verification.md, then builds
fry

# Or use Claude Code instead
fry --engine claude
```

That's it. fry will:
1. Detect that `.fry/epic.md` doesn't exist and run `fry prepare` automatically
2. If `plans/plan.md` is missing but `plans/executive.md` exists, auto-generate `plan.md` (Step 0)
3. Generate `.fry/AGENTS.md` (operational rules for the AI)
4. Generate `.fry/epic.md` (sprint definitions)
5. Generate `.fry/verification.md` (independent checks per sprint)
6. Parse the epic file and validate its structure
7. Run preflight checks (tools, project structure, `.fry/AGENTS.md` content validation)
8. Execute each sprint, iterating until completion or max iterations
9. Run independent verification checks after each sprint

All generated artifacts are stored in the `.fry/` directory (gitignored automatically). Only `plans/` (your input) lives in the project root. If `plan.md` was auto-generated from `executive.md`, it is also written to `plans/` so you can review the LLM's design decisions.

**Auto-generation trigger:** `fry run` calls `fry prepare` only when the epic file does not exist on disk. By default it generates software-mode artifacts; pass `--planning` for document projects instead. This is a simple file-existence check -- if `.fry/epic.md` is present (even from a prior `--dry-run`), the prepare step is skipped entirely and fry uses the existing files as-is.

When `fry prepare` is run (standalone or via `fry run`), it always **overwrites** all `.fry/` artifacts -- `.fry/AGENTS.md`, `.fry/epic.md`, and `.fry/verification.md` are regenerated fresh each time. If `plan.md` was auto-generated (Step 0), it persists in `plans/` and is treated as user-authored on subsequent runs -- delete it manually to force re-generation. To re-run fry with a new plan, update your input files and delete `.fry/epic.md` (or run `fry prepare` directly).

### 3. Validate before running (recommended)

```bash
fry --dry-run
```

This parses the epic, checks prerequisites, and shows the sprint plan without executing anything.

## Usage

```
fry [command] [flags]
```

When invoked without a subcommand, `fry` is equivalent to `fry run`.

### Commands

| Command | Description |
|---|---|
| `fry run` | Execute sprints from an epic file (default command) |
| `fry prepare` | Generate `.fry/AGENTS.md`, `.fry/epic.md`, and `.fry/verification.md` from your plan |
| `fry replan` | Replan an epic after a deviation |
| `fry version` | Print fry version |

---

### `fry run`

```
fry run [epic.md] [start] [end] [flags]
```

All arguments are optional. With no arguments, fry uses `.fry/epic.md` as the epic file and runs all sprints.

#### Positional Arguments

Positional arguments are order-dependent: the epic file must come first, followed by sprint numbers.

| Position | Argument | Description |
|---|---|---|
| 1 | `epic_file` | Path to the epic definition file (default: `.fry/epic.md`). Auto-generated via `fry prepare` if the file doesn't exist. |
| 2 | `start_sprint` | First sprint to run (default: 1) |
| 3 | `end_sprint` | Last sprint to run (default: last sprint in epic) |

To specify a sprint range, you must also specify the epic file:

```bash
fry run epic.md 3 5       # Correct: run sprints 3-5
fry run 3 5               # Wrong: treats "3" as the epic filename
```

#### Flags

| Flag | Description |
|---|---|
| `--project-dir <path>` | Project directory to operate on (default: current directory) |
| `--engine <codex\|claude>` | AI engine to use (default: codex) |
| `--prepare-engine <codex\|claude>` | Engine for auto-generating the epic (defaults to `--engine` or `FRY_ENGINE`) |
| `--planning` | Use planning-domain prompts for auto-generation |
| `--user-prompt <text>` | Top-level directive injected into every sprint prompt (saved to `.fry/user-prompt.txt`, reused on subsequent runs unless overridden) |
| `--no-review` | Disable sprint review even if the epic enables `@review_between_sprints` |
| `--simulate-review <verdict>` | Test the review pipeline without LLM calls. Verdict: `CONTINUE` or `DEVIATE` |
| `--verbose` | Print agent output to terminal (default: silent, logs only) |
| `--dry-run` | Parse epic and show plan without running anything |

#### Examples

```bash
fry                                               # Run all sprints (.fry/epic.md default)
fry --dry-run                                     # Validate and preview
fry --engine claude                               # Run all sprints with Claude Code
fry run epic.md                                   # Run all sprints with Codex
fry run epic-phase2.md                            # Use a custom epic filename
fry run epic.md 4                                 # Resume from sprint 4
fry run epic.md 4 4                               # Run only sprint 4
fry run epic.md 3 5                               # Run sprints 3 through 5
fry --verbose                                     # Print agent output to terminal
fry --prepare-engine claude                       # Use Claude for generation, Codex for build
fry --planning --engine claude                    # Planning project (documents, not code)
fry --user-prompt "focus on backend API, skip frontend"
fry --project-dir /path/to/project                # Operate on a different project
FRY_ENGINE=claude fry                             # Set engine via environment variable
```

---

### `fry prepare`

Generates `.fry/AGENTS.md`, `.fry/epic.md`, and `.fry/verification.md` from your plan. Requires at least one of `plans/plan.md` or `plans/executive.md`. If only `executive.md` is provided, `plan.md` is auto-generated first (Step 0). Called automatically by `fry run` when the epic file doesn't exist, or run standalone:

```
fry prepare [epic_filename] [flags]
```

| Flag | Description |
|---|---|
| `--project-dir <path>` | Project directory to operate on (default: current directory) |
| `--engine <codex\|claude>` | AI engine for generation (default: codex, or `FRY_ENGINE`) |
| `--user-prompt <text>` | Top-level directive to guide artifact generation |
| `--validate-only` | Check that the epic is valid, then exit |
| `--planning` | Use planning-domain prompts |

All artifacts are **always regenerated** (overwritten) on each run.

```bash
fry prepare                                        # Generate all with Codex (default)
fry prepare --engine claude                        # Generate all with Claude Code
fry prepare epic-phase1.md                         # Custom epic filename
fry prepare --project-dir /path                    # Operate on a different project
fry prepare --user-prompt "no ORMs, use raw SQL only"
fry prepare --validate-only                        # Validate existing epic only
```

**Step 0** (conditional) generates `plans/plan.md` from `plans/executive.md` -- only runs when `plan.md` doesn't exist but `executive.md` does.

**Step 1** generates `.fry/AGENTS.md` -- an operational rules file with numbered rules derived from your plan.

**Step 2** generates the epic (`.fry/epic.md`) -- a sequenced set of sprints decomposed from your plan.

**Step 3** generates `.fry/verification.md` -- machine-executable checks per sprint.

---

### `fry replan`

Replan an epic after a deviation. Used by the dynamic sprint review system, or can be invoked directly:

```
fry replan <deviation_spec> [flags]
```

| Flag | Description |
|---|---|
| `--epic <path>` | Epic file to update (default: `.fry/epic.md`) |
| `--completed <N>` | Completed sprint count |
| `--max-scope <N>` | Maximum deviation scope (default: 3) |
| `--engine <codex\|claude>` | Replanning engine |
| `--model <model>` | Model override |
| `--dry-run` | Preview replanning prompt without executing |

---

### Engine Selection

The AI engine is resolved with this precedence (highest wins):

1. `--engine` CLI flag
2. `@engine` directive in the epic file
3. `FRY_ENGINE` environment variable
4. Default: `codex`

This lets you mix engines -- for example, generate the epic with Claude and run the build with Codex:

```bash
fry --prepare-engine claude --engine codex
```

## User Prompt

The `--user-prompt` flag lets you inject a top-level directive that applies to every sprint. This is useful for steering the build without modifying `plan.md` or `executive.md` -- especially when no `executive.md` exists and you want to provide lightweight guidance.

```bash
# Steer the build toward backend work
fry --user-prompt "focus on backend API, skip frontend styling"

# Constrain the approach
fry --user-prompt "use only standard library, no third-party dependencies"
```

The user prompt is:

- **Injected as Layer 1.5** in the prompt hierarchy -- between executive context ("why") and strategic plan ("what"). The agent sees it as a priority directive that applies to all sprints.
- **Passed through to prepare** -- when `fry run` auto-generates the epic, the user prompt is forwarded to `fry prepare`, influencing the generation of `.fry/AGENTS.md`, `.fry/epic.md`, and `.fry/verification.md`.
- **Included in heal prompts** -- when self-healing runs, the user directive is listed as context so heal passes respect it.
- **Persisted to `.fry/user-prompt.txt`** -- the prompt is saved on first use and automatically reused on subsequent runs. Override it by passing a new `--user-prompt` value. Delete `.fry/user-prompt.txt` to clear it.

When `--dry-run` is used with `--user-prompt`, the directive is displayed in the dry-run report but not persisted to disk.

## Project Structure

### Your project with fry

```
your-project/
  .gitignore
  plans/                             # YOUR INPUT (committed to your repo)
    plan.md                          #   Detailed build plan -- at least one of
    executive.md                     #   plan.md or executive.md is required
  .fry/                              # Generated artifacts (gitignored)
    AGENTS.md                        #   Operational rules for the AI agent
    epic.md                          #   Sprint definitions
    verification.md                  #   Independent verification checks
    prompt.md                        #   Assembled per sprint
    user-prompt.txt                  #   Persisted user directive (optional)
    sprint-progress.txt              #   Per-sprint iteration memory
    epic-progress.txt                #   Cross-sprint compacted summary
    deviation-log.md                 #   Review decision audit trail
    review-prompt.md                 #   Assembled reviewer prompt (transient)
    replan-prompt.md                 #   Assembled replanner prompt (transient)
    build-logs/                      #   Per-iteration logs
    .fry.lock                        #   Concurrency lock
```

Unlike the bash version, fry is installed as a standalone binary -- it does not live inside your project's `.fry/` directory. The `.fry/` directory contains only generated artifacts.

### Setup

```bash
# Start a new project
mkdir my-project && cd my-project
mkdir -p plans
# Write plans/plan.md and/or plans/executive.md
fry --engine claude

# Add fry to an existing project
mkdir -p plans
# Write your plan files, then run:
fry --engine claude
```

fry automatically creates the `.fry/` directory, initializes git (if needed), and sets up `.gitignore` entries on first run.

## Epic File Format

An epic file defines global configuration and a sequence of sprint blocks. It can be auto-generated from your plan via `fry prepare` or hand-authored.

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
@verification .fry/verification.md
@review_between_sprints
@max_deviation_scope 3
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
| `@verification <file>` | Verification checks file (default: `.fry/verification.md`) |
| `@max_heal_attempts <N>` | Auto-heal attempts after verification failure (default: 3; 0 = disabled). Per-sprint override supported in sprint blocks. |
| `@compact_with_agent` | Use AI agent to summarize sprint progress (default: mechanical extraction) |
| `@review_between_sprints` | Enable mid-build sprint review (default: disabled). See [Dynamic Sprint Review](#dynamic-sprint-review). |
| `@review_engine <codex\|claude>` | AI engine for reviewer session (default: same as `@engine`) |
| `@review_model <model>` | Model override for the reviewer session |
| `@max_deviation_scope <N>` | Maximum sprints a single deviation can touch (default: 3) |

### Sprint Blocks

```
@sprint 1
@name Scaffolding & Infrastructure
@max_iterations 20
@promise SPRINT1_DONE
@prompt
Sprint 1: Scaffolding for My Project.

Read .fry/AGENTS.md first, then plans/plan.md.

Build:
1. package.json with TypeScript, Express, pg dependencies
2. tsconfig.json with strict mode
3. src/index.ts -- minimal Express server
4. docker-compose.yml -- PostgreSQL 16
...

Verify:
Verification checks are defined in .fry/verification.md (sprint 1).
- npm run build succeeds
- npm test passes
- Docker services start and become healthy

CRITICAL: This is a TypeScript project. No JavaScript files.

If stuck after 10 iterations: check import paths and tsconfig paths.

===PROMISE: SPRINT1_DONE=== when all checks pass.
@end
```

Each sprint prompt follows a 7-part structure:

1. **Opener** -- "Sprint N: [What] for [Project]."
2. **References** -- "Read .fry/AGENTS.md, then plans/plan.md [relevant sections]."
3. **Build list** -- Numbered, specific: exact filenames, function signatures, SQL DDL
4. **Constraints** -- "CRITICAL: [thing that will go wrong if ignored]."
5. **Verification** -- Reference to .fry/verification.md checks, plus prose summary of key outcomes
6. **Stuck hint** -- "If stuck after N iterations: [most likely cause + fix]."
7. **Promise** -- "Output `===PROMISE: TOKEN===` when [exit criteria]."

## Manual Epic Generation

If you prefer to generate the epic with a different LLM (ChatGPT, Claude web, etc.), fry ships with embedded templates to help:

1. Run `fry prepare --validate-only` to check prerequisites
2. Use the epic-example.md template (embedded in the binary, written to a temp dir during prepare) as a format reference
3. Save your epic as `.fry/epic.md`
4. Validate: `fry --dry-run`

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

## Preflight Checks

Before executing any sprints, fry runs a preflight validation phase. All checks must pass or the build is aborted.

### What's checked

| Check | Fails when |
|---|---|
| AI engine CLI | `codex` or `claude` binary not on PATH |
| `git` | Not installed |
| `bash` | Not installed |
| `plans/` directory | Missing |
| Input files | Both `plans/plan.md` and `plans/executive.md` are missing |
| `.fry/AGENTS.md` existence | Missing |
| `.fry/AGENTS.md` placeholder | First line matches the standard placeholder marker |
| `.fry/AGENTS.md` content | Fewer than 5 lines |
| Docker | Not installed (only when sprints at or after `@docker_from_sprint` are in range) |
| `@require_tool` | Any declared tool not on PATH |
| `@preflight_cmd` | Any custom preflight command exits non-zero |
| Disk space | Warning (not fatal) if less than 2 GB free |

## Verification

fry supports independent verification of each sprint's deliverables. When a `.fry/verification.md` file is present, fry runs machine-executable checks after the agent signals completion.

### Check Primitives

| Primitive | Example | Passes when |
|---|---|---|
| `@check_file <path>` | `@check_file src/index.ts` | File exists and is non-empty |
| `@check_file_contains <path> <pattern>` | `@check_file_contains package.json "typescript"` | File contains pattern (grep -E) |
| `@check_cmd <command>` | `@check_cmd npm run build` | Command exits 0 |
| `@check_cmd_output <cmd> \| <pattern>` | `@check_cmd_output curl -s /health \| "ok"` | stdout matches pattern |

### Outcome Matrix

| Promise Token | Checks Pass | Result |
|---|---|---|
| Found | All pass | **PASS** |
| Found | Some fail | Enters **heal loop** |
| Not found | All pass | **PASS** (verification passed, no promise) |
| Not found | Some fail | Enters **heal loop** |

If the heal loop exhausts all attempts without passing, the sprint is marked **FAIL**. If healing succeeds, the sprint is marked **PASS (healed)**.

### Verification File Format

`.fry/verification.md` uses the `@sprint N` block structure:

```markdown
@sprint 1
@check_file go.mod
@check_file_contains go.mod "module myproject"
@check_cmd go build ./...
@check_cmd_output go version | "go1\."

@sprint 2
@check_cmd go test ./...
```

The `@verification` directive in the epic file can override the default filename:

```
@verification .fry/checks-phase2.md
```

### Graceful Degradation

- If `.fry/verification.md` does not exist, fry falls back to promise-only behavior
- If `fry prepare` fails to generate `.fry/verification.md`, it logs a warning and continues
- If a sprint has no checks defined, it behaves as if no verification file exists for that sprint

## Self-Healing

When verification checks fail after a sprint completes, fry enters a **heal loop** that automatically re-runs the AI agent with a targeted fix prompt, then re-verifies.

### How It Works

1. **Verification fails** -- one or more checks return non-zero
2. **Diagnostics collected** -- all checks re-run, capturing pass/fail status and stderr/stdout (truncated to 20 lines per check)
3. **Heal prompt assembled** -- `.fry/prompt.md` is overwritten with a targeted heal prompt containing the specific failures, instructions for minimum changes, and pointers to context files
4. **Agent re-runs** -- a fresh agent session executes with the heal prompt
5. **Pre-sprint hook re-runs** -- if configured (e.g., `npm install`), runs again to pick up changes
6. **Re-verification** -- all checks for the sprint run again
7. **Repeat or exit** -- if checks still fail, the failure report is appended to `.fry/sprint-progress.txt` and steps 2-6 repeat. Exits when all checks pass (**PASS (healed)**) or max attempts exhausted (**FAIL**)

### Configuration

| Directive | Scope | Default | Description |
|---|---|---|---|
| `@max_heal_attempts <N>` | Global | 3 | Maximum heal attempts for all sprints |
| `@max_heal_attempts <N>` | Per-sprint | Inherits global | Override for a specific sprint |

Set `@max_heal_attempts 0` globally or per-sprint to disable healing entirely.

```
# Global: allow up to 5 heal attempts for all sprints
@max_heal_attempts 5

@sprint 3
@name Complex Integration
@max_heal_attempts 8       # This sprint gets more attempts
```

## Agent Session Banners

Every time fry starts a new AI agent session, it prints a `▶ AGENT` banner to the terminal. This is always on -- not gated by `--verbose`.

```
[2026-03-10 12:00:00] ▶ AGENT  sprint 3/8 "Auth & Permissions"  iter 1/5  engine=claude  model=default
[2026-03-10 12:05:12] ▶ AGENT  sprint 3/8 "Auth & Permissions"  iter 2/5  engine=claude  model=default
[2026-03-10 12:10:30] ▶ AGENT  sprint 3/8 "Auth & Permissions"  heal 1/3  engine=claude  model=default
```

## Dynamic Sprint Review

When `@review_between_sprints` is enabled in the epic, fry inserts a review gate between sprints. After each passing sprint, a **separate LLM session** evaluates whether downstream sprints need adjustment based on what was actually built. If the reviewer recommends a deviation, a **replanner agent** makes targeted edits to the affected `@prompt` blocks in the epic.

This is fully opt-in. When disabled (the default), fry behaves exactly as before. Use `--no-review` to disable at runtime even when the epic enables it.

### How It Works

1. Sprint N completes and passes verification
2. A **reviewer** LLM session receives: what was built (progress logs), what's planned (remaining sprint prompts), the original plan, and prior deviation history
3. The reviewer outputs a verdict: `CONTINUE` (proceed as-is) or `DEVIATE` (adjust downstream sprints)
4. If `DEVIATE`: a **replanner** LLM session makes surgical edits to affected `@prompt` blocks in `epic.md`, within the scope cap
5. fry re-parses the modified sprints and continues the build

The reviewer has an explicit **bias toward CONTINUE** -- it only recommends deviation when a downstream sprint prompt references something that was built differently than assumed.

### Configuration

```
@review_between_sprints         # Enable the feature
@review_engine claude           # Use a specific engine for the reviewer (optional)
@review_model claude-sonnet-4-6 # Use a specific model for the reviewer (optional)
@max_deviation_scope 3          # Max sprints a single deviation can touch (default: 3)
```

### Safeguards

- `plans/plan.md` is never modified -- it remains the human-authored source of truth
- Completed sprints cannot be touched by the replanner
- Structural directives (`@sprint`, `@name`, `@max_iterations`, `@promise`) cannot be changed
- The original `epic.md` is backed up before any replan edit
- If replan validation fails (scope exceeded, completed sprint modified), the replan is rejected and the build continues with the original epic

### Audit Trail

Every review decision is recorded in `.fry/deviation-log.md`. After the build completes, a summary is appended.

### Testing

Use `--simulate-review` to test the review/replan pipeline without making LLM calls:

```bash
fry --simulate-review CONTINUE   # Injects CONTINUE verdict at every review gate
fry --simulate-review DEVIATE    # Injects DEVIATE verdict with a synthetic deviation spec
```

## Resuming Failed Builds

When a sprint fails, fry first attempts to [self-heal](#self-healing). Only after exhausting all heal attempts does the build stop.

In all failure cases, fry commits partial work and stops with a resume command:

```
Resume: fry run .fry/epic.md 4
```

Progress is preserved in `.fry/sprint-progress.txt`, `.fry/epic-progress.txt`, and git history. The agent picks up where it left off.

## Planning Mode (Non-Code Projects)

fry's execution engine is project-agnostic -- the sprint loop, verification runner, and heal loop work identically regardless of whether the output is code or documents.

Pass `--planning` to use planning-domain prompts that generate sprints for producing structured documents instead of code. Use it for business plans, trip planning, research reports, strategic analyses, or any endeavor that requires rigorous, phased document creation.

### How It Differs

| Aspect | Default (software) | `--planning` |
|---|---|---|
| `.fry/AGENTS.md` | Technology constraints, architecture rules, testing patterns | Domain boundaries, analytical frameworks, document quality standards |
| Sprint phasing | Scaffolding -> Schema -> Logic -> Integration -> E2E | Research -> Analysis -> Strategy -> Detailed Planning -> Synthesis |
| Sprint deliverables | Source files, configs, tests | Markdown documents, analyses, strategies |
| Verification | Build succeeds, tests pass, files exist | Documents exist, contain required sections, meet minimum depth |

### Quick Start (Planning)

```bash
mkdir -p plans
cat > plans/plan.md << 'EOF'
# Coffee Shop Launch Plan

## Vision
Open a specialty coffee shop in downtown Portland targeting remote workers.

## Key Challenges
- Location selection and lease negotiation
- Menu development and supplier sourcing
- Financial projections and funding strategy
- Marketing and pre-launch buzz
EOF

# Generate and run (one command)
fry --planning --engine claude
```

### Verification for Documents

The same four check primitives verify document deliverables:

```
@check_file plans/market-analysis.md
@check_file_contains plans/market-analysis.md "## Market Size"
@check_cmd test $(wc -w < plans/market-analysis.md) -ge 500
@check_cmd_output grep -c '^## ' plans/market-analysis.md | ^[5-9]
```

## File Reference

| File | Purpose | Created by |
|---|---|---|
| `plans/plan.md` | Detailed build plan | You or auto-generated |
| `plans/executive.md` | Executive context | You |
| `.fry/AGENTS.md` | Operational rules for the AI agent | `fry prepare` |
| `.fry/epic.md` | Sprint definitions | `fry prepare` or hand-authored |
| `.fry/verification.md` | Independent verification checks | `fry prepare` or hand-authored |
| `.fry/prompt.md` | Assembled per-sprint prompt | `fry run` at runtime |
| `.fry/user-prompt.txt` | Persisted user directive | `fry run` or `fry prepare` |
| `.fry/sprint-progress.txt` | Per-sprint iteration memory | `fry run` at runtime |
| `.fry/epic-progress.txt` | Cross-sprint compacted summary | `fry run` at runtime |
| `.fry/deviation-log.md` | Audit trail of review decisions | `fry run` at runtime |
| `.fry/build-logs/` | Per-iteration and per-sprint logs | `fry run` at runtime |
| `.fry/.fry.lock` | Concurrency lock | `fry run` at runtime |

## Environment Variables

| Variable | Description |
|---|---|
| `FRY_ENGINE` | Default engine (`codex` or `claude`). Overridden by `--engine` flag and `@engine` directive. |

## Requirements

- **git** -- for automatic checkpointing
- **bash** -- required by AI engine CLIs and verification commands
- **Go 1.22+** -- to build from source
- **OpenAI Codex CLI** (`npm i -g @openai/codex`) -- if using the codex engine
- **Claude Code CLI** (`npm i -g @anthropic-ai/claude-code`) -- if using the claude engine
- **Docker** (optional) -- only needed if your epic uses `@docker_from_sprint`

## License

See repository for license information.
