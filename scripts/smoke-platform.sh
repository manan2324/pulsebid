#!/usr/bin/env bash
# Run on Linux/macOS: validates real process signals, not go-run wrapper signals.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."
source scripts/database-env.sh
work=$(mktemp -d)
pid=''
cleanup() {
  if [[ -n $pid ]]; then kill -TERM "$pid" 2>/dev/null || true; wait "$pid" || true; fi
  rm -rf -- "$work"
}
trap cleanup EXIT
go build -o "$work/" ./cmd/...
port=18090
for service in bid-gateway campaign-service event-aggregator mock-bidder seed; do
  SERVICE_NAME="$service" ADMIN_ADDR="127.0.0.1:$port" HTTP_ADDR="127.0.0.1:$((port+100))" GRPC_ADDR="127.0.0.1:$((port+200))" \
    SHUTDOWN_DELAY=1s SHUTDOWN_TIMEOUT=5s OTEL_TRACES_SAMPLER_ARG=0 \
    "$work/$service" >"$work/$service.log" 2>&1 &
  pid=$!
  base="http://127.0.0.1:$port"
  if ! bash scripts/wait-for.sh "$base/readyz" 15; then cat "$work/$service.log"; exit 1; fi
  curl --fail --silent --show-error --max-time 2 -D "$work/headers" \
    -H 'X-Request-Id: smoke-request' "$base/healthz" >"$work/health"
  grep -qi '^X-Request-Id: smoke-request' "$work/headers"
  curl --fail --silent --show-error --max-time 2 "$base/metrics" >"$work/metrics"
  grep -q 'http_server_requests_total' "$work/metrics"
  grep -q 'go_goroutines' "$work/metrics"
  kill -TERM "$pid"
  code=0
  for _ in {1..50}; do
    code=$(curl --silent --max-time 1 -o /dev/null -w '%{http_code}' "$base/readyz" || true)
    if [[ $code == 503 ]]; then break; fi
    sleep 0.01
  done
  [[ $code == 503 ]] || { echo "$service did not become unready before closing" >&2; cat "$work/$service.log"; exit 1; }
  curl --fail --silent --show-error --max-time 1 "$base/healthz" >/dev/null
  wait "$pid"
  pid=''
  grep -q '"msg":"service stopped"' "$work/$service.log"
  echo "$service: health, readiness, metrics, request ID, drain, and SIGTERM exit passed"
  port=$((port+1))
done
