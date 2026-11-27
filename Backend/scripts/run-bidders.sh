#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."

count=${1:-${N:-${BIDDER_COUNT:-8}}}
base_port=${2:-${BIDDER_BASE_PORT:-9101}}

echo "Starting $count mock bidders on ports $base_port-$((base_port + count - 1))..."
exec go run ./cmd/mock-bidder --count "$count" --base-port "$base_port"
