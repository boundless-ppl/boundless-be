#!/usr/bin/env bash
set -euo pipefail

APP_URL="${APP_URL:?APP_URL is required}"
PPROF_BASE_URL="${PPROF_BASE_URL:?PPROF_BASE_URL is required}"

echo "[monitor] checking application URL"
curl -fsS "$APP_URL" > /dev/null

echo "[monitor] checking pprof index"
curl -fsS "$PPROF_BASE_URL/debug/pprof/" > /dev/null

echo "[monitor] checking heap profile"
curl -fsS "$PPROF_BASE_URL/debug/pprof/heap" > /dev/null

echo "[monitor] monitoring endpoints reachable"
