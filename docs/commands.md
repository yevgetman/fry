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

To specify a sprint range with positional arguments, you must also specify the epic file:

```bash
fry run epic.md 3 5       # Correct: run sprints 3-5
fry run 3 5               # Wrong: treats "3" as the epic filename
```

Alternatively, use `--sprint` to avoid specifying the epic file:

```bash
fry run --sprint 3         # Start from sprint 3 (uses .fry/epic.md)
```

### Flags

| Flag | Description |
|---|---|
| `--project-dir <path>` | Project directory to operate on (default: current directory) |
| `--engine <codex\|claude\|ollama>` | AI engine to use (default: claude) |
| `--effort <low\|medium\|high\|max>` | Effort level — controls sprint count, density, and review rigor (default: auto-detect). Ignored with a warning if the epic already has an `@effort` directive. See [Effort Levels](effort-levels.md). |
| `--model <model>` | Override the agent model for sprints, alignment, review, and replan sessions (e.g. `opus[1m]`, `sonnet`, `haiku`). Takes precedence over `@model` in the epic and the effort-based automatic model selection. Use this to pair a lower effort level (fewer sprints) with a more capable model. |
| `--mode <software\|planning\|writing>` | Execution mode (default: `software`). `planning` generates structured documents; `writing` generates human-language content (books, guides, reports). See [Planning Mode](planning-mode.md), [Writing Mode](writing-mode.md). |
| `--prepare-engine <codex\|claude\|ollama>` | Engine for auto-generating the epic (defaults to `--engine`, `FRY_ENGINE`, or claude) |
| `--planning` | Alias for `--mode planning`. Kept for backwards compatibility. |
| `--user-prompt <text>` | Top-level directive injected into every sprint prompt. When no `plan.md` or `executive.md` exists, bootstraps the entire project from this prompt (interactive review). |
| `--user-prompt-file <path>` | Path to a file containing the user prompt. Alternative to `--user-prompt` for longer prompts. Cannot be combined with `--user-prompt`. |
| `--review` | Enable sprint review between sprints. Instructs the epic generator to include `@review_between_sprints`. Also offered interactively during the adjust flow for medium/high effort builds. Max effort auto-enables review. |
| `--no-review` | Disable sprint review even if the epic enables `@review_between_sprints` |
| `--no-project-overview` | Skip interactive confirmations (triage classification and project overview) |
| `--no-audit` | Disable sprint and build audits for this run |
| `--no-observer` | Disable the observer metacognitive layer (event stream and wake-ups). Observer is also disabled at `low` effort and during `--dry-run`. See [Observer](observer.md). |
| `--simulate-review <verdict>` | Test the review pipeline without LLM calls. Verdict: `CONTINUE` or `DEVIATE` |
| `--verbose` | Stream full agent output to terminal (default: status banners only) |
| `--sprint <N>` | Start from sprint N. Alternative to the positional start sprint argument — no need to specify the epic file path. Cannot be combined with positional sprint arguments. |
| `--resume` | Resume a failed sprint: skip iterations, go straight to sanity checks + alignment with boosted attempts (2x normal, minimum 6). Preserves existing progress for full context. Only applies to the first sprint in the range; subsequent sprints run normally. |
| `--continue` | Auto-detect where a previous build left off and resume. Uses an LLM agent to analyze `.fry/` build artifacts, determine the next sprint, and decide whether to resume or start fresh. Automatically restores the build mode (`software`, `planning`, or `writing`) from the previous run unless `--mode` is explicitly passed. Cannot be combined with `--sprint`, `--resume`, or positional sprint arguments. |
| `--simple-continue` | Resume from the first incomplete sprint without LLM analysis. Scans sprint completion markers in the build state and resumes from the first sprint without a marker. Lightweight alternative to `--continue` — no LLM call, no cost. Cannot be combined with `--continue`, `--resume`, or `--sprint`. |
| `--full-prepare` | Skip triage and run the full prepare pipeline when no epic exists. Equivalent to the pre-triage behavior. See [Triage](triage.md). |
| `--triage-only` | Run the triage classifier and exit without generating any artifacts (no epic, sanity checks, or AGENTS.md). Prints the classification result. Cannot be combined with `--full-prepare`, `--continue`, `--resume`, or `--simple-continue`. See [Triage](triage.md). |
| `--git-strategy <auto\|current\|branch\|worktree>` | Git isolation strategy (default: `auto`). `auto` lets triage decide (complex -> worktree, simple/moderate -> branch). `current` works on the current branch (previous behavior). See [Git Strategy](git-strategy.md). |
| `--branch-name <name>` | Explicit branch name for `branch` or `worktree` strategies. Overrides the auto-generated `fry/<slug>` name. |
| `--always-verify` | Force sanity checks, alignment, and audit to run regardless of effort level or triage complexity. Generates heuristic sanity checks if none exist. Useful for CI/CD and automated builds. |
| `--sarif` | Write `build-audit.sarif` in SARIF 2.1.0 format alongside `build-audit.md`. Only written when the build audit runs. See [Build Audit](build-audit.md). |
| `--json-report` | Write `.fry/build-report.json` containing sprint results, sanity check outcomes, and timing data. |
| `--show-tokens` | Print a per-sprint token usage table to stderr at the end of the run. When using the Ollama engine, token counts are always 0 (Ollama does not report token usage); the column displays `-`. |
| `--telemetry` | Enable experience upload to the consciousness API for this run. See [Consciousness](consciousness.md). |
| `--no-telemetry` | Disable experience upload. Takes precedence over `--telemetry` if both set. |
| `--no-color` | Disable colored terminal output. Color is also disabled by setting the `NO_COLOR` environment variable or when stdout is not a TTY. |
| `--dry-run` | Parse epic and show plan without running anything |

### Examples

```bash
fry                                               # Run all sprints (.fry/epic.md default)
fry --dry-run                                     # Validate and preview
fry --triage-only --user-prompt "add a CLI flag"  # Preview triage classification only
fry --engine claude                               # Run all sprints with Claude Code
fry --effort low                                  # Quick task: 1-2 sprints, minimal overhead
fry --effort medium --engine claude               # Moderate task: 2-4 sprints
fry --effort max --engine claude                  # Maximum rigor: extended prompts, thorough reviews
fry run epic.md                                   # Run all sprints with Codex
fry run epic-phase2.md                            # Use a custom epic filename
fry run epic.md 4                                 # Resume from sprint 4
fry run --sprint 4                                # Same, without specifying the epic file
fry run epic.md 4 4                               # Run only sprint 4
fry run epic.md 3 5                               # Run sprints 3 through 5
fry run --resume --sprint 4                        # Resume failed sprint 4 (sanity checks + alignment only)
fry run --resume --sprint 4 --planning             # Resume with planning mode
fry run --continue                                # Auto-detect and resume from where you left off
fry run --continue --dry-run                      # Preview what --continue would do
fry run --continue --engine claude                # Resume with a different engine
fry --verbose                                     # Print agent output to terminal
fry --prepare-engine codex                        # Override: use Codex for generation, Codex for build
fry --planning                                    # Planning project (documents, not code) — uses Claude for both stages
fry --mode writing --user-prompt "Write a guide"  # Writing project (books, guides) — uses Claude for both stages
fry --user-prompt "focus on backend API, skip frontend"
fry --user-prompt "build a todo app" --engine claude  # Start from just a prompt
fry --user-prompt-file ./prompt.txt --engine claude   # Load prompt from a file
fry --no-project-overview                             # Skip triage confirmation and project overview
fry --git-strategy worktree                       # Force worktree isolation
fry --git-strategy branch --branch-name feat/auth # Branch with explicit name
fry --git-strategy current                        # Work on current branch (previous behavior)
fry --always-verify                               # Force sanity checks+alignment+audit on all tasks
fry --model opus[1m] --effort medium               # Medium sprint count with frontier model
fry --no-observer                                 # Disable the observer metacognitive layer
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
| `--engine <codex\|claude\|ollama>` | AI engine for generation (default: claude, or `FRY_ENGINE`) |
| `--effort <low\|medium\|high\|max>` | Effort level — controls sprint count and density in the generated epic (default: auto-detect). See [Effort Levels](effort-levels.md). |
| `--user-prompt <text>` | Top-level directive to guide artifact generation. Can bootstrap the entire project when no plan files exist (interactive review). |
| `--user-prompt-file <path>` | Path to a file containing the user prompt. Alternative to `--user-prompt` for longer prompts. Cannot be combined with `--user-prompt`. |
| `--mode <software\|planning\|writing>` | Execution mode (default: `software`). See [Planning Mode](planning-mode.md), [Writing Mode](writing-mode.md). |
| `--validate-only` | Check that the epic is valid, then exit |
| `--review` | Enable sprint review between sprints. Instructs the epic generator to include `@review_between_sprints`. |
| `--no-project-overview` | Skip the interactive project overview confirmation |
| `--planning` | Alias for `--mode planning`. Kept for backwards compatibility. |
| `--no-color` | Disable colored terminal output |
| `--verbose` | Stream full agent output to terminal (default: status banners only) |

All artifacts are **always regenerated** (overwritten) on each run.

### Generation Steps

| Step | Condition | Output |
|---|---|---|
| Bootstrap | Both files missing, `--user-prompt` provided | Generates `plans/executive.md` from user prompt (interactive review) |
| Step 0 | `plan.md` missing, `executive.md` exists | Generates `plans/plan.md` from executive context |
| Project Overview | `plan.md` exists, `--no-project-overview` not set | Displays AI-generated project overview for user confirmation |
| Step 1 | Always | Generates `.fry/AGENTS.md` (numbered operational rules) |
| Step 2 | Always | Generates `.fry/epic.md` (sprint definitions) |
| Step 3 | Always | Generates `.fry/verification.md` (sanity check primitives) |

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
fry prepare --mode writing --user-prompt "Write a guide to Go concurrency"  # Writing mode
fry prepare --no-project-overview                   # Skip project overview confirmation
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
| `--engine <codex\|claude\|ollama>` | Replanning engine |
| `--model <model>` | Model override |
| `--dry-run` | Preview replanning prompt without executing |
| `--verbose` | Stream full agent output to terminal (default: status banners only) |

---

## `fry clean`

Archive build artifacts from `.fry/` and root-level build outputs (`build-audit.md`, `build-summary.md`) into a timestamped folder under `.fry-archive/`.

```
fry clean [flags]
```

### Flags

| Flag | Description |
|---|---|
| `--force` | Skip the confirmation prompt |
| `--project-dir <path>` | Project directory to operate on (default: current directory) |

### Behavior

1. Checks if a build is currently running (lock file active) and warns if so.
2. Prompts for confirmation unless `--force` is passed.
3. Moves `.fry/` into `.fry-archive/.fry--build--YYYYMMDD-HHMMSS`.
4. Moves `build-audit.md` and `build-summary.md` from the project root into the same archive folder (skips silently if they don't exist).

Auto-archiving also happens automatically after a successful full build (all sprints from 1 to the last). The `fry clean` command is for manual archiving between builds.

### Examples

```bash
fry clean                              # Archive with confirmation prompt
fry clean --force                      # Archive without confirmation
fry clean --project-dir /path/to/proj  # Archive a different project
```

---

## `fry init`

Scaffold the fry project structure in the current (or specified) directory. Creates `plans/`, `assets/`, and `media/` directories, writes a `plan.example.md` template, initializes a git repository, and configures `.gitignore` with fry entries.

```
fry init [flags]
```

### Flags

| Flag | Description |
|---|---|
| `--project-dir <path>` | Project directory to operate on (default: current directory) |

### Behavior

1. Creates `plans/`, `assets/`, and `media/` directories if they don't exist.
2. Writes `plans/plan.example.md` with a starter template for reference.
3. Initializes a git repository if one doesn't exist.
4. Adds `.fry/`, `.fry-archive/`, `.env`, `.DS_Store`, and `.fry-worktrees/` to `.gitignore`.
5. Prints created items and next steps.

`fry init` does **not** create `plans/plan.md`. You write that yourself using `plan.example.md` as a reference, or provide a `--user-prompt` to `fry prepare` / `fry run` and fry will generate the plan for you (the normal flow via `executive.md` and/or prompt).

Running `fry init` in an already-initialized project is safe — it only creates missing directories and always refreshes the example file.

### Examples

```bash
fry init                                # Initialize in the current directory
fry init --project-dir /path/to/proj    # Initialize a different directory
```

---

## `fry status`

Show the current build state without making an LLM call. Reads `.fry/` artifacts and displays a summary of completed sprints, active/partial sprint work, environment checks, and deferred failures.

```
fry status [flags]
```

### Flags

| Flag | Description |
|---|---|
| `--project-dir <path>` | Project directory to inspect (default: current directory) |

### Behavior

- If no `.fry/epic.md` exists, prints "No active build found" and exits cleanly.
- Otherwise, prints a full build state report including sprint completion, environment readiness, deferred failures, and deviation count.

### Examples

```bash
fry status                              # Show build state for current directory
fry status --project-dir /path/to/proj  # Show build state for a different project
```

---

## `fry version`

Print the Fry version string.

```bash
fry version
```

---

## `fry identity`

Print Fry's current compiled-in identity.

```
fry identity [flags]
```

### Flags

| Flag | Description |
|---|---|
| `--full` | Print all identity layers including domain-specific files |

### Behavior

- Default: prints core identity + disposition (~1000 tokens total)
- With `--full`: also includes any domain-specific identity files (e.g., iOS/Swift, API backend)
- Identity is compiled into the binary via `go:embed` and updated only by the Reflection process between builds

### Examples

```bash
fry identity                         # Print core identity + disposition
fry identity --full                  # Print all layers including domains
```

---

## Environment Variables

| Variable | Description |
|---|---|
| `FRY_ENGINE` | Default engine (`codex`, `claude`, or `ollama`). Overridden by `--engine` flag and `@engine` directive. |
