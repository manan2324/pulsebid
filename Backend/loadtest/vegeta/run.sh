#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/../.."
rate="${RATE:-5000}"
duration="${DURATION:-60s}"
gateway="${GATEWAY:-http://127.0.0.1:8080}"
report="${REPORT_DIR:-docs/test-reports/$(date +%F)/verification}"
mkdir -p "$report"
if [[ -n ${VEGETA_BIN:-} ]]; then
  runner=("$VEGETA_BIN")
elif command -v vegeta >/dev/null; then
  runner=(vegeta)
else
  root=$PWD
  if command -v cygpath >/dev/null; then root=$(cygpath -m "$PWD"); fi
  runner=(docker run --rm -i --network host --mount "type=bind,source=$root,target=/work" --workdir /work --entrypoint vegeta pulsebid/vegeta:local)
fi
printf 'POST %s/v1/bid\n' "$gateway" | "${runner[@]}" attack \
  -rate="$rate" -duration="$duration" -timeout=2s -max-workers=1000 -header 'Content-Type: application/json' \
  -body loadtest/vegeta/bid.json | python3 -I scripts/buffer-stream.py "$report/vegeta.bin"
"${runner[@]}" report -type=json "$report/vegeta.bin" > "$report/vegeta.json"
"${runner[@]}" report "$report/vegeta.bin" | tee "$report/vegeta.txt"
python3 -I - "$report/vegeta.json" "$rate" <<'PY'
import json, sys
with open(sys.argv[1], encoding='utf-8') as stream:
    report = json.load(stream)
if report['success'] < 0.99 or report['latencies']['99th'] >= 120_000_000 or report['throughput'] < float(sys.argv[2]) * 0.99:
    raise SystemExit('Vegeta throughput, success, or p99 threshold failed')
PY
