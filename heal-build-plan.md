# Self-Healing Build Plan

When a sprint fails verification checks, fry.sh currently stops the build and tells the user to fix the issue manually. This plan adds an automatic **heal loop**: on verification failure, fry collects the failed checks, sends a targeted fix prompt to the AI agent, re-runs verification, and repeats up to a configurable limit before giving up.

---

## Current Behavior (What Changes)

Today, `run_sprint()` has four code paths that `return 1` on verification failure (lines 1091-1139). Each one logs a FAIL message, sets `SPRINT_RESULTS`, commits a git checkpoint, and returns. The main loop then prints "Build stopped at Sprint N" and exits.

The heal loop wraps these verification-failure exit points so the agent gets a chance to fix the issues before the sprint is declared failed.

---

## Design

### Core Concept

After the iteration loop finishes (promise found or max iterations exhausted), verification checks run as they do today. If checks fail, instead of immediately returning failure, a new **heal loop** begins:

```
for heal_attempt in 1..max_heal_attempts:
    1. Collect list of failed checks (type, args, PASS/FAIL status)
    2. Build a heal prompt with: failed checks, sprint context, progress.txt
    3. Run the AI agent with the heal prompt
    4. Re-run verification checks
    5. If all pass -> break, mark sprint PASS
    6. If still failing -> continue loop
If loop exhausted -> mark sprint FAIL as today
```

### Where It Plugs In

The heal loop lives in a single new function `run_heal_loop()` called from `run_sprint()`. The four verification-failure code paths (lines 1091-1096, 1111-1116, 1134-1139, and the no-promise-no-checks path at 1119-1123) are modified to call `run_heal_loop()` before declaring failure. The no-promise-no-checks path (line 1119) is **not** eligible for healing since there are no verification checks to guide the fix.

Specifically, healing applies to these three failure modes:
- Promise found, verification failed (line 1091)
- No promise found, verification failed (line 1111)
- No promise defined, verification failed (line 1134)

### What Does NOT Change

- The iteration loop (lines 1025-1063) is untouched. Healing happens *after* iterations complete.
- `run_verification_checks()` is untouched as a function. It's called as-is during healing.
- The promise-token mechanism is unaffected.
- The "no promise after N iters" failure (no checks defined) still fails immediately — there's nothing to heal against.

---

## Implementation Steps

### Step 1: Add `@max_heal_attempts` directive

**File:** `fry.sh`

Add a new global directive parsed in `parse_epic()` (GLOBAL state, ~line 309):

```bash
[[ "$line" =~ ^@max_heal_attempts[[:space:]]+([0-9]+) ]] && MAX_HEAL_ATTEMPTS="${BASH_REMATCH[1]}" && continue
```

Add the global variable (~line 124):

```bash
MAX_HEAL_ATTEMPTS=3
```

This is the default. Setting `@max_heal_attempts 0` in the epic disables healing entirely (preserves current stop-on-fail behavior).

Also support a per-sprint override parsed in SPRINT_META state (~line 330):

```bash
[[ "$line" =~ ^@max_heal_attempts[[:space:]]+([0-9]+) ]] && SPRINT_HEAL_ATTEMPTS[$current_sprint]="${BASH_REMATCH[1]}" && continue
```

With a corresponding associative array:

```bash
declare -A SPRINT_HEAL_ATTEMPTS
```

Resolution order: per-sprint `@max_heal_attempts` > global `@max_heal_attempts` > default (3).

### Step 2: Add `collect_failed_checks()` function

**File:** `fry.sh`, new function after `run_verification_checks()` (~line 490)

This function mirrors `run_verification_checks()` but instead of just returning pass/fail, it builds a structured text report of what failed. Output goes to a file that the heal prompt will reference.

```bash
collect_failed_checks() {
  local sprint_num=$1
  local output_file=$2
  local total=0
  local passed=0
  local failed_details=""

  for entry in "${VERIFICATION_CHECKS[@]+"${VERIFICATION_CHECKS[@]}"}"; do
    local snum="${entry%%|*}"
    [[ "$snum" -ne "$sprint_num" ]] && continue

    local rest="${entry#*|}"
    local check_type="${rest%%|*}"
    local args="${rest#*|}"
    total=$((total + 1))

    case "$check_type" in
      FILE)
        if [[ ! -s "${PROJECT_DIR}/${args}" ]]; then
          failed_details+="- FAILED: File missing or empty: ${args}"$'\n'
        else
          passed=$((passed + 1))
        fi
        ;;
      FILE_CONTAINS)
        local fpath="${args%%|*}"
        local pattern="${args#*|}"
        if ! grep -qE "$pattern" "${PROJECT_DIR}/${fpath}" 2>/dev/null; then
          failed_details+="- FAILED: File '${fpath}' does not contain pattern: ${pattern}"$'\n'
        else
          passed=$((passed + 1))
        fi
        ;;
      CMD)
        if ! (cd "$PROJECT_DIR" && eval "$args") &>/dev/null; then
          # Capture actual error output for diagnostic value
          local err_output
          err_output=$(cd "$PROJECT_DIR" && eval "$args" 2>&1 || true)
          failed_details+="- FAILED: Command failed: ${args}"$'\n'
          if [[ -n "$err_output" ]]; then
            # Truncate to avoid overwhelming the prompt
            failed_details+="  Output (truncated): $(echo "$err_output" | head -20)"$'\n'
          fi
        else
          passed=$((passed + 1))
        fi
        ;;
      CMD_OUTPUT)
        local cmd="${args%%|*}"
        local pattern="${args#*|}"
        local output
        output=$(cd "$PROJECT_DIR" && eval "$cmd" 2>/dev/null) || true
        if ! echo "$output" | grep -qE "$pattern" 2>/dev/null; then
          failed_details+="- FAILED: Command output mismatch: ${cmd}"$'\n'
          failed_details+="  Expected pattern: ${pattern}"$'\n'
          failed_details+="  Got: $(echo "$output" | head -10)"$'\n'
        else
          passed=$((passed + 1))
        fi
        ;;
    esac
  done

  {
    echo "Verification: ${passed}/${total} checks passed."
    echo ""
    if [[ -n "$failed_details" ]]; then
      echo "Failed checks:"
      echo "$failed_details"
    fi
  } > "$output_file"
}
```

**Key design choice:** For `CMD` failures, capture stderr/stdout from the failing command (truncated to 20 lines) so the AI agent sees actual compiler errors, test failures, etc. This is the most actionable diagnostic data.

### Step 3: Add `run_heal_loop()` function

**File:** `fry.sh`, new function after `collect_failed_checks()` (~line 550)

```bash
run_heal_loop() {
  local sprint_num=$1
  local sprint_name="${SPRINT_NAMES[$sprint_num]:-Sprint ${sprint_num}}"
  local max_heals="${SPRINT_HEAL_ATTEMPTS[$sprint_num]:-$MAX_HEAL_ATTEMPTS}"

  if [[ "$max_heals" -eq 0 ]]; then
    return 1  # Healing disabled
  fi

  local heal_attempt=0

  while [[ $heal_attempt -lt $max_heals ]]; do
    heal_attempt=$((heal_attempt + 1))
    log "  Heal attempt ${heal_attempt}/${max_heals} for Sprint ${sprint_num}..."

    # 1. Collect failed checks into a report file
    local fail_report="${WORK_DIR}/heal_report_s${sprint_num}_h${heal_attempt}.txt"
    collect_failed_checks "$sprint_num" "$fail_report"

    # 2. Build the heal prompt
    local heal_prompt_file="${PROJECT_DIR}/prompt.md"
    {
      echo "# HEAL MODE — Sprint ${sprint_num}: ${sprint_name}"
      echo ""
      echo "## What happened"
      echo "The sprint finished its work but FAILED independent verification checks."
      echo "Your job is to fix ONLY the issues described below. Do not start the sprint over."
      echo "Do not refactor or reorganize. Make the minimum changes needed to pass the checks."
      echo ""
      echo "## Failed verification checks"
      echo ""
      cat "$fail_report"
      echo ""
      echo "## Instructions"
      echo "1. Read progress.txt for context on what was built"
      echo "2. Read the failed checks above carefully"
      echo "3. Fix each failure — create missing files, fix build errors, correct config"
      echo "4. After fixing, do a final sanity check (e.g., run the build command if applicable)"
      echo "5. Append a brief note to progress.txt about what you fixed in this heal pass"
      echo ""
      echo "## Context files"
      echo "- Read progress.txt for iteration history"
      echo "- Read ${PLAN_FILE} for the overall project plan"
      if [[ -f "${PROJECT_DIR}/${CONTEXT_FILE}" ]]; then
        echo "- Read ${CONTEXT_FILE} for executive context"
      fi
      echo ""
      echo "Do NOT output any promise tokens. Just fix the issues."
    } > "$heal_prompt_file"

    log "  Wrote heal prompt ($(wc -l < "$heal_prompt_file" | tr -d ' ') lines)"

    # 3. Run the agent
    local heal_log="${LOG_DIR}/sprint${sprint_num}_heal${heal_attempt}_${TIMESTAMP}.log"

    cd "$PROJECT_DIR"
    run_agent \
      "Read and execute ALL instructions in prompt.md in the project root. This is a HEAL pass — fix the verification failures described in the prompt." \
      2>&1 | tee -a "$heal_log" "${LOG_DIR}/sprint${sprint_num}_${TIMESTAMP}.log"

    # 4. Re-run verification
    log "  Re-running verification checks after heal attempt ${heal_attempt}..."
    if run_verification_checks "$sprint_num"; then
      log "  Heal attempt ${heal_attempt} SUCCEEDED — all checks now pass"
      return 0
    else
      log "  Heal attempt ${heal_attempt} — checks still failing"
      # Append heal result to progress.txt so next heal attempt has context
      {
        echo ""
        echo "--- Heal attempt ${heal_attempt} ($(date)) ---"
        echo "Attempted to fix verification failures. Some checks still failing."
        cat "$fail_report"
      } >> "${PROJECT_DIR}/progress.txt"
    fi
  done

  log "  All ${max_heals} heal attempts exhausted. Sprint ${sprint_num} remains failed."
  return 1
}
```

### Step 4: Modify `run_sprint()` outcome logic

**File:** `fry.sh`, modify the verification-failure branches in `run_sprint()` (lines 1083-1146)

Replace the three direct `return 1` paths that involve verification failure with a call to `run_heal_loop()`. If healing succeeds, continue to the PASS path. If it fails, fall through to the existing FAIL path.

**Example for the "promise found, verification failed" branch (lines 1091-1096):**

Before:
```bash
else
  log "SPRINT ${sprint_num} FAILED — promise found but verification failed"
  log "Resume: $0 ${EPIC_FILE} ${sprint_num}"
  SPRINT_RESULTS[$sprint_num]="FAIL (promise found, verification failed)"
  git_checkpoint "$sprint_num" "failed-verification"
  return 1
fi
```

After:
```bash
else
  log "  Verification failed. Attempting self-heal..."
  if run_heal_loop "$sprint_num"; then
    log "SPRINT ${sprint_num} COMPLETED — healed, verification now passes (${minutes}m ${seconds}s)"
    SPRINT_RESULTS[$sprint_num]="PASS (healed)"
    git_checkpoint "$sprint_num" "complete-healed"
  else
    log "SPRINT ${sprint_num} FAILED — verification failed, healing exhausted (${minutes}m ${seconds}s)"
    log "Resume: $0 ${EPIC_FILE} ${sprint_num}"
    SPRINT_RESULTS[$sprint_num]="FAIL (verification failed, heal exhausted)"
    git_checkpoint "$sprint_num" "failed-verification"
    return 1
  fi
fi
```

Apply the same pattern to all three verification-failure branches. The git checkpoint labels differentiate healed sprints (`complete-healed`) from normal completions.

### Step 5: Update `print_summary()` to show heal status

**File:** `fry.sh`, modify `print_summary()` (~line 1153)

No structural changes needed. The `SPRINT_RESULTS` values already flow into the summary. The new values like `"PASS (healed)"` and `"FAIL (verification failed, heal exhausted)"` will display naturally.

### Step 6: Update documentation

**Files to update:**

1. **`fry.sh` header comment** (~line 50-62): Add `@max_heal_attempts` to the global directives list and per-sprint directives list.

2. **`usage()`** (~line 157-204): Mention heal behavior in the description.

3. **`epic-example.md`**: Add `@max_heal_attempts` example in the global directives section and show a per-sprint override example.

4. **`verification-example.md`**: Add a note in the PURPOSE section mentioning that failed checks trigger automatic heal attempts.

5. **`README.md`**: Add heal loop to the "Key mechanisms" list. Update the flow diagram to show the heal feedback loop.

### Step 7: Update dry-run report

**File:** `fry.sh`, modify `dry_run_report()` function

Add a line showing the effective `max_heal_attempts` for each sprint (accounting for per-sprint overrides).

---

## Logging

Each heal attempt produces its own log file:
```
build-logs/sprint1_heal1_<timestamp>.log
build-logs/sprint1_heal2_<timestamp>.log
```

These are also appended to the combined sprint log (`sprint1_<timestamp>.log`) as today's iteration logs are.

The main build log gets entries like:
```
[2026-03-06 14:22:01]   Verification failed. Attempting self-heal...
[2026-03-06 14:22:01]   Heal attempt 1/3 for Sprint 1...
[2026-03-06 14:22:01]   Wrote heal prompt (28 lines)
[2026-03-06 14:24:15]   Re-running verification checks after heal attempt 1...
[2026-03-06 14:24:16]     [PASS] check_file: package.json
[2026-03-06 14:24:16]     [FAIL] check_cmd: npm run build
[2026-03-06 14:24:16]   Verification: 5/6 checks passed.
[2026-03-06 14:24:16]   Heal attempt 1 — checks still failing
[2026-03-06 14:24:16]   Heal attempt 2/3 for Sprint 1...
...
[2026-03-06 14:26:30]   Heal attempt 2 SUCCEEDED — all checks now pass
[2026-03-06 14:26:30] SPRINT 1 COMPLETED — healed, verification now passes (12m 30s)
```

---

## Git Checkpoints

- Each heal attempt does **not** get its own git checkpoint. Only the final outcome is committed.
- If healing succeeds: `git_checkpoint "$sprint_num" "complete-healed"`
- If healing fails: `git_checkpoint "$sprint_num" "failed-verification"` (same as today)

This keeps the git history clean — one commit per sprint, not one per heal attempt.

---

## Edge Cases

| Scenario | Behavior |
|---|---|
| `@max_heal_attempts 0` (global or per-sprint) | Healing disabled, current stop-on-fail behavior preserved |
| No verification checks defined, promise not found | Not eligible for healing (no checks to guide fixes). Fails immediately as today |
| Heal attempt makes things worse (more checks fail) | Loop continues. Each attempt gets fresh context from the fail report. Agent sees current state, not stale data |
| All heal attempts fix some but not all checks | Final state after last attempt is what gets committed. Partial progress is preserved |
| Sprint has no promise but has checks | Healing still applies if checks fail. The heal prompt doesn't reference promises |
| Agent crashes during heal (non-zero exit) | Loop continues to next attempt. Same resilience as the iteration loop |
| User interrupts during heal (Ctrl+C) | Caught by existing `on_signal` trap. Partial heal work is committed as "interrupted (partial)" |

---

## Configuration Summary

| Directive | Scope | Default | Description |
|---|---|---|---|
| `@max_heal_attempts <N>` | Global | `3` | Max heal attempts after verification failure |
| `@max_heal_attempts <N>` | Per-sprint | Inherits global | Override for a specific sprint |

Setting to `0` disables healing. There is no CLI flag — this is controlled via the epic file (consistent with how all other build behavior is configured).

---

## Files Modified

| File | Changes |
|---|---|
| `fry.sh` | Add `MAX_HEAL_ATTEMPTS` global, `SPRINT_HEAL_ATTEMPTS` assoc array, parse `@max_heal_attempts` in both GLOBAL and SPRINT_META states, add `collect_failed_checks()` and `run_heal_loop()` functions, modify three verification-failure branches in `run_sprint()`, update `dry_run_report()` |
| `epic-example.md` | Add `@max_heal_attempts` examples |
| `verification-example.md` | Add note about heal behavior |
| `README.md` | Document heal loop in key mechanisms and flow diagram |

No new files are created.
