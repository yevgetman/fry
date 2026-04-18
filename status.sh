#!/bin/zsh
# status.sh — human-readable snapshot of the fry build mission.
# Usage: /Users/julie/code/fry/status.sh

ROOT="/Users/julie/code/fry"
CREATED_AT="2026-04-18T14:49:18Z"

echo "========================================"
echo "  FRY v0.1 BUILD MISSION STATUS"
echo "========================================"

python3 - <<EOF
from datetime import datetime, timezone
c = datetime.fromisoformat("2026-04-18T14:49:18+00:00")
n = datetime.now(timezone.utc)
elapsed = (n - c).total_seconds() / 3600
soft_remaining = max(0, 12 - elapsed)
hard_remaining = max(0, 24 - elapsed)
if elapsed < 12: phase = "building"
elif elapsed < 24: phase = "building (overtime) / shutdown window"
else: phase = "hard stop"
print(f"Now (UTC):              {n.strftime('%Y-%m-%dT%H:%M:%SZ')}")
print(f"CREATED_AT (UTC):       2026-04-18T14:49:18Z")
print(f"Elapsed hours:          {elapsed:.2f}")
print(f"Soft remaining (12h):   {soft_remaining:.2f}")
print(f"Hard remaining (24h):   {hard_remaining:.2f}")
print(f"Expected phase:         {phase}")
EOF

echo ""
echo "--- LaunchAgent ---"
launchctl list 2>/dev/null | grep fry || echo "  (not loaded)"

echo ""
echo "--- Lock ---"
if [ -d /tmp/fry_build.lock ]; then
  echo "  HELD (a wake is running)"
else
  echo "  free"
fi

echo ""
echo "--- Recent wakes (last 3) ---"
if [ -f "$ROOT/logs/wake_log.jsonl" ]; then
  tail -3 "$ROOT/logs/wake_log.jsonl" | python3 -c "
import sys, json
for line in sys.stdin:
    try:
        d = json.loads(line)
        ms = d.get('current_milestone', '?')
        print(f\"  wake {d['wake_number']} | {d['phase']} | {ms} | {d['timestamp_utc']} | {d['wake_goal'][:60]}\")
    except Exception:
        print(f\"  [parse error] {line[:120]}\")
"
else
  echo "  (no log file yet)"
fi

echo ""
echo "--- Current focus (claude.md) ---"
if [ -f "$ROOT/claude.md" ]; then
  awk '/^## Current Focus/{flag=1; next} /^## /{flag=0} flag && NF' "$ROOT/claude.md" | head -5 | sed 's/^/  /'
else
  echo "  (claude.md not yet created — bootstrap pending)"
fi

echo ""
echo "--- Milestones (from claude.md DELIVERABLES STATUS) ---"
if [ -f "$ROOT/claude.md" ]; then
  awk '/DELIVERABLES STATUS/{flag=1; next} /^## /{flag=0} flag' "$ROOT/claude.md" | grep -E '^- \[' | sed 's/^/  /'
else
  echo "  (claude.md not yet created)"
fi

echo ""
echo "--- Source-of-truth docs ---"
for f in prompt.md product-spec.md build-plan.md claude.md agents.md SHIPPED.md; do
  if [ -f "$ROOT/$f" ]; then
    sz=$(wc -c < "$ROOT/$f" | tr -d ' ')
    echo "  [OK] $f (${sz} bytes)"
  else
    echo "  [  ] $f"
  fi
done

echo ""
echo "--- Code tree ---"
if [ -d "$ROOT/cmd" ] || [ -d "$ROOT/internal" ]; then
  find "$ROOT" -type f \( -name '*.go' -o -name 'go.mod' \) -not -path '*/vendor/*' -not -path '*/vendor-fry/*' 2>/dev/null | sed 's|'"$ROOT"'/|  |' | head -20
else
  echo "  (no Go source yet)"
fi

echo ""
echo "--- git state ---"
if [ -d "$ROOT/.git" ]; then
  commits=$(git -C "$ROOT" rev-list --count HEAD 2>/dev/null || echo 0)
  remote=$(git -C "$ROOT" remote get-url origin 2>/dev/null || echo "(no remote)")
  echo "  $commits commits, remote: $remote"
  git -C "$ROOT" log --oneline -5 2>/dev/null | sed 's/^/  /'
else
  echo "  (no git init yet)"
fi

echo ""
echo "--- Controls ---"
echo "  Force-fire next wake:    launchctl kickstart -k gui/\$(id -u)/com.julie.fry.wake"
echo "  Stop mission early:      launchctl unload ~/Library/LaunchAgents/com.julie.fry.wake.plist"
echo "  Tail cron log:           tail -f $ROOT/logs/cron.log"
echo "  Full log history:        less $ROOT/logs/wake_log.jsonl"
