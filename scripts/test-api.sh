#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."
read -r -a compose <<< "${COMPOSE:-docker compose}"
env_file=${COMPOSE_ENV_FILE:-.env}
[[ -f $env_file ]] || env_file=.env.example
compose+=(--env-file "$env_file" -f deploy/docker-compose.yml)
base_compose=("${compose[@]}")
"${compose[@]}" up -d --wait --wait-timeout 180
export TEST_DATABASE_NAME="pulsebid_api_$(date +%s)_$$"
"${base_compose[@]}" exec -T postgres createdb -U pulsebid "$TEST_DATABASE_NAME"
cleanup() {
  result=$?
  trap - EXIT
  set +e
  # Restore the normal databases before dropping only this run's private DB.
  "${base_compose[@]}" --profile apps up --no-build -d --wait --wait-timeout 180 campaign-service event-aggregator bid-gateway
  restored=$?
  "${base_compose[@]}" exec -T postgres dropdb -U pulsebid --force "$TEST_DATABASE_NAME"
  dropped=$?
  if (( restored != 0 || dropped != 0 )); then exit 1; fi
  exit "$result"
}
trap cleanup EXIT
compose+=(-f deploy/docker-compose.test.yml)
"${compose[@]}" --profile tools run --build --rm migrate up
"${compose[@]}" --profile apps up --build -d --wait --wait-timeout 180 campaign-service bid-gateway event-aggregator mock-bidders
report="docs/test-reports/$(date +%F)/${PHASE:-phase-12}"
mkdir -p "$report"
"${compose[@]}" --profile tools run --build --rm seed
host_root=$PWD
if command -v cygpath >/dev/null; then host_root=$(cygpath -m "$PWD"); fi
MSYS_NO_PATHCONV=1 docker run --rm --network pulsebid_default \
  --mount "type=bind,source=$host_root,target=/work" \
  --mount type=volume,source=pulsebid-newman-cache,target=/root/.npm \
  --workdir /work -e "REPORT_DIR=$report" node:22-alpine sh -c '
    npx --yes --package newman@6.2.1 --package newman-reporter-htmlextra@1.23.1 newman run postman/PulseBid.postman_collection.json \
      -e postman/local.postman_environment.json \
      --ignore-redirects \
      --env-var gateway_url=http://bid-gateway:8080 \
      --env-var gateway_admin_url=http://bid-gateway:8090 \
      --env-var campaign_admin_url=http://campaign-service:8091 \
      --env-var aggregator_admin_url=http://event-aggregator:8093 \
      --env-var reporting_url=http://event-aggregator:8083 \
      --env-var campaign_url=http://campaign-service:8081 \
      --env-var bidder_host=mock-bidders \
      --reporters cli,htmlextra,junit,json \
      --reporter-htmlextra-export "$REPORT_DIR/postman-report.html" \
      --reporter-junit-export "$REPORT_DIR/junit.xml" \
      --reporter-json-export "$REPORT_DIR/results.json" \
      --timeout-request 5000 --delay-request 20
  '
printf 'Newman report: %s/postman-report.html\n' "$report"
bash scripts/verify-no-overspend.sh
