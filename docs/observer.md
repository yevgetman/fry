# Observer

The observer is a metacognitive layer that gives Fry persistent, evolving self-awareness. It watches builds, notices patterns, and develops insight over time. Observer failures are non-fatal -- they never break a build.

## Architecture

The observer has four components:

### Event stream

Mechanical JSON-line events emitted at build checkpoints. Zero LLM cost -- these are structured data, not agent output. Written to **`.fry/observer/events.jsonl`**.

### Identity document

A persistent self-description at **`.fry/observer/identity.md`**. Seeded from an embedded template on first run. The observer can edit its own identity during wake-ups when its self-understanding genuinely evolves. Identity persists across builds.

### Scratchpad

Working memory at **`.fry/observer/scratchpad.md`**. Reset at the start of each build. Carries observations, hypotheses, and notes between wake-ups within a single build.

### Wake-ups

Short LLM sessions at natural breakpoints. Each wake-up reads the identity, scratchpad, and recent events (~12-15 KB total), then writes observations back. The observer responds with structured XML-style tags: `<thoughts>`, `<scratchpad>`, `<identity_update>` (optional), and `<directives>` (optional).

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

- **Identity persists across builds.** The observer's self-description survives between runs and evolves as the observer edits it. A new identity is seeded from an embedded template on first encounter.
- **Scratchpad carries working memory within a build.** Reset at `build_start`, it accumulates observations across wake-ups during a single build.
- **Events are cheap structured data, not raw logs.** Emitting events costs zero LLM tokens. Only wake-ups invoke the engine.
- **Each wake-up reads ~12-15 KB total** (identity + scratchpad + recent events). The last 50 events are included by default.
- **Identity updates are conservative.** The observer is instructed to update its identity only when self-understanding has genuinely evolved -- not on every wake-up.

## Directives

During wake-ups, the observer can emit structured directives in the `<directives>` tag:

| Type | Purpose |
|---|---|
| `WARN` | Flag a potential problem (e.g., stuck heal loop) |
| `NOTE` | Record a neutral observation |
| `SUGGEST` | Propose an adjustment |

**V1 limitation:** Directives are logged only. The build system does not act on them at runtime. Future versions may use directives to adjust build parameters dynamically.

## CLI Flag

`--no-observer` disables the observer entirely. No events are emitted, no wake-ups run, no observer files are created.

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
| `.fry/observer/identity.md` | Observer self-description | Across builds |
| `.fry/observer/scratchpad.md` | Working memory | Per build (reset at start) |
| `.fry/observer/wake-prompt.md` | Wake-up prompt (transient) | Deleted after use |

All observer files live under **`.fry/observer/`**, which is gitignored as part of the `.fry/` directory.

## Model Selection

The observer session uses the `observer` session type for [automatic model selection](engines.md#automatic-model-selection-tier-system). At `high`/`max` effort, it uses Standard-tier models. At `medium` effort, it uses Mini-tier models.

## Related Documentation

- [Effort Levels](effort-levels.md) -- controls observer wake-up schedule
- [Sprint Execution](sprint-execution.md) -- build loop where events are emitted
- [Sprint Audit](sprint-audit.md) -- audit completion triggers observer events
