#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RANGE="${1:-HEAD~2..HEAD}"

subjects=()
while IFS= read -r line; do
  subjects+=("$line")
done < <(git -C "$ROOT_DIR" log --reverse --format=%s "$RANGE")

if [[ ${#subjects[@]} -ne 3 ]]; then
  echo "expected exactly 3 commits in range $RANGE, got ${#subjects[@]}" >&2
  exit 1
fi

expected=("[RED]" "[GREEN]" "[REFACTOR]")
for i in "${!expected[@]}"; do
  if [[ "${subjects[$i]}" != "${expected[$i]}"* ]]; then
    echo "commit $((i + 1)) does not start with ${expected[$i]}: ${subjects[$i]}" >&2
    exit 1
  fi
done

echo "verified $RANGE"
