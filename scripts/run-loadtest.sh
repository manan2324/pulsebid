#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."

RATE="${1:-${RATE:-5000}}"
DURATION="${2:-${DURATION:-2m}}"
GATEWAY="${GATEWAY:-http://bid-gateway:8080}"
REPORT_DIR="docs/test-reports/$(date +%F)/phase-12"
mkdir -p "$REPORT_DIR"

echo "Executing k6 constant-arrival-rate test: RATE=${RATE} rps, DURATION=${DURATION} against ${GATEWAY}..."

host_root=$PWD
if command -v cygpath >/dev/null; then host_root=$(cygpath -m "$PWD"); fi

MSYS_NO_PATHCONV=1 docker run --rm -i --network pulsebid_default \
  -v "${host_root}/loadtest/k6:/scripts" \
  -e "GATEWAY=${GATEWAY}" \
  -e "RATE=${RATE}" \
  -e "DURATION=${DURATION}" \
  grafana/k6:latest run /scripts/bid.js | tee "${REPORT_DIR}/k6-rate-${RATE}.txt"
