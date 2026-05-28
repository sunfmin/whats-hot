#!/usr/bin/env bash
# Profile a single PID with sample(1). No sudo needed for processes you own.
# Usage: profile-pid.sh <pid> [seconds]   (default seconds=5)
set -euo pipefail

pid=${1:?Usage: profile-pid.sh <pid> [seconds]}
seconds=${2:-5}

if ! ps -p "$pid" >/dev/null 2>&1; then
  echo "PID $pid is not running." >&2
  exit 1
fi

cmd=$(ps -o comm= -p "$pid" | tr -d ' ')
owner=$(ps -o user= -p "$pid" | tr -d ' ')
me=$(whoami)

if [[ "$owner" != "$me" && "$owner" != "root" ]]; then
  echo "WARN: PID $pid is owned by '$owner' (you are '$me')." >&2
  echo "      sample may need sudo. If it fails, re-run with: sudo $0 $pid $seconds" >&2
fi

outfile=$(mktemp -t "sample.${pid}.XXXXXX").txt
echo "Sampling PID $pid ($cmd) for ${seconds}s..." >&2
echo "Output: $outfile" >&2

# sample(1): -f writes to file, -mayDie tolerates the target exiting mid-sample.
if ! sample "$pid" "$seconds" -f "$outfile" -mayDie >/dev/null 2>&1; then
  echo "sample failed. Try: sudo sample $pid $seconds" >&2
  exit 2
fi

echo ""
echo "=== Sample header (first 30 lines) ==="
head -30 "$outfile"

echo ""
echo "Full sample at: $outfile"
echo "Next: bash scripts/summarize-sample.sh $outfile"
