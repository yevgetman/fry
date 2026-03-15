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

All arguments are optional. With no arguments, Fry uses `.fry/epic.md` and runs all sprints.

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
| `--engine <codex\|claude>` | AI engine to use (default: codex for software mode, claude for planning mode) |
| `--effort <low\|medium\|high\|max>` | Effort level — controls sprint count, density, and review rigor (default: auto-detect). Ignored with a warning if the epic already has an `@effort` directive. See [Effort Levels](effort-levels.md). |
| `--prepare-engine <codex\|claude>` | Engine for auto-generating the epic (defaults to `--engine`, `FRY_ENGINE`, or claude) |
| `--planning` | Use planning-domain prompts for auto-generation |
| `--user-prompt <text>` | Top-level directive injected into every sprint prompt. When no `plan.md` or `executive.md` exists, bootstraps the entire project from this prompt (interactive review). |
| `--user-prompt-file <path>` | Path to a file containing the user prompt. Alternative to `--user-prompt` for longer prompts. Cannot be combined with `--user-prompt`. |
| `--no-review` | Disable sprint review even if the epic enables `@review_between_sprints` |
| `--no-audit` | Disable sprint and build audits for this run |
| `--simulate-review <verdict>` | Test the review pipeline without LLM calls. Verdict: `CONTINUE` or `DEVIATE` |
| `--verbose` | Stream full agent output to terminal (default: status banners only) |
| `--retry` | Retry a failed sprint: skip iterations, go straight to verification + healing with boosted attempts (2x normal, minimum 6). Preserves existing progress for full context. Only applies to the first sprint in the range; subsequent sprints run normally. |
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
fry run --retry epic.md 4                         # Retry failed sprint 4 (verify + heal only)
fry run --retry epic.md 4 6                       # Retry sprint 4, then run 5-6 normally
fry --verbose                                     # Print agent output to terminal
fry --prepare-engine codex                        # Override: use Codex for generation, Codex for build
fry --planning                                    # Planning project (documents, not code) — uses Claude for both stages
fry --user-prompt "focus on backend API, skip frontend"
fry --user-prompt "build a todo app" --engine claude  # Start from just a prompt
fry --user-prompt-file ./prompt.txt --engine claude   # Load prompt from a file
fry --project-dir /path/to/project                # Operate on a different project
FRY_ENGINE=claude fry                             # Set engine via environment variable
```

---

## `fry prepare`

Generates `.fry/AGENTS.md`, `.fry/epic.md`, and `.fry/verification.md` from your plan. Requires at least one of `plans/plan.md`, `plans/executive.md`, `--user-prompt`, or `--user-prompt-file`.

```
fry prepare [epic_filename] [flags]
```

### Flags

| Flag | Description |
|---|---|
| `--project-dir <path>` | Project directory to operate on (default: current directory) |
| `--engine <codex\|claude>` | AI engine for generation (default: claude, or `FRY_ENGINE`) |
| `--effort <low\|medium\|high\|max>` | Effort level — controls sprint count and density in the generated epic (default: auto-detect). See [Effort Levels](effort-levels.md). |
| `--user-prompt <text>` | Top-level directive to guide artifact generation. Can bootstrap the entire project when no plan files exist (interactive review). |
| `--user-prompt-file <path>` | Path to a file containing the user prompt. Alternative to `--user-prompt` for longer prompts. Cannot be combined with `--user-prompt`. |
| `--validate-only` | Check that the epic is valid, then exit |
| `--planning` | Use planning-domain prompts |

All artifacts are **always regenerated** (overwritten) on each run.

### Generation Steps

| Step | Condition | Output |
|---|---|---|
| Bootstrap | Both files missing, `--user-prompt` provided | Generates `plans/executive.md` from user prompt (interactive review) |
| Step 0 | `plan.md` missing, `executive.md` exists | Generates `plans/plan.md` from executive context |
| Step 1 | Always | Generates `.fry/AGENTS.md` (numbered operational rules) |
| Step 2 | Always | Generates `.fry/epic.md` (sprint definitions) |
| Step 3 | Always | Generates `.fry/verification.md` (check primitives) |

### Examples

```bash
fry prepare                                        # Generate all with Claude (default)
fry prepare --engine codex                         # Generate all with Codex
fry prepare --effort low                           # Generate a compact 1-2 sprint epic
fry prepare --effort max --engine claude           # Generate with maximum detail
fry prepare epic-phase1.md                         # Custom epic filename
fry prepare --project-dir /path                    # Operate on a different project
fry prepare --user-prompt "no ORMs, use raw SQL only"
fry prepare --user-prompt "build a blog engine" --engine claude  # Bootstrap from prompt
fry prepare --user-prompt-file ./requirements.txt --engine claude # Prompt from file
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

Print the Fry version string.

```bash
fry version
```

---

## Environment Variables

| Variable | Description |
|---|---|
| `FRY_ENGINE` | Default engine (`codex` or `claude`). Overridden by `--engine` flag and `@engine` directive. |
