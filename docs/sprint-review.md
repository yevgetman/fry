# Dynamic Sprint Review

When `@review_between_sprints` is enabled in the epic, fry inserts a review gate between sprints. After each passing sprint, a **separate LLM session** evaluates whether downstream sprints need adjustment based on what was actually built. If the reviewer recommends a deviation, a **replanner agent** makes targeted edits to the affected `@prompt` blocks in the epic.

This is fully opt-in. When disabled (the default), fry proceeds directly from one sprint to the next.

## How It Works

1. Sprint N completes and passes verification
2. A **reviewer** LLM session receives: what was built (progress logs), what's planned (remaining sprint prompts), the original plan, and prior deviation history
3. The reviewer outputs a verdict: `CONTINUE` (proceed as-is) or `DEVIATE` (adjust downstream sprints)
4. If `DEVIATE`: a **replanner** LLM session makes surgical edits to affected `@prompt` blocks in `epic.md`, within the scope cap
5. fry re-parses the modified sprints and continues the build

The reviewer has an explicit **bias toward CONTINUE** — it only recommends deviation when a downstream sprint prompt references something that was built differently than assumed.

**Effort level effects on reviews:**
- At `low` effort, sprint reviews are **skipped entirely**, even if `@review_between_sprints` is enabled
- At `max` effort, the reviewer bias shifts to **THOROUGH REVIEW** — it applies heightened scrutiny and recommends DEVIATE for any deviation that could affect system correctness, not just downstream prompt mismatches

See [Effort Levels](effort-levels.md) for full details.

## Configuration

```
@review_between_sprints         # Enable the feature
@review_engine claude           # Use a specific engine for the reviewer (optional)
@review_model claude-sonnet-4-6 # Use a specific model for the reviewer (optional)
@max_deviation_scope 3          # Max sprints a single deviation can touch (default: 3)
```

### Disabling at Runtime

Use `--no-review` to disable sprint review even when the epic enables it:

```bash
fry --no-review
```

## Reviewer Prompt

The reviewer receives structured context:

1. **Role**: Build plan reviewer
2. **Bias**: CONTINUE by default
3. **What was built**: Cumulative progress + detailed sprint log
4. **Remaining plan**: Prompts for upcoming sprints
5. **Original plan**: Reference to `plans/plan.md`
6. **Prior deviations**: Audit trail from `.fry/deviation-log.md`

## Deviation Spec

When the reviewer recommends `DEVIATE`, it produces a structured deviation spec:

- **Trigger**: What specifically differs from the plan
- **Affected sprints**: List of sprint numbers to modify
- **Per-sprint changes**: What specifically needs to change in each `@prompt` block
- **Risk assessment**: Low | Medium | High

## Replanner

When a deviation is approved, the replanner:

1. Loads the deviation spec
2. Reads the original epic, plan, and deviation log
3. Makes **surgical edits** to affected `@prompt` blocks only
4. Validates the result:
   - Scope check — can't affect more than `@max_deviation_scope` sprints
   - No completed sprints touched
   - No structural directives changed (`@sprint`, `@name`, `@promise`, `@max_iterations`)
5. Backs up the original epic before writing changes
6. Re-parses and re-validates the modified epic

## Safeguards

- `plans/plan.md` is **never modified** — it remains the human-authored source of truth
- Completed sprints cannot be touched by the replanner
- Structural directives (`@sprint`, `@name`, `@max_iterations`, `@promise`) cannot be changed
- The original `epic.md` is backed up before any replan edit
- If replan validation fails (scope exceeded, completed sprint modified), the replan is rejected and the build continues with the original epic

## Audit Trail

Every review decision is recorded in `.fry/deviation-log.md`. After the build completes, a summary is appended.

## Testing

Use `--simulate-review` to test the review/replan pipeline without making LLM calls:

```bash
fry --simulate-review CONTINUE   # Injects CONTINUE verdict at every review gate
fry --simulate-review DEVIATE    # Injects DEVIATE verdict with a synthetic deviation spec
```

## Terminal Output

```
[2026-03-10 12:15:00] --- SPRINT REVIEW: after Sprint 3 ---
[2026-03-10 12:16:30] Review verdict: CONTINUE
```

Or when a deviation is needed:

```
[2026-03-10 12:15:00] --- SPRINT REVIEW: after Sprint 3 ---
[2026-03-10 12:16:30] Review verdict: DEVIATE
[2026-03-10 12:16:31] Running replanner agent...
[2026-03-10 12:18:00] Epic updated successfully.
```
