#!/usr/bin/env bash
# docs/loadtest/load-test.sh
#
# Reproducible MemoryHog load test for goboxd.
#
# Prereqs:
#   - goboxd running in its container, capped at 2 vCPU / 2 GB
#     (docker compose up -d goboxd; see docker-compose.yml)
#   - Go toolchain (the generator runs on the host)
#   - python3 + matplotlib (for the graphs)
#
# Produces, in docs/loadtest/:
#   results.csv, breaking-point.png, latency.png
set -euo pipefail

cd "$(dirname "$0")/../.."   # repo root

URL="${URL:-http://localhost:8080/run}"
INFO="${INFO:-http://localhost:8080/info}"
RATES="${RATES:-1,2,3,5,10,25,50,75,100,150,200,300,400}"
DURATION="${DURATION:-30s}"
TIMEOUT="${TIMEOUT:-10s}"
STOP_AFTER_FAIL="${STOP_AFTER_FAIL:-3}"
DRAIN_MAX="${DRAIN_MAX:-60s}"

echo "==> Driving load (open-loop, ${DURATION}/step, ${TIMEOUT} timeout)"
go run ./scripts/memhog \
  -url "$URL" -info "$INFO" \
  -source docs/loadtest/MemoryHog.java \
  -out docs/loadtest/results.csv \
  -rates "$RATES" -duration "$DURATION" -timeout "$TIMEOUT" \
  -stop-after-fail "$STOP_AFTER_FAIL" -drain-max "$DRAIN_MAX"

echo "==> Plotting"
python3 docs/loadtest/plot.py docs/loadtest/results.csv

echo "==> Done. See docs/loadtest/results.csv, breaking-point.png, latency.png"
