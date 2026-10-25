#!/bin/sh
# The definition of "may land": the Go module vets and tests, and every
# script parses.
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
