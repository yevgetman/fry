# Self-Improvement Pipeline

Fry is a self-improving codebase. An automated pipeline periodically scans the Fry source code for issues, selects items to work on, implements fixes and features, and merges the results — all without human intervention. A human can optionally review changes via pull requests before they land.

## Overview

The self-improvement loop has two phases:

1. **Planning phase** — Fry scans its own codebase across 9 categories and produces a JSON list of new findings. These are appended to a canonical roadmap.
2. **Build phase** — Fry reads the full roadmap, selects 2-3 items based on effort balance, implements them in a git worktree, runs the full test suite, and either merges directly or opens a pull request.

An orchestrator script (`.self-improve/orchestrate.sh`) drives the loop. It can run manually, on a cron schedule via macOS launchd, or in CI via GitHub Actions.

## Architecture

```
.self-improve/
├── orchestrate.sh          # Bash script that drives the full loop
├── executive.md            # Static directive — context for Fry about the loop
├── planning-prompt.md      # User prompt for the planning phase
├── build-prompt.md         # User prompt for the build phase
├── roadmap.json            # Canonical roadmap (all open items)
├── roadmap-schema.json     # JSON schema for roadmap validation
├── .gitignore              # Excludes logs, lock, status files
├── logs/                   # Per-run log files (timestamped)
└── 2026-03-19-roadmap.md   # Historical first roadmap (archive)
```

## The Roadmap

The roadmap (`roadmap.json`) is the source of truth for all open items. Each item has:

| Field | Description |
|---|---|
| `id` | Unique identifier (`A3`, `C1`, `D5`) — letter = category, number = sequence |
| `category` | `bug`, `testing`, `feature`, `improvement`, `sunset`, `refactor`, `security`, `ui_ux`, `documentation` |
| `title` | Short descriptive title |
| `priority` | `high`, `medium`, `low` |
| `effort` | `low` (1-2 files, no API changes), `medium` (several files, signature changes), `high` (cross-cutting refactor) |
| `files` | Affected source files with optional line numbers |
| `description` | Detailed problem description |
| `fix` | Concrete implementation plan |
| `status` | `open`, `in_progress`, `pr_created` |
| `discovered` | ISO date when first identified |
| `pr_url` | GitHub PR URL (when status is `pr_created`) |
| `attempted_count` | Number of failed build attempts |

**Lifecycle:** Items start as `open`. When a build succeeds, they move to `pr_created` (with PR URL) or are removed entirely (when auto-merge is used). The orchestrator removes items whose PRs have been merged on startup. Items that fail 3 times are skipped until their count is reset. Completed items are removed from the roadmap — git history is the archive.

The roadmap conforms to the schema in `roadmap-schema.json`.

## Planning Phase

The planning phase scans the codebase for new issues across 9 categories:

| Category | Prefix | Focus |
|---|---|---|
| Bugs | A | Logic errors, race conditions, unhandled errors |
| Testing | B | Coverage gaps, weak tests, missing edge cases |
| Features | C | New capabilities compatible with architecture invariants |
| Improvements | D | Better defaults, robustness, ergonomics for existing features |
| Sunset | E | Dead code, unused exports, vestigial features |
| Refactor | F | Same behavior, better internals |
| Security | G | Injection vectors, unsafe input, secrets exposure |
| UI/UX | H | Terminal output, error messages, user flows |
| Documentation | I | Stale docs, missing sections, inaccurate references |

### When planning runs

Planning does not run every cycle. The orchestrator evaluates roadmap health:

- **Runs if** total open items < 5, or 2+ core categories (bug/testing/feature/improvement) are empty, or any category holds > 50% of items
- **Skips if** total items >= 15, last build failed, or `--skip-planning` is passed
- **Never fabricates** — the prompt explicitly allows zero findings per category

### How it works

1. The orchestrator scaffolds `plans/executive.md` and `assets/roadmap.json`
2. Fry runs with the planning prompt (`--mode planning`, `--effort medium`, `--no-audit`)
3. Fry reads the existing roadmap to avoid re-discovering known items
4. New findings are written to `output/new-findings.json`
5. The orchestrator validates the JSON, assigns sequential IDs, and appends to `roadmap.json`
6. Changes are committed and pushed to master

## Build Phase

The build phase implements items from the roadmap.

### Item selection

Fry reads the full roadmap and chooses 2-3 items based on effort balance:

- 1 high-effort item, or
- 1 medium + 1 low, or
- 2 medium, or
- 3 low

Fry prioritizes higher-priority items with fewer prior attempts and avoids items with vague fix plans.

### How it works

1. The orchestrator creates a git worktree on a timestamped branch
2. Scaffolds `plans/executive.md` and `assets/roadmap.json` into the worktree
3. Fry runs with `--full-prepare --always-verify --no-sanity-check`
4. Complex tasks are auto-elevated to `effort=high` for thorough audit cycles
5. Each item is implemented as a separate commit referencing the item ID
6. Documentation updates are required for every change
7. After Fry completes, `make test && make build` runs as a post-build check
8. If tests fail, a heal agent (claude with sonnet) attempts to fix the failures (up to 3 attempts)
9. On success: merge directly to master (with `--auto-merge`) or create a PR
10. Fry writes `output/worked-items.txt` listing the IDs it implemented — the orchestrator uses this to update the roadmap accurately

### Post-build healing

If `make test && make build` fails after Fry completes:

1. The orchestrator captures the failure output and the git diff
2. A claude agent runs in the worktree with the failure context and instructions to fix
3. Tests are re-run
4. Repeats up to 3 times
5. If healed: the fix is committed and the build proceeds to merge/PR
6. If exhausted: the build is marked as failed

## The Orchestrator

`.self-improve/orchestrate.sh` is the glue script. It handles:

- **Lock file** — prevents concurrent runs
- **Merged PR cleanup** — checks `pr_created` items against GitHub on startup, removes merged ones
- **Planning trigger logic** — evaluates roadmap balance to decide if planning is needed
- **Last-build-status tracking** — skips planning after a failed build to focus on building
- **Worktree lifecycle** — create, scaffold, run, cleanup
- **Item ID extraction** — reads `output/worked-items.txt` manifest (falls back to sprint name parsing)
- **PR creation** — with detailed per-item descriptions (category, priority, effort, problem, fix)
- **Auto-merge** — optional direct merge to master with `--auto-merge`
- **Per-run logging** — timestamped log files in `.self-improve/logs/`

### Flags

| Flag | Description |
|---|---|
| `--skip-planning` | Skip the planning phase |
| `--skip-build` | Skip the build phase |
| `--dry-run` | Run all logic except Fry invocations and git mutations |
| `--auto-merge` | Merge directly to master instead of creating a PR |

### Running manually

```bash
# Full loop (planning if needed + build + PR)
fry-improve

# Full loop with auto-merge
fry-improve --auto-merge

# Build only (skip planning)
fry-improve --skip-planning

# Preview what would happen
fry-improve --dry-run
```

### Running on a schedule

A macOS launchd agent runs the orchestrator daily:

```
~/Library/LaunchAgents/com.fry.self-improve.plist
```

The agent runs every 24 hours when the user is logged in. If the Mac is asleep at the scheduled time, the job runs when it wakes up (with no overlap — `StartInterval` guarantees at least 24 hours between runs). The orchestrator's lock file provides an additional guard.

**Useful commands:**

```bash
# Check status
launchctl list | grep fry

# Trigger immediately
launchctl start com.fry.self-improve

# Stop
launchctl unload ~/Library/LaunchAgents/com.fry.self-improve.plist

# Restart
launchctl unload ~/Library/LaunchAgents/com.fry.self-improve.plist
launchctl load ~/Library/LaunchAgents/com.fry.self-improve.plist

# View logs
tail -f .self-improve/cron.log           # launchd output
tail -f .self-improve/orchestrate.log    # aggregate orchestrator log
ls .self-improve/logs/                   # per-run logs
```

## Fry Features Used

The self-improvement loop uses several Fry features:

| Feature | Usage |
|---|---|
| `--mode planning` | Planning phase runs in planning mode (output to `output/`) |
| `--always-verify` | Build phase forces verification on all tasks regardless of effort |
| `--full-prepare` | Build phase forces full prepare so the LLM reads the roadmap |
| `--no-sanity-check` | Both phases skip interactive confirmation (automated) |
| `--no-audit` | Planning phase skips audit (analysis-only, no code to audit) |
| `--effort medium` | Planning phase uses medium effort (3 audit cycles — sufficient for discovery) |
| `--git-strategy current` | Both phases work on the current branch/worktree (orchestrator manages worktrees externally) |
| Worktrees | Orchestrator creates worktrees for build isolation |
| Triage gate | Auto-classifies build complexity; complex tasks auto-elevate to `effort=high` |

## Configuration

Constants at the top of `orchestrate.sh`:

| Constant | Default | Description |
|---|---|---|
| `MAX_BUILD_ITEMS` | 3 | Maximum items Fry can select per build |
| `MAX_ATTEMPTS` | 3 | Skip items that have failed this many times |
| `MAX_POST_BUILD_HEALS` | 3 | Heal attempts for post-build test/build failures |
| `PLANNING_THRESHOLD` | 15 | Skip planning if this many open items exist |

## Safety

- **Tests always gate merges** — `make test && make build` must pass before any merge or PR
- **Post-build healing** — test failures get 3 attempts at automated repair before giving up
- **Lock file** — prevents concurrent orchestrator runs
- **Attempted count** — items that fail 3 times are automatically skipped
- **Auto-merge fallback** — if merge conflicts occur, falls back to creating a PR
- **Cleanup trap** — worktrees, branches, temp files, and scaffolding are cleaned up on any exit
- **Manifest-based tracking** — only items Fry explicitly reports as worked on get status updates
