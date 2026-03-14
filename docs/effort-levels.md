# Effort Levels

Effort levels control how many sprints Fry generates, how dense each sprint is, and how much verification rigor is applied. This prevents over-engineering simple tasks (e.g., 7 sprints for a basic HTML page) and lets you dial up thoroughness for mission-critical builds.

## The Four Levels

| Level | Sprint Count | Max Iterations | Prompt Detail | Review Behavior |
|---|---|---|---|---|
| `low` | 1-2 | 10-15 | Concise, essentials only | Reviews skipped |
| `medium` | 2-4 | 15-25 | Moderate, all 7 parts but brief | Reviews optional |
| `high` | 4-10 | 15-35 | Full 7-part prompts (current default) | Normal review bias |
| `max` | 4-10 | 30-50 | Extended 9-part prompts | Thorough review, lower deviation threshold |

### `low` — Quick Tasks

For simple, well-bounded work: a single page, a config change, a small utility, 1-3 files.

- Generates at most 2 sprints
- Skips scaffolding as a separate sprint — folds it into Sprint 1
- Sprint prompts omit REFERENCES and STUCK HINT sections for brevity
- Sprint reviews are skipped entirely, even if `@review_between_sprints` is enabled
- Sprint and build audits are skipped entirely (regardless of audit settings)
- Focus is on core deliverables only; exhaustive edge cases are omitted

```bash
fry --effort low --engine claude
```

### `medium` — Moderate Features

For multi-component features with some integration: 4-15 files, a few interconnected pieces.

- Generates 2-4 sprints (prefers the lower end)
- Merges layers that would be separate at `high` (e.g., schema + domain types in one sprint)
- Full 7-part prompt structure, but each section is concise
- Essential edge cases included, exhaustive coverage skipped

```bash
fry --effort medium --engine claude
```

### `high` — Complex Systems (Default)

This is the current default behavior. For complex systems with databases, APIs, extensive testing, 15+ files.

- Generates 4-10 sprints following the standard layering order
- Full 7-part prompt structure with comprehensive detail
- All standard sizing guidelines apply
- Normal review behavior when enabled

```bash
fry --effort high --engine claude
# or equivalently:
fry --engine claude
```

### `max` — Mission-Critical Builds

For high-stakes projects where correctness is paramount. Same sprint count as `high`, but with significantly more rigor per sprint.

- Same sprint count as `high` (4-10), but with higher iteration budgets (30-50 per sprint)
- Sprint prompts are extended beyond the standard 7-part structure:
  - **Part 8: Analysis & Edge Cases** — enumerates every edge case, race condition, error scenario, and boundary condition
  - **Part 9: Quality Gates** — explicit quality criteria beyond verification (performance targets, security considerations, code review checklist items)
- Automatically enables `@review_between_sprints` and `@compact_with_agent`
- Sets `@max_heal_attempts` to 5 (default is 3)
- Review bias shifts from CONTINUE to THOROUGH REVIEW — the reviewer applies heightened scrutiny and recommends DEVIATE for any deviation that could affect system correctness
- No-op detection threshold is raised from 2 to 3 consecutive iterations, giving the agent more room to iterate
- A quality directive is injected into every sprint prompt, instructing the agent to handle all edge cases, write defensive code, and validate assumptions

```bash
fry --effort max --engine claude
```

## Auto-Detection

When no `--effort` flag is specified, Fry instructs the AI to analyze the plan document and select the appropriate level automatically:

- **1-3 files** described in the plan → `low`
- **4-15 files** → `medium`
- **15+ files** → `high`

The auto-detected level is written to the epic as an `@effort` directive so it persists across runs.

```bash
# Let the AI decide
fry --engine claude

# Check what it chose
fry --dry-run
# Output includes: Effort: medium
```

Auto-detection watches for common over-engineering signals:
- Creating separate scaffolding sprints for projects that need no scaffolding
- Splitting 3-file changes across 3+ sprints
- Adding schema/migration sprints for projects with no database
- Creating separate "wiring" sprints for simple, flat architectures

## Usage

### CLI Flag

The `--effort` flag is available on both `fry run` and `fry prepare`:

```bash
# During a full run (prepare + execute)
fry --effort low
fry run --effort medium --engine claude

# During preparation only
fry prepare --effort high --engine claude

# Auto-detect (default when flag is omitted)
fry --engine claude
```

### Epic Directive

The effort level is stored in the epic file as a global directive:

```
@epic My Project Phase 1
@engine claude
@effort medium
@docker_from_sprint 2
```

When both `--effort` flag and `@effort` directive are present, the **epic directive takes precedence** and the CLI flag is ignored with a warning. To change the effort level, re-run `fry prepare` with the new `--effort` value so it is baked into the epic.

### Dry Run

The dry-run report includes the effort level:

```bash
fry --dry-run
```

```
Epic: My Project Phase 1
Project dir: /path/to/project
Epic file: .fry/epic.md
Engine: claude
Effort: medium
Sprints: 1-3 of 3
```

## Validation

The epic validator enforces sprint count limits based on effort level:

| Level | Max Sprints |
|---|---|
| `low` | 2 |
| `medium` | 4 |
| `high` | 10 |
| `max` | 10 |

If an epic has `@effort low` but contains 5 sprints, validation will fail:

```
error: effort level "low" allows at most 2 sprints, but epic has 5
```

This validation only applies when `@effort` is explicitly set. Epics without an `@effort` directive have no sprint count limit.

When no effort level is set (auto-detect or unset), the default max iterations per sprint is 25 (same as `high`).

## Effort Level Effects Summary

| Aspect | `low` | `medium` | `high` | `max` |
|---|---|---|---|---|
| Sprint count cap | 2 | 4 | 10 | 10 |
| Default max iterations | 12 | 20 | 25 | 40 |
| Prompt structure | Concise | Moderate 7-part | Full 7-part | Extended 9-part |
| Scaffolding sprint | Merged into Sprint 1 | Normal | Normal | Normal |
| Review behavior | Skipped | Normal | Normal | Thorough (lower DEVIATE threshold) |
| Sprint audit | Skipped | Normal (if enabled) | Normal (if enabled) | Normal (if enabled) |
| Build audit | Skipped | Runs on full epic completion | Runs on full epic completion | Runs on full epic completion |
| No-op threshold | 2 iterations | 2 iterations | 2 iterations | 3 iterations |
| Quality directive | No | No | No | Yes (injected into every prompt) |
| Heal attempts | Default (3) | Default (3) | Default (3) | 5 |
| Compact with agent | Default | Default | Default | Enabled |

## Backward Compatibility

- Existing epics without `@effort` work exactly as before — no behavior changes
- The `--effort` flag is entirely optional
- Auto-detection only activates during `fry prepare` (Step 2 epic generation), not on existing epics
- The `high` effort level produces identical behavior to the pre-effort-level system

## Examples

### Simple static site

```bash
# Plan describes 2 HTML pages and a CSS file
fry --effort low --engine claude
# Result: 1 sprint, ~12 iterations, done in minutes
```

### REST API with database

```bash
# Plan describes Express + PostgreSQL + 5 endpoints
fry --effort medium --engine claude
# Result: 3 sprints (scaffold+schema, handlers, integration), ~20 iterations each
```

### Full-stack application

```bash
# Plan describes React frontend + Go backend + PostgreSQL + Docker + CI/CD
fry --engine claude
# Auto-detects: high
# Result: 6 sprints, standard sizing
```

### Payment processing system

```bash
# Correctness is critical — financial transactions, compliance requirements
fry --effort max --engine claude
# Result: 7 sprints with extended prompts, thorough reviews, 40+ iterations each
```
