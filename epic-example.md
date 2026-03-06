# Epic Template — [Project Name] [Phase Name]
#
# This file defines sprint phases for fry.sh.
# Run: ./fry.sh epic-[project].md
#
# Supports two AI engines: OpenAI Codex (default) and Claude Code.
# Select via: @engine directive, --engine flag, or FRY_ENGINE env var.
#
# This file can be generated automatically:
#   ./fry-prepare.sh epic.md          (standalone)
#   ./fry.sh epic.md                  (auto-generates if epic.md doesn't exist)
#
# Or use GENERATE_EPIC.md as a prompt with any LLM.
#
# ─── HOW TO USE THIS TEMPLATE ───
#
# 1. Replace all [bracketed placeholders] with project-specific content
# 2. Adjust the number of sprints (delete or add @sprint blocks as needed)
# 3. Each sprint should be completable in 10-35 iterations
# 4. Sprints execute sequentially — later sprints can depend on earlier ones
# 5. Remove this instructions block and the example sprint content
#
# ─── ANATOMY OF A GOOD SPRINT PROMPT ───
#
# Each @prompt block follows a consistent 7-part structure:
#
#   OPENER        → "Sprint N: [What] for [Project]."
#   REFERENCES    → "Read AGENTS.md, then [plan sections]."
#   BUILD LIST    → Numbered list of specific files/components to create,
#                   with exact filenames, function signatures, behavior specs.
#   CONSTRAINTS   → "CRITICAL: [thing that will go wrong if ignored]"
#   VERIFICATION  → Bulleted checklist the agent runs after each iteration.
#   STUCK HINT    → "If stuck after N iterations: [most likely cause + fix]"
#   PROMISE       → "Output <promise>TOKEN</promise> when [exit criteria]."
#
# ─── SPRINT SIZING GUIDELINES ───
#
#   Too small: "Create a single config file" (1-2 iterations, wasteful)
#   Right-sized: "Scaffolding: module init, config, Docker, DB pool, Makefile" (10-20 iters)
#   Too large: "Build the entire backend" (50+ iterations, context thrashing)
#
#   Rule of thumb: each sprint should produce 5-20 files and be testable
#   independently of later sprints. If you can't write a verification
#   checklist, the sprint is too vague.
#
# ─── SPRINT ORDERING PRINCIPLES ───
#
#   Sprint 1: Always scaffolding (project structure, config, tooling, Docker)
#   Early sprints: Foundation layers (schema, types, interfaces)
#   Middle sprints: Implementation layers (business logic, data access)
#   Late sprints: Integration layers (API handlers, wiring, service layer)
#   Final sprint: Always integration tests, E2E verification, wiring
#
#   Dependencies flow forward only: Sprint 4 may depend on Sprint 1-3,
#   never on Sprint 5+. Each sprint should state its dependencies.
#
# ─── ITERATION COUNT GUIDELINES ───
#
#   Scaffolding/config:     15-20 max_iterations
#   Schema/migrations:      15-20
#   Domain models/types:    20-25
#   Business logic/engines: 25-30
#   API handlers/routes:    20-25
#   Wiring/integration/E2E: 30-35
#
# =============================================================================

# =============================================================================
# Global Configuration
# =============================================================================

@epic [Project Name] [Phase Name] — [Brief Description]
@engine codex
# @engine claude
@docker_from_sprint [N]
@docker_ready_cmd [custom health check command, e.g.: docker compose exec -T postgres pg_isready -U myapp]
@docker_ready_timeout [seconds, default 30]
@require_tool [tool1, e.g.: go, node, python3]
@require_tool [tool2, e.g.: docker]
@pre_sprint [command to run before each sprint, e.g.: go mod tidy 2>/dev/null || true]

# ─── GLOBAL DIRECTIVE REFERENCE ───
#
# @epic <name>                    Display name for logs/summaries
# @engine <codex|claude>          AI engine to use (default: codex)
# @docker_from_sprint <N>         Start Docker from sprint N (omit if no Docker)
# @docker_ready_cmd <cmd>         Health check after docker-compose up
# @docker_ready_timeout <secs>    How long to wait for Docker (default: 30)
# @require_tool <name>            CLI tool that must be on PATH (repeatable)
# @preflight_cmd <cmd>            Custom check before build starts (repeatable)
# @pre_sprint <cmd>               Run before every sprint starts
# @pre_iteration <cmd>            Run before every agent exec call
# @model <model>                  Override the agent model (alias: @codex_model)
# @engine_flags <flags>           Extra flags for agent exec (alias: @codex_flags)

# =============================================================================
# Sprint 1 — Scaffolding & Infrastructure
# Creates: [list of deliverables]
# Depends on: nothing
# =============================================================================

@sprint 1
@name Scaffolding & Infrastructure
@max_iterations 20
@promise SPRINT1_DONE
@prompt
Sprint 1: Scaffolding & Infrastructure for [Project Name].

Read AGENTS.md first, then plans/plan.md [relevant sections].

[Project context: naming conventions, module paths, key decisions.]

IMPORTANT: AGENTS.md already exists with critical project rules. Do NOT create, overwrite, or modify AGENTS.md. It is read-only for all sprints.

Build:
1. [Module/package init — e.g., go.mod, package.json, pyproject.toml]
2. [Directory tree — list all directories to create]
3. [Entry point — e.g., main.go, index.ts, app.py — minimal placeholder]
4. [Config system — struct/class, env var loading, defaults, unit test]
5. [Database connection pool — connect, ping, error handling]
6. [Cache/Redis client — if applicable]
7. [Migration runner — if applicable]
8. [docker-compose.yml — services, ports, health checks, volumes]
9. [Dockerfile — multi-stage build]
10. [Makefile/scripts — build, run, test, migrate, docker-up targets]
11. [.env.example — all environment variables with defaults]
12. [.gitignore — build artifacts, env files, logs]

Dependencies to install: [list packages/modules with exact names]

Verify:
Concrete verification checks are defined in verification.md (sprint 1).
Review those checks — they will be run independently after you signal completion.
Additionally, confirm after each iteration:
- [Build command succeeds — e.g., go build ./..., npm run build, pytest --collect-only]
- [Lint/vet passes — e.g., go vet ./..., eslint, ruff check]
- [Docker services start and become healthy]
- [Config unit tests pass]
- [AGENTS.md is unchanged]

CRITICAL: [Key constraint — e.g., "This is a Go project. No Node.js." or "Use TypeScript strict mode."]

If stuck after 10 iterations: [Most likely cause and fix — e.g., "import path mismatch" or "Docker networking issue"]

Output <promise>SPRINT1_DONE</promise> when [specific exit criteria].

# =============================================================================
# Sprint 2 — [Layer Name]
# Creates: [deliverables]
# Depends on: Sprint 1 ([what specifically])
# =============================================================================

@sprint 2
@name [Layer Name]
@max_iterations 20
@promise SPRINT2_DONE
@prompt
Sprint 2: [Layer Name] for [Project Name].

Read AGENTS.md [relevant rules], then plans/plan.md [relevant sections].

[Context for this sprint — what exists from Sprint 1 that this builds on.]

[Build instructions — same numbered list pattern as Sprint 1. Be specific:
 exact filenames, function signatures, SQL DDL, API shapes, etc.]

Verify:
Concrete verification checks are defined in verification.md (sprint 2).
Review those checks — they will be run independently after you signal completion.
- [Additional verification specific to this sprint's deliverables]

CRITICAL: [Key constraint for this sprint]

If stuck: [Diagnosis and fix for the most common failure mode]

Output <promise>SPRINT2_DONE</promise> when [exit criteria].

# =============================================================================
# Sprint 3 — [Layer Name]
# Creates: [deliverables]
# Depends on: Sprint 1 ([what]), Sprint 2 ([what])
# =============================================================================

@sprint 3
@name [Layer Name]
@max_iterations 25
@promise SPRINT3_DONE
@prompt
Sprint 3: [Layer Name] for [Project Name].

Read AGENTS.md, then plans/plan.md [relevant sections].

[Build instructions.]

Verify:
Concrete verification checks are defined in verification.md (sprint 3).
- [Checklist]

If stuck: [Hint]

Output <promise>SPRINT3_DONE</promise> when [exit criteria].

# =============================================================================
# ... (add more sprints as needed — typically 4-10 total)
# =============================================================================

# =============================================================================
# Sprint N (Final) — Wiring, Integration & Verification
# Creates: dependency injection wiring, integration tests, E2E smoke tests
# Depends on: ALL previous sprints
#
# NOTE: The final sprint should ALWAYS be wiring + integration + E2E.
# This is where you verify that everything from earlier sprints connects
# correctly. It typically needs the highest max_iterations (30-35).
# =============================================================================

@sprint N
@name Wiring, Integration & E2E
@max_iterations 35
@promise SPRINTN_DONE
@prompt
Sprint N: Wire everything together, write integration tests, and verify E2E for [Project Name].

This is the FINAL sprint. [Wire the entry point, build test infrastructure, write all integration tests, run full E2E verification.]

Read plans/plan.md [acceptance criteria sections].

PART 1 — Wiring ([entry point] with full dependency injection):
[Numbered list of DI steps — create pools, repos, services, handlers, wire router, start server]

PART 2 — Test infrastructure:
[Test helpers, fixtures, setup/teardown utilities]

PART 3 — Integration tests:
[List every test with name, setup, assertion. Be exhaustive.]

PART 4 — End-to-end smoke test:
[Curl commands or equivalent to verify the running application]

Verify:
Concrete verification checks are defined in verification.md (sprint N).
All acceptance criteria (ALL must pass):
- [Exhaustive checklist from the plan document]

If stuck after 15 iterations: [Most likely wiring issue — e.g., "interface mismatch in DI" or "DB connection in tests"]

Output <promise>SPRINTN_DONE</promise> when [the application starts, all tests pass, E2E verified].
