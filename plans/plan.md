# Build Plan

> **This is a sample file.** Replace this entire document with your project's
> build plan before running `./fry.sh`. The format is free-form — write it
> however makes sense for your project. The only requirement is that it
> contains enough detail for an AI agent to decompose it into implementation
> sprints.

## What goes here

This file is the holistic build plan for your project. It should describe
**what** to build (not how to build it — that's what the sprints are for).
Think of it as the technical blueprint that an engineer would read before
starting work.

There is no enforced format. Use whatever structure fits your project:
prose, bullet lists, numbered sections, tables, diagrams in ASCII — anything.
The better the detail here, the better the auto-generated AGENTS.md and
epic.md will be.

## Good plan content includes

- **Architecture overview** — what are the major components and how do they
  connect? (API server, database, cache, queue, frontend, etc.)

- **Technology choices** — language, framework, database, key libraries,
  and why they were chosen.

- **Data model** — tables/collections, their fields, relationships,
  constraints. The more specific (column names, types, indexes), the better.

- **API surface** — endpoints, methods, request/response shapes, auth model.

- **Business logic** — algorithms, scoring rules, workflows, state machines,
  validation rules. Include thresholds, weights, formulas.

- **Infrastructure** — Docker services, environment variables, ports,
  health checks, deployment targets.

- **Testing strategy** — what gets unit tested, what gets integration tested,
  what the E2E smoke test looks like.

- **Acceptance criteria** — how do you know the build is done? What specific
  behaviors must work?

## Example (abbreviated)

Below is a minimal example for a hypothetical task management API. A real
plan would be 10-50x more detailed.

---

### Task Manager API — Build Plan

**Stack:** Go 1.22, PostgreSQL 16, Redis 7, chi router, pgx (raw SQL).

**Database tables:**
- `users` — id (UUID PK), email (UNIQUE), name, password_hash, created_at
- `tasks` — id (UUID PK), user_id (FK → users), title, description,
  status (CHECK: open/in_progress/done), priority (1-5), due_date, created_at

**API endpoints:**
- POST /auth/register — create user, hash password with bcrypt
- POST /auth/login — verify password, return JWT
- GET /tasks — list tasks for authenticated user, cursor pagination
- POST /tasks — create task
- PUT /tasks/{id} — update task (owner only)
- DELETE /tasks/{id} — soft delete (owner only)

**Business rules:**
- Tasks default to status "open" and priority 3
- Overdue tasks (past due_date + status != done) flagged in list response
- Rate limit: 100 req/min per user via Redis sliding window

**Docker:** PostgreSQL + Redis via docker-compose, health checks on both.

**Tests:** Unit tests for business logic, integration tests for each
endpoint, E2E smoke test with curl.

---

## Next steps

1. Replace this file with your actual build plan
2. Optionally create `plans/executive.md` with high-level project context
3. Run `./fry.sh epic.md` — it will generate AGENTS.md and epic.md for you
