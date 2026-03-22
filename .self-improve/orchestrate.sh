#!/usr/bin/env bash
#
# Fry Self-Improvement Orchestrator
#
# Drives the automated self-improvement loop:
#   1. Planning phase — scan codebase for new issues, create GitHub Issues
#   2. Build phase — implement 2-3 approved items, push branch, create PR or auto-merge
#
# The roadmap lives in GitHub Issues. Labels encode category, status, priority, and effort.
# No local roadmap.json — GitHub Issues is the single source of truth.
#
# Dependencies: git, jq, fry, gh (GitHub CLI), make, go
# Usage: .self-improve/orchestrate.sh [--skip-planning] [--skip-build] [--dry-run] [--auto-merge]

set -euo pipefail

# --- Configuration ---
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

LOCK_FILE="$SCRIPT_DIR/.lock"
LOG_DIR="$SCRIPT_DIR/logs"
LOG_FILE="$SCRIPT_DIR/orchestrate.log"
EXECUTIVE="$SCRIPT_DIR/executive.md"
PLANNING_PROMPT="$SCRIPT_DIR/planning-prompt.md"
BUILD_PROMPT="$SCRIPT_DIR/build-prompt.md"
LAST_BUILD_STATUS="$SCRIPT_DIR/.last-build-status"

MAX_BUILD_ITEMS=3
MAX_ATTEMPTS=3          # Skip items that have failed this many times
MAX_POST_BUILD_HEALS=3  # Attempts to heal test/build failures after Fry completes
PLANNING_THRESHOLD=15   # Skip planning if this many open issues already exist
DATE="$(date +%Y-%m-%d)"
RUN_ID="$(date +%Y%m%d-%H%M%S)"

# Label constants
LABEL_SELF_IMPROVE="self-improve"
LABEL_STATUS_PROPOSED="status/proposed"
LABEL_STATUS_APPROVED="status/approved"
LABEL_MAX_ATTEMPTS="max-attempts"

# Categories that are auto-approved (safe to build without human review)
AUTO_APPROVE_CATEGORIES=("bug" "security" "testing" "documentation")

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

# --- Validate that required GitHub labels exist ---
validate_labels() {
    local required_labels=("$LABEL_SELF_IMPROVE" "$LABEL_STATUS_PROPOSED" "$LABEL_STATUS_APPROVED" "$LABEL_MAX_ATTEMPTS")
    local existing
    existing="$(gh label list --limit 100 --json name -q '.[].name')"

    local missing=()
    for label in "${required_labels[@]}"; do
        if ! echo "$existing" | grep -qx "$label"; then
            missing+=("$label")
        fi
    done

    if [ ${#missing[@]} -gt 0 ]; then
        die "Missing GitHub labels: ${missing[*]}. Run the label setup first."
    fi
}

# ===========================================================================
# GITHUB ISSUES HELPERS
# ===========================================================================

# Check if a category is auto-approved (safe to build without human review)
is_auto_approve_category() {
    local category="$1"
    local _aac
    for _aac in "${AUTO_APPROVE_CATEGORIES[@]}"; do
        if [ "$_aac" = "$category" ]; then
            return 0
        fi
    done
    return 1
}

# Count open issues (approved only — ready to build)
count_approved_items() {
    gh issue list \
        --label "$LABEL_SELF_IMPROVE" \
        --label "$LABEL_STATUS_APPROVED" \
        --state open \
        --json number \
        -q 'length'
}

# Count all open self-improve issues (proposed + approved)
count_all_open_items() {
    gh issue list \
        --label "$LABEL_SELF_IMPROVE" \
        --state open \
        --limit 500 \
        --json number \
        -q 'length'
}

# Count open issues for a specific category
count_category_items() {
    local category="$1"
    gh issue list \
        --label "$LABEL_SELF_IMPROVE" \
        --label "category/${category}" \
        --state open \
        --json number \
        -q 'length'
}

# Export all open issues to a JSON file for Fry to read.
# Format matches what the build prompt expects.
export_issues_json() {
    local output_file="$1"
    local label_filter="${2:-}"  # optional additional label filter

    local args=(
        --label "$LABEL_SELF_IMPROVE"
        --state open
        --limit 500
        --json "number,title,labels,body"
    )
    if [ -n "$label_filter" ]; then
        args+=(--label "$label_filter")
    fi

    local raw_issues
    raw_issues="$(gh issue list "${args[@]}")"

    # Transform GitHub Issues JSON into the format Fry expects
    echo "$raw_issues" | jq '
        [.[] | {
            number: .number,
            title: .title,
            category: (
                [.labels[].name | select(startswith("category/"))] |
                if length > 0 then .[0] | ltrimstr("category/") else "unknown" end
            ),
            priority: (
                [.labels[].name | select(startswith("priority/"))] |
                if length > 0 then .[0] | ltrimstr("priority/") else "medium" end
            ),
            effort: (
                [.labels[].name | select(startswith("effort/"))] |
                if length > 0 then .[0] | ltrimstr("effort/") else "medium" end
            ),
            status: (
                if ([.labels[].name] | index("status/approved")) then "approved"
                elif ([.labels[].name] | index("status/proposed")) then "proposed"
                else "open" end
            ),
            max_attempts: (
                [.labels[].name] | index("max-attempts") | if . then true else false end
            ),
            description: (
                .body |
                if . then
                    # Extract Problem section
                    (capture("## Problem\n(?<desc>[\\s\\S]*?)(\n## |$)") | .desc | gsub("^\\s+|\\s+$"; "")) // .
                else "" end
            ),
            fix: (
                .body |
                if . then
                    # Extract Fix Plan section
                    (capture("## Fix Plan\n(?<fix>[\\s\\S]*?)(\n## |\n---|\n_Managed|$)") | .fix | gsub("^\\s+|\\s+$"; "")) // ""
                else "" end
            ),
            files: (
                .body |
                if . then
                    # Extract Files line
                    (capture("\\*\\*Files:\\*\\* (?<files>[^\n]+)") | .files | split(", ") | map(gsub("`"; ""))) // []
                else [] end
            )
        }]
    ' > "$output_file"
}

# Count how many "Build attempt failed" comments exist on an issue
count_failed_attempts() {
    local issue_number="$1"
    gh issue view "$issue_number" \
        --json comments \
        -q '[.comments[] | select(.body | startswith("Build attempt failed"))] | length' \
        2>/dev/null || echo "0"
}

# Create a GitHub issue from a finding JSON object
create_issue_from_finding() {
    local finding="$1"

    local title category priority effort description fix files_str

    title="$(echo "$finding" | jq -r '.title')"
    category="$(echo "$finding" | jq -r '.category')"
    priority="$(echo "$finding" | jq -r '.priority // "medium"')"
    effort="$(echo "$finding" | jq -r '.effort // "medium"')"
    description="$(echo "$finding" | jq -r '.description // ""')"
    fix="$(echo "$finding" | jq -r '.fix // ""')"
    files_str="$(echo "$finding" | jq -r '(.files // []) | map("`" + . + "`") | join(", ")')"

    # Check for duplicate — skip if an open issue with the same title exists
    local existing_count
    existing_count="$(gh issue list \
        --label "$LABEL_SELF_IMPROVE" \
        --label "category/${category}" \
        --state open \
        --search "$title" \
        --json number \
        -q 'length' 2>/dev/null || echo "0")"
    if [ "$existing_count" -gt 0 ]; then
        log "  Skipping duplicate: $title"
        return 0
    fi

    # Build labels
    local labels="${LABEL_SELF_IMPROVE},category/${category},priority/${priority},effort/${effort}"

    # Auto-approve or propose based on category
    if is_auto_approve_category "$category"; then
        labels="${labels},${LABEL_STATUS_APPROVED}"
    else
        labels="${labels},${LABEL_STATUS_PROPOSED}"
    fi

    # Build issue body
    local body
    body="$(cat <<EOF
**Priority:** ${priority}
**Effort:** ${effort}
**Files:** ${files_str}

## Problem
${description}

## Fix Plan
${fix}

---
_Managed by [Fry Self-Improvement Pipeline](docs/self-improvement.md)_
EOF
)"

    if [ "$DRY_RUN" = true ]; then
        log "  [DRY RUN] Would create issue: $title (${category}, ${priority}, ${effort})"
        return 0
    fi

    local issue_url
    issue_url="$(gh issue create \
        --title "[Fry] ${title}" \
        --body "$body" \
        --label "$labels" 2>&1)"

    if [ $? -eq 0 ]; then
        local status_label="approved"
        is_auto_approve_category "$category" || status_label="proposed"
        log "  Created #$(basename "$issue_url"): $title (${category}, ${status_label})"
    else
        log "  WARNING: Failed to create issue: $title — $issue_url"
    fi
}

# ===========================================================================
# PLANNING PHASE
# ===========================================================================

# Decide whether planning is needed.
# Returns 0 (true) if planning should run, 1 (false) if not.
should_run_planning() {
    local open_count
    open_count="$(count_all_open_items)"

    # Hard ceiling — too many items already
    if [ "$open_count" -ge "$PLANNING_THRESHOLD" ]; then
        log "Planning not needed: $open_count open issues (threshold: $PLANNING_THRESHOLD)"
        return 1
    fi

    # Running low — always plan
    if [ "$open_count" -lt 5 ]; then
        log "Planning needed: only $open_count open issues (< 5)"
        return 0
    fi

    # Check core category coverage (bug, testing, feature, improvement)
    local empty_core=0
    for cat in bug testing feature improvement; do
        local cat_count
        cat_count="$(count_category_items "$cat")"
        if [ "$cat_count" -eq 0 ]; then
            empty_core=$((empty_core + 1))
        fi
    done
    if [ "$empty_core" -ge 2 ]; then
        log "Planning needed: $empty_core core categories have zero items"
        return 0
    fi

    # Check for imbalance — any category > 50% of all items
    local categories
    categories="$(gh issue list \
        --label "$LABEL_SELF_IMPROVE" \
        --state open \
        --limit 500 \
        --json labels \
        -q '[.[].labels[].name | select(startswith("category/"))]')"

    local imbalanced
    imbalanced="$(echo "$categories" | jq --argjson total "$open_count" '
        group_by(.) | any(length > ($total / 2))
    ')"
    if [ "$imbalanced" = "true" ]; then
        log "Planning needed: issues are imbalanced (one category > 50%)"
        return 0
    fi

    log "Planning not needed: $open_count issues, balanced across categories"
    return 1
}

run_planning_phase() {
    log "--- Planning Phase ---"

    # Redundant guard
    local open_count
    open_count="$(count_all_open_items)"
    if [ "$open_count" -ge "$PLANNING_THRESHOLD" ]; then
        log "Skipping planning: $open_count open issues already (threshold: $PLANNING_THRESHOLD)"
        return 0
    fi

    # Clean stale artifacts from any prior run, then scaffold
    rm -rf "$REPO_DIR/.fry" "$REPO_DIR/plans" "$REPO_DIR/assets" "$REPO_DIR/output"
    mkdir -p "$REPO_DIR/plans" "$REPO_DIR/assets" "$REPO_DIR/output"
    cp "$EXECUTIVE" "$REPO_DIR/plans/executive.md"

    # Export all open issues so Fry knows what's already tracked
    log "Exporting existing issues for deduplication..."
    export_issues_json "$REPO_DIR/assets/existing-issues.json"
    log "  Exported $(jq 'length' "$REPO_DIR/assets/existing-issues.json") existing issues"

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

    log "Found $count new item(s) — creating GitHub Issues"
    create_issues_from_findings "$findings_file"

    # Clean up scaffolding
    rm -rf "$REPO_DIR/.fry" "$REPO_DIR/plans" "$REPO_DIR/assets" "$REPO_DIR/output"
}

# Create GitHub Issues from a findings JSON array
create_issues_from_findings() {
    local findings_file="$1"

    local tmp_findings
    tmp_findings="$(mktemp)"
    jq -c '.[]' "$findings_file" > "$tmp_findings"

    while IFS= read -r item; do
        create_issue_from_finding "$item"
    done < "$tmp_findings"

    rm -f "$tmp_findings"
}

# ===========================================================================
# BUILD PHASE
# ===========================================================================
run_build_phase() {
    log "--- Build Phase ---"

    # Check there are approved items to work on (excluding max-attempts)
    local approved_issues
    approved_issues="$(gh issue list \
        --label "$LABEL_SELF_IMPROVE" \
        --label "$LABEL_STATUS_APPROVED" \
        --state open \
        --limit 500 \
        --json "number,title,labels")"

    # Filter out max-attempts issues
    local buildable_issues
    buildable_issues="$(echo "$approved_issues" | jq '
        [.[] | select(
            [.labels[].name] | index("max-attempts") | not
        )]
    ')"

    local approved_count
    approved_count="$(echo "$buildable_issues" | jq 'length')"
    if [ "$approved_count" -eq 0 ]; then
        log "No approved items to build — skipping build"
        return 0
    fi
    log "$approved_count approved item(s) — Fry will choose what to work on"

    if [ "$DRY_RUN" = true ]; then
        log "[DRY RUN] Would run Fry build"
        echo "$buildable_issues" | jq -r '.[] | "  #\(.number): \(.title)"'
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

    # Scaffold worktree — export approved items for Fry to read
    mkdir -p "$WORKTREE_DIR/plans" "$WORKTREE_DIR/assets"
    cp "$EXECUTIVE" "$WORKTREE_DIR/plans/executive.md"

    log "Exporting approved items for build..."
    export_issues_json "$WORKTREE_DIR/assets/approved-items.json" "$LABEL_STATUS_APPROVED"
    local exported_count
    exported_count="$(jq 'length' "$WORKTREE_DIR/assets/approved-items.json")"
    log "  Exported $exported_count approved items"

    # Run Fry build — full prepare so the LLM reads assets/approved-items.json
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

                # Capture the diff of what changed
                local diff_output
                diff_output="$(cd "$WORKTREE_DIR" && git diff HEAD~1 --stat 2>/dev/null)" || true

                # Write the heal prompt to a temp file
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
    # Primary: read output/worked-items.txt manifest written by Fry (issue numbers).
    # Fallback: parse sprint names from log for issue numbers.
    local worked_numbers=""
    local manifest="$WORKTREE_DIR/output/worked-items.txt"
    if [ -f "$manifest" ]; then
        worked_numbers="$(grep -oE '^[0-9]+$' "$manifest" | sort -u | tr '\n' ' ')"
        log "Items from manifest: ${worked_numbers:-none}"
    else
        log "No manifest file — falling back to sprint name parsing"
        worked_numbers="$(grep "STARTING SPRINT" "$LOG_FILE" 2>/dev/null \
            | grep -oE '#[0-9]+' \
            | tr -d '#' \
            | sort -u \
            | tr '\n' ' ')"
        log "Items from sprint names (approximate): ${worked_numbers:-none}"
    fi

    # Handle result and persist status for next run
    if [ "$build_success" = true ]; then
        handle_build_success "$worked_numbers"
        echo "success" > "$LAST_BUILD_STATUS"
    else
        handle_build_failure "$worked_numbers"
        echo "failure" > "$LAST_BUILD_STATUS"
    fi

    # Clean up worktree
    log "Removing worktree..."
    git -C "$REPO_DIR" worktree remove "$WORKTREE_DIR" --force 2>/dev/null || true
    WORKTREE_DIR=""

    # Ensure we're back on master
    cd "$REPO_DIR"
    git checkout master 2>/dev/null || true
}

# Handle successful build: either merge directly or create a PR
handle_build_success() {
    local worked_numbers="$1"

    if [ "$AUTO_MERGE" = true ]; then
        handle_build_auto_merge "$worked_numbers"
    else
        handle_build_create_pr "$worked_numbers"
    fi
}

# Auto-merge: merge worktree branch into master locally and push
handle_build_auto_merge() {
    local worked_numbers="$1"
    local num_list
    num_list="$(echo "$worked_numbers" | xargs | sed 's/ /, #/g; s/^/#/')"

    log "Build succeeded — auto-merging into master (${num_list})"

    # Switch to master in the main repo and merge the build branch
    cd "$REPO_DIR"
    git checkout master 2>/dev/null || true

    if ! git merge "$BUILD_BRANCH" --no-edit -m "Self-improve: ${num_list} [auto-merged]" 2>&1 | tee -a "$LOG_FILE"; then
        log "WARNING: Auto-merge failed (conflict) — falling back to PR"
        git merge --abort 2>/dev/null || true
        handle_build_create_pr "$worked_numbers"
        return
    fi

    # Push to remote
    if ! git push origin master 2>&1 | tee -a "$LOG_FILE"; then
        log "WARNING: Push failed after auto-merge"
        handle_build_failure "$worked_numbers"
        return
    fi

    log "Auto-merged and pushed: ${num_list}"
    PR_CREATED=true  # Prevents cleanup from deleting the branch prematurely

    # Close worked issues
    for num in $worked_numbers; do
        gh issue close "$num" \
            --comment "Implemented and auto-merged to master. [Fry Self-Improvement]" \
            2>/dev/null || log "  WARNING: Failed to close issue #$num"
        log "  #$num: closed (merged)"
    done

    # Rebuild binary with merged changes
    log "Rebuilding fry with merged changes..."
    make -C "$REPO_DIR" install 2>&1 | tee -a "$LOG_FILE" || log "WARNING: post-merge make install failed"
}

# Create PR: push branch and open a pull request for review
handle_build_create_pr() {
    local worked_numbers="$1"

    log "Build succeeded — pushing branch and creating PR"

    # Push the branch from the worktree
    if ! git -C "$WORKTREE_DIR" push origin "$BUILD_BRANCH" 2>&1 | tee -a "$LOG_FILE"; then
        log "WARNING: Failed to push branch $BUILD_BRANCH"
        handle_build_failure "$worked_numbers"
        return
    fi

    # Build PR title
    local num_list
    num_list="$(echo "$worked_numbers" | xargs | sed 's/ /, #/g; s/^/#/')"
    local pr_title="Self-improve: ${num_list}"
    pr_title="${pr_title:0:70}"

    # Build PR body with details for each worked issue
    local item_details=""
    local closes_clause=""
    for num in $worked_numbers; do
        local issue_info
        issue_info="$(gh issue view "$num" --json title,labels,body 2>/dev/null)" || continue

        local title category priority effort description fix
        title="$(echo "$issue_info" | jq -r '.title')"
        category="$(echo "$issue_info" | jq -r '
            [.labels[].name | select(startswith("category/"))] |
            if length > 0 then .[0] | ltrimstr("category/") else "unknown" end
        ')"
        priority="$(echo "$issue_info" | jq -r '
            [.labels[].name | select(startswith("priority/"))] |
            if length > 0 then .[0] | ltrimstr("priority/") else "medium" end
        ')"
        effort="$(echo "$issue_info" | jq -r '
            [.labels[].name | select(startswith("effort/"))] |
            if length > 0 then .[0] | ltrimstr("effort/") else "medium" end
        ')"

        item_details="${item_details}### #${num}: ${title}\n\n"
        item_details="${item_details}| | |\n|---|---|\n"
        item_details="${item_details}| **Category** | ${category} |\n"
        item_details="${item_details}| **Priority** | ${priority} |\n"
        item_details="${item_details}| **Effort** | ${effort} |\n\n"

        closes_clause="${closes_clause}Closes #${num}\n"
    done

    local pr_body
    pr_body="$(cat <<EOF
## Summary

Automated self-improvement build addressing ${num_list}.

## Items

$(echo -e "$item_details")

$(echo -e "$closes_clause")

## Test plan

- [ ] \`make test\` passes
- [ ] \`make build\` produces a working binary

Generated by [Fry Self-Improvement Loop](https://github.com/yevgetman/fry)
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
        handle_build_failure "$worked_numbers"
        return
    fi

    log "PR created: $pr_url"
    PR_CREATED=true
}

# Handle failed build: comment on issues and potentially add max-attempts label
handle_build_failure() {
    local worked_numbers="$1"

    log "Build failed — commenting on worked issues"

    for num in $worked_numbers; do
        # Add failure comment
        gh issue comment "$num" \
            --body "Build attempt failed on ${DATE} (run ${RUN_ID}). The orchestrator will retry on the next run." \
            2>/dev/null || log "  WARNING: Failed to comment on issue #$num"

        # Check if we've hit max attempts
        local attempt_count
        attempt_count="$(count_failed_attempts "$num")"
        if [ "$attempt_count" -ge "$MAX_ATTEMPTS" ]; then
            log "  #$num: $attempt_count failed attempts — adding max-attempts label"
            gh issue edit "$num" --add-label "$LABEL_MAX_ATTEMPTS" 2>/dev/null \
                || log "  WARNING: Failed to add max-attempts label to #$num"
        else
            log "  #$num: attempt $attempt_count/$MAX_ATTEMPTS"
        fi
    done
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
    validate_labels

    cd "$REPO_DIR"

    # Pull latest
    log "Pulling latest from origin..."
    git pull origin master --ff-only || die "git pull failed — resolve conflicts manually"

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
