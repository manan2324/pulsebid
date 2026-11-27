#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."
rate="${1:-${RATE:-5000}}"
duration="${2:-${DURATION:-2m}}"
report="${REPORT_DIR:-docs/test-reports/$(date +%F)/verification}"
mkdir -p "$report"
gateway="${GATEWAY:-http://127.0.0.1:8080}"
if [[ -n ${K6_BIN:-} ]]; then
  runner=("$K6_BIN")
elif command -v k6 >/dev/null; then
  runner=(k6)
elif [[ -x loadtest/k6/k6 ]]; then
  runner=(./loadtest/k6/k6)
elif [[ -f loadtest/k6/k6.exe ]]; then
  runner=(./loadtest/k6/k6.exe)
else
  root=$PWD
  if command -v cygpath >/dev/null; then root=$(cygpath -m "$PWD"); fi
  runner=(docker run --rm --network host --mount "type=bind,source=$root,target=/work" --workdir /work grafana/k6:latest)
fi
"${runner[@]}" run --summary-export "$report/k6-${rate}rps.json" \
  -e "GATEWAY=$gateway" -e "RATE=$rate" -e "DURATION=$duration" \
  -e "PRE_VUS=${PRE_VUS:-500}" -e "MAX_VUS=${MAX_VUS:-1000}" \
  loadtest/k6/bid.js 2>&1 | tee "$report/k6-${rate}rps.txt"
