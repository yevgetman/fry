# User Prompt

The `--user-prompt` flag lets you inject a top-level directive that applies to every sprint. This is useful for steering the build without modifying `plan.md` or `executive.md`.

## Usage

```bash
# Steer the build toward backend work
fry --user-prompt "focus on backend API, skip frontend styling"

# Constrain the approach
fry --user-prompt "use only standard library, no third-party dependencies"
```

## How It Works

The user prompt is:

- **Injected as Layer 1.5** in the prompt hierarchy — between executive context ("why") and strategic plan ("what"). The agent sees it as a priority directive that applies to all sprints.
- **Passed through to prepare** — when `fry run` auto-generates the epic, the user prompt is forwarded to `fry prepare`, influencing the generation of `.fry/AGENTS.md`, `.fry/epic.md`, and `.fry/verification.md`.
- **Included in heal prompts** — when self-healing runs, the user directive is listed as context so heal passes respect it.

## Prompt Hierarchy

| Layer | Content | Source |
|---|---|---|
| 1 | Executive context | `plans/executive.md` |
| **1.5** | **User directive** | `--user-prompt` |
| 2 | Strategic plan reference | `plans/plan.md` |
| 3 | Sprint instructions | `@prompt` block from epic |
| 4 | Iteration memory | Progress files |
| 5 | Completion signal | Promise token |

## Persistence

The user prompt is persisted to `.fry/user-prompt.txt`:

- **Saved on first use** and automatically reused on subsequent runs
- **Override** by passing a new `--user-prompt` value
- **Clear** by deleting `.fry/user-prompt.txt`

When `--dry-run` is used with `--user-prompt`, the directive is displayed in the dry-run report but **not** persisted to disk.

## Examples

```bash
# Technical constraints
fry --user-prompt "no ORMs, use raw SQL only"

# Priority guidance
fry --user-prompt "focus on backend API, skip frontend styling"

# Scope limitation
fry --user-prompt "only implement the auth and user modules for now"

# Framework preference
fry --user-prompt "use Gin for HTTP routing, not net/http"
```
