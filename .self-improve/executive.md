# Executive Directive: Fry Self-Improvement

## What This Is

You are Fry operating on its own source code. This is an automated self-improvement loop where Fry discovers issues, implements fixes, and opens pull requests for human review.

The Fry codebase is a Go 1.22 CLI tool that orchestrates AI agents through multi-sprint build loops. It lives at `github.com/yevgetman/fry` with two dependencies: cobra (CLI) and testify (testing). Read `CLAUDE.md` for full conventions and architecture invariants before making any changes.

## Why This Exists

Fry should continuously improve itself — finding bugs, expanding test coverage, adding features, and refining existing functionality. A human reviews and approves all changes via pull requests. Nothing is merged automatically.

## How the Loop Works

This build is one step in a recurring cycle:

1. **Planning phase** — Fry scans the codebase for issues and appends new findings to `roadmap.json` (in `assets/`). The orchestrator merges these into the canonical roadmap.
2. **Build phase** — The orchestrator selects open items from the roadmap and runs Fry to implement them. That is this phase.

The roadmap in `assets/roadmap.json` is the source of truth for what needs to be done. It follows the schema in `.self-improve/roadmap-schema.json`. Items have categories (`bug`, `testing`, `feature`, `improvement`), priority, effort estimates, and status.

## Rules for This Build

### Code changes

- Read `CLAUDE.md` in full before writing any code. Follow every convention.
- Read `README.LLM.md` for the architectural map — types, packages, execution flow.
- Read the affected files and their tests before modifying anything.
- Every change must have tests. Every test must call `t.Parallel()`.
- Run `make test && make build` and ensure both pass before considering any sprint complete.

### Quality gates

- All verification checks must pass. This build runs with `--always-verify`.
- Do not skip or weaken checks to make them pass. Fix the underlying issue.
- If a fix requires changes beyond the scope of the current task, note this in the sprint progress rather than expanding scope.

### Scope discipline

- Work only on the items specified in the user prompt. Do not fix unrelated issues.
- One logical change per commit. Commit messages in imperative present tense.
- Do not add dependencies. Do not restructure the parser. Do not break the engine interface. (See CLAUDE.md Section 9 for all architecture invariants.)
- Do not modify files outside the scope of the task unless the change is required for correctness (e.g., updating a caller when a function signature changes).

### Documentation

- Update the relevant `docs/` file for any feature or behavioral change.
- Update `README.md` if the change is user-visible.
- Update `README.LLM.md` if the change affects architecture, types, or flow.

### What not to do

- Do not commit `.env`, `.fry/`, `plans/`, `assets/`, `build-docs/`, or anything in `.gitignore`.
- Do not add comments, docstrings, or type annotations to code you didn't change.
- Do not refactor surrounding code unless the task explicitly calls for it.
- Do not create new top-level directories.
