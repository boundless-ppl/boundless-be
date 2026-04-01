#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 1 ]]; then
  echo "usage: bash scripts/profile-endpoint.sh <url>" >&2
  exit 1
fi

URL="$1"
PPROF_URL="${PPROF_URL:?PPROF_URL is required}"
REQUESTS="${REQUESTS:-1000}"
CONCURRENCY="${CONCURRENCY:-20}"

HEADER_ARGS=()
if [[ -n "${TOKEN:-}" ]]; then
  HEADER_ARGS=(-H "Authorization: Bearer $TOKEN")
fi

echo "[profile] target: $URL"
echo "[profile] requests=$REQUESTS concurrency=$CONCURRENCY"
echo "[profile] pprof=$PPROF_URL"

(
  seq 1 "$REQUESTS" | xargs -P "$CONCURRENCY" -I{} \
    curl -s "${HEADER_ARGS[@]}" "$URL" > /dev/null
) &
LOAD_PID=$!

go tool pprof "$PPROF_URL"
wait "$LOAD_PID"
