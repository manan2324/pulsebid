#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."
docker compose --env-file "${COMPOSE_ENV_FILE:-.env}" \
  -f deploy/docker-compose.yml -f deploy/docker-compose.scale.yml \
  --profile apps --profile scale up -d --wait --wait-timeout 180 --scale bid-gateway=3
for attempt in {1..30}; do
  if curl -sS -D - -o /dev/null -H 'Content-Type: application/json' -d '{}' \
      "${GATEWAY_URL:-http://localhost:8088}/v1/bid" | grep -qi '^X-Request-Id:'; then
    break
  fi
  if [[ $attempt == 30 ]]; then echo 'Traefik did not discover the gateway route' >&2; exit 1; fi
  sleep 1
done
python3 -I scripts/verify-system.py overspend
