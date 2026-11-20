#!/usr/bin/env bash
set -euo pipefail
report=$(find docs/test-reports -name postman-report.html -type f | sort | tail -n 1)
[[ -n $report ]] || { echo 'No Newman report found. Run make test-api first.' >&2; exit 1; }
case "$(uname -s)" in
  MINGW*|MSYS*) cygstart "$report" ;;
  Darwin) open "$report" ;;
  *) if command -v xdg-open >/dev/null; then xdg-open "$report"; else echo "$report"; fi ;;
esac
