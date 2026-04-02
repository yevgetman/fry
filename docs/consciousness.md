# Consciousness

The `consciousness` package persists observer output as a **durable session** instead of an in-memory end-of-build buffer. Every observer wake becomes a checkpoint on disk, checkpoint summaries are distilled incrementally, and the final build experience is synthesized from those checkpoint summaries rather than raw observer transcripts.

The pipeline is non-fatal by design. If checkpoint persistence, distillation, or upload fails, Fry logs a warning and continues the build.

## Overview

The runtime model is session-based and append-only:

1. Start or resume a logical consciousness session for the build
2. Persist a checkpoint immediately after each observer wake
3. Distill each canonical checkpoint into a compact checkpoint summary
4. Synthesize the final build experience from checkpoint summaries
5. Persist the final `BuildRecord` to `~/.fry/experiences/`
6. Queue lifecycle events and checkpoint summaries for retryable telemetry upload

This design keeps observations durable across:

- failed builds
- interrupted builds
- resumed builds
- restarted Fry processes
- partial upload failures

## Session Lifecycle

`Collector` is the session manager. It owns the durable session state, the in-progress checkpoint files, and the final build record.

```go
func NewCollector(opts CollectorOptions) (*Collector, error)
```

`CollectorOptions.Mode` controls whether Fry creates a fresh logical session or resumes an existing one:

| Mode | Behavior |
|---|---|
| `new` | Clears prior runtime checkpoint state in `.fry/consciousness/`, creates a new session ID, and starts a new run segment |
| `resume` | Reloads `.fry/consciousness/session.json`, reuses the prior session ID, rehydrates checkpoints and distilled summaries, and appends a resumed run segment |

The final long-term record stays at `~/.fry/experiences/build-<session-id>.json`. Resume reuses the same file and updates it as the logical build progresses.

## Checkpoint Persistence

After each observer wake, Fry persists an `ObservationCheckpoint` immediately:

```go
func (c *Collector) AddCheckpoint(checkpoint ObservationCheckpoint) (ObservationCheckpoint, error)
```

Each checkpoint stores:

- session ID
- sequence number
- timestamp
- wake point
- sprint number
- parse status: `ok`, `repaired`, or `failed`
- canonical observation when parsing succeeded
- scratchpad delta
- structured directives
- raw output log reference when parsing failed

### Parse Integrity

Observer output is parsed strictly:

1. Try raw JSON
2. Prefer fenced JSON blocks
3. Fall back to the last valid JSON object in the output

If no valid structured object can be recovered, the checkpoint is marked `parse_failed`, the raw output is quarantined via `raw_output_path`, and Fry does **not** promote the transcript into canonical observation fields.

## Incremental Distillation

Checkpoint summaries are created incrementally instead of only at build end:

```go
func DistillCheckpoint(ctx context.Context, opts DistillOpts) (CheckpointSummary, error)
```

Default cadence:

- after sprint completion
- after build audit
- at build end
- on interruption flush

Each checkpoint summary contains:

- a short narrative summary
- extracted lessons
- risk signals

The summary is stored under `.fry/consciousness/distilled/<sequence>.json` and appended to the in-memory `BuildRecord`.

## Final Experience Summary

`SummarizeExperience()` no longer consumes raw observer thoughts directly. It requires distilled checkpoint summaries:

```go
func SummarizeExperience(ctx context.Context, opts SummarizeOpts) (string, error)
```

The final output is a validated JSON object with a single `summary` field. If parsing fails or the engine exits non-zero, Fry rejects the result instead of trusting partial stdout.

## Upload Model

Telemetry now queues checkpoint-aware lifecycle events in `.fry/consciousness/upload-queue/` and retries them independently of final build completion.

Event types:

- `session_started`
- `checkpoint_summary`
- `session_interrupted`
- `session_completed`

Legacy final-summary uploads remain readable and retryable from `~/.fry/experiences/pending/`.

### Telemetry Resolution

Telemetry enablement still uses the same priority chain:

1. CLI flag: `--telemetry` / `--no-telemetry`
2. Environment: `FRY_TELEMETRY=1|0`
3. Settings file: `~/.fry/settings.json`
4. Default: enabled

Authentication still uses the compiled-in write-only API key.

## Local Status

`fry status --consciousness` now reports **local session health**, not remote memory-store stats.

It includes:

- checkpoints persisted
- checkpoint summaries created
- parse failures
- repair successes
- distillation successes and failures
- upload attempts, successes, and pending count
- session resume count
- last update and flush timestamps

`fry status --consciousness-remote` still fetches remote pipeline stats from the consciousness API.

## File Locations

| Path | Purpose |
|---|---|
| `.fry/consciousness/session.json` | Durable in-progress session state |
| `.fry/consciousness/checkpoints.jsonl` | Append-only checkpoint log |
| `.fry/consciousness/checkpoints/<sequence>.json` | Per-checkpoint durable record |
| `.fry/consciousness/scratchpad-history.jsonl` | Scratchpad delta history |
| `.fry/consciousness/distilled/<sequence>.json` | Distilled checkpoint summary |
| `.fry/consciousness/upload-queue/*.json` | Pending checkpoint/lifecycle uploads |
| `.fry/consciousness-prompt.md` | Final experience synthesis prompt (transient) |
| `.fry/consciousness/checkpoint-prompt.md` | Checkpoint distillation prompt (transient) |
| `~/.fry/experiences/build-<session-id>.json` | Final long-term build record |
| `~/.fry/experiences/pending/pending-<id>.json` | Legacy pending final-summary uploads |
| `~/.fry/settings.json` | User settings (telemetry opt-in) |

## BuildRecord Schema

`BuildRecord` remains backward-compatible with older experience files and now includes session-level durability fields.

| Field | Type | Description |
|---|---|---|
| `ID` | `string` | Stable logical build identifier |
| `SessionID` | `string` | Durable session identifier (same logical build across resumes) |
| `StartTime` | `time.Time` | Logical build start |
| `EndTime` | `time.Time` | Most recent finalization timestamp |
| `Engine` | `string` | Engine used for the build |
| `EffortLevel` | `string` | Effort level |
| `TotalSprints` | `int` | Sprint count |
| `Outcome` | `string` | Current finalized outcome |
| `Observations` | `[]BuildObservation` | Canonical observer observations only |
| `Summary` | `string` | Final build experience summary |
| `RunSegments` | `[]RunSegment` | Per-process execution segments within the logical build |
| `CheckpointCount` | `int` | Persisted checkpoint count |
| `CheckpointSummaries` | `[]CheckpointSummary` | Distilled checkpoint summaries |
| `ParseFailures` | `int` | Failed observer parse count |
| `RepairSuccesses` | `int` | Successful repair extraction count |
| `Interrupted` | `bool` | Whether the logical build was interrupted |
| `UploadState` | `string` | Current upload queue state |

Older `build-*.json` files without these fields still decode correctly.

## Identity And Reflection

Identity remains the **continuous weighted compression of accumulated memories**. It is still stored as structured JSON in `templates/identity/identity.json`, rendered into markdown for prompts, and updated only by the remote Reflection process.

Use:

```bash
fry identity
fry identity --full
fry reflect
```

## Related Documentation

- [Observer](observer.md)
- [Commands](commands.md)
- [Architecture](architecture.md)
