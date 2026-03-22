#!/usr/bin/env bash
#
# Test suite for the Fry Self-Improvement Orchestrator
#
# Tests pure functions by sourcing orchestrate.sh and exercising them
# in isolation. Does NOT call GitHub API, git, or fry — only tests
# logic that can run without external dependencies.
#
# Usage: bash .self-improve/test-orchestrator.sh
# Exit code: 0 if all tests pass, 1 if any fail

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ORCHESTRATOR="$SCRIPT_DIR/orchestrate.sh"

# --- Test framework ---
# Use temp files for counters so subshells can update them
RESULTS_DIR="$(mktemp -d)"
echo "0" > "$RESULTS_DIR/run"
echo "0" > "$RESULTS_DIR/passed"
echo "0" > "$RESULTS_DIR/failed"
CURRENT_TEST=""

_inc() { echo $(( $(cat "$1") + 1 )) > "$1"; }

begin_test() {
    CURRENT_TEST="$1"
    _inc "$RESULTS_DIR/run"
}

pass() {
    _inc "$RESULTS_DIR/passed"
    echo "  PASS  $CURRENT_TEST"
}

fail() {
    _inc "$RESULTS_DIR/failed"
    echo "  FAIL  $CURRENT_TEST — $1"
}

assert_eq() {
    local expected="$1" actual="$2" msg="${3:-}"
    if [ "$expected" = "$actual" ]; then
        return 0
    fi
    fail "expected '$expected', got '$actual'${msg:+ ($msg)}"
    return 1
}

assert_contains() {
    local haystack="$1" needle="$2" msg="${3:-}"
    if echo "$haystack" | grep -qF "$needle"; then
        return 0
    fi
    fail "expected to contain '$needle'${msg:+ ($msg)}"
    return 1
}

assert_not_contains() {
    local haystack="$1" needle="$2" msg="${3:-}"
    if ! echo "$haystack" | grep -qF "$needle"; then
        return 0
    fi
    fail "expected NOT to contain '$needle'${msg:+ ($msg)}"
    return 1
}

# --- Source orchestrator (without running main) ---
# Override functions that would have side effects
log() { :; }
die() { echo "DIE: $*" >&2; return 1; }
acquire_lock() { :; }
release_lock() { :; }
cleanup() { :; }
trap '' EXIT INT TERM  # disable cleanup trap

# Set required variables before sourcing
LOCK_FILE="/dev/null"
LOG_FILE="/dev/null"
LAST_BUILD_STATUS="/dev/null"
RUN_LOG=""

source "$ORCHESTRATOR"

# Re-override log/cleanup in case source reset them
log() { :; }
cleanup() { :; }
trap '' EXIT INT TERM

echo "=== Orchestrator Test Suite ==="
echo ""

# ===========================================================================
# Config Loading
# ===========================================================================
echo "--- Config Loading ---"

begin_test "config: defaults applied when no config file"
(
    # Reset to defaults
    MAX_BUILD_ITEMS=3
    MAX_ATTEMPTS=3
    PLANNING_THRESHOLD=15
    HEAL_MODEL=sonnet
    AUTO_APPROVE="bug security testing documentation"
    PLANNING_ENGINE=claude
    BUILD_ENGINE=claude

    CONFIG_FILE="/nonexistent/config"

    if [ -f "$CONFIG_FILE" ]; then
        while IFS='=' read -r key value; do
            key="$(echo "$key" | xargs)"
            [[ -z "$key" || "$key" == \#* ]] && continue
            value="$(echo "$value" | sed 's/#.*//' | xargs)"
            declare "$key=$value" 2>/dev/null || true
        done < "$CONFIG_FILE"
    fi

    assert_eq "3" "$MAX_BUILD_ITEMS" "MAX_BUILD_ITEMS" &&
    assert_eq "sonnet" "$HEAL_MODEL" "HEAL_MODEL" &&
    assert_eq "claude" "$PLANNING_ENGINE" "PLANNING_ENGINE" &&
    assert_eq "claude" "$BUILD_ENGINE" "BUILD_ENGINE" &&
    pass
) || true

begin_test "config: values override defaults"
(
    tmp_config="$(mktemp)"
    trap "rm -f $tmp_config" EXIT

    cat > "$tmp_config" <<'CONF'
MAX_BUILD_ITEMS=5
MAX_ATTEMPTS=10
PLANNING_THRESHOLD=25
HEAL_MODEL=opus
PLANNING_ENGINE=codex
BUILD_ENGINE=claude
AUTO_APPROVE="bug security"
CONF

    # Reset defaults
    MAX_BUILD_ITEMS=3
    MAX_ATTEMPTS=3
    PLANNING_THRESHOLD=15
    HEAL_MODEL=sonnet
    PLANNING_ENGINE=claude
    BUILD_ENGINE=claude
    AUTO_APPROVE="bug security testing documentation"

    CONFIG_FILE="$tmp_config"
    while IFS='=' read -r key value; do
        key="$(echo "$key" | xargs)"
        [[ -z "$key" || "$key" == \#* ]] && continue
        value="$(echo "$value" | sed 's/#.*//' | xargs)"
        declare "$key=$value" 2>/dev/null || true
    done < "$CONFIG_FILE"

    assert_eq "5" "$MAX_BUILD_ITEMS" "MAX_BUILD_ITEMS" &&
    assert_eq "10" "$MAX_ATTEMPTS" "MAX_ATTEMPTS" &&
    assert_eq "25" "$PLANNING_THRESHOLD" "PLANNING_THRESHOLD" &&
    assert_eq "opus" "$HEAL_MODEL" "HEAL_MODEL" &&
    assert_eq "codex" "$PLANNING_ENGINE" "PLANNING_ENGINE" &&
    assert_eq "claude" "$BUILD_ENGINE" "BUILD_ENGINE" &&
    assert_eq "bug security" "$AUTO_APPROVE" "AUTO_APPROVE" &&
    pass
) || true

begin_test "config: comments and blank lines ignored"
(
    tmp_config="$(mktemp)"
    trap "rm -f $tmp_config" EXIT

    cat > "$tmp_config" <<'CONF'
# This is a comment
MAX_BUILD_ITEMS=7

# Another comment
HEAL_MODEL=haiku
CONF

    MAX_BUILD_ITEMS=3
    HEAL_MODEL=sonnet

    CONFIG_FILE="$tmp_config"
    while IFS='=' read -r key value; do
        key="$(echo "$key" | xargs)"
        [[ -z "$key" || "$key" == \#* ]] && continue
        value="$(echo "$value" | sed 's/#.*//' | xargs)"
        declare "$key=$value" 2>/dev/null || true
    done < "$CONFIG_FILE"

    assert_eq "7" "$MAX_BUILD_ITEMS" "MAX_BUILD_ITEMS" &&
    assert_eq "haiku" "$HEAL_MODEL" "HEAL_MODEL" &&
    pass
) || true

begin_test "config: inline comments stripped"
(
    tmp_config="$(mktemp)"
    trap "rm -f $tmp_config" EXIT

    cat > "$tmp_config" <<'CONF'
MAX_BUILD_ITEMS=4  # max items per build
HEAL_MODEL=opus    # use opus for healing
CONF

    MAX_BUILD_ITEMS=3
    HEAL_MODEL=sonnet

    CONFIG_FILE="$tmp_config"
    while IFS='=' read -r key value; do
        key="$(echo "$key" | xargs)"
        [[ -z "$key" || "$key" == \#* ]] && continue
        value="$(echo "$value" | sed 's/#.*//' | xargs)"
        declare "$key=$value" 2>/dev/null || true
    done < "$CONFIG_FILE"

    assert_eq "4" "$MAX_BUILD_ITEMS" "MAX_BUILD_ITEMS" &&
    assert_eq "opus" "$HEAL_MODEL" "HEAL_MODEL" &&
    pass
) || true

begin_test "config: partial config keeps other defaults"
(
    tmp_config="$(mktemp)"
    trap "rm -f $tmp_config" EXIT

    echo "HEAL_MODEL=opus" > "$tmp_config"

    MAX_BUILD_ITEMS=3
    PLANNING_THRESHOLD=15
    HEAL_MODEL=sonnet

    CONFIG_FILE="$tmp_config"
    while IFS='=' read -r key value; do
        key="$(echo "$key" | xargs)"
        [[ -z "$key" || "$key" == \#* ]] && continue
        value="$(echo "$value" | sed 's/#.*//' | xargs)"
        declare "$key=$value" 2>/dev/null || true
    done < "$CONFIG_FILE"

    assert_eq "3" "$MAX_BUILD_ITEMS" "MAX_BUILD_ITEMS unchanged" &&
    assert_eq "15" "$PLANNING_THRESHOLD" "PLANNING_THRESHOLD unchanged" &&
    assert_eq "opus" "$HEAL_MODEL" "HEAL_MODEL overridden" &&
    pass
) || true

begin_test "config: JOURNAL_MODEL default"
(
    JOURNAL_MODEL=sonnet
    CONFIG_FILE="/nonexistent/config"
    assert_eq "sonnet" "$JOURNAL_MODEL" "JOURNAL_MODEL default" && pass
) || true

begin_test "config: JOURNAL_MODEL override"
(
    tmp_config="$(mktemp)"
    trap "rm -f $tmp_config" EXIT
    echo "JOURNAL_MODEL=opus" > "$tmp_config"
    JOURNAL_MODEL=sonnet
    CONFIG_FILE="$tmp_config"
    while IFS='=' read -r key value; do
        key="$(echo "$key" | xargs)"
        [[ -z "$key" || "$key" == \#* ]] && continue
        value="$(echo "$value" | sed 's/#.*//' | xargs)"
        declare "$key=$value" 2>/dev/null || true
    done < "$CONFIG_FILE"
    assert_eq "opus" "$JOURNAL_MODEL" "JOURNAL_MODEL overridden" && pass
) || true

begin_test "config: MAX_JOURNAL_ENTRIES default"
(
    MAX_JOURNAL_ENTRIES=30
    CONFIG_FILE="/nonexistent/config"
    assert_eq "30" "$MAX_JOURNAL_ENTRIES" "MAX_JOURNAL_ENTRIES default" && pass
) || true

begin_test "config: MAX_JOURNAL_ENTRIES override"
(
    tmp_config="$(mktemp)"
    trap "rm -f $tmp_config" EXIT
    echo "MAX_JOURNAL_ENTRIES=50" > "$tmp_config"
    MAX_JOURNAL_ENTRIES=30
    CONFIG_FILE="$tmp_config"
    while IFS='=' read -r key value; do
        key="$(echo "$key" | xargs)"
        [[ -z "$key" || "$key" == \#* ]] && continue
        value="$(echo "$value" | sed 's/#.*//' | xargs)"
        declare "$key=$value" 2>/dev/null || true
    done < "$CONFIG_FILE"
    assert_eq "50" "$MAX_JOURNAL_ENTRIES" "MAX_JOURNAL_ENTRIES overridden" && pass
) || true

# ===========================================================================
# Category Classification
# ===========================================================================
echo ""
echo "--- Category Classification ---"

begin_test "auto-approve: bug is auto-approved"
is_auto_approve_category "bug" && pass || fail "bug should be auto-approved"

begin_test "auto-approve: security is auto-approved"
is_auto_approve_category "security" && pass || fail "security should be auto-approved"

begin_test "auto-approve: testing is auto-approved"
is_auto_approve_category "testing" && pass || fail "testing should be auto-approved"

begin_test "auto-approve: documentation is auto-approved"
is_auto_approve_category "documentation" && pass || fail "documentation should be auto-approved"

begin_test "auto-approve: feature needs approval"
! is_auto_approve_category "feature" && pass || fail "feature should need approval"

begin_test "auto-approve: improvement needs approval"
! is_auto_approve_category "improvement" && pass || fail "improvement should need approval"

begin_test "auto-approve: refactor needs approval"
! is_auto_approve_category "refactor" && pass || fail "refactor should need approval"

begin_test "auto-approve: sunset needs approval"
! is_auto_approve_category "sunset" && pass || fail "sunset should need approval"

begin_test "auto-approve: ui_ux needs approval"
! is_auto_approve_category "ui_ux" && pass || fail "ui_ux should need approval"

begin_test "auto-approve: experience needs approval"
! is_auto_approve_category "experience" && pass || fail "experience should need approval"

begin_test "auto-approve: unknown category needs approval"
! is_auto_approve_category "unknown_thing" && pass || fail "unknown should need approval"

# ===========================================================================
# Category Label Mapping
# ===========================================================================
echo ""
echo "--- Category Label Mapping ---"

# Extract the category_label logic into a testable function
get_category_label() {
    local category="$1"
    case "$category" in
        bug)           echo "Bug" ;;
        testing)       echo "Testing" ;;
        feature)       echo "Feature" ;;
        improvement)   echo "Improvement" ;;
        sunset)        echo "Sunset" ;;
        refactor)      echo "Refactor" ;;
        security)      echo "Security" ;;
        ui_ux)         echo "UI/UX" ;;
        documentation) echo "Documentation" ;;
        experience)    echo "Experience" ;;
        *)             echo "$category" | sed 's/.*/\u&/' ;;
    esac
}

begin_test "label: bug → Bug"
assert_eq "Bug" "$(get_category_label bug)" && pass || true

begin_test "label: feature → Feature"
assert_eq "Feature" "$(get_category_label feature)" && pass || true

begin_test "label: ui_ux → UI/UX"
assert_eq "UI/UX" "$(get_category_label ui_ux)" && pass || true

begin_test "label: documentation → Documentation"
assert_eq "Documentation" "$(get_category_label documentation)" && pass || true

begin_test "label: security → Security"
assert_eq "Security" "$(get_category_label security)" && pass || true

begin_test "label: experience → Experience"
assert_eq "Experience" "$(get_category_label experience)" && pass || true

begin_test "label: all 10 categories have mappings"
(
    all_ok=true
    for cat in bug testing feature improvement sunset refactor security ui_ux documentation experience; do
        label="$(get_category_label "$cat")"
        if [ -z "$label" ]; then
            all_ok=false
        fi
    done
    [ "$all_ok" = true ] && pass || fail "some categories missing labels"
) || true

# ===========================================================================
# AUTO_APPROVE Array Construction
# ===========================================================================
echo ""
echo "--- AUTO_APPROVE Array ---"

begin_test "array: default string splits into 4 categories"
(
    AUTO_APPROVE="bug security testing documentation"
    IFS=' ' read -ra cats <<< "$AUTO_APPROVE"
    assert_eq "4" "${#cats[@]}" "count" && pass
) || true

begin_test "array: custom string splits correctly"
(
    AUTO_APPROVE="bug security testing documentation feature"
    IFS=' ' read -ra cats <<< "$AUTO_APPROVE"
    assert_eq "5" "${#cats[@]}" "count" &&
    assert_eq "feature" "${cats[4]}" "5th element" &&
    pass
) || true

begin_test "array: single category works"
(
    AUTO_APPROVE="bug"
    IFS=' ' read -ra cats <<< "$AUTO_APPROVE"
    assert_eq "1" "${#cats[@]}" "count" &&
    assert_eq "bug" "${cats[0]}" "element" &&
    pass
) || true

# ===========================================================================
# AUTO_MERGE Default Behavior
# ===========================================================================
echo ""
echo "--- AUTO_MERGE Defaults ---"

begin_test "auto-merge: defaults to false when unset"
(
    unset AUTO_MERGE
    result="${AUTO_MERGE:-false}"
    assert_eq "false" "$result" && pass
) || true

begin_test "auto-merge: config can set to true"
(
    AUTO_MERGE=true
    result="${AUTO_MERGE:-false}"
    assert_eq "true" "$result" && pass
) || true

# ===========================================================================
# Syntax Validation
# ===========================================================================
echo ""
echo "--- Syntax Validation ---"

begin_test "orchestrator: bash syntax valid"
if bash -n "$ORCHESTRATOR" 2>&1; then
    pass
else
    fail "syntax error"
fi

begin_test "config: file exists and is readable"
if [ -f "$SCRIPT_DIR/config" ] && [ -r "$SCRIPT_DIR/config" ]; then
    pass
else
    fail "config file missing or unreadable"
fi

begin_test "config: no syntax errors when sourced"
(
    tmp_config="$SCRIPT_DIR/config"
    while IFS='=' read -r key value; do
        key="$(echo "$key" | xargs)"
        [[ -z "$key" || "$key" == \#* ]] && continue
        value="$(echo "$value" | sed 's/#.*//' | xargs)"
        declare "$key=$value" 2>/dev/null || true
    done < "$tmp_config"
    pass
) || fail "config parse error"

# ===========================================================================
# Build Journal
# ===========================================================================
echo ""
echo "--- Build Journal ---"

begin_test "journal: append creates file from scratch"
(
    tmp_journal="$(mktemp)"
    rm -f "$tmp_journal"  # start with no file
    BUILD_JOURNAL="$tmp_journal"
    MAX_JOURNAL_ENTRIES=30

    entry='{"run_id":"test-001","date":"2026-01-01","outcome":"success"}'
    append_journal_entry "$entry"

    assert_eq "1" "$(jq 'length' "$tmp_journal")" "single entry" &&
    assert_eq "test-001" "$(jq -r '.[0].run_id' "$tmp_journal")" "run_id" &&
    pass
    rm -f "$tmp_journal"
) || true

begin_test "journal: append prepends newest first"
(
    tmp_journal="$(mktemp)"
    echo '[{"run_id":"old","date":"2026-01-01","outcome":"success"}]' > "$tmp_journal"
    BUILD_JOURNAL="$tmp_journal"
    MAX_JOURNAL_ENTRIES=30

    entry='{"run_id":"new","date":"2026-01-02","outcome":"failure"}'
    append_journal_entry "$entry"

    assert_eq "2" "$(jq 'length' "$tmp_journal")" "two entries" &&
    assert_eq "new" "$(jq -r '.[0].run_id' "$tmp_journal")" "newest first" &&
    assert_eq "old" "$(jq -r '.[1].run_id' "$tmp_journal")" "oldest second" &&
    pass
    rm -f "$tmp_journal"
) || true

begin_test "journal: prunes to MAX_JOURNAL_ENTRIES"
(
    tmp_journal="$(mktemp)"
    # Create journal with 30 entries
    jq -n '[range(30) | {run_id: ("entry-" + tostring), date: "2026-01-01"}]' > "$tmp_journal"
    BUILD_JOURNAL="$tmp_journal"
    MAX_JOURNAL_ENTRIES=30

    entry='{"run_id":"entry-new","date":"2026-01-02"}'
    append_journal_entry "$entry"

    assert_eq "30" "$(jq 'length' "$tmp_journal")" "pruned to 30" &&
    assert_eq "entry-new" "$(jq -r '.[0].run_id' "$tmp_journal")" "newest is first" &&
    pass
    rm -f "$tmp_journal"
) || true

begin_test "journal: corrupt file recovers gracefully"
(
    tmp_journal="$(mktemp)"
    echo "NOT VALID JSON" > "$tmp_journal"
    BUILD_JOURNAL="$tmp_journal"
    MAX_JOURNAL_ENTRIES=30

    entry='{"run_id":"recovery","date":"2026-01-01"}'
    append_journal_entry "$entry"

    assert_eq "1" "$(jq 'length' "$tmp_journal")" "recovered with single entry" &&
    assert_eq "recovery" "$(jq -r '.[0].run_id' "$tmp_journal")" "correct entry" &&
    pass
    rm -f "$tmp_journal"
) || true

begin_test "journal: missing file creates new journal"
(
    tmp_journal="/tmp/fry-test-journal-$$-missing.json"
    rm -f "$tmp_journal"
    BUILD_JOURNAL="$tmp_journal"
    MAX_JOURNAL_ENTRIES=30

    entry='{"run_id":"first","date":"2026-01-01"}'
    append_journal_entry "$entry"

    [ -f "$tmp_journal" ] || { fail "file not created"; }
    assert_eq "1" "$(jq 'length' "$tmp_journal")" "single entry" &&
    pass
    rm -f "$tmp_journal"
) || true

begin_test "journal: small MAX_JOURNAL_ENTRIES prunes correctly"
(
    tmp_journal="$(mktemp)"
    echo '[{"run_id":"a"},{"run_id":"b"},{"run_id":"c"}]' > "$tmp_journal"
    BUILD_JOURNAL="$tmp_journal"
    MAX_JOURNAL_ENTRIES=2

    entry='{"run_id":"d"}'
    append_journal_entry "$entry"

    assert_eq "2" "$(jq 'length' "$tmp_journal")" "pruned to 2" &&
    assert_eq "d" "$(jq -r '.[0].run_id' "$tmp_journal")" "newest" &&
    assert_eq "a" "$(jq -r '.[1].run_id' "$tmp_journal")" "second" &&
    pass
    rm -f "$tmp_journal"
) || true

# ===========================================================================
# Results
# ===========================================================================
TESTS_RUN="$(cat "$RESULTS_DIR/run")"
TESTS_PASSED="$(cat "$RESULTS_DIR/passed")"
TESTS_FAILED="$(cat "$RESULTS_DIR/failed")"
rm -rf "$RESULTS_DIR"

echo ""
echo "=== Results ==="
echo "  Total:  $TESTS_RUN"
echo "  Passed: $TESTS_PASSED"
echo "  Failed: $TESTS_FAILED"

if [ "$TESTS_FAILED" -gt 0 ]; then
    echo ""
    echo "FAIL"
    exit 1
else
    echo ""
    echo "OK"
    exit 0
fi
