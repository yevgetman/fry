# Architecture

fry is a single static Go binary organized into focused internal packages. This document describes the internal structure for contributors and anyone wanting to understand the codebase.

## Package Overview

```
cmd/fry/                 Entry point — calls cli.Execute()
internal/
  cli/                   Cobra command definitions (root, run, prepare, replan, version)
  config/                Constants: file paths, defaults, version string
  docker/                Docker Compose lifecycle management
  engine/                AI engine abstraction (Codex and Claude implementations)
  epic/                  Epic file parser, types, and validator
  git/                   Git operations (init, checkpoint, commit)
  heal/                  Self-healing loop (re-run agent on verification failure)
  lock/                  File-based concurrency lock (PID-based)
  log/                   Timestamped logging with verbose mode
  media/                 Media directory scanner and manifest builder
  preflight/             Pre-build validation checks
  prepare/               Artifact generation (Steps 0-3)
  review/                Dynamic sprint review, replanning, deviation tracking
  shellhook/             Shell command execution for hooks
  sprint/                Sprint execution loop, prompt assembly, progress tracking
  textutil/              Text utilities (markdown stripping, file timestamps, artifact resolution)
  verify/                Verification check parsing, execution, and diagnostic collection
templates/               Embedded templates (AGENTS.md, epic-example, verification-example, etc.)
```

## Key Interfaces

### Engine Interface

Both AI engines implement a common interface, making them interchangeable:

```go
type Engine interface {
    Run(ctx context.Context, prompt string, opts RunOpts) (output string, exitCode int, err error)
    Name() string
}
```

`RunOpts` includes model override, extra flags, working directory, output streams, and log file paths.

## Data Flow

```
User Input (plans/, media/)
       │
       ▼
   fry prepare ──► .fry/AGENTS.md, epic.md, verification.md
                   (scans media/ for asset manifest)
       │
       ▼
   fry run
       │
       ├─ Parse & validate epic (epic/)
       ├─ Acquire lock (lock/)
       ├─ Preflight checks (preflight/)
       ├─ Init git (git/)
       │
       ├─ For each sprint:
       │   ├─ Docker up (docker/)
       │   ├─ Shell hooks (shellhook/)
       │   ├─ Assemble prompt (sprint/prompt.go)
       │   ├─ Agent loop (sprint/runner.go → engine/)
       │   ├─ Verification (verify/)
       │   ├─ Heal loop (heal/ → engine/ → verify/)
       │   ├─ Compact progress (sprint/compactor.go)
       │   ├─ Git checkpoint (git/)
       │   └─ Sprint review (review/)
       │
       └─ Release lock, print summary
```

## Epic Parsing

The epic parser (`internal/epic/parser.go`) is a line-by-line state machine with three states:

| State | Description |
|---|---|
| `stateGlobal` | Parsing global directives before any `@sprint` block |
| `stateSprintMeta` | Parsing sprint metadata (`@name`, `@max_iterations`, etc.) |
| `stateSprintPrompt` | Collecting multi-line prompt text between `@prompt` and `@end` |

## Embedded Templates

Templates are embedded in the binary via Go's `embed` package (`templates/embed.go`). They include:

- `AGENTS.md` — example operational rules
- `epic-example.md` — epic format reference
- `verification-example.md` — verification check examples
- `GENERATE_EPIC.md` — instructions for epic generation

During `fry prepare`, templates are extracted to a temp directory and referenced by the AI engine.

## Build System

The `Makefile` provides standard targets:

| Target | Command |
|---|---|
| `build` | `go build -o bin/fry ./cmd/fry` |
| `test` | `go test -race ./...` |
| `lint` | `golangci-lint run` |
| `clean` | `rm -rf bin/` |
| `install` | `cp bin/fry /usr/local/bin/fry` |

## Dependencies

- `github.com/spf13/cobra` — CLI framework
- `github.com/stretchr/testify` — Testing utilities (test-only)

No other external dependencies. The binary is fully self-contained.
