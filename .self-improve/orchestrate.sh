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
LOG_FILE="$SCRIPT_DIR/orchestrate.log"
ROADMAP="$SCRIPT_DIR/roadmap.json"
EXECUTIVE="$SCRIPT_DIR/executive.md"
PLANNING_PROMPT="$SCRIPT_DIR/planning-prompt.md"
BUILD_PROMPT="$SCRIPT_DIR/build-prompt.md"

MAX_BUILD_ITEMS=3
MAX_ATTEMPTS=3          # Skip items that have failed this many times
PLANNING_THRESHOLD=20   # Skip planning if this many open items already exist
DATE="$(date +%Y-%m-%d)"
RUN_ID="$(date +%Y%m%d-%H%M%S)"

# Flags
SKIP_PLANNING=false
SKIP_BUILD=false
DRY_RUN=false

# State (used by cleanup trap)
WORKTREE_DIR=""
BUILD_BRANCH=""
PR_CREATED=false
INJECTED_PROMPT=""

# --- Argument parsing ---
for arg in "$@"; do
    case "$arg" in
        --skip-planning) SKIP_PLANNING=true ;;
        --skip-build)    SKIP_BUILD=true ;;
        --dry-run)       DRY_RUN=true ;;
        --help|-h)
            echo "Usage: orchestrate.sh [--skip-planning] [--skip-build] [--dry-run]"
            exit 0
            ;;
        *) echo "Unknown argument: $arg" >&2; exit 1 ;;
    esac
done

# --- Logging ---
log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $*" | tee -a "$LOG_FILE"
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

    # Remove temporary injected prompt
    if [ -n "$INJECTED_PROMPT" ] && [ -f "$INJECTED_PROMPT" ]; then
        rm -f "$INJECTED_PROMPT"
    fi

    # Clean up scaffolding directories (if they were created by us)
    rm -rf "$REPO_DIR/plans" "$REPO_DIR/assets" "$REPO_DIR/output"

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

# ===========================================================================
# PLANNING PHASE
# ===========================================================================
run_planning_phase() {
    log "--- Planning Phase ---"

    # Skip if enough open items already
    local open_count
    open_count="$(count_open_items)"
    if [ "$open_count" -ge "$PLANNING_THRESHOLD" ]; then
        log "Skipping planning: $open_count open items already (threshold: $PLANNING_THRESHOLD)"
        return 0
    fi

    # Scaffold directories
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

    # Process findings
    local findings_file="$REPO_DIR/output/new-findings.json"
    if [ ! -f "$findings_file" ]; then
        log "No findings file produced"
        rm -rf "$REPO_DIR/plans" "$REPO_DIR/assets" "$REPO_DIR/output"
        return 0
    fi

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

        # Find next available number for this prefix
        local max_num
        max_num="$(jq -r --arg p "$prefix" \
            '[.items[] | select(.id | startswith($p)) | .id | ltrimstr($p) | tonumber] | max // 0' \
            "$ROADMAP")"
        local next_num=$((max_num + 1))
        local new_id="${prefix}${next_num}"

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

    # Select items
    local selected
    selected="$(select_items)"

    local count
    count="$(echo "$selected" | jq 'length')"
    if [ "$count" -eq 0 ]; then
        log "No eligible items to work on"
        return 0
    fi

    log "Selected $count item(s) for build:"
    echo "$selected" | jq -r '.[] | "  \(.id): \(.title) [priority=\(.priority), effort=\(.effort)]"' | tee -a "$LOG_FILE"

    if [ "$DRY_RUN" = true ]; then
        log "[DRY RUN] Would build these items"
        return 0
    fi

    # Mark items as in_progress
    mark_items_status "$selected" "in_progress"

    # Commit status change
    cd "$REPO_DIR"
    git add "$ROADMAP"
    if ! git diff --cached --quiet; then
        git commit -m "Mark items as in_progress for build [$DATE]"
        git push origin master || log "WARNING: push of in_progress status failed"
    fi

    # Determine branch name from category and date
    local category
    category="$(echo "$selected" | jq -r '.[0].category')"
    BUILD_BRANCH="self-improve/${RUN_ID}-${category}"

    # Clean up any prior branch with this name
    if git -C "$REPO_DIR" rev-parse --verify "refs/heads/$BUILD_BRANCH" &>/dev/null; then
        log "Branch $BUILD_BRANCH already exists — deleting"
        git -C "$REPO_DIR" branch -D "$BUILD_BRANCH"
    fi

    # Create worktree
    WORKTREE_DIR="$REPO_DIR/.fry-worktrees/build-${RUN_ID}-${category}"
    if [ -d "$WORKTREE_DIR" ]; then
        log "Worktree dir already exists — removing"
        git -C "$REPO_DIR" worktree remove "$WORKTREE_DIR" --force 2>/dev/null || rm -rf "$WORKTREE_DIR"
        git -C "$REPO_DIR" worktree prune
    fi

    log "Creating worktree at $WORKTREE_DIR on branch $BUILD_BRANCH"
    mkdir -p "$(dirname "$WORKTREE_DIR")"
    git -C "$REPO_DIR" worktree add "$WORKTREE_DIR" -b "$BUILD_BRANCH"

    # Scaffold worktree
    mkdir -p "$WORKTREE_DIR/plans" "$WORKTREE_DIR/assets"
    cp "$EXECUTIVE" "$WORKTREE_DIR/plans/executive.md"
    cp "$ROADMAP" "$WORKTREE_DIR/assets/roadmap.json"

    # Create build prompt with injected items
    INJECTED_PROMPT="$(mktemp)"
    inject_items_into_prompt "$selected" "$INJECTED_PROMPT"

    # Run Fry build
    log "Running Fry build in worktree..."
    local build_success=true
    if ! fry run \
        --user-prompt-file "$INJECTED_PROMPT" \
        --always-verify \
        --no-sanity-check \
        --git-strategy current \
        --project-dir "$WORKTREE_DIR" 2>&1 | tee -a "$LOG_FILE"; then
        log "WARNING: Fry build run failed"
        build_success=false
    fi

    # Post-build verification (belt and suspenders)
    if [ "$build_success" = true ]; then
        log "Running post-build verification (make test && make build)..."
        if ! (cd "$WORKTREE_DIR" && make test && make build) 2>&1 | tee -a "$LOG_FILE"; then
            log "WARNING: Post-build verification failed"
            build_success=false
        fi
    fi

    # Handle result
    if [ "$build_success" = true ]; then
        handle_build_success "$selected"
    else
        handle_build_failure "$selected"
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

# Select 2-3 open items: 1 bigger item (feature/improvement) + 2 smaller items
# (bug, testing, sunset, refactor, security, ui_ux, documentation).
# Falls back to whatever is available if one pool is empty.
# Skips items that have failed too many times.
select_items() {
    jq --argjson max_attempts "$MAX_ATTEMPTS" '
        def priority_rank:
            if . == "high" then 0
            elif . == "medium" then 1
            else 2
            end;

        def is_big: .category == "feature" or .category == "improvement";

        # Filter: open, not exceeded max attempts
        [.items[] | select(.status == "open" and .attempted_count < $max_attempts)]
        | sort_by(.priority | priority_rank) as $eligible

        # Split into big (feature/improvement) and small (everything else)
        | [$eligible[] | select(is_big)] as $big
        | [$eligible[] | select(is_big | not)] as $small

        # Pick 1 big + up to 2 small. Fall back if a pool is empty.
        | if ($big | length) > 0 and ($small | length) >= 2 then
            [$big[0]] + $small[:2]
          elif ($big | length) > 0 and ($small | length) == 1 then
            [$big[0]] + $small[:1]
          elif ($big | length) > 0 then
            $big[:3]
          elif ($small | length) > 0 then
            $small[:3]
          else
            []
          end
    ' "$ROADMAP"
}

# Inject selected items into the build prompt, replacing the placeholder
inject_items_into_prompt() {
    local selected="$1"
    local output_file="$2"

    # Format items as markdown and write to a temp file.
    # Using a file avoids awk -v backslash interpretation issues
    # when descriptions contain \t, \n, or other escape sequences.
    local items_file
    items_file="$(mktemp)"
    echo "$selected" | jq -r '.[] |
        "### \(.id). \(.title)",
        "- **Category:** \(.category)",
        "- **Priority:** \(.priority)",
        "- **Files:** \(.files | join(", "))",
        "- **Description:** \(.description)",
        "- **Fix:** \(.fix)",
        ""
    ' > "$items_file"

    # Replace the placeholder line with the items file content
    awk -v items_file="$items_file" '
        /<!-- ORCHESTRATOR:/ {
            while ((getline line < items_file) > 0) print line
            close(items_file)
            next
        }
        { print }
    ' "$BUILD_PROMPT" > "$output_file"

    rm -f "$items_file"
}

# Update status for selected items in roadmap
mark_items_status() {
    local selected="$1"
    local new_status="$2"

    local ids
    ids="$(echo "$selected" | jq -r '.[].id')"
    for id in $ids; do
        local tmp
        tmp="$(mktemp)"
        jq --arg id "$id" --arg status "$new_status" \
            '(.items[] | select(.id == $id)).status = $status' \
            "$ROADMAP" > "$tmp"
        mv "$tmp" "$ROADMAP"
    done
}

# Handle successful build: push branch, create PR, update roadmap
handle_build_success() {
    local selected="$1"

    log "Build succeeded — pushing branch and creating PR"

    # Push the branch from the worktree
    if ! git -C "$WORKTREE_DIR" push origin "$BUILD_BRANCH" 2>&1 | tee -a "$LOG_FILE"; then
        log "WARNING: Failed to push branch $BUILD_BRANCH"
        handle_build_failure "$selected"
        return
    fi

    # Build PR title and body
    local item_ids
    item_ids="$(echo "$selected" | jq -r '[.[].id] | join(", ")')"
    local category
    category="$(echo "$selected" | jq -r '.[0].category')"
    local pr_title="Self-improve: ${category} fixes (${item_ids})"
    pr_title="${pr_title:0:70}"

    local item_details
    item_details="$(echo "$selected" | jq -r '.[] |
        "### \(.id): \(.title)",
        "",
        "| | |",
        "|---|---|",
        "| **Category** | \(.category) |",
        "| **Priority** | \(.priority) |",
        "| **Effort** | \(.effort) |",
        "| **Files** | \(.files | join(", ")) |",
        "",
        "**Problem:** \(.description)",
        "",
        "**Fix:** \(.fix)",
        ""
    ')"

    local pr_body
    pr_body="$(cat <<EOF
## Summary

Automated self-improvement build addressing ${item_ids}.

## Items

${item_details}

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
        handle_build_failure "$selected"
        return
    fi

    log "PR created: $pr_url"
    PR_CREATED=true

    # Update roadmap: status → pr_created, set pr_url
    cd "$REPO_DIR"
    local ids
    ids="$(echo "$selected" | jq -r '.[].id')"
    for id in $ids; do
        local tmp
        tmp="$(mktemp)"
        jq --arg id "$id" --arg url "$pr_url" \
            '(.items[] | select(.id == $id)) |= (.status = "pr_created" | .pr_url = $url)' \
            "$ROADMAP" > "$tmp"
        mv "$tmp" "$ROADMAP"
    done
}

# Handle failed build: reset status to open, increment attempted_count
handle_build_failure() {
    local selected="$1"

    log "Build failed — resetting item status to open"

    cd "$REPO_DIR"
    local ids
    ids="$(echo "$selected" | jq -r '.[].id')"
    for id in $ids; do
        local tmp
        tmp="$(mktemp)"
        jq --arg id "$id" \
            '(.items[] | select(.id == $id)) |= (.status = "open" | .attempted_count += 1)' \
            "$ROADMAP" > "$tmp"
        mv "$tmp" "$ROADMAP"
    done
}

# ===========================================================================
# MAIN
# ===========================================================================
main() {
    log ""
    log "==========================================================="
    log "  Fry Self-Improvement Orchestrator"
    log "  Date: $DATE"
    log "==========================================================="

    check_deps
    acquire_lock
    validate_roadmap

    cd "$REPO_DIR"

    # Pull latest
    log "Pulling latest from origin..."
    git pull origin master --ff-only || die "git pull failed — resolve conflicts manually"

    # Build latest fry
    log "Building latest fry..."
    if ! make -C "$REPO_DIR" install 2>&1 | tee -a "$LOG_FILE"; then
        die "make install failed"
    fi

    # Planning phase
    if [ "$SKIP_PLANNING" = true ]; then
        log "Skipping planning phase (--skip-planning)"
    else
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
