# README.LLM.md — Fry Codebase Reference for AI Agents

> **Purpose:** Compact, comprehensive codebase map for AI agents. Eliminates the need for deep directory traversal. Covers architecture, file layout, key types, execution flow, and configuration.

## What Is Fry

Fry is a Go CLI tool that orchestrates AI agents (OpenAI Codex, Claude Code, or Ollama) to autonomously build software or generate documents. It decomposes a human-authored plan into sequential "sprints," executes each sprint through an iterative AI agent loop, verifies outputs with machine-executable checks, self-heals on failure, audits quality, and git-checkpoints every sprint.

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
│   ├── cli/                     # Cobra commands: root, run, init, prepare, replan, clean, version, status, identity
│   │   ├── root.go              # Persistent flags (--project-dir, --verbose, --engine, etc.)
│   │   ├── run.go               # Main orchestration: sprint loop, audit, review, continue
│   │   ├── init.go              # Scaffold project structure (plans/, assets/, media/, git, .gitignore)
│   │   ├── prepare.go           # Generate .fry/ artifacts from plans
│   │   ├── replan.go            # Mid-build replanning
│   │   ├── clean.go             # Archive .fry/ and build outputs to .fry-archive/
│   │   ├── version.go           # Version subcommand
│   │   └── status.go            # Show current build state (no LLM call)
│   ├── color/
│   │   ├── color.go             # ANSI color utilities, TTY detection, NO_COLOR support
│   │   └── logcolor.go          # Pattern-matched log line colorizer
│   ├── config/config.go         # All constants: paths, defaults, invocation prompts
│   ├── engine/
│   │   ├── engine.go            # Engine interface + ResolveEngine + NewEngine
│   │   ├── models.go            # Tier-based model selection, validation, session types
│   │   ├── codex.go             # Codex CLI wrapper
│   │   ├── claude.go            # Claude Code CLI wrapper
│   │   └── ollama.go            # Ollama engine — shells out to `ollama run <model>`, reads prompt via stdin
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
│   │   ├── types.go             # CheckType: FILE, FILE_CONTAINS, CMD, CMD_OUTPUT, TEST
│   │   ├── parser.go            # verification.md parser
│   │   ├── runner.go            # Check execution with timeout
│   │   └── collector.go         # Failure report aggregation
│   ├── heal/heal.go             # Self-healing loop on verification failure
│   ├── agentrun/agentrun.go     # Shared dual-log agent execution helper used by sprint and heal packages
│   ├── audit/
│   │   ├── audit.go             # Per-sprint two-level audit: outer audit cycles + inner fix loops
│   │   └── build_audit.go       # Final holistic codebase audit
│   ├── triage/
│   │   ├── types.go             # Complexity, TriageDecision types
│   │   ├── triage.go            # Classify (single LLM call), ParseClassification, prompt builder
│   │   ├── confirm.go           # ConfirmDecision (interactive triage confirmation/adjustment)
│   │   └── builder.go           # BuildSimpleEpic, BuildModerateEpic, WriteEpicFile, GenerateVerificationChecks, WriteVerificationFile
│   ├── review/
│   │   ├── reviewer.go          # Sprint review (CONTINUE vs DEVIATE verdict)
│   │   ├── replanner.go         # Dynamic epic modification
│   │   ├── deviation.go         # Deviation log entry management and build summary
│   │   └── types.go             # ReviewVerdict, DeviationSpec
│   ├── prepare/
│   │   ├── prepare.go           # Steps 0-3 artifact generation
│   │   ├── mode.go              # Mode type (software, planning, writing) + ParseMode
│   │   ├── sanity.go            # Interactive project summary sanity check
│   │   ├── software.go          # Software-mode prompt builders
│   │   ├── planning.go          # Planning-mode prompt builders
│   │   └── writing.go           # Writing-mode prompt builders
│   ├── git/
│   │   ├── git.go               # Git init, checkpoints (commit format: "EpicName — SprintName: Sprint N label [automated]"), diff capture
│   │   ├── types.go             # GitStrategy type, StrategySetup, ParseGitStrategy
│   │   └── strategy.go          # SetupStrategy, ResolveAutoStrategy, GenerateBranchName, worktree/branch helpers
│   ├── docker/docker.go         # Docker Compose lifecycle, health checks
│   ├── preflight/preflight.go   # Pre-build tool/command validation
│   ├── archive/archive.go       # Build archiving (.fry/ → .fry-archive/)
│   ├── lock/lock.go             # File-based build concurrency lock + IsLocked check
│   ├── log/log.go               # Verbose logging, agent banners
│   ├── media/media.go           # Binary asset scanning (images, PDFs, fonts)
│   ├── assets/assets.go         # Text asset scanning + prompt injection
│   ├── continuerun/
│   │   ├── types.go             # BuildState, ContinueDecision, verdict types
│   │   ├── collector.go         # Programmatic build state collection from .fry/ artifacts
│   │   ├── report.go            # BuildState → human-readable markdown report
│   │   └── analyzer.go          # LLM analysis agent for resume decisions
│   ├── summary/summary.go       # AI-generated build summary
│   ├── consciousness/
│   │   ├── identity.go          # Identity loading from go:embed: LoadCoreIdentity, LoadDisposition, LoadFullIdentity
│   │   ├── collector.go         # Build observation collection: Collector, BuildRecord, BuildObservation
│   │   └── summarize.go         # End-of-build experience synthesis: SummarizeExperience, SprintOutcome
│   ├── observer/
│   │   ├── observer.go          # Observer lifecycle: InitBuild, WakeUp, ShouldWakeUp, scratchpad I/O
│   │   ├── event.go             # Event types, EmitEvent, ReadEvents, ReadRecentEvents
│   │   ├── identity.go          # ReadIdentity (delegates to consciousness.LoadCoreIdentity)
│   │   └── prompt.go            # Wake-up prompt builder, response parser, directive extraction
│   ├── metrics/tokens.go        # Token usage parsing for Claude and Codex engines
│   ├── report/report.go         # BuildReport types and JSON serialisation (--json-report)
│   ├── shellhook/shellhook.go   # Pre-sprint/iteration shell commands
│   └── textutil/textutil.go     # Shell quoting, file timestamps, artifact resolution
├── templates/
│   ├── embed.go                 # //go:embed *.md identity/*.md for compile-time inclusion
│   ├── AGENTS.md                # Placeholder (generated via fry prepare)
│   ├── GENERATE_EPIC.md         # LLM prompt template for epic generation
│   ├── epic-example.md          # Fully-commented epic file example
│   ├── verification-example.md  # Verification check examples
│   └── identity/                # Compiled-in identity layers (read-only during builds)
│       ├── core.md              # Fundamental self-knowledge (~500 tokens, always loaded)
│       └── disposition.md       # Behavioral tendencies (~500 tokens, always loaded)
├── docs/                        # 21 user-facing documentation files (see below)
├── plans/                       # User-authored inputs
│   ├── plan.md                  # Build strategy (what to build)
│   └── executive.md             # Project context (why to build it)
├── output/                      # Planning/writing mode deliverables (--mode planning|writing)
├── Makefile                     # build, test, lint, clean, install
├── go.mod / go.sum
├── .fry-worktrees/              # Git worktrees for worktree strategy (gitignored)
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
| `triage-prompt.md` | Classifier prompt for triage gate |
| `triage-decision.txt` | Triage classifier output (complexity, effort, sprints, reason) |
| `git-strategy.txt` | Persisted git strategy for `--continue`/`--resume` reattachment |
| `observer/events.jsonl` | Observer event stream (JSONL, reset per build) |
| `observer/scratchpad.md` | Observer working memory (reset per build) |
| `observer/wake-prompt.md` | Observer wake-up prompt (transient, deleted after use) |
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

Resolution precedence: CLI flag → epic `@engine` → `FRY_ENGINE` env → default (claude for all modes and stages).

### Effort Levels (`internal/epic/types.go`)

| Level | MaxIterations | MaxSprints |
|-------|---------------|------------|
| low | 12 | 2 |
| medium | 20 | 4 |
| high | 25 | 10 |
| max | 40 | 10 |

### Verification Checks (`internal/verify/types.go`)

Five check primitives: `@check_file` (file exists), `@check_file_contains` (regex match in file), `@check_cmd` (command exits 0), `@check_cmd_output` (command output matches regex), `@check_test` (go test command passes).

---

## Execution Flow

```
User Input                          Generated Artifacts
─────────────────────               ────────────────────
plans/plan.md         ──┐
plans/executive.md    ──┤  Triage Gate (1 cheap LLM call)
--user-prompt "..."   ──┤    ↓ Interactive confirmation [Y/n/a] (skipped with --no-sanity-check)
assets/               ──┤    SIMPLE   → programmatic epic (0 LLM calls)
media/                ──┘    MODERATE → programmatic epic + auto-verification (0 LLM calls)
                               COMPLEX  → full prepare (3-4 LLM calls):
                               (manifest only)   .fry/AGENTS.md, .fry/epic.md, .fry/verification.md
                             (--full-prepare bypasses triage → always full prepare)

                        fry run
                        ───────
Before sprint loop:
  1. Preflight checks (required tools + custom commands)
  2. Git init (if needed)
  3. Git strategy setup (branch/worktree creation, artifact copy; persisted to .fry/git-strategy.txt)
  4. --continue: collect build state + LLM analysis (auto-detect resume point, reattach to persisted strategy)

For each sprint (startSprint → endSprint):
  4. Docker up (if @docker_from_sprint <= current sprint)
  5. Pre-sprint shell hook
  6. Assemble layered prompt → .fry/prompt.md
  7. Agent iteration loop:
     │  for iter = 1 to maxIterations:
     │    ├─ Pre-iteration shell hook
     │    ├─ Run AI engine (codex|claude CLI)
     │    ├─ Append output → .fry/sprint-progress.txt
     │    ├─ Check promise token → early exit if found
     │    └─ No-op detection (no git diff) → early exit
  8. Run verification checks
  9. If checks fail: heal loop (effort-level-aware: low=skip, medium=3, high=up to 10 with progress detection, max=unlimited with progress detection)
 10. Sprint audit (if enabled & effort != low) — two-level loop:
     │  ├─ Outer loop (audit cycles): audit agent reviews + verifies previous issues
     │  ├─ Inner loop (fix iterations): fix agent → verify agent → repeat until resolved
     │  ├─ Issues tracked per-finding, FIFO ordered (oldest first)
     │  ├─ medium: bounded (3 outer cycles, 3 inner fix iterations)
     │  └─ high: progress-based (cap 12 outer, 7 inner), max: progress-based (cap 20 outer, 10 inner)
 11. Git checkpoint commit
 12. Compact sprint progress → .fry/epic-progress.txt
 13. Optional sprint review:
     │  ├─ Review agent: CONTINUE or DEVIATE
     │  └─ If DEVIATE: replan affected sprints

Final: build audit (if full epic completed) → deferred check re-run → build summary (includes audit results) → auto-archive (if full epic succeeded)
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
fry clean                            # Archive .fry/ + build outputs to .fry-archive/
fry version                          # Print version
fry status                           # Show current build state (no LLM call)

Key flags:
  --engine codex|claude|ollama       # AI engine for build
  --prepare-engine codex|claude      # AI engine for prepare phase
  --effort low|medium|high|max       # Effort level (auto-detect if omitted)
  --model model-id                   # Override agent model (e.g. opus[1m], sonnet, haiku)
  --mode software|planning|writing   # Execution mode (default: software)
  --planning                         # Alias for --mode planning (backwards compat)
  --user-prompt "..."                # Inject directive into prompts
  --user-prompt-file path            # Load directive from file
  --dry-run                          # Validate without executing
  --sprint N                         # Start from sprint N
  --resume                           # Skip iterations, verify + heal with boosted attempts
  --continue                         # LLM-assisted auto-resume from where build left off
  --git-strategy auto|current|branch|worktree  # Git isolation strategy (default: auto)
  --branch-name name                 # Explicit branch name (overrides auto-generated)
  --always-verify                    # Force verification, healing, and audit regardless of effort/complexity
  --full-prepare                     # Skip triage, run full prepare pipeline
  --triage-only                      # Classify task and exit (no artifact generation)
  --no-sanity-check                  # Skip interactive confirmations (triage + project summary)
  --no-observer                      # Disable observer metacognitive layer
  --no-review                        # Skip mid-build sprint review
  --no-audit                         # Skip audits
  --verbose                          # Verbose logging
  --no-color                         # Disable colored output (also: NO_COLOR env, TERM=dumb)
  --project-dir path                 # Working directory (default: .)
```

---

## Key Constants (`internal/config/config.go`)

| Constant | Value | Purpose |
|----------|-------|---------|
| `DefaultEngine` | `claude` | Default build engine |
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
| `MaxOuterCyclesHighCap` | `12` | Outer audit cycles at high effort |
| `MaxOuterCyclesMaxCap` | `100` | Outer audit cycles at max effort (safety valve; stale detection governs actual exit) |
| `MaxInnerFixIterHigh` | `7` | Inner fix iterations at high effort |
| `MaxInnerFixIterMax` | `10` | Inner fix iterations at max effort |
| `DefaultDockerReadyTimeout` | `30` | Seconds for Docker health check |
| `DefaultMaxDeviationScope` | `3` | Max sprints affected by replan |
| `MaxAuditDiffBytes` | `100000` | Max diff size for audit context |
| `ResumeHealMultiplier` | `2` | Heal iteration multiplier on resume |
| `ResumeMinHealAttempts` | `6` | Minimum heal attempts on resume |
| `BuildModeFile` | `.fry/build-mode.txt` | Persisted build mode for `--continue` |
| `ArchiveDir` | `.fry-archive` | Directory for archived builds |
| `ArchivePrefix` | `.fry--build--` | Prefix for archive folder names |
| `TriagePromptFile` | `.fry/triage-prompt.md` | Classifier prompt for triage gate |
| `TriageDecisionFile` | `.fry/triage-decision.txt` | Classifier output (complexity, effort, sprints, reason) |
| `DefaultGitStrategy` | `auto` | Default git isolation strategy |
| `GitWorktreeDir` | `.fry-worktrees` | Parent directory for worktree checkouts |
| `GitStrategyFile` | `.fry/git-strategy.txt` | Persisted strategy for continue/resume |
| `GitBranchPrefix` | `fry/` | Prefix for auto-generated branch names |
| `ObserverDir` | `.fry/observer` | Observer files directory |
| `ObserverEventsFile` | `.fry/observer/events.jsonl` | Structured event stream |
| `ObserverScratchpadFile` | `.fry/observer/scratchpad.md` | Per-build working memory |
| `ObserverPromptFile` | `.fry/observer/wake-prompt.md` | Transient wake-up prompt |
| `MaxObserverEvents` | `50` | Max recent events included in wake-up prompt |
| `IdentityCoreFile` | `identity/core.md` | Core identity (go:embed path) |
| `IdentityDispositionFile` | `identity/disposition.md` | Disposition (go:embed path) |
| `IdentityDomainsDir` | `identity/domains` | Domain files directory (go:embed path) |
| `ExperiencesDir` | `.fry/experiences` | Build experience records |
| `ConsciousnessPromptFile` | `.fry/consciousness-prompt.md` | Experience synthesis prompt (transient) |

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

**39 test files** covering all packages. Tests use `t.Parallel()`, temp directories, env mocking, and mock engines. No CI/CD configured — local testing only.

---

## Documentation Index (`docs/`)

| File | Topic |
|------|-------|
| `getting-started.md` | Install, prerequisites, first build |
| `commands.md` | Full CLI reference |
| `effort-levels.md` | Effort triage and sprint sizing |
| `epic-format.md` | Epic syntax, directives, validation |
| `engines.md` | Codex/Claude/Ollama config, mixing, model overrides |
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
| `observer.md` | Metacognitive layer: events, identity, wake-ups |
| `git-strategy.md` | Branch/worktree isolation strategies |
| `self-improvement.md` | Automated self-improvement pipeline |

---

## Self-Improvement Pipeline (`.self-improve/`)

Fry improves itself via an automated loop driven by `.self-improve/orchestrate.sh`:

| File | Purpose |
|------|---------|
| `orchestrate.sh` | Bash orchestrator — planning, build, merge/PR, cleanup |
| `executive.md` | Static directive copied to `plans/` before each run |
| `planning-prompt.md` | User prompt for codebase scanning (10 categories) |
| `build-prompt.md` | User prompt for implementation (Fry selects items) |
| `build-journal.json` | Build history — last 30 entries, used by planning for experience category |
| `config` | KEY=VALUE configuration (overrides script defaults) |
| `logs/` | Per-run timestamped log files |

The roadmap lives in GitHub Issues (labels: category/*, priority/*, effort/*, status/*). See docs/self-improvement.md for the full architecture.

**Flow:** Planning (scan codebase + analyze build journal → create findings) → Build (select items → worktree → implement → test → merge/PR → write journal entry). Planning runs only when roadmap needs replenishment (< 5 items, category gaps, or imbalance). After each build, a structured journal entry is written to `build-journal.json` with outcome, items, heal rounds, and AI observations. During planning, the journal feeds **Category J: Build Experience** for pattern-based improvements.

**Key flags:** `--auto-merge` (direct merge to master), `--skip-planning`, `--skip-build`, `--dry-run`.

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
- **Two-level audit loop:** outer cycles discover issues, inner loops fix them FIFO; per-finding tracking across cycles with verify agents; CRITICAL/HIGH block, MODERATE is advisory, LOW included in fix at high/max effort (non-blocking)
- **Graceful signal handling:** Ctrl+C saves partial work via git checkpoint
- **Engine abstraction:** any CLI-based AI tool can be added by implementing `Engine` interface (2 methods: `Run`, `Name`)
