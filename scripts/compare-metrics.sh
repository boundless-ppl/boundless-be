#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 2 ]]; then
  echo "usage: bash scripts/compare-metrics.sh <ref_a> <ref_b>" >&2
  exit 1
fi

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
REF_A="$1"
REF_B="$2"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

run_for_ref() {
  local ref="$1"
  local key
  key="$(printf '%s' "$ref" | tr '/:' '__')"
  local worktree="$TMP_DIR/worktree-$key"
  local artifact_dir="$TMP_DIR/artifacts/$key"

  git -C "$ROOT_DIR" worktree add --detach "$worktree" "$ref" >/dev/null
  (
    cd "$worktree"
    GOCACHE="$worktree/.gocache" ARTIFACT_DIR="$artifact_dir/test" bash scripts/run-tests.sh >/dev/null
    GOCACHE="$worktree/.gocache" ARTIFACT_DIR="$artifact_dir/bench" bash scripts/run-benchmarks.sh >/dev/null
  )
  git -C "$ROOT_DIR" worktree remove "$worktree" --force >/dev/null
}

extract_total_coverage() {
  awk '/^total:/ {print $3}' "$1"
}

extract_benchmark_line() {
  local file="$1"
  local name="$2"
  awk -v target="$name" '$1 == target {print $0}' "$file"
}

run_for_ref "$REF_A"
run_for_ref "$REF_B"

ART_A="$TMP_DIR/artifacts/$(printf '%s' "$REF_A" | tr '/:' '__')"
ART_B="$TMP_DIR/artifacts/$(printf '%s' "$REF_B" | tr '/:' '__')"

echo "Coverage"
echo "  $REF_A: $(extract_total_coverage "$ART_A/test/coverage-summary.txt")"
echo "  $REF_B: $(extract_total_coverage "$ART_B/test/coverage-summary.txt")"
echo

for benchmark in BenchmarkLoginSuccess BenchmarkCreateDreamTracker BenchmarkGetDreamTrackerDetail; do
  echo "$benchmark"
  echo "  $REF_A: $(extract_benchmark_line "$ART_A/bench/bench.txt" "$benchmark")"
  echo "  $REF_B: $(extract_benchmark_line "$ART_B/bench/bench.txt" "$benchmark")"
  echo
done
