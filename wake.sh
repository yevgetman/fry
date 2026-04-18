#!/bin/zsh
# wake.sh — autonomous wake runner for the fry v0.1 build mission.
# Invoked every ~10 minutes by LaunchAgent com.julie.fry.wake.
# Do NOT modify unless the runner itself is demonstrably broken.

set -uo pipefail

ROOT="/Users/julie/code/fry"
LOCK="/tmp/fry_build.lock"
CRON_LOG="$ROOT/logs/cron.log"
INSTRUCTION_FILE="$ROOT/WAKE_INSTRUCTION.md"

export HOME="/Users/julie"
export USER="julie"
export LOGNAME="julie"
export SHELL="/bin/zsh"
export PATH="/Users/julie/.local/bin:/opt/homebrew/bin:/opt/homebrew/sbin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin"

mkdir -p "$ROOT/logs"

START=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

# Overlap guard: mkdir is atomic. If the lock dir exists, the prior wake is still running.
if ! mkdir "$LOCK" 2>/dev/null; then
  echo "[$START] SKIPPED — prior wake still holds $LOCK" >> "$CRON_LOG"
  exit 0
fi
trap 'rmdir "$LOCK" 2>/dev/null' EXIT INT TERM

echo "" >> "$CRON_LOG"
echo "=== WAKE START $START ===" >> "$CRON_LOG"

cd "$ROOT" || { echo "[$START] FATAL cd $ROOT failed" >> "$CRON_LOG"; exit 1; }

# Hard-cap wall clock at 540s (macOS has no timeout(1) by default, so use perl alarm).
# --max-budget-usd caps $ spend.
# --permission-mode bypassPermissions + --dangerously-skip-permissions allows autonomy.
# --model sonnet → latest Sonnet (4.6). --effort high → deep reasoning per call.
# --add-dir scopes tool access explicitly to the project root.
# Feeding instruction via stdin avoids shell-quoting edge cases.
perl -e 'alarm 540; exec @ARGV' -- \
  /Users/julie/.local/bin/claude \
  -p \
  --permission-mode bypassPermissions \
  --dangerously-skip-permissions \
  --model sonnet \
  --effort high \
  --max-budget-usd 2 \
  --add-dir "$ROOT" \
  < "$INSTRUCTION_FILE" \
  >> "$CRON_LOG" 2>&1
RC=$?

END=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
if [ "$RC" -ne 0 ]; then
  echo "[$END] WAKE EXITED NON-ZERO rc=$RC" >> "$CRON_LOG"
else
  echo "[$END] WAKE OK" >> "$CRON_LOG"
fi
echo "=== WAKE END $END ===" >> "$CRON_LOG"

exit 0
