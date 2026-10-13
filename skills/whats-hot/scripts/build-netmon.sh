#!/usr/bin/env bash
# Build the netmon binary (network monitor + dashboard + downloader + speed test) into
# ~/.local/bin. Run once after installing the skill. Requires Go (go.dev/dl).
set -euo pipefail

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN_DIR="${NETMON_BIN_DIR:-$HOME/.local/bin}"

if ! command -v go >/dev/null 2>&1; then
  echo "error: Go is required to build netmon (https://go.dev/dl). The whats-hot" >&2
  echo "diagnostic scripts still work without it; only the network features need netmon." >&2
  exit 1
fi

mkdir -p "$BIN_DIR"
echo "building netmon -> $BIN_DIR/netmon"
( cd "$DIR" && go build -o "$BIN_DIR/netmon" ./cmd/netmon )
echo "ok. ensure $BIN_DIR is on your PATH, then: netmon -h"
