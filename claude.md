# CLAUDE.md — Persistent Operating Memory

> This file is read and updated by every wake. Read top-to-bottom before you decide anything. Update relevant sections before you write your log entry and end the wake.

## Mission Clock

- **CREATED_AT:** `2026-04-18T14:49:18Z` (authoritative start; also at top of `prompt.md`)
- **Soft deadline:** `2026-04-19T02:49:18Z` (12h from CREATED_AT)
- **Hard deadline:** `2026-04-19T14:49:18Z` (24h from CREATED_AT)
- **Phase gate rule:** compute `elapsed_hours = (now - CREATED_AT) / 3600`.
  - < 12.0 → `building` (or `bootstrap` if wake 1)
  - 12.0 to < 24.0 → `building (overtime)` — only if M7 exit gate not yet passed
  - ≥ 24.0 → hard stop → `complete`

## DELIVERABLES STATUS

- [x] **M1** — Filesystem + state (`fry new`, `fry list`, `fry status` work against disk)
- [x] **M2** — Scheduler + macOS LaunchAgent (`fry start`/`fry stop` install/remove LaunchAgent)
- [x] **M3** — Wake: claude invocation + promise token (one real wake end-to-end)
- [x] **M4** — Prompt assembly + notes.md round-trip (cross-wake memory demonstrated)
- [x] **M5** — Chat session (`fry chat demo` → Claude answers in 1 turn)
- [x] **M6** — Deadlines + no-op detection + shutdown (dogfood mission auto-transitions to `complete`)
- [x] **M7** — Polish + README + ship (all spec §15 gates green)
- [x] `SHIPPED.md` at repo root (honest documentation of what works / doesn't)
- [ ] End-to-end smoke test: trivial mission runs one wake, logs one entry, self-terminates

## Current Phase

`complete` (M7 done, SHIPPED.md written, v0.1.0 tagged)

## Current Wake Number

`8`

## Elapsed Hours Since CREATED_AT

`2.0` (at close of wake 8)

## Current Focus

> Wake 8 (M7): README.md written (pitch, install, quickstart, file layout, how-it-works). SHIPPED.md written (honest gaps: no formal acceptance run, stale lock not implemented, no CI). Fixed all golangci-lint issues (errcheck defer patterns + staticcheck S1005/QF1012). go vet clean, golangci-lint 0 issues, all tests pass. 1917 non-test LOC (under 2000). Committed and tagged v0.1.0.

## Next Wake Should

Mission complete. All M1–M7 deliverables done. SHIPPED.md exists. Soft shutdown when scheduler next fires.

## Key Decisions Made (append-only)

- **2026-04-18T14:49:18Z (wake 1 / CREATED_AT):** Root is `/Users/julie/code/fry/`. All work lives here. Legacy code at `/Users/julie/code/fry-legacy/` for reference only (never import).
- **2026-04-18T14:49:18Z (wake 1):** Scheduler is a **macOS LaunchAgent** (inherited pattern from saas_build). Chose launchd over cron: LaunchAgents run in the user GUI session; `claude` CLI's keychain-backed OAuth is accessible there.
- **2026-04-18T14:49:18Z (wake 1):** Module path: `github.com/yevgetman/fry`. External deps budget: 2–3 (cobra, testify/assert, maybe uuid). No CGO allowed.
- **2026-04-18T14:49:18Z (wake 1):** Milestones M1–M7 are strictly sequential. No skipping ahead. Blocker in M<N> → document + try alternative within M<N>.
- **2026-04-18T14:49:18Z (wake 1):** Promise token for wake completion: `===WAKE_DONE===` as the final line of wake stdout.
- **2026-04-18T14:49:18Z (wake 1):** Scheduler teardown authority: agent signals (`FRY_STATUS_TRANSITION=complete` on stdout), tool terminates. Agent must NOT call `launchctl` directly.
- **2026-04-18T15:19:27Z (wake 2):** Legacy fry binary at `/Users/julie/.local/bin/fry` shadows new binary in shell PATH. New binary is correctly at `/Users/julie/go/bin/fry`. All verification must use full path `~/go/bin/fry` or ensure `~/go/bin` precedes `~/.local/bin` in PATH. Runner.sh template sets PATH explicitly — this is safe for scheduled execution.
- **2026-04-18T15:35:XX Z (wake 3):** `launchctl load` works fine on macOS 25.2 (Darwin) for LaunchAgents — no need for `bootstrap gui/<uid>/<label>`. Closed the open question.
- **2026-04-18T15:35:XX Z (wake 3):** `lock/` directory is NOT pre-created by scaffold; it is the mutex itself (created by `wake.Acquire`, removed by `wake.Release`). Fixed layout.go and layout_test.go.
- **2026-04-18T15:50:XX Z (wake 4):** `claude -p --output-format json` → cost field is `total_cost_usd` (NOT `cost_usd`). Result text field is `result`. Closed open question.

## Open Questions / Things To Resolve (living list)

- [x] LaunchAgent plist: `launchctl load` works on macOS 25.2 — confirmed in wake 3.
- [x] Cost parsing from `claude -p --output-format json`: field is `total_cost_usd`. Verified empirically in M3.
- [ ] Stale lock recovery: build-plan §8 risk register notes `mtime > 2×interval` → assume stale + take over. Implement in M2/M3.
- [x] `make install` target: resolved — `go install` installs to `~/go/bin/fry`. Confirmed working.
- [x] `go version`: verified in wake 2 — Go 1.26.1, well above 1.22 requirement.
- [ ] PATH shadowing: legacy fry at `~/.local/bin/fry` shadows new binary. Document in README when M7 is reached — users should ensure `~/go/bin` precedes `~/.local/bin` or remove legacy binary.

## Environment / Paths Cheat Sheet

- **Root:** `/Users/julie/code/fry/`
- **Logs:** `logs/wake_log.jsonl` (structured), `logs/cron.log` (runner stdout/stderr), `logs/launchd.stdout.log` / `logs/launchd.stderr.log` (launchd-level)
- **Runner:** `wake.sh`
- **Wake prompt:** `WAKE_INSTRUCTION.md` (passed as `-p` to each claude invocation)
- **LaunchAgent:** `~/Library/LaunchAgents/com.julie.fry.wake.plist`
  - Inspect: `launchctl list | grep fry.wake`
  - Force-fire: `launchctl kickstart -k gui/$(id -u)/com.julie.fry.wake`
  - Unload: `launchctl unload ~/Library/LaunchAgents/com.julie.fry.wake.plist`
- **Missions base dir (once built):** `~/missions/`
- **`claude` CLI:** `/Users/julie/.local/bin/claude` (expected; verify in M3)
- **User:** `julie` on macOS (Darwin 25.2). Shell: zsh. Apple Silicon (`/opt/homebrew/bin` on PATH).
- **Go binary (once built):** `/Users/julie/go/bin/fry` (via `go install`)
- **Legacy reference:** `/Users/julie/code/fry-legacy/` (read-only)

## Shutdown Protocol

**Soft shutdown (elapsed ≥ 12h AND M7 exit gate passed AND SHIPPED.md exists):**
1. Confirm `SHIPPED.md` exists at `/Users/julie/code/fry/SHIPPED.md` and is accurate.
2. Confirm smoke test works (trivial mission runs one wake, logs one entry, self-terminates).
3. Commit and push final state.
4. Append final log entry with `phase: "complete"`.
5. Unload LaunchAgent: `launchctl unload ~/Library/LaunchAgents/com.julie.fry.wake.plist`
6. Do NOT delete any files.

**Overtime (12h ≤ elapsed < 24h AND M7 exit gate NOT passed):**
- Log each wake's `phase: "building (overtime)"` with explicit `self_assessment` justifying why (e.g., "M5 chat not yet wired"; "M6 deadline logic missing"). Feature polish is NEVER a valid justification.
- Every overtime wake must move a core M1–M7 deliverable closer. Transition to `complete` as soon as M7 is done + SHIPPED.md exists.

**Hard stop (elapsed ≥ 24h):** No discretion.
1. Write the best SHIPPED.md right now, honestly documenting incomplete milestones.
2. Commit/push everything committable.
3. Append final log entry with `phase: "complete"` and `self_assessment: "hard stop at 24h"`.
4. Unload LaunchAgent: `launchctl unload ~/Library/LaunchAgents/com.julie.fry.wake.plist`
5. Exit.

## Self-Critique Hook (every 6th wake)

Every 6th wake (6, 12, 18, 24, ...), spend 1–2 minutes doing an honest self-review:
- Am I on schedule? (M1 by wake ~5, M2 by wake ~10, M3 by wake ~16, M4 by wake ~21, M5 by wake ~25, M6 by wake ~30, M7 by wake ~34)
- Is every wake producing a visible artifact?
- Am I drifting from locked decisions (especially non-goals list)?
- Write the self-review into the `self_assessment` field of that wake's log entry.
