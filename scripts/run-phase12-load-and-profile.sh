#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."

REPORT_DIR="docs/test-reports/$(date +%F)/phase-12"
mkdir -p "$REPORT_DIR"

host_root=$PWD
if command -v cygpath >/dev/null; then host_root=$(cygpath -m "$PWD"); fi

echo "=========================================================="
echo "Phase 12: Load Testing & Performance Profiling"
echo "Target Directory: ${REPORT_DIR}"
echo "=========================================================="

# 1. 1,000 rps load test (30s)
echo ""
echo ">>> [1/5] Running k6 at 1,000 rps for 30s..."
MSYS_NO_PATHCONV=1 docker run --rm -i --network pulsebid_default \
  -v "${host_root}/loadtest/k6:/scripts" \
  -e "GATEWAY=http://bid-gateway:8080" \
  -e "RATE=1000" \
  -e "DURATION=30s" \
  -e "PRE_VUS=100" \
  -e "MAX_VUS=500" \
  grafana/k6:latest run /scripts/bid.js | tee "${REPORT_DIR}/k6-1000rps.txt"

# 2. 3,000 rps load test (30s)
echo ""
echo ">>> [2/5] Running k6 at 3,000 rps for 30s..."
MSYS_NO_PATHCONV=1 docker run --rm -i --network pulsebid_default \
  -v "${host_root}/loadtest/k6:/scripts" \
  -e "GATEWAY=http://bid-gateway:8080" \
  -e "RATE=3000" \
  -e "DURATION=30s" \
  -e "PRE_VUS=300" \
  -e "MAX_VUS=1500" \
  grafana/k6:latest run /scripts/bid.js | tee "${REPORT_DIR}/k6-3000rps.txt"

# 3. 5,000 rps load test (60s) with concurrent pprof capture
echo ""
echo ">>> [3/5] Running k6 at 5,000 rps for 60s with pprof capture..."

# Start k6 in background
MSYS_NO_PATHCONV=1 docker run --rm -i --network pulsebid_default \
  -v "${host_root}/loadtest/k6:/scripts" \
  -e "GATEWAY=http://bid-gateway:8080" \
  -e "RATE=5000" \
  -e "DURATION=60s" \
  -e "PRE_VUS=500" \
  -e "MAX_VUS=3000" \
  grafana/k6:latest run /scripts/bid.js > "${REPORT_DIR}/k6-5000rps.txt" &
K6_PID=$!

echo "Waiting 10s for k6 to ramp to 5,000 rps steady state..."
sleep 10

echo "Capturing heap, goroutine, block, mutex pprof profiles..."
curl -s "http://localhost:8090/debug/pprof/heap" > "${REPORT_DIR}/heap.pprof" || true
curl -s "http://localhost:8090/debug/pprof/goroutine" > "${REPORT_DIR}/goroutine.pprof" || true
curl -s "http://localhost:8090/debug/pprof/block" > "${REPORT_DIR}/block.pprof" || true
curl -s "http://localhost:8090/debug/pprof/mutex" > "${REPORT_DIR}/mutex.pprof" || true

echo "Capturing 30-second CPU profile during active 5,000 rps load..."
curl -s "http://localhost:8090/debug/pprof/profile?seconds=30" > "${REPORT_DIR}/cpu.pprof" || true
echo "CPU profile capture complete."

# Wait for k6 to finish
wait $K6_PID || true
cat "${REPORT_DIR}/k6-5000rps.txt"

# 4. Slow bidder isolation scenario (30s at 1,000 rps)
echo ""
echo ">>> [4/5] Running slow-bidder isolation test (500ms latency on bidder-1)..."
MSYS_NO_PATHCONV=1 docker run --rm -i --network pulsebid_default \
  -v "${host_root}/loadtest/k6:/scripts" \
  -e "GATEWAY=http://bid-gateway:8080" \
  -e "BIDDER_URL=http://mock-bidders:9101" \
  -e "RATE=1000" \
  -e "DURATION=30s" \
  -e "PRE_VUS=100" \
  -e "MAX_VUS=1000" \
  grafana/k6:latest run /scripts/slow-bidder.js | tee "${REPORT_DIR}/k6-slow-bidder.txt"

# 5. Vegeta cross-check (30s at 5,000 rps)
echo ""
echo ">>> [5/5] Running Vegeta cross-check attack at 5,000 rps for 30s..."
RATE=5000 DURATION=30s OUTPUT="${REPORT_DIR}/vegeta-report.txt" bash loadtest/vegeta/run.sh

# 6. Analyze pprof profiles
echo ""
echo ">>> Generating pprof analysis summaries..."
go tool pprof -top -cum "${REPORT_DIR}/cpu.pprof" > "${REPORT_DIR}/cpu-profile.txt" 2>&1 || true
go tool pprof -top -cum "${REPORT_DIR}/heap.pprof" > "${REPORT_DIR}/heap-profile.txt" 2>&1 || true
go tool pprof -top -cum "${REPORT_DIR}/goroutine.pprof" > "${REPORT_DIR}/goroutine-profile.txt" 2>&1 || true

echo "All Phase 12 load tests and profiling completed successfully!"
