# User Prompt

The `--user-prompt` flag lets you inject a top-level directive that applies to every sprint. It can also bootstrap an entire project from scratch when no plan files exist.

## Usage

```bash
# Start a project from just a prompt (no plan files needed)
fry --user-prompt "build a REST API for a todo app with PostgreSQL" --engine claude

# Steer an existing build toward backend work
fry --user-prompt "focus on backend API, skip frontend styling"

# Constrain the approach
fry --user-prompt "use only standard library, no third-party dependencies"
```

## Bootstrapping from a Prompt

When `--user-prompt` is provided and neither `plans/plan.md` nor `plans/executive.md` exists, Fry bootstraps the entire project:

1. Generates an executive context document from your prompt using the AI engine
2. Displays the generated content in the terminal for your review
3. Asks you to confirm: `Proceed with this executive context? [y/N]`
4. On approval (`y`): saves `plans/executive.md`, then continues to generate `plans/plan.md` and all build artifacts normally
5. On decline (`n` or Enter): exits without writing any files

This lets you go from a one-line idea to a fully orchestrated build with a single command.

## How It Works

The user prompt is:

- **Used for bootstrapping** — when no plan files exist, it generates `plans/executive.md` with interactive review, then the normal pipeline takes over
- **Injected as Layer 1.5** in the prompt hierarchy — between executive context ("why") and strategic plan ("what"). The agent sees it as a priority directive that applies to all sprints.
- **Passed through to prepare** — when `fry run` auto-generates the epic, the user prompt is forwarded to `fry prepare`, influencing the generation of `.fry/AGENTS.md`, `.fry/epic.md`, and `.fry/verification.md`.
- **Included in heal prompts** — when self-healing runs, the user directive is listed as context so heal passes respect it.

## Prompt Hierarchy

| Layer | Content | Source |
|---|---|---|
| 1 | Executive context | `plans/executive.md` |
| 1.25 | Media assets | Manifest of files in `media/` (if directory exists) |
| **1.5** | **User directive** | `--user-prompt` |
| 1.75 | Quality directive | Injected at `max` effort only |
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
# Bootstrap a project from scratch
fry --user-prompt "build a real-time chat app with WebSockets and Redis" --engine claude

# Bootstrap a planning project
fry --user-prompt "competitive analysis for a new SaaS product" --planning --engine claude

# Technical constraints (with existing plan files)
fry --user-prompt "no ORMs, use raw SQL only"

# Priority guidance
fry --user-prompt "focus on backend API, skip frontend styling"

# Scope limitation
fry --user-prompt "only implement the auth and user modules for now"

# Framework preference
fry --user-prompt "use Gin for HTTP routing, not net/http"
```
