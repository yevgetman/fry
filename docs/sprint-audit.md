# Sprint Audit

The sprint audit is a semantic quality gate that runs by default after each sprint passes sanity checks. It uses a two-level loop: an outer audit cycle discovers issues, and an inner fix loop resolves them before re-auditing. Issues are tracked individually across cycles and prioritized FIFO (oldest first). The prompt adapts to sprint complexity, intentional deviations, and prior failed fix attempts so the loop spends less time rediscovering the same problems. This complements the syntactic sanity check system (`@check_file`, `@check_cmd`, etc.) with deeper, AI-driven review.

## How It Works

```
Sprint passes sanity checks
       │
       ▼
  Outer audit cycle 1
       │
       ├─ Audit agent reviews work (read-only)
       │       │
       │       ├─ PASS (no issues or all LOW) → continue to git checkpoint
       │       │
       │       └─ FAIL (CRITICAL/HIGH/MODERATE found)
       │               │
       │               ▼
       │         Inner fix loop (FIFO order, oldest issues first)
       │               │
       │               ├─ Fix agent remediates listed issues
       │               ├─ Verify agent checks which issues are resolved
       │               ├─ If all resolved → break inner loop
       │               ├─ If stale (no new resolutions) → break inner loop
       │               └─ Repeat up to inner cap
       │
       ▼
  Outer audit cycle 2 (re-audit)
       │
       ├─ Verifies previous issues are resolved
       ├─ Discovers any NEW issues introduced by fixes
       │       │
       │       ├─ All resolved, no new → PASS
       │       └─ Issues remain or new found → inner fix loop again
       │
       └─ Repeat until PASS, iterations exhausted, stale, or low-yield stop
               │
               ├─ CRITICAL or HIGH → sprint FAILS, epic stops
               └─ MODERATE → advisory warning, build continues
```

The audit runs **after** sanity checks pass but **before** the git checkpoint, so that the checkpoint commits both the sprint's work and any audit fixes in one clean commit.

## Two-Level Loop Design

The audit uses a two-level loop with three types of agent sessions:

### Outer loop (audit cycles)

Each outer cycle runs the **audit agent** to review the codebase. On cycle 2+, the audit prompt includes previously identified issues and asks the agent to verify each prior issue while also scanning for new issues.

### Inner loop (fix iterations per cycle)

For each audit report, the **fix agent** runs repeatedly until all issues above LOW severity from that report are resolved:

1. **Fix agent** -- reads the structured issue list (FIFO ordered, oldest first) and makes minimal code changes. It is instructed to focus exclusively on the listed issues and not search for new ones.
2. **Verify agent** -- checks whether the specific issues have been resolved without modifying code. Reports each issue with a concrete outcome such as `RESOLVED`, `PARTIALLY_RESOLVED`, `BEHAVIOR_UNCHANGED`, `EVIDENCE_INCONCLUSIVE`, `BLOCKED`, or `STILL_PRESENT`.
3. If all actionable issues are resolved, the inner loop breaks and triggers a re-audit.
4. If no new issues are resolved for 2 consecutive fix iterations, the inner loop breaks (stale detection).

### Inner-loop efficiency

The inner loop carries forward structured history so each fix iteration has more context than the last:

- **No-op detection** -- Fry fingerprints the worktree before and after each fix pass. If the fix agent made no material file changes (excluding progress artifacts), Fry logs a no-op, skips verify, increments the stale counter, and moves directly to the next fix attempt or re-audit.
- **Fix contract validation** -- Every fix pass carries a Fry-owned diff contract: numbered issue IDs, declared target files derived from the findings, and expected evidence. Empty diffs, comment-only diffs, and out-of-scope diffs are rejected before they count as real remediation attempts.
- **Already-fixed verification routing** -- If the fix agent claims an issue is already fixed and produces no behavioral diff, Fry routes that claim to verify instead of counting the pass as a normal remediation attempt.
- **Behavior-unchanged verification** -- Verify can explicitly report `BEHAVIOR_UNCHANGED` when a remediation left the executable logic path untouched. The next fix prompt then carries issue-specific guidance with the unchanged path summary and a direct instruction not to answer with comments, rationale-only edits, or other non-behavioral changes.
- **Fix attempt history** -- The fix prompt includes concise summaries of prior attempts that targeted the same findings, including whether the attempt was a no-op, which issues remained, and any verification notes.
- **Strategy escalation** -- If the same finding comes back `BEHAVIOR_UNCHANGED` repeatedly, Fry narrows the next fix batch to the stuck findings, refreshes fix-session context, and can stop the current fix loop early instead of grinding through more low-yield retries.
- **Yield-aware mode shifts** -- Fry records per-cycle productivity (fix yield, verify yield, no-op rate, and milliseconds per resolved finding). When progress-based cycles keep spending calls with weak returns, Fry refreshes audit context and runs the next cycle in single-issue mode before deciding whether to stop early.
- **Explicit verify boundary** -- Verify sessions stay stateless and never inherit fix-session context. This keeps the trust boundary between remediation and validation intact.

On Claude and Codex, Fry also reuses same-role session continuity within the sprint audit:

- **Audit continuity** -- outer audit cycle 1, cycle 2, and the final audit pass reuse the same audit session when possible.
- **Fix continuity** -- fix iteration 1, 2, 3, and so on within one outer cycle reuse the same fix session.
- **Verify isolation** -- verify never resumes the fix session.
- **Budgeted refresh** -- audit and fix continuity are capped by per-role call, prompt-size, token, and carry-forward budgets. When a session exceeds its budget, Fry clears the stored session ID and starts the next same-role call from a fresh session.
- **Carry-forward summary** -- refreshed sessions receive a compact summary that explains why the refresh happened, lists the unresolved findings being carried forward, and includes recent failed fix attempts relevant to those findings.

### Issue tracking across cycles

Each finding has a stable identity and tracks:
- **Finding key** -- normalized file location plus description when a location is available; otherwise normalized description only
- **Affected files** -- file targets derived from the finding location
- **Artifact state** -- a lightweight fingerprint of the relevant file content or local code window
- **Origin cycle** -- which outer audit cycle discovered it (for FIFO ordering)
- **Last seen cycle** -- the most recent audit cycle that observed the finding family
- **Resolution status** -- whether it has been verified as resolved

When the re-audit runs (cycle 2+), findings are classified as:
- **Resolved** -- previously known issue no longer found
- **Persisting** -- previously known issue still present (keeps its original cycle number)
- **New** -- issue not seen in previous cycles (assigned current cycle number)
- **Repeated unchanged** -- same issue family against unchanged code; merged back into the existing active issue instead of re-queued as brand-new work

This ensures older issues are always addressed before newer ones, avoids collapsing distinct same-worded issues in different files, and reduces wasted fix effort on re-discovering known issues.

### Blocker categories

Sprint audit also distinguishes between broken product work and blocked execution prerequisites. Findings can be classified as:

- `product_defect`
- `environment_blocker`
- `harness_blocker`
- `external_dependency_blocker`

Blocker findings are still tracked and can stop the sprint, but Fry does **not** send them through the normal code-fix loop. Instead, the audit result preserves the blocker details so the build can report that the sprint is blocked rather than broken.

### Reopen detection

When the audit agent re-raises a previously resolved requirement theme under different wording, Fry detects this as a **probable reopening** rather than treating it as a new finding. This prevents the audit loop from churning on the same requirement family when the agent shifts its interpretation between cycles.

Reopen detection uses fuzzy theme matching:
- **File family** -- the directory and base filename without extension or line numbers. Two findings must share the same file family (or at least one must lack a location) to be considered related.
- **Description similarity** -- significant words are extracted from the description (stop words removed), and the [Jaccard index](https://en.wikipedia.org/wiki/Jaccard_index) of the two token sets must reach 0.5 or higher.

Probable reopenings are **suppressed** from the active findings list and logged:
```
[2026-04-01 12:20:00]   AUDIT: 2 findings classified as probable reopenings (suppressed)
```

If the relevant code state is **unchanged**, a reopened finding must include explicit `**New Evidence:**` explaining the new proof or new contract interpretation. Without that field, Fry suppresses the reopening as unchanged-code churn. If the unchanged-code reopening does include `**New Evidence:**`, Fry admits it and records that separately in the audit metrics.

**Exception: genuine regressions.** If the re-raised finding has a **higher severity** than the original resolved finding and the artifact fingerprint changed, it is admitted as genuinely new. This ensures that actual regressions (where a later change made things worse) are not suppressed.

A **resolved-finding ledger** accumulates all findings resolved across audit cycles (both by the inner fix loop and by the re-audit discovering they disappeared). The ledger persists for the lifetime of one `RunAuditLoop` call.

The audit prompt also lists resolved themes on cycle 2+ and instructs the agent not to re-raise them without evidence of regression.

## Metrics and Status

Sprint audit metrics now distinguish between several kinds of churn:

- **Repeated unchanged findings** -- the auditor restated an already-known issue family against the same artifact fingerprint
- **Suppressed unchanged findings** -- a reopening against unchanged code was rejected because it lacked `**New Evidence:**`
- **Reopened with new evidence** -- a reopening against unchanged code was admitted because the auditor supplied explicit justification
- **Behavior-unchanged outcomes** -- verify concluded that the attempted remediation did not materially change the executable code path
- **Behavior-unchanged escalations** -- Fry narrowed the next fix batch, refreshed fix-session context, or stopped the current fix loop early because the same issue kept coming back behavior-unchanged
- **Session refreshes** -- Fry reset a same-role audit or fix session because it exceeded a configured continuity budget
- **Session refresh reasons** -- per-reason counts for why a refresh occurred (call budget, prompt budget, token budget, or oversized carry-forward set)
- **Cycle summaries** -- per-cycle productivity snapshots with fix yield, verify yield, no-op rate, and milliseconds per resolved finding
- **Trailing yield** -- a rolling summary across the most recent low-yield window so Fry can distinguish one bad cycle from a sustained decline
- **Low-yield strategy changes** -- count of times Fry refreshed context and changed tactics because productivity dropped
- **Low-yield stop reason** -- an explicit machine-readable reason when a progress-based audit stopped because productivity stayed low instead of converging

These counters are written to `.fry/build-logs/sprintN_audit_metrics.json` and surfaced in `.fry/build-status.json` under `sprints[].audit.metrics`. When the audit exits early for low yield, Fry also sets `sprints[].audit.stop_reason`.

## Blocking vs Advisory

After the audit loop exhausts its cycles, the outcome depends on the remaining findings:

- **Blocker findings remain** -- The sprint is marked **blocked**. Fry preserves the blocker category and details in `build-status.json` and does not spend normal fix iterations trying to patch around missing secrets, runtimes, or external services.
- **CRITICAL or HIGH** -- The sprint **fails** and the epic stops. The build summary shows `FAIL (audit: CRITICAL)` or `FAIL (audit: HIGH)`. You can resume with `fry run <epic> <sprint>` after fixing the issues.
- **MODERATE** -- The sprint **continues** with an advisory warning logged to the build output and build summary. This prevents moderate semantic disagreements between two AI agents from stalling the entire build.
- **LOW or none** -- The audit passes cleanly. At **max** effort, LOW-only findings trigger one fix agent attempt before accepting (see below). At all other effort levels, LOW-only is an immediate pass.

```
# Blocking (CRITICAL/HIGH) — exact severity counts:
[2026-03-10 12:20:00]   AUDIT: FAILED — 1 HIGH remain after 3 audit cycles

# Advisory (MODERATE) — exact severity counts:
[2026-03-10 12:20:00]   AUDIT: 2 MODERATE remain after 3 audit cycles (advisory)
```

## LOW-Only Exit Behavior

When an audit finds only LOW-severity issues (no CRITICAL, HIGH, or MODERATE), the behavior depends on effort level:

| Effort | LOW-only result | Behavior |
|---|---|---|
| fast / standard / high | Immediate pass | No fix attempt; LOW findings are non-blocking |
| max | Single fix attempt, then pass | One fix agent pass targets the LOW findings. No re-audit after — the result is accepted regardless of whether the fix succeeded. This prevents the audit loop from cycling indefinitely on acknowledged LOWs while still giving max effort one chance to resolve them. |

The max effort audit iteration cap is set high (100) as a safety valve. The actual exit is governed by stale detection (3 consecutive cycles without progress) and the LOW-only exit condition above.

## Configuration

Sprint audits are **enabled by default**. No directive is needed -- audits will run automatically after each sprint passes sanity checks.

To customize audit settings:

```
@max_audit_iterations 5
@audit_engine claude
@audit_model claude-sonnet-4-20250514
```

To disable audits for a specific epic, add `@no_audit`:

```
@epic Quick Prototype
@engine claude
@no_audit
```

## Epic Directives

| Directive | Description |
|---|---|
| `@audit_after_sprint` | Enable post-sprint audit (default: enabled) |
| `@no_audit` | Disable post-sprint audit |
| `@max_audit_iterations <N>` | Maximum outer audit cycles per sprint (default: 3) |
| `@audit_engine <codex\|claude>` | AI engine for audit/fix sessions (default: same as `@engine`) |
| `@audit_model <model>` | Model override for audit/fix sessions |

## CLI Flags

| Flag | Description |
|---|---|
| `--no-audit` | Disable sprint and build audits for this run |

## Severity Classification

The audit agent classifies each finding with a severity level. Descriptions are mode-specific; the table below shows software-mode descriptions. In writing mode, CRITICAL maps to factual errors/contradictions, HIGH to structural gaps, MODERATE to voice/transition issues, and LOW to formatting/word choice. The blocking behavior is identical across modes.

| Level | Description (software) | Action | If unresolved |
|---|---|---|---|
| CRITICAL | Data loss, security breach, or crash under normal use | Fix agent remediates | **Blocks** sprint |
| HIGH | Significant bug; affects core functionality | Fix agent remediates | **Blocks** sprint |
| MODERATE | Edge case gaps, poor error handling, quality concerns | Fix agent remediates | Advisory warning |
| LOW | Style, naming, cosmetic | Fix agent remediates (high/max effort) | Non-blocking; noted in summary |

Severity is parsed from structured `**Severity:**` lines in the audit output, preventing false positives from severity keywords appearing in code diffs or prose.

## Audit Criteria

The audit agent evaluates the sprint's work against six criteria. The criteria vary by mode.

### Software and planning modes (default)

1. **Correctness** -- Does the code do what the sprint goals require?
2. **Usability** -- Are APIs, CLIs, and interfaces intuitive and consistent?
3. **Edge Cases** -- Are boundary conditions and error paths handled?
4. **Security** -- Are there injection, auth, or data-exposure risks?
5. **Performance** -- Are there obvious bottlenecks or resource leaks?
6. **Code Quality** -- Is the code readable, well-structured, and idiomatic?

### Writing mode (`--mode writing`)

1. **Coherence** -- Logical flow between sections; consistent narrative thread
2. **Accuracy** -- Factual correctness of claims, examples, and references
3. **Completeness** -- All topics from the sprint prompt are covered at appropriate depth
4. **Tone & Voice** -- Consistent register, audience-appropriate language, no jarring shifts
5. **Structure** -- Clear headings, logical section ordering, effective use of lists and examples
6. **Depth** -- Sufficient detail and analysis; not superficial or padded

See [Writing Mode](writing-mode.md) for the full writing-mode reference.

## Audit Output Format

The audit agent writes findings to `.fry/sprint-audit.txt`:

```
## Summary
Brief overview of the audit results.

## Findings
- **Location:** src/handler.go:42
- **Description:** SQL query uses string concatenation instead of parameterized queries
- **Severity:** HIGH
- **Recommended Fix:** Use db.Query with $1 placeholders

- **Location:** src/auth.go:15
- **Description:** Variable name `x` is unclear
- **Severity:** LOW
- **Recommended Fix:** Rename to `tokenExpiry`

## Verdict
FAIL (HIGH issues found)
```

If the agent forgets to write `.fry/sprint-audit.txt` but its final stdout/log output still contains a structured report or review-style findings, Fry reconstructs the file from that output and continues. If no structured recovery is possible, the audit fails.

On cycle 2+, the audit output also includes a "Verified Previous Issues" section:

```
## Verified Previous Issues
- **Issue:** 1
- **Status:** RESOLVED
- **Notes:** Parameterized query now used correctly.

- **Issue:** 2
- **Status:** STILL PRESENT
- **Notes:** Variable name unchanged.

## Findings
(includes both STILL PRESENT previous issues and any NEW issues)
```

## Context Provided to Audit Agent

The audit prompt includes:

| Context | Source | Limit |
|---|---|---|
| Codebase context | `.fry/codebase.md` | First 8,000 characters |
| Codebase memories | `.fry/codebase-memories/*.md` | Up to 10KB total |
| Executive summary | `plans/executive.md` | First 2,000 characters |
| Sprint goals | `@prompt` block from the epic | Full content |
| What was done | `.fry/sprint-progress.txt` | First 50KB |
| Code changes | `git diff` of sprint work | First 100KB |
| Previously identified issues | Findings from prior audit cycles | Cycle 2+ only |
| Intentional divergences | `.fry/deviation-log.md` filtered to the active sprint | When relevant |

The git diff is refreshed before each audit cycle (via a callback) so that fixes made by the fix agent are reflected in subsequent audits.
The audit agent uses the current repository state and sprint diff as primary evidence. `sprint-progress.txt` is supporting context only.
When `.fry/codebase.md` exists, the auditor uses it as ground truth for architecture and conventions. Pre-existing issues should only be raised when the sprint introduced, worsened, or clearly exposed them.
For moderate- and high-complexity sprints, the audit prompt starts with a targeted reconciliation pass that focuses the agent on numerical consistency before broader review criteria.

## Context Provided to Fix Agent

The fix prompt includes:

| Context | Source |
|---|---|
| Codebase context | `.fry/codebase.md` (if present) |
| Codebase memories | `.fry/codebase-memories/*.md` (if present) |
| Sprint goals | `@prompt` block from the epic |
| Issues to fix | Structured list, FIFO ordered (oldest first, highest severity within age group) |
| Previous fix attempts | Relevant subset of prior failed or partial attempts against the same findings |
| Context pointers | References to `sprint-progress.txt` and `plans/plan.md` |

Issues are grouped by origin audit cycle when multiple cycles have contributed findings. The fix agent is explicitly instructed to focus only on the listed issues, preserve unrelated behavior, follow existing patterns, and not search for new ones.

## Context Provided to Verify Agent

After each fix iteration, a lightweight verify agent checks resolution:

| Context | Source |
|---|---|
| Issues to verify | Numbered list of unresolved issues with location and severity |
| Instructions | Check each issue and report `RESOLVED`, `PARTIALLY_RESOLVED`, `BEHAVIOR_UNCHANGED`, `EVIDENCE_INCONCLUSIVE`, `BLOCKED`, or `STILL_PRESENT` |

The verify agent does not look for new issues and does not modify source code. Each verify session must write explicit statuses and optional notes to `.fry/sprint-audit.txt`. When verify uses `BEHAVIOR_UNCHANGED`, it should name the exact logic path, branch, or data flow that remained unchanged so Fry can feed that evidence into the next remediation prompt. Fry first tries to recover those statuses from the agent's final stdout/log output; if recovery fails, the audit fails rather than treating missing output as an implicit pass.

## Effort Level Interaction

- **`fast`** -- Sprint audits are skipped entirely, regardless of audit settings. This matches the behavior of sprint reviews at fast effort.
- **`standard`** -- Bounded audit with complexity-aware caps. Low-complexity sprints run up to 2 outer cycles, moderate stays at the default 3, and high-complexity sprints can use up to 5. Inner fix iterations stay at 3 except high-complexity standard audits, which get 4. LOW findings are ignored by the fix agent.
- **`high`** -- Progress-based audit with complexity-aware caps. Low-complexity sprints use up to 4 outer cycles and 5 inner fix iterations, moderate uses up to 8 outer cycles and 7 inner iterations, and high complexity uses up to 12 outer cycles and 7 inner iterations. **LOW findings are included** in the fix agent's scope alongside higher-severity items (non-blocking).
- **`max`** -- Progress-based audit with the largest adaptive budget. Low-complexity sprints use up to 6 outer cycles and 7 inner fix iterations, moderate uses up to 20 outer cycles and 10 inner iterations, and high complexity uses up to 100 outer cycles and 10 inner iterations. **LOW findings are included** in fix scope.

When `@max_audit_iterations` is explicitly set in the epic, it is always respected as the outer cycle cap regardless of effort level, and progress detection is disabled. Progress-based behavior only activates when the iteration count is not explicitly configured.
If Fry cannot classify complexity, it falls back to the legacy effort defaults.

### Progress detection (outer loop)

At `high` and `max` effort, Fry tracks audit findings across outer cycles by comparing finding key sets:

1. If no findings remain above LOW → pass (exit).
2. If findings are identical to the previous cycle (no issues resolved, no new issues) → increment stale counter.
3. If any previous findings were resolved or new findings appeared → reset stale counter (progress made).
4. After 3 consecutive stale cycles → stop and run final audit.

### Stale detection (inner loop)

Within each fix cycle, the inner loop tracks how many issues are resolved after each fix+verify iteration:

1. If all actionable issues are resolved → break inner loop (success).
2. If no new issues were resolved for 2 consecutive fix iterations → break inner loop (stale).
3. Otherwise, continue to the next fix iteration.

## Terminal Output

### Clean audit (no issues):
```
[2026-03-10 12:10:36] ▶ AUDIT  sprint 3/8 "Auth & Permissions"  cycle 1/3  engine=claude
[2026-03-10 12:12:00]   AUDIT: pass (none)
```

### Issues found and fixed (bounded):
```
[2026-03-10 12:10:36] ▶ AUDIT  sprint 3/8 "Auth & Permissions"  cycle 1/3  engine=claude
[2026-03-10 12:12:00]   AUDIT: 1 HIGH, 2 MODERATE — entering fix loop (3 issues)...
[2026-03-10 12:12:01]   AUDIT FIX  cycle 1  fix 1/3 — targeting 3 issues (oldest first)
[2026-03-10 12:14:00]   AUDIT VERIFY  cycle 1  fix 1/3 — 2 of 3 resolved
[2026-03-10 12:14:01]   AUDIT FIX  cycle 1  fix 2/3 — targeting 1 issues (oldest first)
[2026-03-10 12:16:00]   AUDIT VERIFY  cycle 1  fix 2/3 — 1 of 1 resolved
[2026-03-10 12:16:01] ▶ AUDIT  sprint 3/8 "Auth & Permissions"  cycle 2/3  engine=claude
[2026-03-10 12:18:00]   AUDIT: pass (1 LOW)
```

### Fixes introduce new issues (multi-cycle):
```
[2026-03-10 12:10:36] ▶ AUDIT  sprint 3/8 "Auth & Permissions"  cycle 1/3  engine=claude
[2026-03-10 12:12:00]   AUDIT: 1 HIGH — entering fix loop (1 issues)...
[2026-03-10 12:12:01]   AUDIT FIX  cycle 1  fix 1/3 — targeting 1 issues (oldest first)
[2026-03-10 12:14:00]   AUDIT VERIFY  cycle 1  fix 1/3 — 1 of 1 resolved
[2026-03-10 12:14:01] ▶ AUDIT  sprint 3/8 "Auth & Permissions"  cycle 2/3  engine=claude
[2026-03-10 12:16:00]   AUDIT: 1 previously known issues resolved
[2026-03-10 12:16:00]   AUDIT: 1 new issues discovered
[2026-03-10 12:16:00]   AUDIT: 1 MODERATE — entering fix loop (1 issues)...
[2026-03-10 12:16:01]   AUDIT FIX  cycle 2  fix 1/3 — targeting 1 issues (oldest first)
[2026-03-10 12:18:00]   AUDIT VERIFY  cycle 2  fix 1/3 — 1 of 1 resolved
[2026-03-10 12:18:01] ▶ AUDIT  sprint 3/8 "Auth & Permissions"  cycle 3/3  engine=claude
[2026-03-10 12:20:00]   AUDIT: pass (none)
```

### Progress-based audit (high/max effort):
```
[2026-03-10 12:10:36] ▶ AUDIT  sprint 3/8 "Auth & Permissions"  cycle 1 (progress-based, cap 12)  engine=claude
[2026-03-10 12:12:00]   AUDIT: 1 HIGH, 1 MODERATE, 2 LOW — entering fix loop (4 issues)...
[2026-03-10 12:12:01]   AUDIT FIX  cycle 1  fix 1/7 — targeting 4 issues (oldest first)
...
```

### Inner loop stale detection:
```
[2026-03-10 12:14:01]   AUDIT FIX  cycle 1  fix 2/5 — targeting 1 issues (oldest first)
[2026-03-10 12:16:00]   AUDIT VERIFY  cycle 1  fix 2/5 — 0 of 1 resolved
[2026-03-10 12:16:00]   AUDIT FIX: no progress after 2 fix iterations — moving to re-audit
```

### No-op fix detection:
```
[2026-04-02 17:51:18]   AUDIT FIX  cycle 1  fix 1/7 — targeting 1 issues (oldest first)
[2026-04-02 17:51:18]   AUDIT FIX: no-op (no file changes) — skipping verify
```

### Outer loop stale detection:
```
[2026-03-10 12:20:00]   AUDIT: no progress detected (3/3 stale cycles)
[2026-03-10 12:20:00]   AUDIT: stopping — no progress after 4 cycles
```

### CRITICAL/HIGH issues persist (blocking):
```
[2026-03-10 12:20:00]   AUDIT: FAILED — 1 HIGH remain after 3 audit cycles
Resume:   fry run --resume --sprint 3
Restart:  fry run --sprint 3
```

### MODERATE issues persist (advisory):
```
[2026-03-10 12:20:00]   AUDIT: 2 MODERATE remain after 3 audit cycles (advisory)
```

### LOW issues remain after audit (high/max effort, non-blocking):
```
[2026-03-10 12:20:00]   AUDIT: 3 LOW issues remain (non-blocking)
[2026-03-10 12:20:00]   AUDIT: pass after 4 cycles (3 LOW)
```

## Build Logs

Audit sessions are logged to `.fry/build-logs/`:

```
sprint1_audit1_20060102_150405.log           # Audit cycle 1
sprint1_auditfix_1_1_20060102_150405.log     # Fix agent: cycle 1, fix 1
sprint1_auditverify_1_1_20060102_150405.log  # Verify agent: cycle 1, fix 1
sprint1_auditfix_1_2_20060102_150405.log     # Fix agent: cycle 1, fix 2
sprint1_auditverify_1_2_20060102_150405.log  # Verify agent: cycle 1, fix 2
sprint1_audit2_20060102_150405.log           # Audit cycle 2 (re-audit)
sprint1_audit_final_20060102_150405.log      # Final audit pass
sprint1_audit_metrics.json                   # Per-call audit metrics for the sprint
```

Fry also writes a machine-readable audit metrics artifact for each sprint at `.fry/build-logs/sprintN_audit_metrics.json`. It records call counts, prompt sizes, durations, token usage, no-op rate, accepted versus rejected fix passes, diff classifications, verify yield, per-cycle productivity summaries, low-yield strategy changes, convergence cycle, and the classified sprint complexity. Live audit progress in `.fry/build-status.json` includes a compact snapshot of the same metrics plus the current complexity tier and any final low-yield stop reason.

## Cleanup

After the audit completes (pass or fail), Fry removes `.fry/sprint-audit.txt`, `.fry/audit-prompt.md`, and any transient same-role session files under `.fry/sessions/` before the git checkpoint. These are transient files that should not be committed.

## Examples

### Minimal setup

```
@epic My API
@engine claude
```

Audits run by default: 3 outer audit cycles, 3 inner fix iterations per cycle, same engine and model as the build agent.

### Custom audit configuration

```
@epic Payment System
@engine codex
@max_audit_iterations 5
@audit_engine claude
@audit_model claude-sonnet-4-20250514
```

Uses Codex for building but Claude for auditing, with up to 5 outer audit cycles.

### Disable in epic

```
@epic Quick Prototype
@engine claude
@no_audit
```

### Disable at runtime

```bash
fry --no-audit --engine claude
```

## See Also

- [Build Audit](build-audit.md) -- final holistic codebase audit that runs after the entire epic completes successfully
