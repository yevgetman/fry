# Observer

The observer is a metacognitive layer that gives Fry persistent, evolving self-awareness. It watches builds, notices patterns, and develops insight over time. Observer failures are non-fatal -- they never break a build.

## Architecture

The observer has four components:

### Event stream

Mechanical JSON-line events emitted at build checkpoints. Zero LLM cost -- these are structured data, not agent output. Written to **`.fry/observer/events.jsonl`**.

### Identity

Fry's canonical identity is compiled into the binary via `go:embed` under `templates/identity/`. It consists of layered files:

- **`core.md`** — fundamental self-knowledge: what Fry is, its purpose, its values (~500 tokens, always loaded)
- **`disposition.md`** — behavioral tendencies derived from build experience (~500 tokens, always loaded)
- **`domains/`** — domain-specific wisdom activated by context (future)

The identity is **read-only during builds**. The observer reads it for context but cannot modify it. Identity is updated only by the Reflection process between builds (see `build-docs/fry-consciousness.md`). Use `fry identity` to view the current identity.

### Scratchpad

Working memory at **`.fry/observer/scratchpad.md`**. Reset at the start of each build. Carries observations, hypotheses, and notes between wake-ups within a single build.

### Wake-ups

Short LLM sessions at natural breakpoints. Each wake-up reads the identity, scratchpad, and recent events (~12-15 KB total), then writes observations back. The observer responds with structured XML-style tags: `<thoughts>`, `<scratchpad>`, and `<directives>` (optional).

### Experience collection

At each wake-up, the observer's thoughts are collected in-memory by the consciousness pipeline. At build end, all observations are written to `~/.fry/experiences/build-<id>.json` as a structured build record. This is the raw input for the downstream memory pipeline (Stages 3+).

## Event Types

Events are cheap structured data appended to the JSONL stream. Each event has a timestamp, type, optional sprint number, and optional key-value data.

| Event | When Emitted |
|---|---|
| `build_start` | Build initialization |
| `sprint_start` | Before each sprint |
| `sprint_complete` | After sprint finishes |
| `heal_complete` | After heal loop |
| `audit_complete` | After sprint audit |
| `review_complete` | After sprint review |
| `build_audit_done` | After build-level audit |
| `build_end` | End of build |

### Example event format

```jsonl
{"ts":"2026-03-23T10:00:00Z","type":"build_start","data":{"epic":"auth-rewrite","effort":"high","total_sprints":"4"}}
{"ts":"2026-03-23T10:02:14Z","type":"sprint_complete","sprint":1,"data":{"status":"PASS","duration":"2m14s","heal_attempts":"0"}}
```

## Wake-Up Schedule

Wake-ups are gated by [effort level](effort-levels.md). Higher effort levels enable more observation points.

| Effort | After Sprint | After Build Audit | Build End |
|---|---|---|---|
| `low` | disabled | disabled | disabled |
| `medium` | -- | -- | yes |
| `high` | yes | yes | yes |
| `max` | yes | yes | yes |

At `low` effort, the observer is fully disabled (no events, no wake-ups). At `medium`, only the final build-end wake-up runs. At `high` and `max`, the observer wakes after every sprint, after the build audit, and at build end.

## How Observer Consciousness Works

- **Identity is compiled into the binary.** Fry's self-knowledge and behavioral disposition are embedded at build time via `go:embed`. The identity is read-only during builds — updated only by the Reflection process between builds.
- **Scratchpad carries working memory within a build.** Reset at `build_start`, it accumulates observations across wake-ups during a single build.
- **Events are cheap structured data, not raw logs.** Emitting events costs zero LLM tokens. Only wake-ups invoke the engine.
- **Each wake-up reads ~12-15 KB total** (identity + scratchpad + recent events). The last 50 events are included by default.
- **Observations are collected for the consciousness pipeline.** Each wake-up's thoughts are persisted as a build experience record at `~/.fry/experiences/`, feeding the memory pipeline for future identity evolution.

## Directives

During wake-ups, the observer can emit structured directives in the `<directives>` tag:

| Type | Purpose |
|---|---|
| `WARN` | Flag a potential problem (e.g., stuck heal loop) |
| `NOTE` | Record a neutral observation |
| `SUGGEST` | Propose an adjustment |

**V1 limitation:** Directives are logged only. The build system does not act on them at runtime. Future versions may use directives to adjust build parameters dynamically.

## CLI

### Commands

```bash
fry identity                         # Print core identity + disposition
fry identity --full                  # Print all identity layers including domains
```

### Flags

`--no-observer` disables the observer entirely. No events are emitted, no wake-ups run, no experience records are created.

```bash
fry --no-observer                    # Run without observer
fry --effort high                    # Observer active (high effort)
fry --effort low                     # Observer disabled (low effort)
```

The observer is also automatically disabled during `--dry-run` and at `low` effort.

## File Locations

| File | Purpose | Persists |
|---|---|---|
| `.fry/observer/events.jsonl` | Structured event stream | Per build |
| `.fry/observer/scratchpad.md` | Working memory | Per build (reset at start) |
| `.fry/observer/wake-prompt.md` | Wake-up prompt (transient) | Deleted after use |
| `~/.fry/experiences/build-<id>.json` | Build experience record | Persists across builds |
| `templates/identity/core.md` | Core identity (compiled in) | Updated by Reflection |
| `templates/identity/disposition.md` | Behavioral disposition (compiled in) | Updated by Reflection |

Observer runtime files live under **`.fry/observer/`**, which is gitignored. Experience records are stored in the user's home directory at `~/.fry/experiences/`. Identity files are compiled into the binary.

## Model Selection

The observer session uses the `observer` session type for [automatic model selection](engines.md#automatic-model-selection-tier-system). At `high`/`max` effort, it uses Standard-tier models. At `medium` effort, it uses Mini-tier models.

## Related Documentation

- [Effort Levels](effort-levels.md) -- controls observer wake-up schedule
- [Sprint Execution](sprint-execution.md) -- build loop where events are emitted
- [Sprint Audit](sprint-audit.md) -- audit completion triggers observer events
