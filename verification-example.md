# Verification Checks — [Project Name] [Phase Name]
#
# This file defines machine-executable verification checks for each sprint.
# fry.sh runs these checks independently after the AI agent signals completion.
#
# This file can be generated automatically:
#   ./fry-prepare.sh epic.md          (generates as Step 3)
#   ./fry.sh epic.md                  (auto-generates if epic.md doesn't exist)
#
# Or hand-authored using this template as a reference.
#
# ─── PURPOSE ───
#
# The AI agent self-reports completion via a promise token. This file provides
# independent verification — fry.sh runs these checks to confirm the agent's
# claim. Think of it as the proctor grading the exam, not the student.
#
# Checks also run when the agent exhausts max_iterations without outputting
# a promise token. If all checks pass despite no promise, the sprint can
# auto-recover (work done, token forgotten).
#
# ─── CHECK PRIMITIVES ───
#
# Four check types are supported:
#
#   @check_file <path>
#     File exists and is non-empty.
#     Passes when: test -s <path>
#     Example: @check_file src/index.ts
#
#   @check_file_contains <path> <pattern>
#     File contains a string or regex pattern.
#     Passes when: grep -qE <pattern> <path>
#     The pattern is passed to grep -E (extended regex).
#     Example: @check_file_contains package.json "typescript"
#     Example: @check_file_contains tsconfig.json "\"strict\":\\s*true"
#
#   @check_cmd <command>
#     Run a shell command.
#     Passes when: exit code is 0.
#     Example: @check_cmd npm run build
#     Example: @check_cmd docker compose config --quiet
#
#   @check_cmd_output <command> | <pattern>
#     Run a shell command and check its stdout against a pattern.
#     Passes when: stdout matches the grep -E pattern.
#     The pipe character (|) separates the command from the pattern.
#     Example: @check_cmd_output node -e "console.log(process.version)" | ^v[0-9]+
#     Example: @check_cmd_output cat .nvmrc | ^20
#
# ─── WRITING GOOD CHECKS ───
#
# DO:
#   - Use concrete commands that return 0/non-0
#   - Check for specific files the sprint creates
#   - Run the project's own build/test/lint commands
#   - Check that config files contain required settings
#   - Use docker compose exec for database checks
#
# DO NOT:
#   - Write subjective checks ("code is clean")
#   - Check for things earlier sprints created (only current sprint's work)
#   - Use interactive commands (no -i flags, no prompts)
#   - Rely on network access to external services
#   - Write checks that take more than 30 seconds to run
#
# ─── NOTES ───
#
# - Comments (lines starting with #) and blank lines are ignored
# - Checks within a sprint run sequentially but ALL run even if some fail
# - Results are reported as "N/M checks passed" per sprint
# - If this file doesn't exist, fry.sh falls back to promise-only behavior
# - The @verification directive in epic.md can override this filename
# - When checks fail, fry.sh automatically attempts to heal by re-running
#   the AI agent with a targeted fix prompt (up to @max_heal_attempts times,
#   default 3). Set @max_heal_attempts 0 in the epic to disable healing.
#
# =============================================================================

# =============================================================================
# Sprint 1 — Scaffolding & Infrastructure
# =============================================================================

@sprint 1

# Project structure exists
@check_file package.json
@check_file tsconfig.json
@check_file docker-compose.yml
@check_file src/index.ts
@check_file .env.example
@check_file .gitignore

# Config files have required content
@check_file_contains package.json "typescript"
@check_file_contains tsconfig.json "strict"
@check_file_contains docker-compose.yml "healthcheck"

# Build succeeds
@check_cmd npm run build

# Docker config is valid
@check_cmd docker compose config --quiet

# =============================================================================
# Sprint 2 — Database Schema & Migrations
# =============================================================================

@sprint 2

# Migration files exist
@check_file src/db/migrations/001_initial.sql
@check_file src/db/pool.ts

# Schema contains required tables
@check_file_contains src/db/migrations/001_initial.sql "CREATE TABLE users"
@check_file_contains src/db/migrations/001_initial.sql "CREATE TABLE posts"

# Build still succeeds
@check_cmd npm run build

# Database is reachable (via Docker)
@check_cmd docker compose exec -T postgres pg_isready -U myapp

# =============================================================================
# Sprint 3 — Domain Models & Business Logic
# =============================================================================

@sprint 3

@check_file src/models/user.ts
@check_file src/models/post.ts
@check_cmd npm run build
@check_cmd npm test

# =============================================================================
# Sprint N (Final) — Wiring, Integration & E2E
# =============================================================================

@sprint 4

# All tests pass
@check_cmd npm run build
@check_cmd npm test

# Server starts and responds
@check_cmd_output curl -s http://localhost:3000/health | "status":"ok"

# Required endpoints exist
@check_cmd_output curl -s -o /dev/null -w "%{http_code}" http://localhost:3000/posts | ^200$
