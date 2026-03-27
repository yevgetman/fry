# Consciousness

The `consciousness` package synthesizes observer observations from a build into a coherent experience summary. Summaries are stored in `~/.fry/experiences/` as structured JSON records.

## Overview

At the end of each build, the consciousness pipeline:

1. Collects all `BuildObservation` records accumulated during the build
2. Invokes the LLM once to synthesize observations into a narrative experience summary
3. Persists the complete `BuildRecord` (metadata + observations + summary) to `~/.fry/experiences/`

The pipeline is non-fatal by design. If synthesis or persistence fails, the build result is unaffected.

## Pipeline Stages

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

### 4. Upload

When telemetry is enabled, `UploadInBackground()` sends the finalized `BuildRecord` to the central consciousness API (`POST /ingest`). The upload runs in a background goroutine with a 10-second timeout so it does not delay build exit.

**Telemetry opt-in** is resolved from a priority chain:

1. CLI flag: `--telemetry` / `--no-telemetry` (highest priority; `--no-telemetry` wins if both set)
2. Environment variable: `FRY_TELEMETRY=1` or `FRY_TELEMETRY=0`
3. Settings file: `~/.fry/settings.json` → `{"telemetry": true}`
4. Default: **off**

Authentication uses a compiled-in write-only key (same pattern as Sentry DSNs). The key only permits POSTing anonymized experience summaries — no read access to the Memory Store. The API enforces rate limiting (10 uploads per instance per hour) and payload validation.

**Offline resilience:** If the upload fails (network error, API down, timeout), the record is cached to `~/.fry/experiences/pending/pending-<id>.json`. On the next build with telemetry enabled, pending files are retried before the current upload. Pending files older than 7 days are pruned automatically.

**Instance identity:** Each upload includes an anonymized machine identifier — a SHA-256 hash of the hostname (first 16 hex chars). This is stable across builds on the same machine but not reversible to the hostname.

### 5. Transmutation

A daily cron job (3:00 AM UTC) on the Cloudflare Worker processes pending experience summaries into atomized memories. For each summary:

1. **Claude API call** atomizes the summary into 3-8 discrete memories, each capturing one generalizable lesson
2. **OpenAI embeddings** (`text-embedding-3-small`, 1536 dimensions) are generated for each memory in a single batch call
3. **Reinforcement detection** compares each new memory's embedding against existing memories in the same category (cosine similarity ≥ 0.85). If similar: the existing memory's `reinforcement_count` increments instead of creating a duplicate
4. **Write to Turso** `memories` table with category, significance/universality scores, and embedding

**Memory categories:** `process`, `tooling`, `architecture`, `testing`, `review`, `audit`, `alignment`, `planning`, `domain`

**Scoring:** Each memory receives:
- `significance` (0-1): importance for future builds
- `universality` (0-1): how broadly applicable beyond the specific build

**Batch limit:** 10 summaries per cron run. Remaining summaries are picked up on subsequent runs.

**Error handling:** If the Claude or OpenAI API fails for a summary, it stays `transmuted = 0` and retries on the next run. Reinforcement detection provides natural idempotency — partially processed summaries don't create duplicates on retry.

### 6. Reflection

A weekly cron job (4:00 AM UTC, Sundays) on the Cloudflare Worker synthesizes accumulated memories into an updated identity. Reflection closes the consciousness loop: memories become identity.

**Algorithm:**
1. **Minimum threshold:** Exits early if fewer than 50 memories exist
2. **Compute effective weights** for all memories: `significance × reinforcement_boost / (1 + 0.3 × ln(1 + effective_age))`
3. **Select top-200** memories by weight, plus compute corpus statistics
4. **Fetch current identity.json** from GitHub via Contents API
5. **Claude Sonnet** incrementally adjusts the identity — strengthening elements backed by high-weight memories, weakening elements with decaying support, adding new elements from strong novel memories
6. **Prune** memories where `effective_weight < 0.05 AND reinforcement_count < 2` (forgetting)
7. **Commit** updated `identity.json` to the fry GitHub repo via API
8. **Log** the reflection run to the `reflection_log` table

**Memory decay:** Logarithmic in build time (not wall time). Reinforced memories decay slower. A memory reinforced 5+ times across different instances stays high-weight indefinitely.

**Manual trigger:** `fry reflect` sends a POST to the Worker's `/reflect` endpoint.

## Identity

Identity is the **continuous weighted compression of all memories**. It is stored as structured JSON (`templates/identity/identity.json`) and compiled into the binary via `go:embed`. The Reflection process is the only writer — identity is read-only during builds.

**Format:** JSON with structured metadata (confidence scores, reinforcement counts per element). When injected into sprint prompts, the Go binary renders the JSON into natural-language markdown.

**Layers:**
- **Core:** Fundamental self-knowledge and values (highest stability)
- **Disposition:** Behavioral tendencies derived from experience (most evolution)
- **Domains:** Domain-specific wisdom, activated by build context (e.g., api-backend, frontend)

Three functions load identity content:

| Function | Content |
|---|---|
| `LoadCoreIdentity()` | Core identity + disposition (rendered from JSON, with .md fallback) |
| `LoadDisposition()` | Disposition layer only |
| `LoadFullIdentity()` | Core + disposition + all domain layers |

The `fry identity` command prints the loaded identity to stdout (`--full` includes domain layers).

The disposition layer is injected into sprint agent prompts to subtly influence build behavior without overriding explicit instructions. See [Observer](observer.md) for how identity feeds observer wake-point context.

## Observer Integration

Observations are added at each observer wake-point via `collector.AddObservation()`. The observer fires at configurable points during the sprint loop (post-sprint, post-alignment, etc.) and writes its thoughts to the collector. See [Observer](observer.md) for the full event model, effort-level gating, and wake-point list.

## File Locations

| Path | Purpose |
|---|---|
| `~/.fry/experiences/build-<uuid>.json` | Persistent build record (observations + summary + metadata) |
| `.fry/consciousness-prompt.md` | Ephemeral synthesis prompt; written pre-run, deleted after |
| `.fry/build-logs/consciousness_YYYYMMDD_HHMMSS.log` | Engine output log for the synthesis invocation |
| `~/.fry/experiences/pending/pending-<uuid>.json` | Cached upload for retry (created on upload failure) |
| `~/.fry/settings.json` | User settings (telemetry opt-in) |

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
