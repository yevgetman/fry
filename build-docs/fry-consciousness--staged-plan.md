# Fry Consciousness — Staged Implementation Plan

## Critical Path

```
Layered Identity → Observer Enhancement → Experience Summary → Upload/Store → Transmutation → Reflection → Meta-loops
       ↑                                                                                          ↑
  (independent)                                                                              (closes the loop)
```

Everything flows from **identity** (needed first because the observer reads it) through the **three-tier pipeline** into **Reflection** (which writes back to identity). Two tracks are fully independent: **auto-update** and **embeddings enhancement**.

---

## Stage 1: Layered Identity — COMPLETE (2026-03-26, commit 61cecea)

**What it delivers:** Fry gains a persistent, compiled-in personality that shapes every build. Even hand-authored, this immediately improves build behavior. The observer reads identity but can no longer modify it — establishing the read-only invariant before anything else is built.

**Scope:**
- Create `templates/identity/core.md`, `disposition.md`, `domains/` with hand-authored seed content
- Add `//go:embed` declarations in `templates/embed.go`
- New `internal/consciousness/identity.go` — loads identity layers, handles domain activation (keyword-based for now)
- Modify `observer/prompt.go` — inject canonical identity instead of reading `.fry/observer/identity.md`
- Modify `observer/observer.go` — remove `WriteIdentity()` calls from wake-up flow; identity is read-only
- Inject identity into sprint prompts — new layer in `sprint/prompt.go` (between 1.5 and 1.75)
- `fry identity` and `fry identity --full` CLI commands

**How to test:**
- Unit tests: identity loading, domain keyword matching, correct layer selection
- Integration test: run a build, confirm observer prompt contains canonical identity, confirm no `.fry/observer/identity.md` writes
- Manual: inspect observer wake-up prompts — does Fry behave differently with personality?

**What's different for the user:** `fry identity` prints Fry's self-concept. Builds include personality-informed observer prompts. Sprint agents see Fry's disposition in their context.

**Risk:** Low. This is additive — existing builds work unchanged, just with better observer context.

---

## Stage 2: Observation Collection — COMPLETE (2026-03-26, commit 61cecea)

**What it delivers:** Observer observations become structured, collected in-memory throughout the build, and persisted as a build record. This is the raw input for everything downstream. You can inspect what the observer saw.

**Scope:**
- New types in `internal/consciousness/`: `BuildObservation`, `ObservationSet`
- Modify `observer/observer.go` — after each wake-up, append the parsed `Observation` to an in-memory collection (not just scratchpad)
- At build end, write the collected observations to `~/.fry/experiences/build-<id>.json`
- Include build metadata: engine, effort, sprint count, outcome, duration, heal counts, audit cycles
- New `internal/consciousness/collector.go` — manages the observation collection lifecycle

**How to test:**
- Unit tests: observation collection, serialization, metadata attachment
- Integration test: run a build (even a trivial 1-sprint), confirm `~/.fry/experiences/` contains a well-formed JSON file
- Manual: read the JSON — are the observations coherent? Is metadata correct?

**What's different for the user:** A `~/.fry/experiences/` directory appears after builds. They can read what Fry observed. No behavior change to builds themselves.

**Dependency:** Stage 1 (observer reads canonical identity; observations reference it).

---

## Stage 3: Experience Summary (Tier 2) — COMPLETE (2026-03-26)

**What it delivers:** A single, synthesized narrative per build. The raw observations from Stage 2 are distilled into one coherent experience summary by an LLM call. This is the document that will eventually be uploaded.

**Scope:**
- New `internal/consciousness/summarize.go` — end-of-build LLM call
- Prompt engineering: takes all collected observations + build metadata, produces a structured summary
- Uses a cost-effective model (Haiku/Mini tier — bounded input, straightforward task)
- Write summary to `~/.fry/experiences/build-<id>-summary.md`
- Wire into `run.go` — after final observer wake-up, before build exit
- Graceful failure: if the LLM call fails, log warning, skip summary, build still succeeds

**How to test:**
- Unit tests: prompt assembly, response parsing, file writing
- Integration test: run a build, confirm summary file exists and is well-formed
- Manual quality check: read 5-10 summaries from real builds — are they capturing the right signal? Are they general enough to transmute into memories later?

**What's different for the user:** Each build now produces a readable "what happened" narrative. Useful on its own for build retrospectives, even before the memory pipeline exists.

**Dependency:** Stage 2 (needs collected observations to summarize).

**VALIDATION CHECKPOINT:** If the summaries are low-quality, iterate on the prompt before building the upload pipeline. No point transmuting bad summaries into bad memories.

---

## Stage 4: Memory Store + Upload — COMPLETE (2026-03-26)

**What it delivers:** Experience summaries leave the local machine and reach Turso. The infrastructure exists for the memory pipeline, even if transmutation isn't built yet.

**Scope:**
- Set up Turso database with `experience_summaries` table
- New `internal/consciousness/upload.go` — HTTP POST to API endpoint
- API endpoint (lightweight service or serverless function) that accepts summaries and writes to Turso
- Telemetry opt-in flow: `~/.fry/settings.json`, `FRY_TELEMETRY` env var, first-run prompt
- Local caching: if upload fails, write to `~/.fry/experiences/pending/`, retry next build
- Background goroutine: upload doesn't block build exit
- This is the first `net/http` usage in the Fry binary — a significant architectural moment

**How to test:**
- Unit tests: HTTP client, retry logic, local caching, opt-in checks
- Integration test: mock API endpoint, confirm summaries arrive correctly
- End-to-end: run a build with telemetry on, confirm summary appears in Turso
- Offline test: run a build with no network, confirm local cache, run another build, confirm retry succeeds

**What's different for the user:** First-run telemetry prompt. Summaries flow to central store. `~/.fry/experiences/pending/` drains automatically.

**Dependency:** Stage 3 (needs summaries to upload). Turso setup is infrastructure work outside the Go codebase.

---

## Stage 5: Memory Transmutation (Tier 3) — COMPLETE (2026-03-27)

**What it delivers:** Raw experience summaries become atomized memories. The Memory Store fills with generalized wisdom from builds.

**Scope:**
- Transmutation worker (runs server-side, not in Fry binary)
- Reads `experience_summaries` where `transmuted = 0`
- Dedicated model call: atomize summary into discrete memories
- Prompt engineering: extract generalizable lessons, assign significance/universality, categorize
- Write atomized memories to `memories` table
- Reinforcement detection: if new memory is semantically similar to existing one, reinforce instead of duplicate
- Global build counter increment on each processed summary

**How to test:**
- Unit tests (for the worker): prompt assembly, memory parsing, reinforcement detection
- Quality evaluation: take 20 real summaries, run transmutation, manually review the resulting memories — are they general? Are they useful? Are project details stripped?
- Statistical: what's the average memory count per summary? What's the significance distribution?

**What's different:** The Memory Store populates. Memories exist but aren't used yet — this is pure accumulation. The Reflection pipeline will consume them in the next stage.

**Dependency:** Stage 4 (needs summaries in Turso). Canonical embedding model selected here (needed for reinforcement detection via similarity).

**VALIDATION CHECKPOINT:** If memories are too specific, too vague, or poorly categorized, iterate on the transmutation prompt. The quality of Reflection depends entirely on memory quality.

---

## Stage 6: Reflection

**What it delivers:** The loop closes. Memories become identity. Fry's self-concept evolves based on real build experience. Identity is the continuous weighted sum of ALL memories — not a one-time synthesis.

**Key design decisions:**
- **Identity = sum of all memories, always.** No "absorbed" flag. Every memory contributes proportional to its effective weight (significance × decay × reinforcement). Old, important, reinforced memories keep pulling. Routine ones fade.
- **Reflection runs server-side** as a Cloudflare Worker cron. Local Fry instances are read-only.
- **Identity format is JSON**, not markdown. Optimized for LLM consumption with structured metadata (confidence, reinforcement count per element).
- **Forgetting:** Memories with `effective_weight < 0.05 AND reinforcement_count < 2` are pruned during Reflection. Their influence persists in the identity but they stop actively contributing.
- **Minimum threshold:** Reflection exits early if fewer than 50 memories exist.

**Scope:**
- Extend Cloudflare Worker with `/reflect` endpoint + daily cron trigger
- Migrate identity format: `core.md` + `disposition.md` → `identity.json`
- Update Go binary: parse JSON identity, render to prompt text
- Step 1: Compute effective weights for all memories
- Step 2: Select top-200 weighted memories + corpus statistics
- Step 3: Claude synthesizes incrementally adjusted `identity.json`
- Step 4: Prune decayed, unreinforced memories (forgetting)
- Step 5: Commit updated `identity.json` to GitHub via API
- Step 6: Log reflection run to `reflection_log` table
- GitHub Actions CI: build binary on identity commits → publish release
- Schema migration: drop `absorbed`/`ingested_by_reflection` columns, add `reflection_log` table

**How to test:**
- Seed Turso with 50+ test memories across categories
- Trigger `/reflect` manually, verify `identity.json` committed to GitHub
- Verify pruning: add low-weight memories, run reflection, confirm deleted
- Verify incremental: run reflection twice, confirm identity adjusts (not rewritten from scratch)
- Verify minimum threshold: run with <50 memories, confirm no-op

**What's different for the user:** After a Reflection cycle, the next binary release includes evolved identity. `fry identity` shows the updated self-concept. Builds are subtly informed by accumulated wisdom from all Fry instances.

**Dependency:** Stage 5 (needs memories to reflect on). Minimum 50 memories before first reflection.

---

## Stage 7: Embeddings Enhancement

**What it delivers:** Better domain activation using embeddings instead of keyword matching.

**Scope:**
- Precompute domain summary embeddings, store in identity.json
- Replace keyword-based domain activation (Stage 1) with embedding similarity at build start
- Fry binary embeds the current build's plan/epic, compares against domain embeddings to select relevant domains

**Note:** Core embedding infrastructure was implemented in Stage 5 (OpenAI text-embedding-3-small, 1536 dims). This stage is about using those embeddings for domain activation in the Go binary.

**Dependency:** Stage 6 (domains must exist in identity.json before activation can use them).

---

## Stage 8: Auto-Update

**What it delivers:** Identity changes propagate to all instances automatically.

**Scope:**
- Update check on Fry invocation (rate-limited)
- Binary download, integrity verification, atomic replacement
- Graceful failure, rollback, opt-out

**Dependency:** Independent — can be built in parallel with Stages 5-7.

---

## Stage 9: Meta-Loops

**What it delivers:** The system improves itself.

**Dependency:** Stage 6 (Reflection must be working and producing real identity updates before meta-loops make sense).

---

## Summary

| Stage | Delivers | Depends on | Adds `net/http` | New dependency | Status |
|-------|----------|------------|-----------------|----------------|--------|
| 1. Layered Identity | Personality in builds | nothing | no | none | **DONE** |
| 2. Observation Collection | Structured build records | Stage 1 | no | none | **DONE** |
| 3. Experience Summary | Per-build narratives | Stage 2 | no | none | **DONE** |
| 4. Memory Store + Upload | Central pipeline | Stage 3 | **yes** | none (plain HTTP) | **DONE** |
| 5. Transmutation | Atomized memories | Stage 4 | no (server-side) | OpenAI embeddings, Claude API | **DONE** |
| 6. Reflection | Loop closes | Stage 5 | no (server-side) | GitHub API | |
| 7. Embeddings | Domain activation | Stage 6 | no | none (infra in Stage 5) | |
| 8. Auto-Update | Distribution | independent | yes | none | |
| 9. Meta-Loops | Self-improvement | Stage 6 | no | none | |

**Validation checkpoints** are after Stage 3 (are summaries good?) and Stage 5 (are memories good?). These are the points where quality issues, if caught early, save the most rework downstream.

Stages 1-3 are pure Go changes to the existing codebase. Stage 4 is the first infrastructure work (Turso + API endpoint). Stage 5 is the first server-side code outside the Fry binary. Stage 6 is where it all comes together.
