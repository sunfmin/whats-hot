#!/usr/bin/env bash
# Capture file I/O and network activity for a PID.
# fs_usage REQUIRES sudo; nettop and lsof do not.
# Usage: io-net-snapshot.sh <pid> [seconds]   (default seconds=5)
set -euo pipefail

pid=${1:?Usage: io-net-snapshot.sh <pid> [seconds]}
seconds=${2:-5}

if ! ps -p "$pid" >/dev/null 2>&1; then
  echo "PID $pid is not running." >&2
  exit 1
fi

echo "=== lsof: open files & sockets (snapshot) ==="
lsof -p "$pid" 2>/dev/null | awk 'NR==1 || /REG|IPv4|IPv6|unix/' | head -40 \
  || echo "(lsof produced no output)"

echo ""
echo "=== nettop: network activity for PID $pid (${seconds}s) ==="
# nettop -P groups by process; -L N takes N snapshots; -p filters by PID.
nettop -P -L "$seconds" -p "$pid" 2>/dev/null | tail -40 \
  || echo "(nettop produced no output — process may have no sockets open)"

echo ""
echo "=== fs_usage: file syscall trace (${seconds}s) ==="
if [[ $EUID -ne 0 ]]; then
  cat <<EOF
fs_usage requires root. To capture, confirm with the user, then run:

  sudo fs_usage -w -f filesys $pid &
  sleep $seconds; sudo killall fs_usage

Or re-run this whole script as:
  sudo bash $0 $pid $seconds
EOF
  exit 0
fi

fs_out=$(mktemp -t "fs_usage.${pid}.XXXXXX").txt
fs_usage -w -f filesys "$pid" > "$fs_out" 2>/dev/null &
fs_pid=$!
sleep "$seconds"
kill "$fs_pid" 2>/dev/null || true
wait 2>/dev/null || true

echo "Last 40 lines of fs_usage trace:"
tail -40 "$fs_out"
echo ""
echo "Full trace at: $fs_out"

echo ""
echo "=== Most-touched paths in trace ==="
# fs_usage formats vary by syscall; the path is usually the last whitespace-separated
# token before the PID/process. This is approximate.
awk '{ for (i=NF; i>0; i--) if ($i ~ /^\//) { print $i; break } }' "$fs_out" \
  | sort | uniq -c | sort -rn | head -10
