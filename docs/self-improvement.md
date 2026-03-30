# Self-Improvement Pipeline

Fry is a self-improving codebase. An automated pipeline periodically scans the Fry source code for issues, creates GitHub Issues, implements approved items, and merges the results. Maintenance items (bugs, security, testing, documentation) are auto-approved and built immediately. Product items (features, improvements, refactors, sunset, UI/UX) require human approval via GitHub Issue labels before they are built.

## Overview

The self-improvement loop has two phases:

1. **Planning phase** — Fry scans its own codebase across 10 categories and produces a JSON list of new findings. The orchestrator creates GitHub Issues for each finding, with auto-approve or proposed status based on category.
2. **Build phase** — Fry reads the approved issues, selects 2-3 items based on effort balance, implements them in a git worktree, runs the full test suite, and either merges directly or opens a pull request.

An orchestrator script (`.self-improve/orchestrate.sh`) drives the loop. It can run manually, on a cron schedule via macOS launchd, or in CI via GitHub Actions.

## Architecture

```
.self-improve/
├── orchestrate.sh          # Bash script that drives the full loop
├── executive.md            # Static directive — context for Fry about the loop
├── planning-prompt.md      # User prompt for the planning phase
├── build-prompt.md         # User prompt for the build phase
├── config                  # KEY=VALUE configuration (overrides script defaults)
├── build-journal.json      # Build history — last 30 entries (auto-generated)
├── .gitignore              # Excludes logs, lock, status, journal files
├── logs/                   # Per-run log files (timestamped)
└── 2026-03-19-roadmap.md   # Historical first roadmap (archive)
```

## GitHub Issues as Roadmap

GitHub Issues is the single source of truth for all open items. Each issue is managed via labels.

### Label Scheme

| Label | Purpose |
|---|---|
| `self-improve` | All Fry-managed issues get this label |
| `category/bug` | Bug fixes |
| `category/testing` | Test coverage and quality |
| `category/feature` | New capabilities |
| `category/improvement` | Enhancements to existing features |
| `category/sunset` | Dead code and vestigial features |
| `category/refactor` | Internal improvements (same behavior) |
| `category/security` | Security fixes |
| `category/ui_ux` | Terminal output and user flows |
| `category/documentation` | Documentation updates |
| `category/experience` | Build experience insights |
| `status/proposed` | Awaiting human approval |
| `status/approved` | Ready to build |
| `priority/high` | High priority |
| `priority/medium` | Medium priority |
| `priority/low` | Low priority |
| `effort/low` | 1-2 files, no API changes |
| `effort/medium` | Several files, signature changes |
| `effort/high` | Cross-cutting refactor |
| `max-attempts` | 3+ failed builds — skipped until manually reset |

### Category Tiers

| Tier | Categories | Behavior |
|---|---|---|
| **Auto-approve** | bug, security, testing, documentation | Created with `status/approved` — built immediately |
| **Needs approval** | feature, improvement, refactor, sunset, ui_ux, experience | Created with `status/proposed` — requires human to add `status/approved` |

### Issue Format

```markdown
**Priority:** high
**Effort:** medium
**Files:** `internal/cli/run.go:296`, `internal/config/config.go`

## Problem
<description>

## Fix Plan
<fix plan>

---
_Managed by [Fry Self-Improvement Pipeline](docs/self-improvement.md)_
```

### Approval Workflow

1. Planning phase creates issues with `status/proposed` label
2. You receive a GitHub notification
3. To approve: remove `status/proposed`, add `status/approved`
4. To reject: close the issue
5. Next orchestrator run picks up approved items for building

### Issue Lifecycle

```
Planning discovers item
  ├── Auto-approve category → status/approved (built on next run)
  └── Product category → status/proposed (awaiting human review)
       ├── Human approves → status/approved → built on next run
       └── Human rejects → issue closed

Build succeeds
  ├── Auto-merge → issue closed with comment
  └── PR created → issue auto-closed when PR is merged (via "Closes #N")

Build fails → failure comment added
  └── 3+ failures → max-attempts label added (skipped until reset)
```

## Planning Phase

The planning phase scans the codebase for new issues across 10 categories:

| Category | Focus |
|---|---|
| Bugs | Logic errors, race conditions, unhandled errors |
| Testing | Coverage gaps, weak tests, missing edge cases |
| Features | New capabilities compatible with architecture invariants |
| Improvements | Better defaults, robustness, ergonomics for existing features |
| Sunset | Dead code, unused exports, vestigial features |
| Refactor | Same behavior, better internals |
| Security | Injection vectors, unsafe input, secrets exposure |
| UI/UX | Terminal output, error messages, user flows |
| Documentation | Stale docs, missing sections, inaccurate references |
| Experience | Build journal patterns, effort mismatches, pipeline improvements |

### When planning runs

Planning does not run every cycle. The orchestrator evaluates roadmap health:

- **Runs if** total open issues < 5, or 2+ core categories (bug/testing/feature/improvement) are empty, or any category holds > 50% of issues
- **Skips if** total issues >= 15, last build failed, or `--skip-planning` is passed
- **Never fabricates** — the prompt explicitly allows zero findings per category

### How it works

1. The orchestrator exports all open issues to `assets/existing-issues.json` for deduplication
2. Scaffolds `plans/executive.md`
3. Fry runs with the planning prompt (`--mode planning`, `--effort standard`, `--no-audit`)
4. Fry reads existing issues to avoid re-discovering known items
5. New findings are written to `output/new-findings.json`
6. The orchestrator creates a GitHub Issue for each finding with appropriate labels

## Build Phase

The build phase implements items from approved GitHub Issues.

### Item selection

Fry reads the exported approved items and chooses 2-3 based on effort balance:

- 1 high-effort item, or
- 1 medium + 1 low, or
- 2 medium, or
- 3 low

Fry prioritizes higher-priority items with fewer prior attempts and avoids items with vague fix plans.

### How it works

1. The orchestrator queries GitHub for approved issues (excluding `max-attempts`)
2. Exports them to `assets/approved-items.json` in the worktree
3. Fry runs with `--full-prepare --always-verify --no-project-overview`
4. Complex tasks are auto-elevated to `effort=high` for thorough audit cycles
5. Each item is implemented as a separate commit referencing the issue number
6. Documentation updates are required for every change
7. After Fry completes, `make test && make build` runs as a post-build check
8. If tests fail, an alignment agent (claude with sonnet) attempts to fix the failures (up to 3 attempts)
9. On success with `--auto-merge`: pull latest from remote, merge locally, push. On success without: create a PR
10. Fry writes `output/worked-items.txt` listing the issue numbers it implemented

### Post-build alignment

If `make test && make build` fails after Fry completes:

1. The orchestrator captures the failure output and the git diff
2. A claude agent runs in the worktree with the failure context and instructions to fix
3. Tests are re-run
4. Repeats up to 3 times
5. If aligned: the fix is committed and the build proceeds to merge/PR
6. If exhausted: the build is marked as failed

## Build Journal

After every build (success or failure), the orchestrator generates a structured journal entry capturing what happened. The journal serves two purposes:

1. **Operational record** — track build outcomes, alignment rounds, and merge methods over time
2. **Pattern source** — during planning, the AI analyzes the journal to discover experience-based improvements

### Journal file

`.self-improve/build-journal.json` — A JSON array of entries, newest first. Bounded to the last 30 entries (configurable via `MAX_JOURNAL_ENTRIES`). Not committed to git.

### Entry structure

Each entry contains:

| Field | Type | Description |
|---|---|---|
| `run_id` | string | Orchestrator run ID (e.g., `20260322-131636`) |
| `date` | string | Build date (YYYY-MM-DD) |
| `outcome` | string | `success` or `failure` |
| `items_attempted` | array | Per-item details: issue number, title, category, effort, result, alignment rounds |
| `heal_rounds_total` | number | Total post-build alignment attempts used |
| `merge_method` | string | `auto-merge`, `pr`, or `none` |
| `files_changed` | array | List of files modified in the build |
| `tests_passed` | boolean | Whether `make test` passed |
| `observations` | string | AI-generated summary of notable patterns |

### AI summarization

After mechanical extraction, the orchestrator runs an AI model (configurable via `JOURNAL_MODEL`, default: `sonnet`) to analyze the build log and produce a brief observations field. The AI looks for recurring patterns, fragility, effort mismatches, and anything surprising. If summarization fails, a fallback message is used — journal generation never blocks the build.

### Experience category

During planning, if the journal exists, it is exported to `assets/build-journal.json`. The planning prompt includes **Category J: Build Experience**, which instructs the AI to analyze the journal for patterns and propose experience-based improvements. Experience findings are always created with `status/proposed` (never auto-approved) to ensure human review.

## The Orchestrator

`.self-improve/orchestrate.sh` is the glue script. It handles:

- **Lock file** — prevents concurrent runs
- **Label validation** — checks that required GitHub labels exist on startup
- **Planning trigger logic** — evaluates issue distribution to decide if planning is needed
- **Last-build-status tracking** — skips planning after a failed build to focus on building
- **Issue creation** — creates GitHub Issues with category-based auto-approve/proposed labels
- **Issue export** — exports approved issues to JSON for Fry to read
- **Duplicate detection** — skips findings that match existing open issues by title
- **Worktree lifecycle** — create, scaffold, run, cleanup
- **Item tracking** — reads `output/worked-items.txt` manifest (issue numbers)
- **PR creation** — with `Closes #N` for auto-closing issues on merge
- **Auto-merge** — pulls latest from remote, merges locally, pushes; falls back to PR on conflict
- **Failure tracking** — comments on issues on failure, adds `max-attempts` label at threshold
- **Build journal** — generates structured journal entries after every build for experience-based planning
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
| `--always-verify` | Build phase forces sanity checks on all tasks regardless of effort |
| `--full-prepare` | Build phase forces full prepare so the LLM reads the approved items |
| `--no-project-overview` | Both phases skip interactive confirmation (automated) |
| `--no-audit` | Planning phase skips audit (analysis-only, no code to audit) |
| `--effort standard` | Planning phase uses standard effort (sufficient for discovery) |
| `--git-strategy current` | Both phases work on the current branch/worktree (orchestrator manages worktrees externally) |
| Worktrees | Orchestrator creates worktrees for build isolation |
| Triage gate | Auto-classifies build complexity; complex tasks auto-elevate to `effort=high` |

## Configuration

Constants at the top of `orchestrate.sh`, overridable via `.self-improve/config`:

| Constant | Default | Description |
|---|---|---|
| `MAX_BUILD_ITEMS` | 3 | Maximum items Fry can select per build |
| `MAX_ATTEMPTS` | 3 | Skip items that have failed this many times |
| `MAX_POST_BUILD_HEALS` | 3 | Alignment attempts for post-build test/build failures |
| `PLANNING_THRESHOLD` | 15 | Skip planning if this many open issues exist |
| `JOURNAL_MODEL` | sonnet | AI model for build journal summarization |
| `MAX_JOURNAL_ENTRIES` | 30 | Maximum entries retained in build journal |

## Safety

- **Tests always gate merges** — `make test && make build` must pass before any merge or PR
- **Post-build alignment** — test failures get 3 attempts at automated repair before giving up
- **Lock file** — prevents concurrent orchestrator runs
- **Max attempts** — issues that fail 3 times get the `max-attempts` label and are skipped
- **Auto-merge safety** — pulls latest from remote before merging; falls back to PR on conflicts or pull failures
- **Cleanup trap** — worktrees, branches, temp files, and scaffolding are cleaned up on any exit
- **Manifest-based tracking** — only items Fry explicitly reports as worked on get status updates
- **Human approval gate** — product-direction items require explicit human approval via GitHub Issue labels
- **Duplicate detection** — planning phase checks existing issues before creating new ones
