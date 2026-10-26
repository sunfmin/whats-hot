#!/bin/sh
# The definition of "may land": vet and test a Go module and parse scripts
# when present. The hostvitals pointer has neither and needs no Go toolchain.
set -e
cd "$(dirname "$0")/skills/whats-hot"
if [ -f go.mod ]; then
  go vet ./...
  go test ./...
fi
for f in scripts/*.sh; do
  [ -e "$f" ] || continue
  bash -n "$f"
done
