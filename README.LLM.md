# README.LLM.md — Fry Codebase Reference for AI Agents

> **Purpose:** Compact, comprehensive codebase map for AI agents. Eliminates the need for deep directory traversal. Covers architecture, file layout, key types, execution flow, and configuration.

## What Is Fry

Fry is a Go CLI tool that orchestrates AI agents (OpenAI Codex or Claude Code) to autonomously build software or generate documents. It decomposes a human-authored plan into sequential "sprints," executes each sprint through an iterative AI agent loop, verifies outputs with machine-executable checks, self-heals on failure, audits quality, and git-checkpoints every sprint.

**Version:** 0.1.0
**Language:** Go 1.22
**Module:** `github.com/yevgetman/fry`
**Binary:** `bin/fry` (single static executable)
**Dependencies:** cobra (CLI), testify (testing only)

---

## Directory Layout

```
fry/
├── cmd/fry/main.go              # Entry point — calls cli.Execute()
├── internal/
│   ├── cli/                     # Cobra commands: root, run, prepare, replan, version
│   │   ├── root.go              # Persistent flags (--project-dir, --verbose, --engine, etc.)
│   │   ├── run.go               # Main orchestration (685 lines): sprint loop, audit, review
│   │   ├── prepare.go           # Generate .fry/ artifacts from plans
│   │   ├── replan.go            # Mid-build replanning
│   │   └── version.go           # Version subcommand
│   ├── config/config.go         # All constants: paths, defaults, invocation prompts
│   ├── engine/
│   │   ├── engine.go            # Engine interface + ResolveEngine + NewEngine
│   │   ├── codex.go             # Codex CLI wrapper
│   │   └── claude.go            # Claude Code CLI wrapper
│   ├── epic/
│   │   ├── types.go             # Epic, Sprint, EffortLevel types
│   │   ├── parser.go            # State-machine .md parser for epic files
│   │   └── validator.go         # Epic structural validation
│   ├── sprint/
│   │   ├── runner.go            # Sprint execution loop (iterations, no-op detection)
│   │   ├── prompt.go            # Layered prompt assembly (8 layers)
│   │   ├── progress.go          # Iteration memory management
│   │   └── compactor.go         # Sprint progress → epic-progress summarization
│   ├── verify/
│   │   ├── types.go             # CheckType: FILE, FILE_CONTAINS, CMD, CMD_OUTPUT
│   │   ├── parser.go            # verification.md parser
│   │   ├── runner.go            # Check execution with timeout
│   │   └── collector.go         # Failure report aggregation
│   ├── heal/heal.go             # Self-healing loop on verification failure
│   ├── audit/
│   │   ├── audit.go             # Per-sprint two-level audit: outer audit cycles + inner fix loops
│   │   └── build_audit.go       # Final holistic codebase audit
│   ├── review/
│   │   ├── reviewer.go          # Sprint review (CONTINUE vs DEVIATE verdict)
│   │   ├── replanner.go         # Dynamic epic modification
│   │   ├── deviation.go         # Deviation spec parsing
│   │   └── types.go             # ReviewVerdict, DeviationSpec
│   ├── prepare/
│   │   ├── prepare.go           # Steps 0-3 artifact generation
│   │   ├── sanity.go            # Interactive project summary sanity check
│   │   ├── software.go          # Software project handling
│   │   ├── planning.go          # Planning-mode (non-code) handling
│   │   └── writing.go           # Writing-mode (books, guides) handling
│   ├── git/git.go               # Git init, checkpoints, diff capture
│   ├── docker/docker.go         # Docker Compose lifecycle, health checks
│   ├── preflight/preflight.go   # Pre-build tool/command validation
│   ├── lock/lock.go             # File-based build concurrency lock
│   ├── log/log.go               # Verbose logging, agent banners
│   ├── media/media.go           # Binary asset scanning (images, PDFs, fonts)
│   ├── assets/assets.go         # Text asset scanning + prompt injection
│   ├── continuerun/
│   │   ├── types.go             # BuildState, ContinueDecision, verdict types
│   │   ├── collector.go         # Programmatic build state collection from .fry/ artifacts
│   │   ├── report.go            # BuildState → human-readable markdown report
│   │   └── analyzer.go          # LLM analysis agent for resume decisions
│   ├── summary/summary.go       # AI-generated build summary
│   ├── shellhook/shellhook.go   # Pre-sprint/iteration shell commands
│   └── textutil/textutil.go     # Shell quoting, file timestamps, artifact resolution
├── templates/
│   ├── embed.go                 # //go:embed *.md for compile-time inclusion
│   ├── AGENTS.md                # Placeholder (generated via fry prepare)
│   ├── GENERATE_EPIC.md         # LLM prompt template for epic generation
│   ├── epic-example.md          # Fully-commented epic file example
│   └── verification-example.md  # Verification check examples
├── docs/                        # 21 user-facing documentation files (see below)
├── plans/                       # User-authored inputs
│   ├── plan.md                  # Build strategy (what to build)
│   └── executive.md             # Project context (why to build it)
├── output/                      # Planning/writing mode deliverables (--mode planning|writing)
├── Makefile                     # build, test, lint, clean, install
├── go.mod / go.sum
├── .env.example                 # FRY_ENGINE=codex|claude
└── .gitignore
```

### Generated Artifacts (`.fry/` at runtime)

| File | Purpose |
|------|---------|
| `epic.md` | Sprint definitions with prompts, parsed by `epic/parser.go` |
| `AGENTS.md` | Project-specific agent rules |
| `verification.md` | Machine-executable acceptance checks |
| `prompt.md` | Assembled prompt for current sprint iteration |
| `sprint-progress.txt` | Append-only iteration log within current sprint |
| `epic-progress.txt` | Compacted summaries of completed sprints |
| `user-prompt.txt` | Persisted user directive |
| `build-mode.txt` | Persisted build mode (software/planning/writing) for `--continue` auto-detection |
| `deviation-log.md` | Deviations detected during sprint reviews |
| `deferred-failures.md` | Verification failures below threshold, deferred to build audit |
| `sprint-audit.txt` | Current sprint's audit findings |
| `audit-prompt.md` | Assembled audit prompt |
| `review-prompt.md` | Assembled review prompt |
| `summary-prompt.md` | Assembled summary prompt |
| `build-logs/` | Timestamped per-iteration/heal/audit/continue logs |
| `continue-prompt.md` | Assembled prompt for --continue analysis agent |
| `continue-decision.txt` | LLM agent's resume decision (verdict, sprint, reason) |
| `continue-report.md` | Programmatic build state report (input to analysis) |
| `.fry.lock` | Concurrency lock |

---

## Core Types

### Epic (`internal/epic/types.go`)

```go
type Epic struct {
    Name, Engine             string
    EffortLevel              EffortLevel    // low|medium|high|max
    DockerFromSprint         int
    DockerReadyCmd           string
    DockerReadyTimeout       int
    RequiredTools            []string
    PreflightCmds            []string
    PreSprintCmd             string
    PreIterationCmd          string
    AgentModel, AgentFlags   string
    VerificationFile         string
    MaxHealAttempts          int
    MaxHealAttemptsSet       bool           // true when @max_heal_attempts was explicitly set
    MaxFailPercent           int            // 0-100; default 20 (10 for max effort)
    MaxFailPercentSet        bool           // true when @max_fail_percent was explicitly set
    CompactWithAgent         bool
    ReviewBetweenSprints     bool
    ReviewEngine, ReviewModel string
    MaxDeviationScope        int
    AuditAfterSprint         bool
    MaxAuditIterations       int
    MaxAuditIterationsSet    bool
    AuditEngine, AuditModel  string
    Sprints                  []Sprint
    TotalSprints             int
}

type Sprint struct {
    Number, MaxIterations    int
    Name, Promise, Prompt    string
    MaxHealAttempts          *int  // per-sprint override
}
```

### Engine Interface (`internal/engine/engine.go`)

```go
type Engine interface {
    Run(ctx context.Context, prompt string, opts RunOpts) (output string, exitCode int, err error)
    Name() string
}
```

Resolution precedence: CLI flag → epic `@engine` → `FRY_ENGINE` env → default (codex for software build, claude for prepare/planning/writing).

### Effort Levels (`internal/epic/types.go`)

| Level | MaxIterations | MaxSprints |
|-------|---------------|------------|
| low | 12 | 2 |
| medium | 20 | 4 |
| high | 25 | 10 |
| max | 40 | 10 |

### Verification Checks (`internal/verify/types.go`)

Four check primitives: `@check_file` (file exists), `@check_file_contains` (regex match in file), `@check_cmd` (command exits 0), `@check_cmd_output` (command output matches regex).

---

## Execution Flow

```
User Input                          Generated Artifacts
─────────────────────               ────────────────────
plans/plan.md         ──┐
plans/executive.md    ──┤  fry prepare        →  .fry/AGENTS.md
--user-prompt "..."   ──┤  (Sanity Check +    →  .fry/epic.md
assets/               ──┘   Steps 0-3)        →  .fry/verification.md
media/                ──(manifest only)

                        fry run
                        ───────
For each sprint (startSprint → endSprint):
  1. Preflight checks (required tools + custom commands)
  2. Docker up (if @docker_from_sprint <= current sprint)
  3. Git init (if needed)
  4. Assemble layered prompt → .fry/prompt.md
  5. Agent iteration loop:
     │  for iter = 1 to maxIterations:
     │    ├─ Pre-iteration shell hook
     │    ├─ Run AI engine (codex|claude CLI)
     │    ├─ Append output → .fry/sprint-progress.txt
     │    ├─ Check promise token → early exit if found
     │    └─ No-op detection (no git diff) → early exit
  6. Run verification checks
  7. If checks fail: heal loop (effort-level-aware: low=skip, medium=3, high=up to 10 with progress detection, max=unlimited with progress detection)
  8. Sprint audit (if enabled & effort != low) — two-level loop:
     │  ├─ Outer loop (audit cycles): audit agent reviews + verifies previous issues
     │  ├─ Inner loop (fix iterations): fix agent → verify agent → repeat until resolved
     │  ├─ Issues tracked per-finding, FIFO ordered (oldest first)
     │  ├─ medium: bounded (3 outer cycles, 3 inner fix iterations)
     │  └─ high: progress-based (cap 10 outer), max: progress-based (cap 15 outer)
  9. Git checkpoint commit
 10. Compact sprint progress → .fry/epic-progress.txt
 11. Optional sprint review:
     │  ├─ Review agent: CONTINUE or DEVIATE
     │  └─ If DEVIATE: replan affected sprints

Final: build audit → build-summary.md
```

### Prompt Layering (8 layers, assembled in `sprint/prompt.go`)

| Layer | Content | Notes |
|-------|---------|-------|
| 1 | Executive context | `plans/executive.md` (optional) |
| 1.25 | Media manifest | categorized list of `media/` files (optional) |
| 1.5 | User directive | `--user-prompt` or `.fry/user-prompt.txt` (optional) |
| 1.75 | Quality directive | injected only at `max` effort |
| 2 | Strategic plan | reference to `plans/plan.md` |
| 3 | Sprint instructions | `@prompt` block from epic |
| 4 | Iteration memory | links to progress files |
| 5 | Completion signal | promise token (optional) |

---

## CLI Commands and Key Flags

```
fry [run] [epic.md] [start] [end]   # Execute sprints (run is default)
fry prepare [epic_filename]          # Generate .fry/ artifacts from plans
fry replan                           # Replan after deviation
fry version                          # Print version

Key flags:
  --engine codex|claude              # AI engine for build
  --prepare-engine codex|claude      # AI engine for prepare phase
  --effort low|medium|high|max       # Effort level (auto-detect if omitted)
  --mode software|planning|writing   # Execution mode (default: software)
  --planning                         # Alias for --mode planning (backwards compat)
  --user-prompt "..."                # Inject directive into prompts
  --user-prompt-file path            # Load directive from file
  --dry-run                          # Validate without executing
  --sprint N                         # Start from sprint N
  --retry                            # Skip iterations, verify + heal with boosted attempts
  --continue                         # LLM-assisted auto-resume from where build left off
  --no-sanity-check                  # Skip interactive project summary
  --no-review                        # Skip mid-build sprint review
  --no-audit                         # Skip audits
  --verbose                          # Verbose logging
  --project-dir path                 # Working directory (default: .)
```

---

## Key Constants (`internal/config/config.go`)

| Constant | Value | Purpose |
|----------|-------|---------|
| `DefaultEngine` | `codex` | Default build engine |
| `DefaultPrepareEngine` | `claude` | Default prepare engine |
| `DefaultPlanningEngine` | `claude` | Default planning-mode engine |
| `DefaultWritingEngine` | `claude` | Default writing-mode engine |
| `WritingOutputDir` | `output` | Output directory for writing-mode deliverables |
| `DefaultMaxHealAttempts` | `3` | Heal loop retries (fallback for auto effort) |
| `DefaultMaxFailPercent` | `20` | Max % of checks that can fail and still pass |
| `HealAttemptsHigh` | `10` | Heal attempts for high effort |
| `HealStuckThresholdHigh` | `2` | Consecutive no-progress attempts before exit (high) |
| `HealStuckThresholdMax` | `3` | Consecutive no-progress attempts before exit (max) |
| `HealMinAttemptsMax` | `10` | Min attempts before mid-loop threshold exit (max) |
| `HealSafetyCapMax` | `50` | Hard safety cap for unlimited max-effort healing |
| `MaxFailPercentMax` | `10` | Stricter threshold for max effort |
| `DefaultMaxOuterAuditCycles` | `3` | Outer audit cycles per sprint (medium/default) |
| `DefaultMaxInnerFixIter` | `3` | Inner fix iterations per audit report (medium/default) |
| `MaxOuterCyclesHighCap` | `10` | Outer audit cycles at high effort |
| `MaxOuterCyclesMaxCap` | `15` | Outer audit cycles at max effort |
| `MaxInnerFixIterHigh` | `5` | Inner fix iterations at high effort |
| `MaxInnerFixIterMax` | `8` | Inner fix iterations at max effort |
| `DefaultDockerReadyTimeout` | `30` | Seconds for Docker health check |
| `DefaultMaxDeviationScope` | `3` | Max sprints affected by replan |
| `MaxAuditDiffBytes` | `100000` | Max diff size for audit context |
| `RetryHealMultiplier` | `2` | Heal iteration multiplier on retry |
| `RetryMinHealAttempts` | `6` | Minimum heal attempts on retry |
| `BuildModeFile` | `.fry/build-mode.txt` | Persisted build mode for `--continue` |

---

## Asset Systems

### Text Assets (`assets/` → `internal/assets/`)
- Scans `assets/` for text files (30+ extensions: .md, .txt, .json, .yaml, .go, .py, etc.)
- Full file contents injected into prepare-phase prompts
- Limits: 512 KB/file, 2 MB total, 100 files max
- Skips hidden files, symlinks, binary files

### Media Assets (`media/` → `internal/media/`)
- Scans `media/` for binary files (images, PDFs, fonts, videos, etc.)
- Generates categorized manifest (path + size) — contents NOT read
- Limit: 10,000 files
- Manifest injected into both prepare and sprint prompts

---

## Build & Test

```bash
make build     # go build -o bin/fry ./cmd/fry
make test      # go test -race ./...
make lint      # golangci-lint run
make install   # build + cp bin/fry /usr/local/bin/fry
make clean     # rm -rf bin/
```

**20 test files** covering all packages. Tests use `t.Parallel()`, temp directories, env mocking, and mock engines. No CI/CD configured — local testing only.

---

## Documentation Index (`docs/`)

| File | Topic |
|------|-------|
| `getting-started.md` | Install, prerequisites, first build |
| `commands.md` | Full CLI reference |
| `effort-levels.md` | Effort triage and sprint sizing |
| `epic-format.md` | Epic syntax, directives, validation |
| `engines.md` | Codex/Claude config, mixing, model overrides |
| `sprint-execution.md` | Agent loop, prompt assembly, progress |
| `verification.md` | Check primitives, format, outcome matrix |
| `self-healing.md` | Heal loop mechanics |
| `sprint-audit.md` | Per-sprint semantic review |
| `build-audit.md` | Final holistic audit |
| `sprint-review.md` | Mid-build review, replanning, deviations |
| `docker.md` | Docker Compose lifecycle |
| `preflight.md` | Pre-build validation |
| `planning-mode.md` | Non-code document generation |
| `writing-mode.md` | Human-language content (books, guides, reports) |
| `media-assets.md` | Binary asset handling |
| `supplementary-assets.md` | Text asset injection |
| `user-prompt.md` | Prompt injection, hierarchy, persistence |
| `project-structure.md` | Directory layout, file reference |
| `terminal-output.md` | Output format, logging |
| `architecture.md` | Internal package structure, data flow |

---

## Epic File Format (Quick Reference)

Epics are markdown files parsed by `epic/parser.go`. Global directives appear before any sprint block:

```markdown
@epic My Project
@engine codex
@effort high
@require_tool go
@require_tool node
@require_tool docker
@preflight_cmd go version
@pre_sprint echo "starting sprint"
@docker_from_sprint 2
@max_heal_attempts 3
@review_between_sprints
@review_engine claude
@audit_after_sprint
@verification .fry/verification.md

@sprint 1
@name Foundation
@max_iterations 20
@promise FOUNDATION_DONE
@prompt
Build the project skeleton...
@end
```

Sprint prompts follow a 7-part convention: OPENER, REFERENCES, BUILD LIST, CONSTRAINTS, VERIFICATION, STUCK HINT, PROMISE.

---

## Design Patterns

- **Two-phase progress tracking:** per-sprint log (`sprint-progress.txt`) + cross-sprint compacted summaries (`epic-progress.txt`) for bounded context
- **Promise tokens:** agent writes `===PROMISE: TOKEN===` to signal sprint completion → early exit
- **No-op detection:** if git diff shows no changes for 2-3 consecutive iterations and verification passes → early exit
- **Two-level audit loop:** outer cycles discover issues, inner loops fix them FIFO; per-finding tracking across cycles with verify agents; CRITICAL/HIGH block, MODERATE is advisory
- **Graceful signal handling:** Ctrl+C saves partial work via git checkpoint
- **Engine abstraction:** any CLI-based AI tool can be added by implementing `Engine` interface (2 methods: `Run`, `Name`)
