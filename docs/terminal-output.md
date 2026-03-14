# Terminal Output

Fry provides status output at every phase so you always know what it's doing, even without `--verbose`. The `--verbose` flag adds the full agent transcript to the terminal; without it, you see progress banners and status lines only.

## Output Modes

| Mode | Behavior |
|---|---|
| Default | Status banners and progress lines at every phase; full output goes to log files only |
| `--verbose` | Everything above, plus full agent transcript streamed to terminal |

## Prepare Phase

Each generation step prints a start and completion message:

```
[2026-03-10 11:50:00] Step 0: Generating plans/plan.md from plans/executive.md (engine: claude)...
[2026-03-10 11:51:12] Generated plans/plan.md.
[2026-03-10 11:51:12] Step 1: Generating .fry/AGENTS.md (engine: claude)...
[2026-03-10 11:52:05] Generated .fry/AGENTS.md.
[2026-03-10 11:52:05] Step 2: Generating .fry/epic.md (engine: claude)...
[2026-03-10 11:53:30] Generated .fry/epic.md.
[2026-03-10 11:53:30] Step 3: Generating .fry/verification.md (engine: claude)...
[2026-03-10 11:54:15] Generated .fry/verification.md.
```

## Preflight

```
[2026-03-10 11:54:16] Preflight checks passed.
```

## Sprint Execution

Each sprint gets a start banner, per-iteration agent banners, verification results, and a completion line:

```
[2026-03-10 12:00:00] =========================================
[2026-03-10 12:00:00] STARTING SPRINT 3: Auth & Permissions
[2026-03-10 12:00:00] Max iterations: 25
[2026-03-10 12:00:00] =========================================
[2026-03-10 12:00:01] ▶ AGENT  sprint 3/8 "Auth & Permissions"  iter 1/25  engine=claude  model=default
[2026-03-10 12:05:12] ▶ AGENT  sprint 3/8 "Auth & Permissions"  iter 2/25  engine=claude  model=default
[2026-03-10 12:10:30] Running verification checks...
[2026-03-10 12:10:35] Verification: 4/4 checks passed.
[2026-03-10 12:10:35] SPRINT 3 PASS (2m35s)
```

## Self-Healing

```
[2026-03-10 12:10:30] ▶ AGENT  sprint 3/8 "Auth & Permissions"  heal 1/3  engine=claude  model=default
[2026-03-10 12:12:00] Re-running verification after heal attempt 1...
[2026-03-10 12:12:05] Heal attempt 1 SUCCEEDED — all checks now pass.
```

## Sprint Audit

Sprint audits run by default after each sprint passes verification:

```
[2026-03-10 12:10:36] ▶ AUDIT  sprint 3/8 "Auth & Permissions"  pass 1/3  engine=claude
[2026-03-10 12:12:00]   AUDIT: pass (max severity: none)
```

When issues are found and remediated:

```
[2026-03-10 12:10:36] ▶ AUDIT  sprint 3/8 "Auth & Permissions"  pass 1/3  engine=claude
[2026-03-10 12:12:00]   AUDIT: HIGH issues found — running fix agent...
[2026-03-10 12:14:30] ▶ AUDIT  sprint 3/8 "Auth & Permissions"  pass 2/3  engine=claude
[2026-03-10 12:16:00]   AUDIT: pass (max severity: LOW)
```

When issues persist after all passes (advisory, non-blocking):

```
[2026-03-10 12:20:00]   AUDIT: MODERATE issues remain after 3 passes (advisory)
```

## Build Audit

After all sprints complete successfully, a final holistic audit runs on the entire codebase:

```
[2026-03-10 13:00:00] > BUILD AUDIT  running final holistic audit for "My Project"
[2026-03-10 13:15:00]   BUILD AUDIT: report written to audit.md
```

If the agent does not produce a report:

```
[2026-03-10 13:15:00]   BUILD AUDIT: WARNING -- agent did not produce audit.md
```

## Sprint Review and Replan

```
[2026-03-10 12:15:00] --- SPRINT REVIEW: after Sprint 3 ---
[2026-03-10 12:16:30] Review verdict: CONTINUE
```

Or when a deviation is needed:

```
[2026-03-10 12:15:00] --- SPRINT REVIEW: after Sprint 3 ---
[2026-03-10 12:16:30] Review verdict: DEVIATE
[2026-03-10 12:16:31] Running replanner agent...
[2026-03-10 12:18:00] Epic updated successfully.
```

## Compaction

When `@compact_with_agent` is enabled:

```
[2026-03-10 12:18:01] Compacting sprint progress with agent...
```

## Build Summary

After all sprints complete, Fry prints a summary table with the status of each sprint.

## Log Format

All log lines follow the format:
```
[YYYY-MM-DD HH:MM:SS] message
```

Logging is thread-safe and dual-output: messages go to stdout and optionally to log files.
