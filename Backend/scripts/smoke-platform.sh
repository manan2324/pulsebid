#!/usr/bin/env bash
# Run on Linux/macOS: validates real process signals, not go-run wrapper signals.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."
source scripts/database-env.sh
work=$(mktemp -d)
pid=''
catalog_pid=''
cleanup() {
  status=$?
  if (( status != 0 )); then
    for log in "$work"/*.log; do
      [[ -f $log ]] || continue
      echo "--- $log ---" >&2
      tail -n 60 "$log" >&2
    done
  fi
  for child in "$pid" "$catalog_pid"; do
    if [[ -n $child ]]; then kill -TERM "$child" 2>/dev/null || true; wait "$child" || true; fi
  done
  rm -rf -- "$work"
  return "$status"
}
trap cleanup EXIT
go build -o "$work/" ./cmd/...
# Gateway readiness requires a real catalog snapshot. Keep a dedicated catalog
# alive while testing each process, including the campaign-service under test.
# Separate ports avoid depending on an application already running on :8082.
SERVICE_NAME=campaign-service ADMIN_ADDR=127.0.0.1:18089 HTTP_ADDR=127.0.0.1:18189 GRPC_ADDR=127.0.0.1:18289 \
  SHUTDOWN_DELAY=1s SHUTDOWN_TIMEOUT=5s OTEL_TRACES_SAMPLER_ARG=0 \
  "$work/campaign-service" >"$work/catalog.log" 2>&1 &
catalog_pid=$!
bash scripts/wait-for.sh http://127.0.0.1:18089/readyz 15
port=18090
for service in bid-gateway campaign-service event-aggregator mock-bidder seed; do
  args=()
  if [[ $service == mock-bidder ]]; then args=(--count 1 --base-port "$((port+300))"); fi
  SERVICE_NAME="$service" ADMIN_ADDR="127.0.0.1:$port" HTTP_ADDR="127.0.0.1:$((port+100))" GRPC_ADDR="127.0.0.1:$((port+200))" \
    CAMPAIGN_GRPC_ADDR=127.0.0.1:18289 \
    CONSUMER_GROUP="pulsebid-platform-smoke-$port-$$" \
    SHUTDOWN_DELAY=1s SHUTDOWN_TIMEOUT=5s OTEL_TRACES_SAMPLER_ARG=0 \
    "$work/$service" "${args[@]}" >"$work/$service.log" 2>&1 &
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
  echo "$service: health, readiness, metrics, request ID, drain, and SIGTERM exit passed"
  port=$((port+1))
done
