#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."
source scripts/database-env.sh
go run ./cmd/migrate "${1:?up or down required}"
