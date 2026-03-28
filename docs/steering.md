# Build Steering

Build steering lets you interact with a running Fry build through natural language via the OpenClaw extension. You can inject directives, pause for review, or abort and redirect -- without restarting the build from scratch.

## The Three Tiers

Steering operates at three natural seams in Fry's execution flow, each with a different risk profile.

### Tier A: Whisper (between iterations)

Inject a note into the next iteration's prompt. The build doesn't stop. The sprint agent sees your directive as an additional prompt section alongside its regular instructions.

**When to use**: "Focus on the API endpoints", "Don't forget to add rate limiting", "Use PostgreSQL not SQLite"

**How it works**:
1. You send a message via any OpenClaw channel
2. The extension writes your directive to `.fry/agent-directive.md`
3. At the start of the next iteration, the sprint loop reads the file (atomic consume to prevent races)
4. Your directive is injected as a "MID-BUILD USER DIRECTIVE" prompt section
5. The file is deleted after reading
6. A `directive_received` event is emitted

**Risk**: Near zero. The agent may incorporate the directive poorly for one iteration; verification catches bad output. No structural changes to the epic or sprint definitions.

### Tier B: Hold at Sprint Boundary (between sprints)

Pause after the current sprint completes. Review what was done and decide how to proceed.

**When to use**: "Hold after this sprint, I want to review", "Pause before the next sprint"

**How it works**:
1. You send a hold request via any OpenClaw channel
2. The extension writes an empty sentinel file `.fry/agent-hold-after-sprint`
3. After the current sprint completes (verified, healed, audited, checkpointed, compacted), the inter-sprint loop detects the hold
4. A `decision_needed` event is emitted with a summary of the completed sprint
5. The build blocks, waiting for your response
6. You respond with one of:
   - `"continue"` — proceed to the next sprint as planned
   - A directive — injected as user context for the next sprint
   - `"replan: <instructions>"` — replan the remaining sprints using Fry's existing review/replan system
7. A `decision_received` event is emitted and the build continues

**Risk**: Low. The sprint boundary is a clean seam. All work is checkpointed via git. Replan uses Fry's existing `internal/review/` system, which accounts for completed work.

### Tier C: Abort (anytime)

Stop the build gracefully. The current iteration finishes, work is committed, and the build exits cleanly.

**When to use**: "Stop", "This is going in the wrong direction", "Pause the build"

**How it works**:
1. You send a pause request via any OpenClaw channel
2. The extension writes an empty sentinel file `.fry/agent-pause`
3. At the end of the current iteration, the sprint loop detects the pause
4. Work is git-checkpointed
5. A `build_paused` event is emitted
6. The build process exits cleanly with a "paused" exit reason
7. Resume with `fry_build_restart` (which calls `fry --continue`), optionally providing new direction via `user_prompt`

**Risk**: Low. This is the designed recovery path. The `--continue` mechanism was built to handle partial builds. It collects build state, analyzes it via LLM, and determines where to resume.

## File-Based IPC

Steering uses files in the `.fry/` directory for communication between the extension and the sprint loop. No network protocols, no shared memory -- just files.

| File | Purpose | Written By | Read By |
|------|---------|-----------|---------|
| `.fry/agent-directive.md` | User directive for the next iteration | Extension | Sprint loop (consumed atomically) |
| `.fry/agent-hold-after-sprint` | Hold sentinel (empty file) | Extension | Inter-sprint loop |
| `.fry/agent-pause` | Pause sentinel (empty file) | Extension | Sprint loop |
| `.fry/decision-needed.md` | Build waiting for human input (contains prompt) | Sprint loop | Extension |

All steering files are cleaned up automatically when the build completes (success or failure) to prevent stale files from affecting the next run.

## Prompt Injection

When a directive is consumed by the sprint loop, it's injected into the prompt assembly as Layer 1.7 (between the disposition layer and the quality directive layer):

```markdown
# ===== MID-BUILD USER DIRECTIVE =====
# The user has sent the following directive during this build. Incorporate it
# into your work for this iteration. This takes priority over earlier instructions
# where they conflict.

[directive content]
```

## Events

Steering emits structured events to `.fry/observer/events.jsonl`:

| Event | When | Data |
|-------|------|------|
| `directive_received` | Sprint loop consumed a directive | `preview` (first 200 chars) |
| `decision_needed` | Build holding for user decision | `reason`, `completed_sprint`, `remaining_sprints` |
| `decision_received` | User responded to a hold | `preview` (first 200 chars) |
| `build_paused` | Build stopped after iteration | `sprint`, `iteration` |

The OpenClaw extension's build watcher translates these events into natural-language notifications sent to whatever messaging channel you're using.

## Atomicity and Race Safety

Directive consumption uses an atomic rename-read-delete pattern (`ConsumeDirective`) to prevent TOCTOU races. If a new directive is written between the rename and the read, it creates a new file at the original path -- neither the old nor the new directive is lost.

The hold and pause sentinels are empty files where atomicity is less critical (the check is existence-based, not content-based).

## Cleanup

All four steering files are removed by `steering.CleanupAll()` when the build completes. This is called before the `build_end` event is emitted, ensuring a clean state for the next run.
