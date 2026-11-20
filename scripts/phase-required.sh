#!/usr/bin/env bash
set -euo pipefail
echo "${2:?target} is implemented in Phase ${1:?phase}; see PulseBid Build Plan.md." >&2
exit 2
