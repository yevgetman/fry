# Consciousness

The `consciousness` package synthesizes observer observations from a build into a coherent experience summary. Summaries are stored in `~/.fry/experiences/` as structured JSON records.

## Overview

At the end of each build, the consciousness pipeline:

1. Collects all `BuildObservation` records accumulated during the build
2. Invokes the LLM once to synthesize observations into a narrative experience summary
3. Persists the complete `BuildRecord` (metadata + observations + summary) to `~/.fry/experiences/`

The pipeline is non-fatal by design. If synthesis or persistence fails, the build result is unaffected.

## Three Stages

### 1. Collection

`Collector` accumulates `BuildObservation` records throughout the build. Each observation captures a single observer wake-point: a timestamp, the wake-point label, the sprint number, and the observer's raw thoughts.

```go
func (c *Collector) AddObservation(thoughts, wakePoint string, sprintNum int)
```

`AddObservation` is safe for concurrent use — it acquires an internal mutex before appending to the record. See [Observer](observer.md) for how and when observations are generated.

### 2. Synthesis

`SummarizeExperience()` invokes the configured LLM at build end, passing all observations as structured prompt context.

```go
func SummarizeExperience(ctx context.Context, opts SummarizeOpts) (string, error)
```

The function writes an ephemeral prompt to `.fry/consciousness-prompt.md`, invokes the engine, then deletes the prompt file. If the engine returns an error — regardless of whether it produced partial output — `SummarizeExperience` returns an error rather than partial data.

### 3. Persistence

`Finalize()` sets the build outcome, stamps `EndTime`, and writes the complete `BuildRecord` to `~/.fry/experiences/build-<id>.json`.

```go
func (c *Collector) Finalize(outcome string) error
```

The build ID is generated at collector creation time via `generateBuildID()` (UUID v4 using `crypto/rand`, with a time-based fallback if `crypto/rand` fails). Retrieve it before or after finalization with `c.BuildID()`.

## Identity Layers

Identity is read from files embedded in the binary at compile time. Three functions load identity content:

| Function | Content |
|---|---|
| `LoadCoreIdentity()` | Core identity (`identity/core.md`) concatenated with disposition (`identity/disposition.md`) |
| `LoadDisposition()` | Disposition layer only |
| `LoadFullIdentity()` | Core + disposition + all domain files from `identity/domains/` |

Identity is read-only during builds — there is no runtime write path. The `fry identity` command prints the loaded identity to stdout (`--full` includes domain layers).

The disposition layer is injected into sprint agent prompts to subtly influence build behavior without overriding explicit instructions. See [Observer](observer.md) for how identity feeds observer wake-point context.

## Observer Integration

Observations are added at each observer wake-point via `collector.AddObservation()`. The observer fires at configurable points during the sprint loop (post-sprint, post-heal, etc.) and writes its thoughts to the collector. See [Observer](observer.md) for the full event model, effort-level gating, and wake-point list.

## File Locations

| Path | Purpose |
|---|---|
| `~/.fry/experiences/build-<uuid>.json` | Persistent build record (observations + summary + metadata) |
| `.fry/consciousness-prompt.md` | Ephemeral synthesis prompt; written pre-run, deleted after |
| `.fry/build-logs/consciousness_YYYYMMDD_HHMMSS.log` | Engine output log for the synthesis invocation |

## BuildRecord Schema

`BuildRecord` is the complete record written to `~/.fry/experiences/`.

| Field | Type | Description |
|---|---|---|
| `ID` | `string` | UUID v4 build identifier |
| `StartTime` | `time.Time` | Build start timestamp |
| `EndTime` | `time.Time` | Build end timestamp (set by `Finalize`) |
| `Engine` | `string` | Engine name used for the build |
| `EffortLevel` | `string` | Effort level (`low`, `medium`, `high`, `max`) |
| `TotalSprints` | `int` | Number of sprints in the epic |
| `Outcome` | `string` | Final build outcome (set by `Finalize`) |
| `Observations` | `[]BuildObservation` | Observer observations collected during the build |
| `Summary` | `string` | LLM-synthesized narrative (empty if synthesis was skipped or failed) |

`BuildObservation` fields: `Timestamp`, `WakePoint`, `SprintNum`, `Thoughts`.
