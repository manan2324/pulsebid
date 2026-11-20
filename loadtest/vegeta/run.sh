#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/../.."

RATE="${RATE:-5000}"
DURATION="${DURATION:-30s}"
GATEWAY="${GATEWAY:-http://bid-gateway:8080}"
OUTPUT="${OUTPUT:-loadtest/vegeta/vegeta-report.txt}"

echo "Running Vegeta attack at ${RATE} rps for ${DURATION} against ${GATEWAY}..."
host_root=$PWD
if command -v cygpath >/dev/null; then host_root=$(cygpath -m "$PWD"); fi

MSYS_NO_PATHCONV=1 docker run --rm -i --entrypoint sh --network pulsebid_default \
  -v "${host_root}/loadtest/vegeta:/work" pulsebid/vegeta:local -c \
  "echo 'POST ${GATEWAY}/v1/bid' | vegeta attack -rate=${RATE} -duration=${DURATION} -header 'Content-Type: application/json' -body /work/bid.json | vegeta report" | tee "${OUTPUT}"
