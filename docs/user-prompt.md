# User Prompt

The `--user-prompt` flag lets you inject a top-level directive that applies to every sprint. It can also bootstrap an entire project from scratch when no plan files exist.

GitHub issues can use the same pathway through `--gh-issue <url>`. Fry fetches the issue through the authenticated `gh` CLI, converts it into a directive, persists the fetched context to `.fry/github-issue.md`, and then treats the result exactly like a user prompt. See [GitHub Issues](github-issues.md).

## Usage

```bash
# Start a project from just a prompt (no plan files needed)
fry --user-prompt "build a REST API for a todo app with PostgreSQL" --engine claude

# Steer an existing build toward backend work
fry --user-prompt "focus on backend API, skip frontend styling"

# Constrain the approach
fry --user-prompt "use only standard library, no third-party dependencies"

# Load a longer prompt from a file
fry --user-prompt-file ./my-prompt.txt --engine claude
```

## Prompt from File

For longer or more complex prompts, use `--user-prompt-file` to point to a local file containing the full prompt text:

```bash
fry --user-prompt-file /path/to/prompt.txt --engine claude
fry prepare --user-prompt-file ./detailed-requirements.txt
```

The file contents are read and used exactly as if they were passed inline via `--user-prompt`. The two flags cannot be combined — use one or the other.

`--gh-issue` is mutually exclusive with both `--user-prompt` and `--user-prompt-file`.

## Bootstrapping from a Prompt

When `--user-prompt` or `--gh-issue` is provided and neither `plans/plan.md` nor `plans/executive.md` exists, Fry bootstraps the entire project:

1. Generates an executive context document from your prompt using the AI engine
2. Displays the generated content in the terminal for your review
3. Asks you to confirm: `Proceed with this executive context? [y/N]`
4. On approval (`y`): saves `plans/executive.md`, then continues to generate `plans/plan.md` and all build artifacts normally
5. On decline (`n` or Enter): exits without writing any files

This lets you go from a one-line idea to a fully orchestrated build with a single command.

If an `assets/` directory exists, supplementary asset contents are also included when generating the executive context and plan. See [Supplementary Assets](supplementary-assets.md).

## How It Works

The user prompt is:

- **Used for bootstrapping** — when no plan files exist, it generates `plans/executive.md` with interactive review, then the normal pipeline takes over
- **Injected as Layer 1.5** in the prompt hierarchy — between executive context ("why") and strategic plan ("what"). The agent sees it as a priority directive that applies to all sprints.
- **Passed through to prepare** — when `fry run` auto-generates the epic, the user prompt is forwarded to `fry prepare`, influencing the generation of `.fry/AGENTS.md`, `.fry/epic.md`, and `.fry/verification.md`.
- **Included in alignment prompts** — when alignment runs, the user directive is listed as context so alignment passes respect it.

## Prompt Hierarchy

| Layer | Content | Source |
|---|---|---|
| 1 | Executive context | `plans/executive.md` |
| 1.25 | Media assets | Manifest of files in `media/` (if directory exists) |
| **1.5** | **User directive** | `--user-prompt`, `--user-prompt-file`, or `--gh-issue` |
| 1.75 | Quality directive | Injected at `max` effort only |
| 2 | Strategic plan reference | `plans/plan.md` |
| 3 | Sprint instructions | `@prompt` block from epic |
| 4 | Iteration memory | Progress files |
| 5 | Completion signal | Promise token |

## Persistence

The resolved directive is persisted to `.fry/user-prompt.txt`:

- **Saved on first use** and automatically reused on subsequent runs
- **Override** by passing a new `--user-prompt`, `--user-prompt-file`, or `--gh-issue` value
- **Clear** by deleting `.fry/user-prompt.txt`

When `--dry-run` is used with `--user-prompt` or `--gh-issue`, the directive is used for planning but **not** persisted to disk.

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

# Use a GitHub issue directly
fry --gh-issue https://github.com/yevgetman/fry/issues/74
```
