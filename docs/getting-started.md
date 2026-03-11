# Getting Started

## Prerequisites

- **git** — for automatic checkpointing
- **bash** — required by AI engine CLIs and verification commands
- **Go 1.22+** — to build from source
- One of the following AI engine CLIs:
  - **OpenAI Codex CLI**: `npm i -g @openai/codex`
  - **Claude Code CLI**: `npm i -g @anthropic-ai/claude-code`
- **Docker** (optional) — only needed if your epic uses `@docker_from_sprint`

## Installation

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

## Your First Build

### 1. Write your plan

Create a `plans/` directory with at least one of `plan.md` or `executive.md`:

```bash
mkdir -p plans
```

**`plans/plan.md`** — the technical build plan. Write it in any format (prose, bullets, tables) as long as it has enough detail for an AI to decompose into implementation sprints.

**`plans/executive.md`** — a higher-level document describing the project's purpose, business goals, target users, and scope. When both files are present, fry feeds `executive.md` into every generation step so the AI understands *why* the project exists.

```bash
cat > plans/plan.md << 'EOF'
# My Project — Build Plan

**Stack:** Node 20, Express, PostgreSQL 16, TypeScript strict mode.

**Database tables:**
- users — id (UUID PK), email (UNIQUE), name, created_at
- posts — id (UUID PK), user_id (FK), title, body, published (bool), created_at

**API endpoints:**
- POST /users — create user
- GET /posts — list published posts, cursor pagination
- POST /posts — create post (authenticated)

**Tests:** Jest for unit tests, supertest for integration tests.
EOF
```

### 2. Run

```bash
# Uses Codex (default)
fry

# Or use Claude Code
fry --engine claude
```

fry will automatically:
1. Detect that `.fry/epic.md` doesn't exist and run `fry prepare`
2. Generate `.fry/AGENTS.md` (operational rules for the AI)
3. Generate `.fry/epic.md` (sprint definitions)
4. Generate `.fry/verification.md` (independent checks per sprint)
5. Parse the epic file and validate its structure
6. Run preflight checks
7. Execute each sprint until completion or max iterations
8. Run independent verification checks after each sprint

### 3. Validate before running (recommended)

```bash
fry --dry-run
```

This parses the epic, checks prerequisites, and shows the sprint plan without executing anything.

## Adding fry to an Existing Project

```bash
cd my-existing-project
mkdir -p plans
# Write plans/plan.md and/or plans/executive.md
fry --engine claude
```

fry automatically creates the `.fry/` directory, initializes git (if needed), and sets up `.gitignore` entries on first run.

## Input File Options

| Setup | Behavior |
|---|---|
| Only `plans/plan.md` | fry uses your plan directly for all generation |
| Only `plans/executive.md` | fry auto-generates `plan.md` from your executive context (Step 0), then proceeds normally |
| Both files | fry uses `executive.md` as alignment context alongside your detailed `plan.md` for better-aligned artifacts |

When `plan.md` is auto-generated from `executive.md`, the LLM makes all design, architecture, and implementation decisions. The generated file is written to `plans/` so you can review it before building.
