# Quality Workflow

This repository includes simple scripts so test and profiling flows are reproducible across the team.

## Test + Coverage

```bash
bash scripts/run-tests.sh
```

Artifacts:

- `.artifacts/test/coverage.out`
- `.artifacts/test/coverage-summary.txt`

## Benchmarks

```bash
bash scripts/run-benchmarks.sh
```

Artifacts:

- `.artifacts/bench/bench.txt`

Benchmarks included:

- `BenchmarkLoginSuccess`
- `BenchmarkCreateDreamTracker`
- `BenchmarkGetDreamTrackerDetail`

## Endpoint Profiling

Run the app with pprof enabled:

```bash
export PPROF_ADDR=127.0.0.1:6060
```

Then profile an endpoint:

```bash
TOKEN=... \
PPROF_URL='http://127.0.0.1:6060/debug/pprof/profile?seconds=30' \
bash scripts/profile-endpoint.sh http://127.0.0.1:8080/dream-trackers/<id>
```

## RED / GREEN / REFACTOR

Verify the last three commits follow the required structure:

```bash
bash scripts/verify-rrr.sh HEAD~2..HEAD
```

## Cross-Sprint Metrics

Compare two refs, tags, or branches:

```bash
bash scripts/compare-metrics.sh sprint-1-tag sprint-2-tag
```

This compares:

- total coverage
- benchmark outputs for the tracked service paths

## Monitoring on Staging / Production

The app exposes pprof only when `PPROF_ADDR` is set. Recommended staging setup:

- keep the main API and pprof on separate ports
- expose pprof only on internal ingress / VPN

Check monitoring availability:

```bash
APP_URL='https://staging.example.com/' \
PPROF_BASE_URL='https://staging-internal.example.com' \
bash scripts/check-monitoring.sh
```
