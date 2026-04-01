#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ARTIFACT_DIR="${ARTIFACT_DIR:-$ROOT_DIR/.artifacts/bench}"
GOCACHE_DIR="${GOCACHE:-$ROOT_DIR/.gocache}"

mkdir -p "$ARTIFACT_DIR" "$GOCACHE_DIR"

OUTPUT_FILE="$ARTIFACT_DIR/bench.txt"

echo "[bench] running service benchmarks"
GOCACHE="$GOCACHE_DIR" go test \
  -run '^$' \
  -bench 'Benchmark(LoginSuccess|CreateDreamTracker|GetDreamTrackerDetail)$' \
  -benchmem \
  ./test/auth/service \
  ./test/dream_tracker/service | tee "$OUTPUT_FILE"

echo "[bench] artifacts saved in $OUTPUT_FILE"
