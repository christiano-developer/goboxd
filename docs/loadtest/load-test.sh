#!/usr/bin/env bash
# docs/loadtest/load-test.sh
#
# Reproducible MemoryHog load test for goboxd.
#
# Prereqs:
#   - goboxd running in its container, capped at 2 vCPU / 2 GB
#     (docker compose up -d goboxd; see docker-compose.yml)
#   - Go toolchain (the generator runs on the host)
#   - a python with matplotlib for the graphs (point PYTHON at it)
#
# The generator auto-labels the run from the live CONCURRENCY_LIMIT (/info),
# writes a timestamped CSV + its plots to docs/loadtest/runs/, and refreshes the
# canonical docs/loadtest/results.csv + breaking-point.png + latency.png.
set -euo pipefail

cd "$(dirname "$0")/../.."   # repo root

URL="${URL:-http://localhost:8080/run}"
INFO="${INFO:-http://localhost:8080/info}"
RATES="${RATES:-1,2,3,5,10,25,50,75,100,150,200,300,400}"
DURATION="${DURATION:-30s}"
TIMEOUT="${TIMEOUT:-10s}"
STOP_AFTER_FAIL="${STOP_AFTER_FAIL:-3}"
DRAIN_MAX="${DRAIN_MAX:-60s}"
PYTHON="${PYTHON:-/tmp/plotenv/bin/python}"   # a python that has matplotlib

echo "==> Driving load (open-loop, ${DURATION}/step, ${TIMEOUT} timeout) + plotting"
go run ./scripts/memhog \
  -url "$URL" -info "$INFO" \
  -source docs/loadtest/MemoryHog.java \
  -rates "$RATES" -duration "$DURATION" -timeout "$TIMEOUT" \
  -stop-after-fail "$STOP_AFTER_FAIL" -drain-max "$DRAIN_MAX" \
  -python "$PYTHON"

echo "==> Done. See docs/loadtest/runs/ (this run) and docs/loadtest/results.csv (latest)"
