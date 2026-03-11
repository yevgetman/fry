# Dynamic Sprint Review & Replanning

## Overview

fry supports optional mid-build review between sprints. When enabled via `@review_between_sprints`, a separate LLM session evaluates whether downstream sprints need adjustment based on what was actually built, then a replanner agent makes targeted edits to the epic.

This is fully opt-in. When disabled (the default), fry behaves exactly as before.

## How It Works

```
Sprint N completes (passes verification)
        |
        v
  ┌─────────────┐
  │  REVIEWER    │  Separate LLM session
  │  (evaluate)  │  Reads: sprint results, remaining plan, deviation history
  └──────┬──────┘
         |
    ┌────┴────┐
    |         |
 CONTINUE   DEVIATE
 (proceed)    |
         ┌────▼────┐
         │REPLANNER│  Separate LLM session
         │(edit)   │  Edits: affected @prompt blocks in epic.md
         └────┬────┘
              |
         ┌────▼─────────┐
         │ VALIDATION   │  Ensures scope cap, completed sprint protection
         └────┬─────────┘
              |
         epic.md updated
         deviation-log.md appended
         Sprint N+1 begins
```

## Configuration

### Epic Directives

| Directive | Description | Default |
|---|---|---|
| `@review_between_sprints` | Enable mid-build review | Disabled |
| `@review_engine <codex\|claude>` | Engine for reviewer LLM session | Same as `@engine` |
| `@review_model <model>` | Model override for reviewer | Same as `@model` |
| `@max_deviation_scope <N>` | Max sprints a single deviation can touch | 3 |

### CLI Flags

| Flag | Description |
|---|---|
| `--no-review` | Disable review even if epic enables it |
| `--simulate-review <verdict>` | Test the review pipeline without LLM calls (`CONTINUE` or `DEVIATE`) |

### Testing & Debugging

Use `--simulate-review` to exercise the review/replan pipeline without LLM calls:

```bash
# Simulate CONTINUE at every review gate — verifies logging and deviation-log.md
.fry/fry.sh --simulate-review CONTINUE

# Simulate DEVIATE — generates a synthetic deviation spec and runs the replanner
.fry/fry.sh --simulate-review DEVIATE

# Combine with --dry-run to see the review configuration without executing
.fry/fry.sh --dry-run --simulate-review DEVIATE
```

### Example

```
@epic My Project Phase 1
@engine claude
@review_between_sprints
@max_deviation_scope 2
```

## The Reviewer

The reviewer is a separate LLM session (not the build agent) that runs between sprints. It receives:

1. **What was built** — compacted sprint progress (`epic-progress.txt`)
2. **Detailed sprint log** — iteration-by-iteration detail (`sprint-progress.txt`)
3. **The plan ahead** — `@prompt` blocks for all remaining sprints
4. **Original intent** — `plans/plan.md`
5. **Prior deviations** — `deviation-log.md` (if any)

### Reviewer Bias: CONTINUE

The reviewer has an explicit bias toward continuing the plan. It only recommends DEVIATE when:

- A downstream sprint prompt references something built differently than assumed (wrong file paths, module names, API shapes)
- A technical decision makes a downstream approach infeasible
- A dependency assumed by a downstream sprint doesn't exist or was built differently

It does NOT recommend DEVIATE for:
- Minor style differences
- "Better" approaches requiring rework of completed sprints
- Cosmetic or subjective improvements

### Verdict Format

The reviewer outputs a structured verdict:

```
### Analysis
Brief description of what happened vs. what was planned.

### Downstream Impact Assessment
- Sprint 4: NO IMPACT
- Sprint 5: MINOR — auth middleware path changed
- Sprint 6: NO IMPACT

### Decision
<verdict>CONTINUE</verdict>
```

or

```
### Decision
<verdict>DEVIATE</verdict>

### Deviation Spec
- **Trigger**: Auth middleware built at internal/middleware/auth instead of pkg/auth
- **Affected sprints**: 4, 5
- **Sprint 4**: Update 3 import path references from pkg/auth to internal/middleware/auth
- **Sprint 5**: Update 1 wiring reference
- **Risk assessment**: Low — purely mechanical path changes
```

## The Replanner

When the reviewer says DEVIATE, fry invokes `.fry/fry-replan.sh` — a purpose-built script that:

1. Reads the deviation spec from the reviewer
2. Reads the current `epic.md`, `plan.md`, and `deviation-log.md`
3. Invokes a separate LLM session to make targeted edits to affected `@prompt` blocks
4. Validates the result:
   - Completed sprints were not modified
   - Changes are within the scope cap (`@max_deviation_scope`)
   - Global directives are unchanged
   - Sprint count and structural directives (@sprint/@name/@max_iterations/@promise) are unchanged
5. Backs up the original epic before applying changes

If validation fails, the replan is rejected and the build continues with the original epic.

### Standalone Usage

```bash
# Run replanner directly (usually called by fry.sh)
.fry/fry-replan.sh deviation-spec.txt --epic .fry/epic.md --completed 3 --engine claude

# Dry run — show prompt without modifying anything
.fry/fry-replan.sh deviation-spec.txt --dry-run
```

## Deviation Log

Every review decision is recorded in `.fry/deviation-log.md` (append-only, gitignored):

```markdown
# Deviation Log — My Project Phase 1

## Review after Sprint 2: Database Schema (CONTINUE)
- **Reviewed**: 2024-03-15 14:32
- **Decision**: CONTINUE
- **Rationale**: Sprint completed as planned. Schema matches plan.md specifications.

## Review after Sprint 3: API Handlers (DEVIATE)
- **Reviewed**: 2024-03-15 16:45
- **Decision**: DEVIATE — adjust sprints 4-5
- **Trigger**: Auth middleware at internal/middleware/auth instead of pkg/auth
- **Impact scope**: Sprint 4 (moderate), Sprint 5 (minor)
- **Changes made**: Updated import path references in sprint 4-5 prompts
- **Risk assessment**: Low
```

After the build completes, a summary section is appended:

```markdown
## Build Summary
- **Total sprints**: 7
- **Reviews conducted**: 6
- **Deviations applied**: 2
- **Retries**: 0
- **All deviations**: Low risk (2/2)
```

### Reconstruction Property

The deviation log is designed so that:

```
plans/plan.md + deviation-log.md ≈ final epic.md
```

A human reading both documents can understand every decision that shaped the final build.

## Safeguards

| Safeguard | Purpose |
|---|---|
| Scope cap (`@max_deviation_scope`) | Prevents runaway changes; default 3 sprints |
| Completed sprint protection | Replanner cannot touch sprints that already ran |
| Diff validation | Replan rejected if out-of-scope sprints were modified |
| Epic backup | Original epic saved to build-logs/ before any edit |
| Reviewer bias | Explicit "bias toward CONTINUE" in reviewer prompt |
| `--no-review` flag | User can disable review at any time |
| Structural preservation | @sprint/@name/@max_iterations/@promise cannot be changed |

## Proportional Response

The system enforces proportional response to deviations:

1. **Small deviation** (wrong import path) → affects 1-2 sprints, mechanical edits only
2. **Medium deviation** (different API shape) → affects 2-3 sprints, prompt content changes
3. **Large deviation** (architectural change) → scope cap prevents editing more than `@max_deviation_scope` sprints; if more would be needed, the build stops for human intervention

This prevents a minor discovery from cascading into a full epic rewrite.
