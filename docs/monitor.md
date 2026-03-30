# Build Monitoring

`fry monitor` provides real-time, multi-source monitoring of active builds. It composes data from the observer event stream, build status, sprint progress, build logs, and process liveness into enriched views.

## Relationship to Existing Commands

| Command | Purpose |
|---------|---------|
| `fry status` | Point-in-time snapshot of build state (no continuous output) |
| `fry events --follow` | Raw event stream from `events.jsonl` (used by build-watcher service) |
| `fry monitor` | Enriched, multi-source, continuous monitoring with multiple view modes |

`fry monitor` complements the existing commands — it does not replace them.

## Usage

```bash
fry monitor [project-dir] [flags]
```

**Project directory resolution** (priority order):
1. Positional argument: `fry monitor /path/to/project`
2. `--project-dir` flag: `fry monitor --project-dir /path/to/project`
3. Current working directory: `fry monitor`

## View Modes

### Stream (default)

Streams enriched events with elapsed times, sprint progress fractions, and phase transitions.

```bash
fry monitor
```

Output:
```
[10:00:05]  +0s      build_start       effort=high epic=MyFeature sprints=3
[10:00:15]  +10s     sprint_start      1/3  name=Setup
[10:05:15]  +5m10s   sprint_complete   1/3  status=PASS duration=5m
[10:05:18]  +5m13s   sprint_start      2/3  name=API  [triage -> sprint]
```

Enrichments added to each event:
- **Elapsed time**: `+5m10s` — time since `build_start`
- **Sprint fraction**: `1/3` — current sprint out of total
- **Phase change**: `[triage -> sprint]` — when the build transitions between phases

### Dashboard

Refreshing status overview that updates in-place (ANSI-aware on TTY; falls back to separator-delimited blocks on non-TTY).

```bash
fry monitor --dashboard
```

Output:
```
Fry Monitor                         PID 12345  10:05:18
────────────────────────────────────────────────────────────────
Epic: My Feature            Engine: claude    Effort: high
Phase: sprint               Mode: software    Branch: fry/my-feature
────────────────────────────────────────────────────────────────
Sprint 1/3: Setup .............. PASS         5m 10s (aligned)
Sprint 2/3: API ................ running      +2m 30s
Sprint 3/3: Polish ............. pending
────────────────────────────────────────────────────────────────
Latest: sprint_start (2/3) name=API
```

### Log Tail

Follow the most recently modified build log in `.fry/build-logs/`.

```bash
fry monitor --logs
```

### JSON (NDJSON)

Machine-readable output — one JSON snapshot per line. Suitable for piping to `jq` or consuming from scripts.

```bash
fry monitor --json | jq '.new_events[]'
```

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--dashboard` | `false` | Refreshing dashboard view |
| `--logs` | `false` | Tail active build log |
| `--json` | `false` | NDJSON snapshot output |
| `--no-wait` | `false` | Exit immediately if no active build |
| `--interval` | `2s` | Polling interval (e.g. `1s`, `500ms`) |

## Data Sources

The monitor polls these artifacts with change detection to minimize syscalls:

| Source | File | Detection |
|--------|------|-----------|
| Events | `.fry/observer/events.jsonl` | Byte offset tracking |
| Build phase | `.fry/build-phase.txt` | File mtime |
| Build status | `.fry/build-status.json` | File mtime |
| Process lock | `.fry/.fry.lock` | PID liveness (signal-0) |
| Sprint progress | `.fry/sprint-progress.txt` | File size |
| Epic progress | `.fry/epic-progress.txt` | File size |
| Build logs | `.fry/build-logs/*.log` | Directory scan + size |
| Exit reason | `.fry/build-exit-reason.txt` | File existence |

**Polling cost when idle**: ~9 syscalls per tick (mostly `Stat` calls). After 10 unchanged ticks the interval slows from 2s to 5s automatically.

## Lifecycle Handling

| Build State | Monitor Behavior |
|-------------|-----------------|
| No `.fry/` directory | Waits (or exits immediately with `--no-wait`) |
| Early phase (triage/prepare) | Shows phase, waits for events |
| Running | All views active, enriched output |
| Paused | Dashboard shows PAUSED status |
| Ended normally | Final snapshot, summary line, exit |
| Process crashed | Detects PID death, reports "exited unexpectedly", exit |

## Worktree Builds

For builds using the worktree git strategy, the monitor automatically resolves the worktree directory. Events are always emitted to the original project directory; build artifacts (status, logs, progress) may reside in the worktree. The monitor checks both locations.

## Architecture

The monitoring logic lives in `internal/monitor/` as a composable library package:

- **`snapshot.go`** — `Snapshot` and `EnrichedEvent` types
- **`source.go`** — `Source` interface with 7 implementations (one per artifact)
- **`enrichment.go`** — Pure functions for event enrichment
- **`stream.go`** — `Monitor` orchestrator with `Run()` (continuous) and `Snapshot()` (one-shot)
- **`render.go`** — Rendering functions for each view mode

The CLI command in `internal/cli/monitor.go` is a thin wrapper. Future consumers (TUI, websocket server, external tools) can import `internal/monitor` directly.
