#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ARTIFACT_DIR="${ARTIFACT_DIR:-$ROOT_DIR/.artifacts/test}"
GOCACHE_DIR="${GOCACHE:-$ROOT_DIR/.gocache}"

mkdir -p "$ARTIFACT_DIR" "$GOCACHE_DIR"

echo "[test] running go test with coverage"
GOCACHE="$GOCACHE_DIR" go test ./... -timeout "${GO_TEST_TIMEOUT:-90s}" \
  -coverprofile "$ARTIFACT_DIR/coverage.out"

go tool cover -func="$ARTIFACT_DIR/coverage.out" | tee "$ARTIFACT_DIR/coverage-summary.txt"

echo "[test] artifacts saved in $ARTIFACT_DIR"
