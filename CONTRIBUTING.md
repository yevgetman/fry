# CONTRIBUTING.md — Codebase Development Doctrine

> **Audience:** anyone modifying, extending, or fixing code in this repository — human or AI agent. Read this file before making any changes.
>
> **Naming note:** the lowercase `agents.md` and `claude.md` at repo root are historical *mission artifacts* from the v0.1 autonomous build — operational memory for a specific wake-loop run, not codebase doctrine. This file is the active codebase development guide. Read the lowercase files for build history; read this one before changing code. (Case-insensitive filesystems make uppercase AGENTS.md/CLAUDE.md collide with the lowercase mission artifacts, so we use the conventional CONTRIBUTING.md.)

---

## 1. Project Overview

`fry` is a Go 1.22+ CLI tool that codifies wake-based autonomous-builder orchestration. It spawns a Claude agent on a repeating schedule; each wake gets a layered prompt and produces artifacts. The tool owns mission state; the agent owns the work. See [`product-spec.md`](product-spec.md) for the thesis and [`build-plan.md`](build-plan.md) for the v0.1 milestones.

Single module `github.com/yevgetman/fry`. Two direct deps: `cobra` (CLI) and `testify` (testing). Binary compiles to `bin/fry` via `make build` or installs to `~/go/bin/fry` via `make install`.

---

## 2. Mandatory Checks Before Submitting Work

Every change must pass all four before being considered complete:

```bash
make lint     # golangci-lint run ./...  (0 issues required)
make build    # go build                  (binary compiles clean)
make test     # go test -race ./...       (includes race detection; never skip -race)
make install  # go install                (reinstall binary so ~/go/bin/fry matches HEAD)
```

If tests fail, fix them before moving on. If your change requires new behavior, write or update tests first (see §7).

### Always run a second pass on your own work

Before declaring a task complete, review the full change set again. Treat this as a mandatory quality pass: look for missed bugs, edge cases, regressions, unclear logic, incomplete tests, and documentation drift. Fix anything you find.

### Post-task workflow

After code + docs are done and all checks pass:

1. `git add` only the files that belong to this change (never `git add -A` blindly).
2. Create a focused commit with an imperative message.
3. Run `make install` — the binary at `~/go/bin/fry` must reflect HEAD, not a prior commit. A stale installed binary is a known foot-gun; do not leave one.
4. Push to `origin main`.

---

## 3. Documentation Requirements

When you add, modify, or remove a feature:

1. **`README.md`** — Update the Commands table, Quickstart, `fry new` flag table, or file layout section as applicable.
2. **`SHIPPED.md`** — If you close a known gap, move it from "doesn't work" to "works". If you add a new known limitation, list it. This file is the source of truth for what is and isn't supported.
3. **`product-spec.md`** — Do NOT modify without explicit user direction. This is the locked design doc for v0.1.
4. **`build-plan.md`** — Do NOT modify. This is the historical milestone plan. Future work lives in new planning docs, not edits to this one.
5. **`CONTRIBUTING.md`** — Update if you change a convention (adding a new lint rule, changing test pattern, adding a new make target).

### Documentation style

- Technical, concise, active voice, imperative mood for instructions
- Use **bold** for file paths and command names
- Use backtick code spans for flags (`--effort`), tokens (`===WAKE_DONE===`), file patterns
- Use tables for reference material (flags, commands, comparisons)
- Use fenced code blocks with language identifiers (`bash`, `go`, `markdown`)
- No fluff — every sentence should carry information

---

## 4. Go Code Conventions

### Package organization

Current layout (keep it this way unless there is a strong reason):

- `cmd/fry/main.go` — Cobra root + subcommand wiring. Entry point and CLI surface only; push logic down into `internal/`.
- `internal/mission/` — Mission directory scaffolding (`fry new`). Owns the `templates/` embedded filesystem.
- `internal/state/` — `state.json` read/write + status transition FSM.
- `internal/scheduler/` — Platform-specific scheduler backends (`darwin.go`, `linux.go`) behind a `Scheduler` interface.
- `internal/wake/` — One-wake execution: lock, prompt assembly, claude invocation, promise token / status transition parsing, no-op detection.
- `internal/notes/` — `notes.md` section-aware parser + renderer (cross-wake memory).
- `internal/wakelog/` — `wake_log.jsonl` append + tail.
- `internal/chat/` — `fry chat` interactive session + `supervisor_log.jsonl` audit.

Keep one responsibility per package. Packages that grow past ~500 LOC should be split along the natural seam.

### Naming

- **Packages:** lowercase, single word (`mission`, `wake`, `state`).
- **Exported functions:** PascalCase, descriptive (`Assemble`, `Execute`, `AppendDecision`).
- **Unexported helpers:** camelCase (`copyFile`, `buildTemplateData`).
- **Constants:** PascalCase (`StatusActive`, `PromiseToken`).
- **Interfaces:** noun or role name (`Scheduler`), not `-er` suffix unless natural.
- **Type aliases:** named types for semantic clarity (`type Status string`).

### Import grouping

Three groups separated by blank lines:

```go
import (
    "context"
    "fmt"
    "os"

    "github.com/spf13/cobra"

    "github.com/yevgetman/fry/internal/state"
)
```

Use aliases only on namespace collision.

### Error handling

- **Wrap with context:** `return fmt.Errorf("state.Load: %w", err)`. The prefix names the function/operation; the `%w` preserves the wrapped error.
- **Non-fatal warnings:** `fmt.Fprintf(os.Stderr, "fry: warning: %v\n", err)` — then continue.
- **Distinguish not-found:** check `os.IsNotExist(err)` separately.
- **Exit codes from subprocesses:** use the `errors.As(err, &exitErr)` pattern (see `internal/wake/claude.go`).
- **Never panic.** Return errors up the call chain.

### File operations

- Build paths with `filepath.Join(missionDir, "state.json")` — never hardcode separators.
- Create directories with `0o755`, files with `0o644`.
- Use `os.MkdirAll()` for nested directories.
- **Atomic writes**: write to `path + ".tmp"`, then `os.Rename` (see `state.Save`, `notes.Save`). Any file that a concurrent reader might see mid-update must be written atomically.

### Concurrency and process execution

- Use `context.Context` for cancellation — pass it through every function that may block or exec a subprocess.
- Use `exec.CommandContext(ctx, ...)` — never bare `exec.Command()`.
- Capture stdout/stderr deliberately; distinguish between exec errors and non-zero exit codes.

### Templates and embedding

- Template files live in `internal/mission/templates/` and `internal/chat/systemprompt.md`.
- Embed via `//go:embed` — do not read templates from the filesystem at runtime. A shipped binary must be self-contained.

---

## 5. Adding a New Feature

Follow this checklist:

1. **Create the package** in `internal/<feature>/` with a main file.
2. **Define public types** at the top.
3. **Implement the logic** — one responsibility per package.
4. **Write tests** in `<feature>_test.go` in the same package (see §7). A new package without tests is not mergeable.
5. **Wire into CLI** in `cmd/fry/main.go` — add subcommand, flags, call into the new package.
6. **Update documentation** — README commands table, SHIPPED.md if the feature closes a gap, this file if a convention changed.
7. **Run `make lint build test install`** — all must pass.

### Adding a new scheduler backend

1. Create `internal/scheduler/<platform>.go` with `//go:build <platform>` tag.
2. Implement the `Scheduler` interface from `scheduler.go` (`Install`, `Uninstall`, `Status`, `Kickstart`).
3. Implement `newScheduler()` for that build tag.
4. Update `README.md` requirements section and `SHIPPED.md` platform support list.

### Adding a new CLI subcommand

1. Add a `fooCmd() *cobra.Command` constructor in `cmd/fry/main.go`.
2. Register via `root.AddCommand(fooCmd())` in `rootCmd()`.
3. Push logic into `internal/<package>/` — keep `main.go` thin.
4. Update README.md commands table.
5. Add a smoke test or unit test for the new command path.

---

## 6. Modifying Existing Code

- **Read the code first.** Understand what exists before changing it. Read the file, its tests, and any comment directives.
- **Minimal changes.** Fix or add what's needed — don't refactor surrounding code or add comments to code you didn't write.
- **Preserve existing patterns.** If a package uses `require.NoError`, don't switch to `assert.NoError`. Match surrounding style.
- **Don't break public APIs.** Exported function signatures and type definitions are the contract across packages. If you must change them, update all callers in the same commit.
- **Update the corresponding `_test.go`** for any behavioral change. If behavior changes, tests must reflect it.

---

## 7. Testing Standards

### Every package must have tests

The v0.1 codebase currently has gaps (cmd/fry, chat, scheduler, wakelog have 0% coverage; wake is 22%). Do not add to these gaps. Any new package or new exported function requires tests in the same commit.

### Test file conventions

- Test files live alongside source: `internal/notes/notes.go` → `internal/notes/notes_test.go`.
- Same package (not `_test` external package) unless testing the package boundary specifically.
- Import `github.com/stretchr/testify/assert` and `github.com/stretchr/testify/require`.

### Test function structure

```go
func TestScaffold(t *testing.T) {
    t.Parallel()

    dir := t.TempDir()
    // ... arrange

    result, err := Scaffold(opts)

    require.NoError(t, err)
    assert.Equal(t, expected, result)
}
```

### Rules

- **Always call `t.Parallel()`** as the first line of every test function unless the test touches shared state (in which case, explain with a comment).
- **Use `t.TempDir()`** for file I/O — never write to the working directory.
- **Use `t.Helper()`** in test helpers so failure messages point to the caller.
- **Use `t.Setenv()`** for environment variable overrides (auto-cleaned up).
- **Use `require` for preconditions** (fatal on failure); **`assert` for the actual assertions under test** (non-fatal so follow-up assertions still run).
- **Test failure paths** — not just happy paths. Verify error messages, edge cases, and boundary conditions.
- **No test should depend on another test's state.** Each must be independently runnable in any order.
- **Race detection required:** `go test -race ./...` — this is what `make test` does.

---

## 8. Git Practices

### Commit messages

- Imperative present tense: "Add scheduler.Kickstart", "Fix deadline rounding in SoftDeadline", "Remove unused helper".
- Describe **what and why**, not how.
- One logical change per commit.

### What not to commit

- `.env` files or any secrets.
- `bin/` (build output).
- `logs/cron.log`, `logs/launchd.stdout.log`, `logs/launchd.stderr.log` (operational, per `.gitignore`).
- `/tmp/fry_build.lock` or any runtime lock directories.

The `.gitignore` is the source of truth — respect it.

### Branching

- `main` is the default branch.
- For changes that span several files, consider a feature branch; for single-package changes, commit to `main` is fine.

---

## 9. Architecture Invariants

These are design decisions that must be preserved:

1. **Single binary, zero runtime dependencies.** `fry` compiles to one static binary. External tools required at runtime are the user's responsibility: `claude`, `launchctl` (macOS), `git`.
2. **Minimal Go dependencies.** Currently `cobra` + `testify`. Do not add new dependencies without strong justification. Prefer stdlib solutions.
3. **Templates are embedded.** All template files are compiled into the binary via `//go:embed`. No runtime filesystem reads of templates.
4. **State is file-based.** Mission state lives in `<mission>/state.json` (machine-readable, atomic-writable), `notes.md` (narrative), `wake_log.jsonl` (audit), `supervisor_log.jsonl` (intervention audit). No database, no network service.
5. **The scheduler owns teardown.** Agent signals via `FRY_STATUS_TRANSITION=complete` on stdout; the Go `wake.Execute` parses, validates the state transition, and calls `scheduler.Uninstall`. Agent MUST NOT run `launchctl` / `systemctl` directly. This separation is safety-critical — do not blur it.
6. **Prompt assembly is layered (L0–L5).** Static prefix (L0–L2) must stay stable wake-to-wake for prompt cache hit rate. New prompt content fits into an existing layer or justifies a new one.
7. **The overlap lock is `os.Mkdir` on `<mission>/lock/`.** Atomic, crash-safe detection of concurrent wakes. Do not swap this for a file-based lock, flock, or sync.Mutex — none of those work across processes on macOS.
8. **Status transitions are validated.** `state.CanTransition(from, to)` is the only entry point to mutate status. Illegal transitions logged to `supervisor_log.jsonl` but not applied. Do not bypass this.

---

## 10. Things to Avoid

- **Don't add global mutable state.** No package-level `var` that gets mutated at runtime.
- **Don't use `init()` functions** except for cobra command registration — and in this codebase we use constructor functions (`fooCmd() *cobra.Command`) instead of package-level `init()`, so avoid `init()` entirely.
- **Don't read from stdin** unless implementing an interactive feature the user explicitly requested.
- **Don't add `//nolint` directives** without a comment explaining why. The one existing `//nolint:errcheck` on `wake.DetectNoop` is an intentional advisory-only call.
- **Don't use `interface{}` / `any`** when a concrete type will do.
- **Don't add logging to library packages.** Return errors; let callers (the CLI layer in `cmd/fry/`) decide how to report.
- **Don't modify the Makefile** unless adding a genuinely new build target.
- **Don't leave dead code.** If you remove a feature, remove its code, tests, docs, and CLI flags in the same commit.
- **Don't leave the installed binary stale.** After changing code and committing, `make install` is not optional — a stale `~/go/bin/fry` is the single most wasteful debugging foot-gun in this repo.

---

## 11. Known gaps (see SHIPPED.md for the authoritative list)

Respect these when planning work:

- Test coverage: every package has tests, but `wake.Execute` + `wake.RunClaude` are untested (shell out to external `claude`). `chat.Launch` similarly untested. New work closes gaps, never widens them.
- No CI pipeline yet. `make` targets are the CI.
- Linux scheduler is a stub (returns `ErrUnsupported`).
- No stale-lock recovery (a crashed wake leaves `<mission>/lock/` that blocks future wakes).
- `fry start` rejects `stopped` / `complete` missions — no resume path.

---

## 12. Quick Reference

| Task | Command |
|------|---------|
| Build | `make build` |
| Test (race-detected) | `make test` |
| Lint | `make lint` |
| Install to `~/go/bin` | `make install` |
| Full quality gate | `make lint build test install` |
| Clean build artifacts | `make clean` |
| Run without building | `go run ./cmd/fry` |
| Run one test | `go test -race -run TestName ./internal/<pkg>/` |
| Check compilation only | `go build ./...` |
| Vet only | `go vet ./...` |

| File | Purpose |
|------|---------|
| `README.md` | User-facing project documentation |
| `SHIPPED.md` | Authoritative list of what works / known gaps |
| `product-spec.md` | LOCKED — v0.1 design thesis |
| `build-plan.md` | Historical v0.1 milestone plan |
| `CONTRIBUTING.md` | This file — codebase development doctrine |
| `claude.md`, `agents.md` | Historical v0.1 build mission memory (lowercase) |
| `Makefile` | Build / test / lint / install targets |
| `.golangci.yml` | Lint configuration |
| `.gitignore` | What must never be committed |
