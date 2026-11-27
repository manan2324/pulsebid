#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."

echo "Starting 3 local Bid Gateway replicas..."
echo "  Replica 1: HTTP :8080, Admin :8090"
echo "  Replica 2: HTTP :8180, Admin :8190"
echo "  Replica 3: HTTP :8280, Admin :8290"

cleanup() {
  echo "Stopping replicas..."
  kill "${PID1:-}" "${PID2:-}" "${PID3:-}" 2>/dev/null || true
  wait "${PID1:-}" "${PID2:-}" "${PID3:-}" 2>/dev/null || true
}
trap cleanup EXIT INT TERM

HTTP_ADDR=":8080" ADMIN_ADDR=":8090" go run ./cmd/bid-gateway &
PID1=$!

HTTP_ADDR=":8180" ADMIN_ADDR=":8190" go run ./cmd/bid-gateway &
PID2=$!

HTTP_ADDR=":8280" ADMIN_ADDR=":8290" go run ./cmd/bid-gateway &
PID3=$!

echo "Replicas running (PIDs: $PID1, $PID2, $PID3). Press Ctrl+C to stop."
wait
