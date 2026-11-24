#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."
export REPORT_DIR="${REPORT_DIR:-docs/test-reports/$(date +%F)/verification}"
mkdir -p "$REPORT_DIR"
# Start native services and seed matching placements before invoking this script.
status=0
bash scripts/run-loadtest.sh 1000 30s || status=1
bash scripts/run-loadtest.sh 3000 30s || status=1
bash scripts/run-loadtest.sh 5000 2m &
load_pid=$!
cleanup() { kill "$load_pid" 2>/dev/null || :; wait "$load_pid" 2>/dev/null || :; }
trap cleanup EXIT
sleep 10
admin="${GATEWAY_ADMIN:-http://127.0.0.1:8090}"
for profile in heap goroutine block mutex; do
  curl -fsS "$admin/debug/pprof/$profile" > "$REPORT_DIR/$profile.pprof"
done
curl -fsS "$admin/debug/pprof/profile?seconds=30" > "$REPORT_DIR/cpu.pprof"
wait "$load_pid" || status=1
trap - EXIT
# Preserve the real k6 exit code; a threshold failure must fail acceptance.
"${K6_BIN:-k6}" run --summary-export "$REPORT_DIR/k6-slow-bidder.json" \
  -e "GATEWAY=${GATEWAY:-http://127.0.0.1:8080}" \
  -e "BIDDER_URL=${BIDDER_URL:-http://127.0.0.1:9101}" \
  -e "PRE_VUS=${PRE_VUS:-200}" -e "MAX_VUS=${MAX_VUS:-1000}" \
  -e RATE=1000 -e DURATION=30s loadtest/k6/slow-bidder.js 2>&1 | tee "$REPORT_DIR/k6-slow-bidder.txt" || status=1
RATE=5000 DURATION=60s bash loadtest/vegeta/run.sh || status=1
if command -v go >/dev/null; then
  for profile in cpu heap goroutine block mutex; do
    go tool pprof -top "$REPORT_DIR/$profile.pprof" > "$REPORT_DIR/$profile-profile.txt"
  done
fi
exit "$status"
