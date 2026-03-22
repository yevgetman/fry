#!/usr/bin/env bash
#
# Fry Self-Improvement Orchestrator
#
# Drives the automated self-improvement loop:
#   1. Planning phase — scan codebase for new issues, append to roadmap
#   2. Build phase — implement 2-3 open items, push branch, create PR
#
# Dependencies: git, jq, fry, gh (GitHub CLI), make, go
# Usage: .self-improve/orchestrate.sh [--skip-planning] [--skip-build] [--dry-run]

set -euo pipefail

# --- Configuration ---
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

LOCK_FILE="$SCRIPT_DIR/.lock"
LOG_DIR="$SCRIPT_DIR/logs"
LOG_FILE="$SCRIPT_DIR/orchestrate.log"
ROADMAP="$SCRIPT_DIR/roadmap.json"
ID_COUNTER="$SCRIPT_DIR/id-counter.json"
EXECUTIVE="$SCRIPT_DIR/executive.md"
PLANNING_PROMPT="$SCRIPT_DIR/planning-prompt.md"
BUILD_PROMPT="$SCRIPT_DIR/build-prompt.md"
LAST_BUILD_STATUS="$SCRIPT_DIR/.last-build-status"

MAX_BUILD_ITEMS=3
MAX_ATTEMPTS=3          # Skip items that have failed this many times
MAX_POST_BUILD_HEALS=3  # Attempts to heal test/build failures after Fry completes
PLANNING_THRESHOLD=15   # Skip planning if this many open items already exist
DATE="$(date +%Y-%m-%d)"
RUN_ID="$(date +%Y%m%d-%H%M%S)"

# Flags
SKIP_PLANNING=false
SKIP_BUILD=false
DRY_RUN=false
AUTO_MERGE=false

# State (used by cleanup trap)
WORKTREE_DIR=""
BUILD_BRANCH=""
PR_CREATED=false

# --- Argument parsing ---
for arg in "$@"; do
    case "$arg" in
        --skip-planning) SKIP_PLANNING=true ;;
        --skip-build)    SKIP_BUILD=true ;;
        --dry-run)       DRY_RUN=true ;;
        --auto-merge)    AUTO_MERGE=true ;;
        --help|-h)
            echo "Usage: orchestrate.sh [--skip-planning] [--skip-build] [--dry-run] [--auto-merge]"
            exit 0
            ;;
        *) echo "Unknown argument: $arg" >&2; exit 1 ;;
    esac
done

# --- Logging ---
RUN_LOG=""  # Set in main() after LOG_DIR is created

log() {
    local msg="[$(date '+%Y-%m-%d %H:%M:%S')] $*"
    echo "$msg" | tee -a "$LOG_FILE"
    if [ -n "$RUN_LOG" ]; then
        echo "$msg" >> "$RUN_LOG"
    fi
}

die() {
    log "FATAL: $*"
    exit 1
}

# --- Lock ---
acquire_lock() {
    if [ -f "$LOCK_FILE" ]; then
        local pid
        pid="$(cat "$LOCK_FILE" 2>/dev/null || echo "")"
        if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
            die "Another orchestrator is running (PID $pid)"
        fi
        log "Removing stale lock (PID $pid)"
        rm -f "$LOCK_FILE"
    fi
    echo $$ > "$LOCK_FILE"
}

release_lock() {
    rm -f "$LOCK_FILE"
}

# --- Cleanup (runs on exit, error, or signal) ---
cleanup() {
    local exit_code=$?
    log "Cleaning up (exit code $exit_code)..."

    # Remove worktree if it was created
    if [ -n "$WORKTREE_DIR" ] && [ -d "$WORKTREE_DIR" ]; then
        git -C "$REPO_DIR" worktree remove "$WORKTREE_DIR" --force 2>/dev/null || true
    fi

    # Delete build branch if PR was not created
    if [ -n "$BUILD_BRANCH" ] && [ "$PR_CREATED" != "true" ]; then
        git -C "$REPO_DIR" branch -D "$BUILD_BRANCH" 2>/dev/null || true
    fi

    # Clean up scaffolding and build artifacts
    rm -rf "$REPO_DIR/.fry" "$REPO_DIR/plans" "$REPO_DIR/assets" "$REPO_DIR/output"

    # Ensure we're back on master
    git -C "$REPO_DIR" checkout master 2>/dev/null || true

    release_lock
    log "Cleanup complete"
}
trap cleanup EXIT INT TERM

# --- Dependency checks ---
check_deps() {
    local missing=()
    for cmd in git jq fry gh make go; do
        if ! command -v "$cmd" &>/dev/null; then
            missing+=("$cmd")
        fi
    done
    if [ ${#missing[@]} -gt 0 ]; then
        die "Missing dependencies: ${missing[*]}"
    fi
}

# --- Validate roadmap JSON ---
validate_roadmap() {
    if [ ! -f "$ROADMAP" ]; then
        die "Roadmap not found at $ROADMAP"
    fi
    if ! jq -e '.version == 1 and (.items | type) == "array"' "$ROADMAP" >/dev/null 2>&1; then
        die "Invalid roadmap format at $ROADMAP"
    fi
}

# --- Count open items ---
count_open_items() {
    jq '[.items[] | select(.status == "open")] | length' "$ROADMAP"
}

# --- Decide whether planning is needed ---
# Returns 0 (true) if planning should run, 1 (false) if not.
should_run_planning() {
    local open_count
    open_count="$(count_open_items)"

    # Hard ceiling — too many items already
    if [ "$open_count" -ge "$PLANNING_THRESHOLD" ]; then
        log "Planning not needed: $open_count open items (threshold: $PLANNING_THRESHOLD)"
        return 1
    fi

    # Running low — always plan
    if [ "$open_count" -lt 5 ]; then
        log "Planning needed: only $open_count open items (< 5)"
        return 0
    fi

    # Check core category coverage (bug, testing, feature, improvement)
    local empty_core
    empty_core="$(jq '
        ["bug", "testing", "feature", "improvement"] as $core |
        [.items[] | select(.status == "open") | .category] | unique as $present |
        [$core[] | select(. as $c | $present | index($c) | not)] | length
    ' "$ROADMAP")"
    if [ "$empty_core" -ge 2 ]; then
        log "Planning needed: $empty_core core categories have zero items"
        return 0
    fi

    # Check for imbalance — any category > 50% of all items
    local imbalanced
    imbalanced="$(jq --argjson total "$open_count" '
        [.items[] | select(.status == "open")] | group_by(.category) |
        any(length > ($total / 2))
    ' "$ROADMAP")"
    if [ "$imbalanced" = "true" ]; then
        log "Planning needed: roadmap is imbalanced (one category > 50%)"
        return 0
    fi

    log "Planning not needed: $open_count items, balanced across categories"
    return 1
}

# ===========================================================================
# PLANNING PHASE
# ===========================================================================
run_planning_phase() {
    log "--- Planning Phase ---"

    # Redundant guard (should_run_planning already checked, but safe for direct calls)
    local open_count
    open_count="$(count_open_items)"
    if [ "$open_count" -ge "$PLANNING_THRESHOLD" ]; then
        log "Skipping planning: $open_count open items already (threshold: $PLANNING_THRESHOLD)"
        return 0
    fi

    # Clean stale artifacts from any prior run, then scaffold
    rm -rf "$REPO_DIR/.fry" "$REPO_DIR/plans" "$REPO_DIR/assets" "$REPO_DIR/output"
    mkdir -p "$REPO_DIR/plans" "$REPO_DIR/assets" "$REPO_DIR/output"
    cp "$EXECUTIVE" "$REPO_DIR/plans/executive.md"
    cp "$ROADMAP" "$REPO_DIR/assets/roadmap.json"

    if [ "$DRY_RUN" = true ]; then
        log "[DRY RUN] Would run Fry planning scan"
        rm -rf "$REPO_DIR/plans" "$REPO_DIR/assets" "$REPO_DIR/output"
        return 0
    fi

    # Run Fry — planning is analysis-only, no verification/audit needed
    log "Running Fry planning scan..."
    if ! fry run \
        --user-prompt-file "$PLANNING_PROMPT" \
        --no-sanity-check \
        --no-audit \
        --git-strategy current \
        --mode planning \
        --effort medium \
        --project-dir "$REPO_DIR" 2>&1 | tee -a "$LOG_FILE"; then
        log "WARNING: Planning run failed — skipping new findings"
        rm -rf "$REPO_DIR/plans" "$REPO_DIR/assets" "$REPO_DIR/output"
        return 0
    fi

    # Process findings — check multiple possible locations
    local findings_file=""
    for candidate in \
        "$REPO_DIR/output/new-findings.json" \
        "$REPO_DIR/new-findings.json" \
        "$REPO_DIR/output/findings.json" \
        "$REPO_DIR/.fry/new-findings.json"; do
        if [ -f "$candidate" ]; then
            findings_file="$candidate"
            break
        fi
    done
    if [ -z "$findings_file" ]; then
        log "No findings file produced (checked output/new-findings.json and alternates)"
        rm -rf "$REPO_DIR/.fry" "$REPO_DIR/plans" "$REPO_DIR/assets" "$REPO_DIR/output"
        return 0
    fi
    log "Found findings at: $findings_file"

    if ! jq -e 'type == "array"' "$findings_file" >/dev/null 2>&1; then
        log "WARNING: Invalid JSON in findings file — skipping"
        rm -rf "$REPO_DIR/plans" "$REPO_DIR/assets" "$REPO_DIR/output"
        return 0
    fi

    local count
    count="$(jq 'length' "$findings_file")"
    if [ "$count" -eq 0 ]; then
        log "No new findings"
        rm -rf "$REPO_DIR/plans" "$REPO_DIR/assets" "$REPO_DIR/output"
        return 0
    fi

    log "Found $count new item(s) — assigning IDs and appending to roadmap"
    assign_ids_and_append "$findings_file"

    # Clean up scaffolding
    rm -rf "$REPO_DIR/plans" "$REPO_DIR/assets" "$REPO_DIR/output"

    # Commit and push
    cd "$REPO_DIR"
    git add "$ROADMAP"
    if git diff --cached --quiet; then
        log "No roadmap changes to commit"
    else
        git commit -m "Add $count new self-improvement finding(s) [$DATE]"
        git push origin master || log "WARNING: push failed — will retry next run"
    fi
}

# Assign sequential IDs to new findings and append to roadmap
assign_ids_and_append() {
    local findings_file="$1"

    # Read all findings into a temp file to avoid subshell issues with pipes
    local tmp_findings
    tmp_findings="$(mktemp)"
    jq -c '.[]' "$findings_file" > "$tmp_findings"

    while IFS= read -r item; do
        local category
        category="$(echo "$item" | jq -r '.category')"

        # Map category to letter prefix
        local prefix
        case "$category" in
            bug)           prefix="A" ;;
            testing)       prefix="B" ;;
            feature)       prefix="C" ;;
            improvement)   prefix="D" ;;
            sunset)        prefix="E" ;;
            refactor)      prefix="F" ;;
            security)      prefix="G" ;;
            ui_ux)         prefix="H" ;;
            documentation) prefix="I" ;;
            *)             prefix="X" ;;
        esac

        # Get next ID from counter (never reuses IDs of removed items)
        local current_num
        current_num="$(jq -r --arg p "$prefix" '.[$p] // 0' "$ID_COUNTER")"
        local next_num=$((current_num + 1))
        local new_id="${prefix}${next_num}"

        # Update counter
        local tmp_counter
        tmp_counter="$(mktemp)"
        jq --arg p "$prefix" --argjson n "$next_num" '.[$p] = $n' "$ID_COUNTER" > "$tmp_counter"
        mv "$tmp_counter" "$ID_COUNTER"

        # Set the ID, discovered date, and ensure status fields
        local updated
        updated="$(echo "$item" | jq \
            --arg id "$new_id" \
            --arg date "$DATE" \
            '.id = $id | .discovered = $date | .status = "open" | .attempted_count = (.attempted_count // 0)')"

        # Append to roadmap
        local tmp_roadmap
        tmp_roadmap="$(mktemp)"
        jq --argjson item "$updated" '.items += [$item]' "$ROADMAP" > "$tmp_roadmap"
        mv "$tmp_roadmap" "$ROADMAP"

        log "  Added: $new_id — $(echo "$item" | jq -r '.title')"
    done < "$tmp_findings"

    rm -f "$tmp_findings"
}

# ===========================================================================
# BUILD PHASE
# ===========================================================================
run_build_phase() {
    log "--- Build Phase ---"

    # Check there are open items to work on
    local open_count
    open_count="$(count_open_items)"
    if [ "$open_count" -eq 0 ]; then
        log "No open items in roadmap — skipping build"
        return 0
    fi
    log "$open_count open item(s) in roadmap — Fry will choose what to work on"

    if [ "$DRY_RUN" = true ]; then
        log "[DRY RUN] Would run Fry build"
        return 0
    fi

    # Create branch and worktree
    BUILD_BRANCH="self-improve/${RUN_ID}"

    if git -C "$REPO_DIR" rev-parse --verify "refs/heads/$BUILD_BRANCH" &>/dev/null; then
        log "Branch $BUILD_BRANCH already exists — deleting"
        git -C "$REPO_DIR" branch -D "$BUILD_BRANCH"
    fi

    WORKTREE_DIR="$REPO_DIR/.fry-worktrees/build-${RUN_ID}"
    if [ -d "$WORKTREE_DIR" ]; then
        log "Worktree dir already exists — removing"
        git -C "$REPO_DIR" worktree remove "$WORKTREE_DIR" --force 2>/dev/null || rm -rf "$WORKTREE_DIR"
        git -C "$REPO_DIR" worktree prune
    fi

    log "Creating worktree at $WORKTREE_DIR on branch $BUILD_BRANCH"
    mkdir -p "$(dirname "$WORKTREE_DIR")"
    git -C "$REPO_DIR" worktree add "$WORKTREE_DIR" -b "$BUILD_BRANCH"

    # Scaffold worktree — Fry gets the full roadmap and chooses items itself
    mkdir -p "$WORKTREE_DIR/plans" "$WORKTREE_DIR/assets"
    cp "$EXECUTIVE" "$WORKTREE_DIR/plans/executive.md"
    cp "$ROADMAP" "$WORKTREE_DIR/assets/roadmap.json"

    # Run Fry build — full prepare so the LLM reads assets/roadmap.json
    # and chooses items before building the epic
    log "Running Fry build in worktree..."
    local build_success=true
    if ! fry run \
        --user-prompt-file "$BUILD_PROMPT" \
        --always-verify \
        --full-prepare \
        --no-sanity-check \
        --git-strategy current \
        --project-dir "$WORKTREE_DIR" 2>&1 | tee -a "$LOG_FILE"; then
        log "WARNING: Fry build run failed"
        build_success=false
    fi

    # Post-build verification with healing loop
    if [ "$build_success" = true ]; then
        log "Running post-build verification (make test && make build)..."
        local test_output
        test_output="$(cd "$WORKTREE_DIR" && make test 2>&1 && make build 2>&1)" || true

        if (cd "$WORKTREE_DIR" && make test && make build) >/dev/null 2>&1; then
            log "Post-build verification passed"
        else
            log "Post-build verification failed — entering heal loop"
            local heal_attempt=0
            local healed=false

            while [ "$heal_attempt" -lt "$MAX_POST_BUILD_HEALS" ]; do
                heal_attempt=$((heal_attempt + 1))
                log "Post-build heal attempt $heal_attempt/$MAX_POST_BUILD_HEALS..."

                # Capture current failure output
                local failure_output
                failure_output="$(cd "$WORKTREE_DIR" && make test 2>&1; make build 2>&1)" || true

                # Capture the diff of what changed (so the agent knows what was modified)
                local diff_output
                diff_output="$(cd "$WORKTREE_DIR" && git diff HEAD~1 --stat 2>/dev/null)" || true

                # Write the heal prompt to a temp file to avoid argument length limits
                local heal_prompt_file
                heal_prompt_file="$(mktemp)"
                cat > "$heal_prompt_file" <<HEALPROMPT
You are fixing test or build failures in the Fry codebase. Read CLAUDE.md for coding conventions.

Recent changes (git diff --stat):
${diff_output}

The following test/build failures occurred after these changes:

${failure_output}

Instructions:
1. Read the failing test files and the source code they test.
2. Fix the underlying source code to make tests pass. Do NOT remove, weaken, or skip tests.
3. If a test expectation is wrong due to an intentional behavior change, update the test to match the new behavior.
4. Run 'make test && make build' to verify your fix before finishing.
5. Keep changes minimal — only fix what is broken.
HEALPROMPT

                if ! (cd "$WORKTREE_DIR" && claude -p --dangerously-skip-permissions --model sonnet < "$heal_prompt_file") 2>&1 | tee -a "$LOG_FILE"; then
                    log "  Heal agent failed to run"
                    rm -f "$heal_prompt_file"
                    continue
                fi
                rm -f "$heal_prompt_file"

                # Re-verify
                if (cd "$WORKTREE_DIR" && make test && make build) 2>&1 | tee -a "$LOG_FILE"; then
                    log "Post-build heal attempt $heal_attempt SUCCEEDED"
                    # Commit the heal fixes
                    (cd "$WORKTREE_DIR" && git add -A && git commit -m "Fix post-build test/build failures [automated heal]") 2>&1 | tee -a "$LOG_FILE"
                    healed=true
                    break
                else
                    log "Post-build heal attempt $heal_attempt FAILED"
                fi
            done

            if [ "$healed" != true ]; then
                log "WARNING: Post-build healing exhausted ($MAX_POST_BUILD_HEALS attempts)"
                build_success=false
            fi
        fi
    fi

    # Determine which items Fry worked on.
    # Primary: read output/worked-items.txt manifest written by Fry.
    # Fallback: parse sprint names from log (less accurate — picks up mentioned IDs).
    local worked_ids=""
    local manifest="$WORKTREE_DIR/output/worked-items.txt"
    if [ -f "$manifest" ]; then
        worked_ids="$(grep -oE '^[A-Z][0-9]+$' "$manifest" | sort -u | tr '\n' ' ')"
        log "Items from manifest: ${worked_ids:-none}"
    else
        log "No manifest file — falling back to sprint name parsing"
        worked_ids="$(grep "STARTING SPRINT" "$LOG_FILE" 2>/dev/null \
            | grep -oE '[A-Z][0-9]+' \
            | sort -u \
            | tr '\n' ' ')"
        log "Items from sprint names (approximate): ${worked_ids:-none}"
    fi

    # Handle result and persist status for next run
    if [ "$build_success" = true ]; then
        handle_build_success "$worked_ids"
        echo "success" > "$LAST_BUILD_STATUS"
    else
        handle_build_failure "$worked_ids"
        echo "failure" > "$LAST_BUILD_STATUS"
    fi

    # Clean up worktree
    log "Removing worktree..."
    git -C "$REPO_DIR" worktree remove "$WORKTREE_DIR" --force 2>/dev/null || true
    WORKTREE_DIR=""

    # Push roadmap updates to master
    cd "$REPO_DIR"
    git checkout master 2>/dev/null || true
    git add "$ROADMAP"
    if ! git diff --cached --quiet; then
        git commit -m "Update roadmap after build [$DATE]"
        git push origin master || log "WARNING: roadmap push failed"
    fi
}

# Handle successful build: either merge directly or create a PR
handle_build_success() {
    local worked_ids="$1"

    if [ "$AUTO_MERGE" = true ]; then
        handle_build_auto_merge "$worked_ids"
    else
        handle_build_create_pr "$worked_ids"
    fi
}

# Auto-merge: merge worktree branch into master locally and push
handle_build_auto_merge() {
    local worked_ids="$1"
    local id_list
    id_list="$(echo "$worked_ids" | xargs | sed 's/ /, /g')"

    log "Build succeeded — auto-merging into master (${id_list})"

    # Switch to master in the main repo and merge the build branch
    cd "$REPO_DIR"
    git checkout master 2>/dev/null || true

    if ! git merge "$BUILD_BRANCH" --no-edit -m "Self-improve: ${id_list} [auto-merged]" 2>&1 | tee -a "$LOG_FILE"; then
        log "WARNING: Auto-merge failed (conflict) — falling back to PR"
        git merge --abort 2>/dev/null || true
        handle_build_create_pr "$worked_ids"
        return
    fi

    # Push to remote
    if ! git push origin master 2>&1 | tee -a "$LOG_FILE"; then
        log "WARNING: Push failed after auto-merge"
        handle_build_failure "$worked_ids"
        return
    fi

    log "Auto-merged and pushed: ${id_list}"
    PR_CREATED=true  # Prevents cleanup from deleting the branch prematurely

    # Remove worked items from roadmap (they're done — no PR to track)
    for id in $worked_ids; do
        local exists
        exists="$(jq --arg id "$id" '[.items[] | select(.id == $id)] | length' "$ROADMAP")"
        if [ "$exists" -gt 0 ]; then
            local tmp
            tmp="$(mktemp)"
            jq --arg id "$id" '.items = [.items[] | select(.id != $id)]' "$ROADMAP" > "$tmp"
            mv "$tmp" "$ROADMAP"
            log "  $id: removed from roadmap (merged)"
        fi
    done

    # Rebuild binary with merged changes
    log "Rebuilding fry with merged changes..."
    make -C "$REPO_DIR" install 2>&1 | tee -a "$LOG_FILE" || log "WARNING: post-merge make install failed"
}

# Create PR: push branch and open a pull request for review
handle_build_create_pr() {
    local worked_ids="$1"

    log "Build succeeded — pushing branch and creating PR"

    # Push the branch from the worktree
    if ! git -C "$WORKTREE_DIR" push origin "$BUILD_BRANCH" 2>&1 | tee -a "$LOG_FILE"; then
        log "WARNING: Failed to push branch $BUILD_BRANCH"
        handle_build_failure "$worked_ids"
        return
    fi

    # Build PR title from worked item IDs
    local id_list
    id_list="$(echo "$worked_ids" | xargs | sed 's/ /, /g')"
    local pr_title="Self-improve: ${id_list}"
    pr_title="${pr_title:0:70}"

    # Build PR body with details for each worked item
    local item_details=""
    for id in $worked_ids; do
        local detail
        detail="$(jq -r --arg id "$id" '
            .items[] | select(.id == $id) |
            "### \(.id): \(.title)\n\n" +
            "| | |\n|---|---|\n" +
            "| **Category** | \(.category) |\n" +
            "| **Priority** | \(.priority) |\n" +
            "| **Effort** | \(.effort) |\n" +
            "| **Files** | \(.files | join(", ")) |\n\n" +
            "**Problem:** \(.description)\n\n" +
            "**Fix:** \(.fix)\n"
        ' "$ROADMAP" 2>/dev/null)"
        if [ -n "$detail" ]; then
            item_details="${item_details}${detail}\n"
        fi
    done

    local pr_body
    pr_body="$(cat <<EOF
## Summary

Automated self-improvement build addressing ${id_list}.

## Items

$(echo -e "$item_details")

## Notes

These changes were generated by the Fry self-improvement loop. Each item has its own commit for cherry-pick flexibility.

## Test plan

- [ ] \`make test\` passes
- [ ] \`make build\` produces a working binary
- [ ] Changes match the fix descriptions in the roadmap

🤖 Generated by [Fry Self-Improvement Loop](https://github.com/yevgetman/fry)
EOF
)"

    # Create PR
    local pr_url
    if ! pr_url="$(gh pr create \
        --base master \
        --head "$BUILD_BRANCH" \
        --title "$pr_title" \
        --body "$pr_body" 2>&1)"; then
        log "WARNING: Failed to create PR: $pr_url"
        handle_build_failure "$worked_ids"
        return
    fi

    log "PR created: $pr_url"
    PR_CREATED=true

    # Update roadmap: mark worked items as pr_created
    cd "$REPO_DIR"
    for id in $worked_ids; do
        local tmp
        tmp="$(mktemp)"
        jq --arg id "$id" --arg url "$pr_url" \
            '(.items[] | select(.id == $id)) |= (.status = "pr_created" | .pr_url = $url)' \
            "$ROADMAP" > "$tmp"
        mv "$tmp" "$ROADMAP"
    done
}

# Handle failed build: only increment attempted_count for items whose sprints ran
handle_build_failure() {
    local worked_ids="$1"

    log "Build failed — updating attempted items"

    cd "$REPO_DIR"
    for id in $worked_ids; do
        # Only increment if the item exists in the roadmap
        local exists
        exists="$(jq --arg id "$id" '[.items[] | select(.id == $id)] | length' "$ROADMAP")"
        if [ "$exists" -gt 0 ]; then
            log "  $id: sprint ran — incrementing attempted_count"
            local tmp
            tmp="$(mktemp)"
            jq --arg id "$id" \
                '(.items[] | select(.id == $id)) |= (.status = "open" | .attempted_count += 1)' \
                "$ROADMAP" > "$tmp"
            mv "$tmp" "$ROADMAP"
        fi
    done
}

# ===========================================================================
# HOUSEKEEPING
# ===========================================================================

# Remove items whose PRs have been merged. Runs at startup before planning/build.
cleanup_merged_items() {
    log "Checking for merged PRs to clean from roadmap..."

    # Get all items with status pr_created and a pr_url
    local pr_items
    pr_items="$(jq -c '[.items[] | select(.status == "pr_created" and .pr_url != null)]' "$ROADMAP")"

    local count
    count="$(echo "$pr_items" | jq 'length')"
    if [ "$count" -eq 0 ]; then
        log "No pr_created items to check"
        return 0
    fi

    local removed=0
    local tmp_pr_items
    tmp_pr_items="$(mktemp)"
    echo "$pr_items" | jq -c '.[]' > "$tmp_pr_items"
    while IFS= read -r item; do
        local id pr_url
        id="$(echo "$item" | jq -r '.id')"
        pr_url="$(echo "$item" | jq -r '.pr_url')"

        # Extract PR number from URL (e.g., https://github.com/owner/repo/pull/123 → 123)
        local pr_number
        pr_number="$(echo "$pr_url" | grep -oE '[0-9]+$')"
        if [ -z "$pr_number" ]; then
            log "  $id: could not parse PR number from $pr_url — skipping"
            continue
        fi

        # Check if PR is merged via gh CLI
        local pr_state
        pr_state="$(gh pr view "$pr_number" --json state -q .state 2>/dev/null || echo "UNKNOWN")"

        if [ "$pr_state" = "MERGED" ]; then
            log "  $id: PR #$pr_number merged — removing from roadmap"
            local tmp
            tmp="$(mktemp)"
            jq --arg id "$id" '.items = [.items[] | select(.id != $id)]' "$ROADMAP" > "$tmp"
            mv "$tmp" "$ROADMAP"
            removed=$((removed + 1))
        elif [ "$pr_state" = "CLOSED" ]; then
            # PR was closed without merging — reset to open so it can be retried
            log "  $id: PR #$pr_number closed without merge — resetting to open"
            local tmp
            tmp="$(mktemp)"
            jq --arg id "$id" '(.items[] | select(.id == $id)) |= (.status = "open" | .pr_url = null)' "$ROADMAP" > "$tmp"
            mv "$tmp" "$ROADMAP"
        else
            log "  $id: PR #$pr_number state=$pr_state — keeping"
        fi
    done < "$tmp_pr_items"
    rm -f "$tmp_pr_items"

    # Commit if roadmap changed
    cd "$REPO_DIR"
    git add "$ROADMAP"
    if ! git diff --cached --quiet; then
        git commit -m "Remove merged items from roadmap [$DATE]"
        git push origin master || log "WARNING: push of roadmap cleanup failed"
    fi
}

# ===========================================================================
# MAIN
# ===========================================================================
main() {
    # Set up per-run log file
    mkdir -p "$LOG_DIR"
    RUN_LOG="$LOG_DIR/${RUN_ID}.log"

    log ""
    log "==========================================================="
    log "  Fry Self-Improvement Orchestrator"
    log "  Run log: $RUN_LOG"
    log "  Date: $DATE"
    log "==========================================================="

    check_deps
    acquire_lock
    validate_roadmap

    cd "$REPO_DIR"

    # Pull latest
    log "Pulling latest from origin..."
    git pull origin master --ff-only || die "git pull failed — resolve conflicts manually"

    # Clean up merged PRs from roadmap
    cleanup_merged_items

    # Build latest fry
    log "Building latest fry..."
    if ! make -C "$REPO_DIR" install 2>&1 | tee -a "$LOG_FILE"; then
        die "make install failed"
    fi

    # Planning phase — skip if last build failed (focus on building, not discovering)
    local last_status=""
    if [ -f "$LAST_BUILD_STATUS" ]; then
        last_status="$(cat "$LAST_BUILD_STATUS")"
    fi

    if [ "$SKIP_PLANNING" = true ]; then
        log "Skipping planning phase (--skip-planning)"
    elif [ "$last_status" = "failure" ]; then
        log "Skipping planning phase (last build failed — focusing on build)"
    elif should_run_planning; then
        run_planning_phase
    fi

    # Build phase
    if [ "$SKIP_BUILD" = true ]; then
        log "Skipping build phase (--skip-build)"
    else
        run_build_phase
    fi

    log "==========================================================="
    log "  Orchestrator complete"
    log "==========================================================="
}

# Only run main when executed directly (not when sourced for testing)
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi
