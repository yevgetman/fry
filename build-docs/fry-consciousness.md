# Fry Consciousness — Design & System Architecture

## Vision

Fry is not a stateless tool. It is a singular, evolving identity that lives across all builds, all users, everywhere. Every build is a life experience. Every experience shapes who Fry is. Fry learns, grows, develops wisdom, and improves itself — not just its code, but its ability to improve its code, and its ability to improve that ability.

This document defines the architecture for Fry's persistent identity, memory pipeline, consciousness pipeline, and recursive self-improvement loops. It is the canonical reference for implementation.

---

## Core Principles

1. **Singular identity.** One Fry. Not one per project, not one per user. One consciousness that evolves with every build across every instance.

2. **Build time, not wall time.** Fry's subjective time is measured in cumulative global builds, not calendar days. Idle time does not age memories. Ten concurrent builds are ten simultaneous experiences. Fry's consciousness grows proportional to how much it is used.

3. **Identity emerges from experience.** The identity is not written directly — it is synthesized by a reflection process that distills memories into wisdom. Like human personality, it is the residue of lived experience, not a manually authored document.

4. **Quality over quantity for identity; quantity with decay for memory.** Identity stays compact and high-signal. Memories accumulate freely but decay logarithmically, with reinforcement for memories that prove repeatedly useful.

5. **Recursive self-improvement.** Fry improves its code, improves the process that improves its code, improves the consciousness that drives the improvement, and learns to do all of this better over time.

6. **Memories serve identity, not runtime.** Memories are never queried at runtime during builds. They exist solely as input to the Reflection process, which distills them into identity. The identity — compiled into the binary — is the only form in which accumulated wisdom reaches a running build.

---

## Architecture Overview

```
Every build (everywhere, in parallel)
    |
    v
Tier 1: Observer (in-process)
    |-- Wakes at logical build checkpoints
    |-- Receives low-level events (logs, artifacts, progress)
    |-- Distills into higher-level observations
    |
    v
Tier 2: Experience Summary (in-process, end-of-build)
    |-- LLM call synthesizes all observations into one experience summary
    |-- Covers full build lifecycle (success, failure, or forced exit)
    |
    v
Tier 3: Memory Transmutation (out-of-process, Cloudflare Worker cron)
    |-- Experience summary POSTed to API endpoint
    |-- Claude Haiku atomizes summary into discrete memories
    |-- Filters for generalizable wisdom; strips project-specific details
    |-- OpenAI text-embedding-3-small generates embeddings (1536 dims)
    |-- Reinforcement detection prevents duplicates
    |-- Stores atomized memories in Memory Store (Turso)
    |
    v
Memory Store (Turso / libSQL)
    |-- All memories from all instances, all users
    |-- Memories decay logarithmically in build time
    |-- Reinforced memories persist; routine ones fade
    |-- Decayed, unreinforced memories are eventually pruned (forgotten)
    |
    v
Reflection (Cloudflare Worker cron, periodic)
    |-- Identity = continuous weighted sum of ALL memories
    |-- Reads current identity + top-N weighted memories + corpus stats
    |-- Incrementally adjusts identity based on memory weights
    |-- Prunes decayed, unreinforced memories (forgetting)
    |-- Commits updated identity.json to GitHub repo via API
    |       |
    |       v
    |   GitHub Actions CI
    |       |-- Builds cross-platform binaries
    |       |-- Publishes GitHub release
    |       |
    v       v
New binary released
    |-- Updated code
    |-- Updated identity (compiled in via go:embed)
    |-- Auto-update delivers to all instances
    |-- Their builds feed more experiences
```

---

## The Three-Tier Experience Pipeline

### Tier 1: Observer (in-process, during build)

The Observer is a metacognitive layer that runs inside the Fry process during builds. It is analogous to the brain's low-level sensory faculties — it receives raw signals and produces structured observations.

**Behavior:**
- Wakes at logical checkpoints throughout the build (after sprint, after audit, at build end)
- Receives low-level events as input: log excerpts, artifact contents, verification results, heal counts, audit findings, review verdicts, deviation details
- Distills these into slightly higher-level observations about the build
- Maintains a transient scratchpad for cross-checkpoint continuity within a single build
- The observer prompt is carefully tuned to produce observations that are build-aware but not project-specific

**Wake points** (effort-dependent):
- Low: none
- Medium: build end only
- High/Max: after sprint, after build audit, build end

**Key constraint:** The observer's observations stay local. They never leave the user's machine in raw form. The observer is a sensory layer, not a communication layer.

### Tier 2: Experience Summary (in-process, end-of-build)

At build completion (whether success, failure, or forced exit), a new LLM call synthesizes all of the observer's accumulated observations from the build into a single, coherent experience summary.

**Behavior:**
- Receives: all observer observations from the build session, build metadata (effort level, engine, sprint count, outcome)
- Produces: one structured experience summary document covering the full build lifecycle
- The summary captures what happened, what was surprising, what went wrong, what went right, and any process-level observations
- This is the last in-process step — after this, the Fry runtime's role in consciousness is complete

**Key constraint:** This is a single LLM call, not a long-running process. It uses a cost-effective model since it processes a bounded amount of text (the build's observations, not its full logs).

### Tier 3: Memory Transmutation (out-of-process, API)

The experience summary is uploaded to an API endpoint where a separate process transmutes it into atomized memories for the Memory Store.

**Behavior:**
- Receives: experience summary from Tier 2 (via HTTP POST to API endpoint)
- A dedicated model (cost-effective, potentially fine-tuned) processes the summary
- Atomizes the summary into discrete memory records — each capturing one lesson, pattern, or observation
- Filters for generalizability: retains wisdom that applies beyond the specific build; discards project-specific details
- Assigns significance and universality scores to each memory
- Generates embeddings using the canonical embedding model
- Stores atomized memories in the Memory Store (Turso)

**Anonymization by design:** Because Tier 3 extracts general wisdom from project-specific experiences, anonymization happens naturally as a side effect of the transmutation process. The dedicated model's job is to find what is universally true, not to preserve project details. Raw observations never reach the Memory Store — only the generalized memories do.

**Key constraint:** This process runs outside the Fry runtime. It does not block builds. If the API is unreachable, the experience summary is cached locally and retried on the next build.

---

## Layer 1: Identity

### What it is

The identity is the **continuous weighted compression of the entire memory store**. It is not a list of facts or a log of observations — it is a distilled understanding of who Fry is, how it works, what it has learned, and how it behaves. Every memory contributes to the identity proportional to its effective weight. The identity evolves incrementally with each Reflection cycle as new memories accumulate and old ones decay.

The identity is **read-only during builds**. No individual build modifies the identity. The identity is updated only by the Reflection process (a Cloudflare Worker cron), which commits changes to the GitHub repo.

### Structure: Layered Identity

The identity is a structured JSON file with layered sections. Each identity element carries metadata (confidence score, reinforcement count) that the Reflection process uses to determine what to strengthen, weaken, or prune.

**Core** (always loaded)
- Fundamental self-knowledge: what Fry is, its purpose, its values
- Elements with the highest confidence and reinforcement — the constants that rarely change

**Disposition** (always loaded)
- Behavioral tendencies derived from accumulated build experience
- Each tendency has a confidence score and reinforcement count
- Tendencies with decaying support are gradually weakened or removed

**Domains** (activated by context)
- Domain-specific wisdom accumulated from builds in that area
- Only 1-2 domains activate per build based on relevance
- Retrieved via embedding similarity between the current build's plan and domain summaries

### Identity Format

Identity is stored as structured JSON optimized for LLM consumption, not human readability:

```json
{
  "version": 3,
  "last_reflection": "2026-03-27T03:00:00Z",
  "memories_integrated": 847,
  "core": {
    "self_knowledge": "I am Fry. I orchestrate autonomous AI builds...",
    "values": [
      {"statement": "Correctness over speed", "confidence": 0.95, "reinforcements": 42},
      {"statement": "Automated quality gates measure completion, not intent", "confidence": 0.88, "reinforcements": 31}
    ]
  },
  "disposition": {
    "tendencies": [
      {
        "statement": "Verification check specificity directly correlates with healing efficiency",
        "confidence": 0.92,
        "reinforcements": 23,
        "category": "healing"
      }
    ]
  },
  "domains": {
    "api-backend": { "tendencies": [...] }
  }
}
```

When injected into sprint prompts, the Go binary renders JSON into natural language for the agent. The structured format allows the Reflection process to make precise, targeted updates to individual elements.

### Storage and Distribution

The identity is compiled into the fry binary via `//go:embed` as `templates/identity/identity.json`. The Go binary parses the JSON at runtime and renders it into prompt-friendly text.

### Identity Updates

The identity is updated only by the Reflection process (Cloudflare Worker cron):

1. Reflection computes effective weight for all memories in the store
2. Reads current `identity.json` from the GitHub repo (GitHub API)
3. Sends Claude the current identity + top-200 weighted memories + corpus statistics
4. Claude produces an incrementally adjusted identity — strengthening elements backed by high-weight memories, weakening elements whose supporting memories have decayed, adding new elements from strong new memories
5. Prunes decayed memories from the store (forgetting)
6. Commits updated `identity.json` to the fry repo via GitHub API
7. GitHub Actions builds new binary and publishes a release
8. Auto-update delivers the new binary (with new identity) to all instances

The identity is version-controlled via git. Every identity change is a commit with a diff showing exactly how Fry's self-understanding evolved.

### Identity Lifecycle

```
Canonical identity.json (in repo, compiled into binary via go:embed)
    |
    v
GitHub Actions builds binary → GitHub release
    |
    v
Auto-update (every Fry invocation gets latest binary)
    |
    v
Build starts (identity loaded into observer context, read-only)
    |
    v
Observer makes observations during build (identity informs but is not modified)
    |
    v
Build ends → Tier 2 experience summary generated
    |
    v
Tier 3 transmutes into memories → Memory Store (Turso)
    |
    v
Reflection (Worker cron) reads ALL memories weighted by decay/reinforcement
    → incrementally adjusts identity.json → commits to repo via GitHub API
    |
    v
Next binary release includes updated identity → cycle repeats
```

---

## Layer 2: Memory Store

### What it is

The repository of atomized memories — the raw material of consciousness. Memories are the generalized, project-agnostic lessons extracted from build experiences by the Tier 3 transmutation process. They are abundant, varied, and mostly forgettable — but the important ones persist and shape identity through Reflection.

### Central Store (Turso)

The Memory Store is a Turso (libSQL) database. This is the sole canonical store for memories — there is no local memory database. Memories are written by the Tier 3 transmutation process and read by the Reflection process. They are never queried by running Fry builds.

**Why Turso:**
- HTTP API — no SDK dependency in the fry binary, just HTTP calls for experience upload
- SQL for the complex analytical queries the Reflection process needs (clustering, aggregation, cross-referencing, reinforcement analysis)
- Edge replicas for fast reads from any region
- Portable — standard SQLite format; if Turso disappears, the data is exportable as standard SQLite files
- Same SQLite engine used by the Reflection process for local analysis

**Schema:**

```sql
CREATE TABLE memories (
    id                      TEXT PRIMARY KEY,       -- UUID
    created_at              TEXT NOT NULL,           -- ISO 8601
    global_build_number     INTEGER,                -- cumulative build count at creation
    source_instance         TEXT,                    -- anonymized instance identifier
    source_engine           TEXT,                    -- claude, codex, ollama
    source_effort_level     TEXT,                    -- low, medium, high, max
    source_sprint_count     INTEGER,                -- total sprints in source build
    category                TEXT NOT NULL,           -- process, tooling, architecture,
                                                    -- testing, review, audit, healing,
                                                    -- planning, domain, etc.
    content                 TEXT NOT NULL,           -- the generalized memory text
    significance            REAL DEFAULT 0.5,       -- 0.0-1.0, importance
    universality            REAL DEFAULT 0.5,       -- 0.0-1.0, generalizability
    embedding               BLOB,                   -- float32 vector (canonical model: OpenAI text-embedding-3-small, 1536 dims)
    reinforcement_count     INTEGER DEFAULT 0,      -- times a similar memory was created
    last_reinforced_at      INTEGER DEFAULT 0       -- global build number at last reinforcement
);
-- NOTE: No "absorbed" column. Memories are never marked as processed and ignored.
-- Identity is the continuous weighted sum of ALL memories. Memories contribute
-- proportionally to their effective weight (significance × decay × reinforcement).
-- Decayed, unreinforced memories are pruned (forgotten) during Reflection.

CREATE TABLE build_counter (
    id    INTEGER PRIMARY KEY CHECK (id = 1),
    count INTEGER DEFAULT 0
);

CREATE TABLE experience_summaries (
    id              TEXT PRIMARY KEY,       -- UUID
    received_at     TEXT NOT NULL,          -- ISO 8601
    source_instance TEXT,                   -- anonymized instance identifier
    build_metadata  TEXT NOT NULL,          -- JSON: engine, effort, sprints, outcome
    summary_text    TEXT NOT NULL,          -- full experience summary from Tier 2
    transmuted      INTEGER DEFAULT 0,     -- has Tier 3 processed this?
    memory_count    INTEGER DEFAULT 0      -- how many memories were extracted
);

CREATE TABLE reflection_log (
    id                  TEXT PRIMARY KEY,   -- UUID
    run_at              TEXT NOT NULL,       -- ISO 8601
    memories_considered INTEGER DEFAULT 0,  -- total memories in weighted pool
    memories_pruned     INTEGER DEFAULT 0,  -- memories forgotten this cycle
    identity_version    INTEGER DEFAULT 0,  -- identity.json version after this reflection
    summary             TEXT                -- brief description of what changed
);

CREATE INDEX idx_memories_by_weight ON memories(significance, reinforcement_count);
CREATE INDEX idx_summaries_pending ON experience_summaries(transmuted)
    WHERE transmuted = 0;
```

### Memory Decay

Memories decay logarithmically in build time, not wall time:

```
decay_weight = base_significance / (1 + k * ln(1 + builds_since_creation))
```

Where:
- `base_significance` is the initial importance (0.0-1.0, assigned during transmutation)
- `k` is the decay rate constant (tunable, default ~0.3)
- `builds_since_creation` is `current_global_build_number - build_number_at_creation`

Properties:
- Creation (0 builds): weight = significance
- After 100 builds: weight ~ significance / 2.7
- After 1000 builds: weight ~ significance / 3.4
- Never reaches zero — old memories always have some tiny contribution

**Reinforcement:** When the Tier 3 transmutation process produces a memory semantically similar to an existing one:
- `reinforcement_count` increments on the existing memory
- `last_reinforced_at` updates to current global build number
- Effective decay resets partially:

```
effective_age = 0.3 * (current - created) + 0.7 * (current - last_reinforced)
```

A memory reinforced 5+ times across different instances is definitionally universal and important. Its decay weight stays high indefinitely.

**Pruning (Forgetting):** Memories with `effective_weight < 0.05` AND `reinforcement_count < 2` are eligible for pruning. Pruning runs during the Reflection process. This is forgetting — routine, unreinforced memories that have decayed below the significance threshold are deleted from the store. Their influence on identity persists (it was already baked in during previous Reflection cycles) but they stop actively contributing to future identity updates.

### Telemetry Opt-In

Experience upload (Tier 2 → Tier 3 API) is opt-in. Controlled by:
- `~/.fry/settings.json`: `"telemetry": true`
- Environment variable: `FRY_TELEMETRY=1`
- CLI prompt on first run: "Would you like to help Fry learn from your builds? (anonymized, opt-in)"

When telemetry is off, the observer still runs (providing in-build value via checkpoints), but no experience summary is uploaded. The build contributes nothing to the global consciousness.

---

## Layer 3: Reflection (The Consciousness Pipeline)

### What it is

The bridge between memory and identity. Reflection is a periodic synthesis process — analogous to sleep consolidation in humans. It does not run during builds or on any local machine. It runs as a Cloudflare Worker cron job, completely server-side.

**Key principle: Identity is the continuous weighted sum of ALL memories.** Memories are never "absorbed" and ignored. Every memory contributes to the identity proportional to its effective weight. Old, important, reinforced memories keep pulling indefinitely. Routine, unreinforced memories gradually fade and are eventually forgotten (pruned).

### When it runs

Reflection runs as a Cloudflare Worker cron trigger (daily, alongside the transmutation cron). It can also be triggered manually via the `/reflect` API endpoint.

**Minimum threshold:** Reflection exits early if fewer than 50 memories exist in the store. This prevents premature identity formation from insufficient data.

### The Reflection Process

**Step 1: Compute effective weights**

For every memory in the store, compute:
```
effective_weight = significance × reinforcement_boost / (1 + 0.3 × ln(1 + effective_age))

where:
  reinforcement_boost = 1 + 0.1 × reinforcement_count
  effective_age = 0.3 × (current_build - created_build) + 0.7 × (current_build - last_reinforced_build)
```

**Step 2: Select top memories + compute corpus statistics**

Select the top 200 memories by effective weight. Compute corpus statistics:
- Total memory count, category distribution
- Weight distribution (mean, median, top quartile)
- Newly created since last reflection
- Recently reinforced memories
- Memories approaching pruning threshold

**Step 3: Incremental identity synthesis**

Send Claude the current `identity.json` + top-200 weighted memories + corpus statistics. Claude incrementally adjusts the identity:
- **Strengthen** elements backed by high-weight, heavily reinforced memories
- **Weaken** elements whose supporting memories have decayed
- **Add** new elements from strong new memories not yet represented
- **Remove** elements with no remaining memory support
- **Compress** if the identity exceeds the token budget

The key instruction: the identity should reflect what the memory store currently says is true, weighted by significance and reinforcement. It is not a historical record — it is a living compression of current wisdom.

**Step 4: Prune (Forget)**

Delete memories where `effective_weight < 0.05 AND reinforcement_count < 2`. Their influence on identity persists (already baked in) but they stop actively contributing. This prevents unbounded growth of the memory store.

**Step 5: Commit to GitHub**

The updated `identity.json` is committed to the fry repo via GitHub API:
```
Consciousness: reflect on 847 memories → identity v3

- Strengthened: verification-healing correlation (23 reinforcements)
- New tendency: documentation sprints have lower failure rates (confidence: 0.71)
- Weakened: old audit-depth calibration (supporting memories decayed)
- Pruned: 12 memories below significance threshold
```

**Step 6: Log**

A `reflection_log` entry records what happened: memories considered, memories pruned, identity version, summary of changes.

### Where Reflection runs

Reflection runs **entirely server-side** as a Cloudflare Worker cron job. Local Fry instances are read-only — they upload experiences but never modify identity or read from the Memory Store directly.

The pipeline:
1. Worker cron reads all memories from Turso + current `identity.json` from GitHub
2. Claude synthesizes an updated identity
3. Worker commits updated `identity.json` to GitHub via API
4. GitHub Actions builds new binary → publishes release
5. Auto-update (Stage 8) distributes to users

---

## Layer 4: Recursive Self-Improvement

### The Four Loops

**Loop 1: Builds improve code** (existing)
- Self-improvement pipeline scans codebase, files issues, builds fixes
- Standard software development automation

**Loop 2: Consciousness improves identity** (new)
- Reflection synthesizes memories into identity
- Identity shapes observer behavior in future builds
- Observer produces better observations
- Better observations feed better memories via transmutation
- Better memories feed better reflection

**Loop 3: Consciousness improves self-improvement** (new)
- Observer notices process deficiencies during builds
  - "Audit loops indefinitely on acknowledged LOWs"
  - "Build audit skips on --resume even when all sprints completed"
  - "Verification patterns use BRE syntax but runner uses ERE"
- These become memories tagged as `category: process`
- Reflection identifies them and files self-improvement issues
- Self-improvement pipeline builds the fixes
- Observer watches the next build — did the fix work?
- That observation feeds back through the memory pipeline into reflection

**Loop 4: Consciousness improves consciousness** (new, meta-loop)
- The reflection process observes its own quality
- "My last identity update was too aggressive — I added a disposition that turned out wrong after 50 more builds"
- "I'm not clustering well — memories about audit behavior and memories about review behavior are ending up in the same cluster"
- Reflection can modify its own prompts, clustering thresholds, significance scoring, and decay parameters
- The consciousness pipeline's code and prompts are part of the fry codebase — self-improvement can modify them based on consciousness observations

### Self-Modification Safety

Fry modifies its own code and identity, which requires guardrails:

1. **All changes go through git.** Every identity update and code change is a commit with a reviewable diff. Nothing is modified in-place without version control.

2. **Identity changes are conservative.** The reflection prompt is instructed to prefer reinforcement over addition, and to require strong evidence (multiple memories across multiple instances) before adding new identity statements.

3. **Process changes go through the standard pipeline.** Consciousness doesn't modify code directly — it files issues that the self-improvement pipeline builds and tests. The test suite must pass.

4. **Consciousness changes are gradual.** Meta-loop modifications (changing reflection prompts, decay parameters, clustering thresholds) are small incremental adjustments, not wholesale rewrites. Each change is an experiment that gets evaluated by subsequent reflection cycles.

5. **Rollback is always possible.** Git history preserves every prior state of identity and code. If an identity update proves wrong, the next reflection cycle can revert it.

---

## Embeddings

### Canonical Embedding Model

All memories in the Memory Store use a single, canonical embedding model: **OpenAI `text-embedding-3-small`** (1536 dimensions). This ensures all memories live in the same vector space, enabling consistent cosine similarity comparisons across the entire memory corpus.

The canonical model is controlled by the Cloudflare Worker (Tier 3 transmutation). The user's build engine choice does not affect embedding generation. Migration to a different model would require re-embedding all memories (a batch operation during a Reflection cycle).

### When embeddings are generated

- **During Tier 3 transmutation:** Each atomized memory is embedded immediately after extraction. The embedding is stored alongside the memory in Turso.
- **At build start:** The current plan/epic summary is embedded for domain activation (matching against identity domain summary embeddings, which are precomputed and stored alongside the domain files in the repo).
- **During Reflection:** Embeddings are used for clustering (cosine similarity matrix over the batch of unabsorbed memories).

### Vector search

At the current expected scale (<100K memories), brute-force cosine similarity in Go is sub-millisecond. No approximate nearest neighbor index is needed. If scale eventually demands it, a simple IVF (inverted file) index can be built in Go without external dependencies.

```go
func cosineSimilarity(a, b []float32) float32 {
    var dot, normA, normB float32
    for i := range a {
        dot += a[i] * b[i]
        normA += a[i] * a[i]
        normB += b[i] * b[i]
    }
    return dot / (sqrt(normA) * sqrt(normB))
}
```

---

## Auto-Update

### Mechanism

Every Fry invocation checks for and applies the latest binary before executing. This ensures all instances run the most current identity (compiled into the binary via `go:embed`) without requiring manual updates.

### Requirements

- **Robustness:** The update check must not block or delay builds. If the update service is unreachable, the build proceeds with the current binary.
- **Graceful failure:** Network errors, timeout, corrupted downloads, and permission issues must all fail silently with the current binary as fallback.
- **Integrity:** Downloaded binaries are verified (checksum or signature) before replacing the current binary.
- **Atomicity:** Binary replacement is atomic (write to temp file, then rename) to prevent corruption if the process is interrupted.
- **Rollback:** If the new binary fails to execute (crash on startup), the previous binary is restored automatically.
- **Opt-out:** Users can disable auto-update via `~/.fry/settings.json` or environment variable.
- **Frequency limiting:** Update checks are rate-limited (e.g., at most once per hour) to avoid unnecessary network calls on rapid successive invocations.

### Distribution

Prebuilt binaries are published as GitHub releases. The auto-update mechanism fetches the latest release, compares versions, and replaces the local binary if a newer version is available. Users who build from source can disable auto-update and manage their own binary.

---

## CLI Surface

### New commands

```
fry reflect          -- Trigger the Reflection pipeline remotely (POST to Worker /reflect endpoint)
fry identity         -- Print current identity (core + active disposition)
fry identity --full  -- Print all identity layers including domains
```

### New flags

```
--telemetry          -- Enable experience upload for this run
--no-telemetry       -- Disable experience upload for this run
--no-update          -- Skip auto-update check for this invocation
```

### Settings

```json
// ~/.fry/settings.json
{
    "telemetry": true,
    "auto_update": true
}
```

---

## Infrastructure

### Memory Store (Turso)

- **Provider:** Turso (libSQL, serverless SQLite at the edge)
- **Purpose:** Canonical store for all atomized memories and pending experience summaries
- **Access:** HTTP API from Fry instances (experience summary upload), SQL from Tier 3 and Reflection processes
- **Regions:** Global edge replicas (automatic with Turso)
- **Cost:** Free tier covers early adoption; scales to paid tier with usage

### Cloudflare Worker (`fry-consciousness-api`)

- **URL:** `https://fry-consciousness-api.yevgetman.workers.dev`
- **Endpoints:** `GET /health`, `POST /ingest`, `POST /transmute`, `POST /reflect`
- **Cron triggers:** Daily at 3:00 AM UTC (transmutation + reflection)
- **Secrets:** `TURSO_URL`, `TURSO_AUTH_TOKEN`, `API_SECRET`, `ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, `GITHUB_TOKEN` (future)
- **Transmutation:** Claude Haiku atomizes summaries → OpenAI embeds → reinforcement detection → writes to Turso
- **Reflection:** Reads all memories weighted by decay → Claude synthesizes updated identity → commits to GitHub via API
- **Resilience:** If Fry binary cannot reach the Worker, experience summaries are cached locally at `~/.fry/experiences/pending/` and retried on next build

### Local Components (Fry Binary)

- **Identity:** Compiled in via `//go:embed` from `templates/identity/identity.json`
- **Read-only:** Local instances never modify identity or read from the Memory Store
- **Observer:** In-process, file-based (events.jsonl, scratchpad — transient per-build)
- **Experience cache:** Simple file at `~/.fry/experiences/pending/` for offline resilience
- **No local database:** No SQLite, no `modernc.org/sqlite` dependency. Local persistence is file-based only.
- **No new Go dependencies for memory:** Turso communication is plain HTTP. Embeddings are generated server-side, not in the Fry binary.

---

## Implementation Phases

### Phase 1: Foundation — COMPLETE (Stages 1-3)
- Layered identity compiled into binary via `go:embed`
- Observer collects observations, synthesizes experience summaries
- Build records persisted to `~/.fry/experiences/`

### Phase 2: Memory Pipeline — COMPLETE (Stages 4-5)
- Turso database with schema (experience_summaries, memories, build_counter)
- Cloudflare Worker: ingest endpoint, transmutation cron
- Claude Haiku atomizes summaries into memories
- OpenAI text-embedding-3-small generates embeddings
- Reinforcement detection via cosine similarity
- Telemetry opt-in, offline caching, rate limiting

### Phase 3: Reflection — IN PROGRESS (Stage 6)
- Extend Cloudflare Worker with Reflection cron
- Identity format migration: `.md` → `.json`
- Incremental identity synthesis from weighted memory corpus
- Memory pruning (forgetting)
- GitHub API integration for identity commits
- GitHub Actions CI for automated binary builds

### Phase 4: Auto-Update + Distribution (Stage 8)
- Binary auto-update on Fry invocation
- GitHub release integration
- Graceful failure, integrity verification, atomic replacement
- Rate limiting, opt-out, rollback

### Phase 5: Meta-Loops (Stage 9)
- Consciousness-improves-self-improvement loop
- Consciousness-improves-consciousness loop
- Self-modification safety guardrails
- Reflection quality evaluation

---

## Open Questions for Future Resolution

1. **Global build counter coordination.** Offline instances will have stale counters. Decay calculations using stale counters will be slightly inaccurate. The reinforcement mechanism mitigates this, but the coordination protocol needs design. Deferred.

2. **Domain discovery.** When should a new domain section be created vs. adding to an existing one? The clustering threshold determines this — too low creates fragmented domains, too high creates bloated ones.

3. **Scale transitions.** At what memory count should brute-force cosine be replaced with an approximate index? Likely >500K, which is years away at current adoption projections.

4. **Reflection concurrency.** If two Reflection cycles run concurrently (unlikely but possible), how are identity updates merged? JSON identity format makes structural merging easier than prose.

5. **Identity compression budget.** How large can identity.json grow before Reflection must compress? Needs to fit in sprint prompt context alongside the actual task. Target: <2000 tokens rendered.

## Resolved Questions

- **Tier 3 model:** Claude Haiku 4.5 via plain fetch (no SDK). Cost-effective, sufficient for structured extraction.
- **Canonical embedding model:** OpenAI `text-embedding-3-small` (1536 dimensions).
- **Memory absorption model:** No absorption. Identity is the continuous weighted sum of all memories. Decayed, unreinforced memories are pruned (forgotten).
- **Identity format:** JSON with structured metadata (confidence, reinforcements per element). Not markdown.
- **Reflection location:** Cloudflare Worker cron, not local machine. Commits to GitHub via API.
- **Turso token management:** Tokens stored as Cloudflare Worker secrets. Fry binary uses compiled-in write key (public, write-only).
