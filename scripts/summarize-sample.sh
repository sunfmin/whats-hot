#!/usr/bin/env bash
# Distill a sample(1) output into hot leaves.
# Usage: summarize-sample.sh <sample-output-file>
set -euo pipefail

infile=${1:?Usage: summarize-sample.sh <sample-output-file>}

if [[ ! -f "$infile" ]]; then
  echo "File not found: $infile" >&2
  exit 1
fi

echo "=== Hottest leaf frames (sample's 'Sort by top of stack' section) ==="
# sample(1) prints this section after the call graph; it lists each leaf frame
# with its sample count, which is what actually consumed CPU.
awk '
  /Sort by top of stack/ { capture=1; next }
  capture && /^Binary Images/ { exit }
  capture { print }
' "$infile" | head -40

echo ""
echo "=== Deep frames appearing in >= 10% of samples ==="
# Scan the call graph for "+ <count> <function>" lines and print those whose
# count meets a threshold relative to the largest count seen.
# BSD-awk compatible: no match()-with-capture-groups (that's a gawk extension).
awk '
  /Call graph:/ { in_graph = 1; next }
  /^Binary Images/ { in_graph = 0 }
  !in_graph { next }
  /^[[:space:]]*\+[[:space:]]+[0-9]+[[:space:]]/ {
    tmp = $0
    sub(/^[[:space:]]*\+[[:space:]]+/, "", tmp)
    cnt = tmp + 0
    if (cnt > max) max = cnt
    lines[NR] = $0
    counts[NR] = cnt
  }
  END {
    if (max == 0) { print "(no call-graph entries found)"; exit }
    threshold = int(max * 0.10)
    for (i in lines) if (counts[i] >= threshold) print lines[i]
  }
' "$infile" | sort -t '+' -k2 -n -r | uniq | head -25
