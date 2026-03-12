# CLI Commands

```
fry [command] [flags]
```

When invoked without a subcommand, `fry` is equivalent to `fry run`.

## `fry run`

Execute sprints from an epic file.

```
fry run [epic.md] [start] [end] [flags]
```

All arguments are optional. With no arguments, fry uses `.fry/epic.md` and runs all sprints.

### Positional Arguments

Positional arguments are order-dependent: the epic file must come first, followed by sprint numbers.

| Position | Argument | Description |
|---|---|---|
| 1 | `epic_file` | Path to the epic definition file (default: `.fry/epic.md`). Auto-generated via `fry prepare` if the file doesn't exist. |
| 2 | `start_sprint` | First sprint to run (default: 1) |
| 3 | `end_sprint` | Last sprint to run (default: last sprint in epic) |

To specify a sprint range, you must also specify the epic file:

```bash
fry run epic.md 3 5       # Correct: run sprints 3-5
fry run 3 5               # Wrong: treats "3" as the epic filename
```

### Flags

| Flag | Description |
|---|---|
| `--project-dir <path>` | Project directory to operate on (default: current directory) |
| `--engine <codex\|claude>` | AI engine to use (default: codex) |
| `--effort <low\|medium\|high\|max>` | Effort level — controls sprint count, density, and review rigor (default: auto-detect). See [Effort Levels](effort-levels.md). |
| `--prepare-engine <codex\|claude>` | Engine for auto-generating the epic (defaults to `--engine` or `FRY_ENGINE`) |
| `--planning` | Use planning-domain prompts for auto-generation |
| `--user-prompt <text>` | Top-level directive injected into every sprint prompt |
| `--no-review` | Disable sprint review even if the epic enables `@review_between_sprints` |
| `--simulate-review <verdict>` | Test the review pipeline without LLM calls. Verdict: `CONTINUE` or `DEVIATE` |
| `--verbose` | Stream full agent output to terminal (default: status banners only) |
| `--dry-run` | Parse epic and show plan without running anything |

### Examples

```bash
fry                                               # Run all sprints (.fry/epic.md default)
fry --dry-run                                     # Validate and preview
fry --engine claude                               # Run all sprints with Claude Code
fry --effort low                                  # Quick task: 1-2 sprints, minimal overhead
fry --effort medium --engine claude               # Moderate task: 2-4 sprints
fry --effort max --engine claude                  # Maximum rigor: extended prompts, thorough reviews
fry run epic.md                                   # Run all sprints with Codex
fry run epic-phase2.md                            # Use a custom epic filename
fry run epic.md 4                                 # Resume from sprint 4
fry run epic.md 4 4                               # Run only sprint 4
fry run epic.md 3 5                               # Run sprints 3 through 5
fry --verbose                                     # Print agent output to terminal
fry --prepare-engine claude                       # Use Claude for generation, Codex for build
fry --planning --engine claude                    # Planning project (documents, not code)
fry --user-prompt "focus on backend API, skip frontend"
fry --project-dir /path/to/project                # Operate on a different project
FRY_ENGINE=claude fry                             # Set engine via environment variable
```

---

## `fry prepare`

Generates `.fry/AGENTS.md`, `.fry/epic.md`, and `.fry/verification.md` from your plan. Requires at least one of `plans/plan.md` or `plans/executive.md`.

```
fry prepare [epic_filename] [flags]
```

### Flags

| Flag | Description |
|---|---|
| `--project-dir <path>` | Project directory to operate on (default: current directory) |
| `--engine <codex\|claude>` | AI engine for generation (default: codex, or `FRY_ENGINE`) |
| `--effort <low\|medium\|high\|max>` | Effort level — controls sprint count and density in the generated epic (default: auto-detect). See [Effort Levels](effort-levels.md). |
| `--user-prompt <text>` | Top-level directive to guide artifact generation |
| `--validate-only` | Check that the epic is valid, then exit |
| `--planning` | Use planning-domain prompts |

All artifacts are **always regenerated** (overwritten) on each run.

### Generation Steps

| Step | Condition | Output |
|---|---|---|
| Step 0 | `plan.md` missing, `executive.md` exists | Generates `plans/plan.md` from executive context |
| Step 1 | Always | Generates `.fry/AGENTS.md` (numbered operational rules) |
| Step 2 | Always | Generates `.fry/epic.md` (sprint definitions) |
| Step 3 | Always | Generates `.fry/verification.md` (check primitives) |

### Examples

```bash
fry prepare                                        # Generate all with Codex (default)
fry prepare --engine claude                        # Generate all with Claude Code
fry prepare --effort low                           # Generate a compact 1-2 sprint epic
fry prepare --effort max --engine claude           # Generate with maximum detail
fry prepare epic-phase1.md                         # Custom epic filename
fry prepare --project-dir /path                    # Operate on a different project
fry prepare --user-prompt "no ORMs, use raw SQL only"
fry prepare --validate-only                        # Validate existing epic only
```

---

## `fry replan`

Replan an epic after a deviation. Used by the dynamic sprint review system, or invoked directly.

```
fry replan <deviation_spec> [flags]
```

### Flags

| Flag | Description |
|---|---|
| `--epic <path>` | Epic file to update (default: `.fry/epic.md`) |
| `--completed <N>` | Completed sprint count |
| `--max-scope <N>` | Maximum deviation scope (default: 3) |
| `--engine <codex\|claude>` | Replanning engine |
| `--model <model>` | Model override |
| `--dry-run` | Preview replanning prompt without executing |

---

## `fry version`

Print the fry version string.

```bash
fry version
```

---

## Environment Variables

| Variable | Description |
|---|---|
| `FRY_ENGINE` | Default engine (`codex` or `claude`). Overridden by `--engine` flag and `@engine` directive. |
