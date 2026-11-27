#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."
target=${GRPC_TARGET:-localhost:8082}
grpcurl=(go run github.com/fullstorydev/grpcurl/cmd/grpcurl@v1.9.3)
report="docs/test-reports/$(date +%F)/phase-3"
mkdir -p "$report"
"${grpcurl[@]}" -plaintext -max-time 5 "$target" pulsebid.catalog.v1.CatalogService/GetSnapshot | tee "$report/grpc-snapshot.json"
health=$("${grpcurl[@]}" -plaintext -max-time 5 \
  -d '{"service":"pulsebid.catalog.v1.CatalogService"}' "$target" grpc.health.v1.Health/Check)
printf '%s\n' "$health" | tee "$report/grpc-health.json"
grep -Eq '"status"[[:space:]]*:[[:space:]]*"SERVING"' <<< "$health"
