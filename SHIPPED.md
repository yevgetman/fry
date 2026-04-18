# SHIPPED.md — fry v0.1.0

Honest documentation of what works and what doesn't, written at ship time.

## What works

### All 9 subcommands are functional
- `fry new` — scaffolds mission directory with state.json, notes.md, runner.sh, LaunchAgent plist, wake_log.jsonl, supervisor_log.jsonl, artifacts/
- `fry list` — lists missions with status and elapsed time
- `fry status` — prints mission snapshot including recent wakes and artifact count
- `fry start` — installs macOS LaunchAgent; first wake fires after one interval
- `fry stop` — unloads LaunchAgent, sets status=stopped
- `fry wake` — fires one wake: acquires lock → assembles layered prompt → calls `claude -p` → parses output → updates state → appends wake_log
- `fry logs` — tails wake_log.jsonl
- `fry chat` — opens interactive Claude session with mission context pre-loaded via `--append-system-prompt`
- `fry version` — prints git-describe version (via ldflags)

### Prompt assembly (layered, cache-friendly)
- L0–L5 layers: wake contract → prompt.md → plan.md → notes.md sections → last 5 wake_log entries → current-wake directive
- Stable prefix (L0–L2) stays constant wake-to-wake, maximizing Claude prompt cache hit rate

### Cross-wake memory
- `notes.md` has structured sections (Current Focus, Next Wake Should, Decisions, Open Questions, Supervisor Injections)
- Agent edits notes.md; next wake reads it via the L3 prompt layer
- Section-aware parser handles malformed markdown gracefully

### Deadline + shutdown logic
- Soft deadline: transitions `active → overtime`
- Hard deadline: wake fires but skips claude call; immediately writes final log entry and uninstalls scheduler
- Agent-driven shutdown: `FRY_STATUS_TRANSITION=complete` on stdout triggers state update + scheduler uninstall
- No-op detection: 3 consecutive promise-token-missing wakes → `noop_warning` in supervisor_log.jsonl

### Overlap lock
- `os.Mkdir`-based atomic lock prevents concurrent wakes
- Lock directory is `<mission>/lock/`; created on acquire, removed on release

### State machine
- Transitions: `active → overtime → complete/stopped/failed`, validated before each save
- Illegal transitions logged to supervisor_log.jsonl, not applied

### Code quality
- `go vet ./...` clean
- `golangci-lint run` clean (0 issues)
- `go test -race ./...` green (all tests pass)
- Non-test LOC: 1917 (under 2000 cap)

## What doesn't work / known gaps

### No end-to-end dogfood acceptance test run
The spec §15 acceptance test (30-minute dogfood mission with ≥5 wakes) was not run to completion before shipping. Unit and integration tests pass, but the full scheduler loop has not been exercised as a formal gate. Manual spot testing of individual wakes with real claude calls succeeded.

### Stale lock recovery not implemented
The build-plan risk register notes: `lock mtime > 2× interval → assume stale + take over`. This is documented as an open question but not implemented. A crashed wake leaves a stale lock directory; the next wake will be skipped until a human removes `<mission>/lock/`.

### Linux support is a stub
`internal/scheduler/linux.go` returns `ErrUnsupported`. Only macOS is supported in v0.1.

### Chat session not smoke-tested with a live mission
`fry chat` launches an interactive claude session and passes context correctly. The behavior of the embedded system prompt has been tested manually but not in a formal acceptance loop.

### No CI pipeline
There is no GitHub Actions or CI configuration. Tests must be run manually: `make test`.

### `fry start` on a stopped mission
`fry start` currently rejects missions with status `stopped` or `complete`. To restart a stopped mission the user must manually edit `state.json` or delete and re-create the mission.

## Version

`v0.1.0` — built in 8 wakes (~2 hours elapsed of 12h soft deadline), dogfooded on the machine it was built on.

## What to do next (v0.2 candidates)

- Stale lock recovery (mtime check)
- `fry start` resume for stopped missions  
- Linux systemd backend
- GitHub Actions CI
- Formal acceptance test suite (`make acceptance`)
