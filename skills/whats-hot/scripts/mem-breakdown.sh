#!/usr/bin/env bash
# Memory breakdown for a PID using vmmap (and footprint if available).
# Usage: mem-breakdown.sh <pid>
set -euo pipefail

pid=${1:?Usage: mem-breakdown.sh <pid>}

if ! ps -p "$pid" >/dev/null 2>&1; then
  echo "PID $pid is not running." >&2
  exit 1
fi

owner=$(ps -o user= -p "$pid" | tr -d ' ')
me=$(whoami)
if [[ "$owner" != "$me" && $EUID -ne 0 ]]; then
  echo "WARN: PID $pid is owned by '$owner'. vmmap may need sudo. If it fails:" >&2
  echo "      sudo $0 $pid" >&2
fi

echo "=== vmmap -summary ==="
if ! vmmap -summary "$pid" 2>/dev/null; then
  echo "vmmap failed (try sudo)." >&2
fi

echo ""
echo "=== Largest VM regions (top 15 by SIZE) ==="
# vmmap regular output: "REGION_TYPE    START-END  [ SIZE  RES PRIV  STATE]  ..."
# We sort by the SIZE column. Sizes may use K/M/G suffixes — sort -h handles them.
vmmap "$pid" 2>/dev/null \
  | awk '/^[A-Za-z]/ && /\[/ { print }' \
  | awk -F '[\\[\\]]' '{ split($2, a, " "); print a[1], $0 }' \
  | sort -h -r \
  | head -15 \
  | cut -d' ' -f2- \
  || echo "(could not parse vmmap regions)"

echo ""
echo "=== footprint (if available) ==="
if command -v footprint >/dev/null 2>&1; then
  footprint -pid "$pid" 2>/dev/null || echo "(footprint failed)"
else
  echo "(footprint not installed — comes with Xcode Command Line Tools)"
fi
