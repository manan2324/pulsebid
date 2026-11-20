#!/usr/bin/env bash
set -euo pipefail

CAMPAIGN_URL="${CAMPAIGN_URL:-http://localhost:8081}"
API_KEY="${API_KEY:-dev-key}"

echo "Checking budget spend across campaigns at ${CAMPAIGN_URL}..."

campaigns_json=$(curl -sS -f -H "X-API-Key: ${API_KEY}" "${CAMPAIGN_URL}/v1/campaigns?limit=200")
campaign_ids=$(echo "${campaigns_json}" | grep -o '"id":"[^"]*"' | cut -d'"' -f4)

if [[ -z "${campaign_ids}" ]]; then
  echo "No campaigns found to verify."
  exit 0
fi

overspend_detected=0

for id in ${campaign_ids}; do
  budget_json=$(curl -sS -f -H "X-API-Key: ${API_KEY}" "${CAMPAIGN_URL}/v1/campaigns/${id}/budget")
  daily_budget_micros=$(echo "${budget_json}" | grep -o '"daily_budget_micros":[0-9]*' | cut -d':' -f2)
  spent_micros=$(echo "${budget_json}" | grep -o '"spent_micros":[0-9]*' | cut -d':' -f2)

  if [[ -n "${daily_budget_micros}" && -n "${spent_micros}" ]]; then
    if (( spent_micros > daily_budget_micros )); then
      echo "OVERSPEND DETECTED for campaign ${id}: spent ${spent_micros} > daily budget ${daily_budget_micros}!"
      overspend_detected=1
    else
      echo "OK campaign ${id}: spent ${spent_micros} <= budget ${daily_budget_micros} micros."
    fi
  fi
done

if (( overspend_detected != 0 )); then
  echo "FAILURE: Budget overspend detected!"
  exit 1
fi

echo "SUCCESS: Zero overspend verified across all campaigns."
