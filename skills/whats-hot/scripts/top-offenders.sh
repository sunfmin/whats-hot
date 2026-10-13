#!/usr/bin/env bash
# Show top resource consumers on macOS. Read-only, no sudo.
# Usage: top-offenders.sh [N]   (default N=10)
set -euo pipefail

n=${1:-10}

# `ps` reports %CPU averaged over the process's lifetime. To get a snapshot of
# *current* CPU we take two `top` samples and use the second one — the first
# is always lifetime-averaged.
printf '=== Top %s processes by current %%CPU (top -l 2 -s 1) ===\n' "$n"
top -l 2 -s 1 -n "$n" -o cpu -stats pid,command,cpu,mem,threads,state 2>/dev/null \
  | awk '/^Processes:/ {seen++} seen==2'

printf '\n=== Top %s processes by RSS (resident memory) ===\n' "$n"
ps -Ao pid=PID,pcpu=%CPU,pmem=%MEM,rss=RSS_KB,comm=COMMAND -m \
  | awk -v n="$n" 'NR==1 || NR<=n+1'

printf '\n=== VM stats ===\n'
vm_stat

printf '\n=== Load average and uptime ===\n'
uptime

printf '\n=== Swap usage ===\n'
sysctl vm.swapusage 2>/dev/null || echo "(swap info unavailable)"
