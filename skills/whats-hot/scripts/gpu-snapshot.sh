#!/usr/bin/env bash
# Snapshot GPU power and per-task energy use. Requires sudo.
# Usage: gpu-snapshot.sh [seconds]   (default 3)
set -euo pipefail

seconds=${1:-3}

if [[ $EUID -ne 0 ]]; then
  cat >&2 <<EOF
powermetrics requires root. Confirm with the user, then re-run as:
  sudo bash $0 $seconds
EOF
  exit 2
fi

echo "=== powermetrics: GPU + tasks (${seconds} samples @ 1s) ==="
# --samplers gpu_power = GPU residency / freq / power
# --samplers tasks = per-process CPU & GPU usage (Apple Silicon)
powermetrics --samplers gpu_power,tasks -i 1000 -n "$seconds" 2>/dev/null \
  | awk '
      /^\*\*\* Sampled system activity/ { sample_no++; print "\n--- Sample " sample_no " ---"; next }
      /GPU HW active|GPU SW|GPU idle|GPU Frequency|GPU Power|Package Power|Combined Power/ { print; next }
      /^Name +ID +CPU ms\/s/ { in_tasks=1; print; next }
      in_tasks && NF==0 { in_tasks=0; next }
      in_tasks { print }
    '
