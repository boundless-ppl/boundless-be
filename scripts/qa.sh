#!/usr/bin/env sh
set -eu

echo "==> Running unit tests"
go test ./...

echo "==> Running coverage for app packages via tests in ./test/..."
mkdir -p artifacts
go test ./test/... -coverpkg=./... -coverprofile=artifacts/coverage.out
go tool cover -func=artifacts/coverage.out | tee artifacts/coverage.txt
go tool cover -html=artifacts/coverage.out -o artifacts/coverage.html

echo "==> Coverage artifacts generated in ./artifacts"
