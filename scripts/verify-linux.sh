#!/usr/bin/env bash
# Run the full required Go suite with a 64-bit compiler on Docker hosts.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."
report="${REPORT_DIR:-docs/test-reports/$(date +%F)/verification}"
mkdir -p "$report"
root=$PWD
if command -v cygpath >/dev/null; then root=$(cygpath -m "$PWD"); fi
docker run --rm --network host \
  --mount "type=bind,source=$root,target=/src" \
  --mount type=bind,source=/var/run/docker.sock,target=/var/run/docker.sock \
  --mount type=volume,source=pulsebid-go-module-cache,target=/go/pkg/mod \
  --mount type=volume,source=pulsebid-go-build-cache,target=/root/.cache/go-build \
  --workdir /src -e REQUIRE_INTEGRATION=1 -e GOFLAGS=-buildvcs=false \
  -e "DATABASE_URL=${DATABASE_URL:-postgres://pulsebid:pulsebid@localhost:15432/pulsebid?sslmode=disable}" \
  -e TESTCONTAINERS_HOST_OVERRIDE=localhost -e "REPORT_DIR=$report" \
  golang:1.27.3-alpine sh -ec '
    apk add --no-cache bash make gcc musl-dev git curl
    git config --global --add safe.directory /src
    make lint build
    go test -race -count=1 -tags=integration -coverprofile="$REPORT_DIR/coverage.out" ./...
    go test -race -count=5 ./internal/budget ./internal/gateway ./internal/aggregator
    make fuzz FUZZTIME=10s
    go test -run "^$" -bench . -benchmem ./internal/gateway
    make test-platform
  ' 2>&1 | tee "$report/go-verification.txt"
