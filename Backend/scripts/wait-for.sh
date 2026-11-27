#!/usr/bin/env bash
# Wait for an HTTP readiness URL with a bounded total timeout.
set -euo pipefail

if [[ $# -lt 1 || $# -gt 2 ]]; then
  echo "Usage: bash scripts/wait-for.sh HTTP_URL [TIMEOUT_SECONDS]" >&2
  exit 2
fi
url=$1
timeout=${2:-60}
if [[ ! $timeout =~ ^[1-9][0-9]*$ || ! $url =~ ^https?:// ]]; then
  echo "Expected an HTTP(S) URL and a positive integer timeout." >&2
  exit 2
fi
command -v curl >/dev/null || { echo "curl is required." >&2; exit 127; }
deadline=$((SECONDS + timeout))
while (( SECONDS < deadline )); do
  remaining=$((deadline - SECONDS))
  if curl --fail --silent --output /dev/null --connect-timeout 2 --max-time "$remaining" "$url"; then
    echo "Ready: $url"
    exit 0
  fi
  if (( SECONDS < deadline )); then sleep 1; fi
done
echo "Timed out after ${timeout}s waiting for $url" >&2
exit 1
