#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."
GATEWAY_URL="${GATEWAY_URL:-http://localhost:8088}" python3 -I scripts/verify-system.py duplicate
