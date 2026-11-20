#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."

CAMPAIGN_URL="${CAMPAIGN_URL:-http://localhost:8081}"
GATEWAY_URL="${GATEWAY_URL:-http://localhost:8080}"
API_KEY="${API_KEY:-dev-key}"
REPORT_DIR="docs/test-reports/$(date +%F)/phase-12"
mkdir -p "$REPORT_DIR"

echo "=== Running Multi-Replica Overspend Proof ==="

# 1. Reset redis spend
docker exec pulsebid-redis-1 redis-cli flushdb >/dev/null 2>&1 || docker compose -f deploy/docker-compose.yml exec -T redis redis-cli flushdb >/dev/null 2>&1 || true

# 2. Find or create an advertiser
adv_json=$(curl -sS -f -H "X-API-Key: ${API_KEY}" "${CAMPAIGN_URL}/v1/advertisers?limit=1")
adv_id=$(echo "${adv_json}" | grep -o '"id":"[^"]*"' | head -n1 | cut -d'"' -f4)

# 3. Create a tiny budget campaign ($0.05 = 50,000 micros)
camp_payload='{
  "advertiser_id": "'"${adv_id}"'",
  "bidder_id": "bidder-1",
  "name": "Multi-Replica Overspend Test",
  "daily_budget": 0.05,
  "max_bid_cpm": 3.00,
  "pacing_mode": "asap"
}'
camp_res=$(curl -sS -f -H "X-API-Key: ${API_KEY}" -H "Content-Type: application/json" \
  -d "${camp_payload}" "${CAMPAIGN_URL}/v1/campaigns")
camp_id=$(echo "${camp_res}" | grep -o '"id":"[^"]*"' | head -n1 | cut -d'"' -f4)
echo "Created tiny budget campaign: ${camp_id} ($0.05 daily budget)"

# Set targeting and creative
curl -sS -f -X PUT -H "X-API-Key: ${API_KEY}" -H "Content-Type: application/json" \
  -d '{"countries":["IN"],"device_types":["mobile"],"placement_ids":["scale-slot"],"timezone":"UTC"}' \
  "${CAMPAIGN_URL}/v1/campaigns/${camp_id}/targeting" >/dev/null

curl -sS -f -H "X-API-Key: ${API_KEY}" -H "Content-Type: application/json" \
  -d '{"name":"Scale Banner","format":"banner","width":300,"height":250,"markup":"<div></div>","landing_url":"https://test.com"}' \
  "${CAMPAIGN_URL}/v1/campaigns/${camp_id}/creatives" >/dev/null

curl -sS -f -X POST -H "X-API-Key: ${API_KEY}" "${CAMPAIGN_URL}/v1/campaigns/${camp_id}/activate" >/dev/null
echo "Campaign activated. Waiting 3s for catalog snapshot refresh..."
sleep 3

# 4. Hammer the campaign across replicas with concurrent requests
echo "Hammering campaign ${camp_id} with 2,000 concurrent requests across replicas..."

mkdir -p scratch
cat <<EOF > scratch/overspend_k6.js
import http from 'k6/http';
import { check } from 'k6';

export const options = {
  scenarios: {
    hammer: {
      executor: 'per-vu-iterations',
      vus: 50,
      iterations: 40, // 50 * 40 = 2,000 requests
      maxDuration: '30s',
    },
  },
};

export default function () {
  const body = JSON.stringify({
    id: 'scale-' + __VU + '-' + __ITER,
    imp: { id: '1', placement_id: 'scale-slot', w: 300, h: 250, bidfloor: 1.00 },
    device: { type: 'mobile' },
    geo: { country: 'IN' },
    tmax: 100,
  });
  const res = http.post('${GATEWAY_URL}/v1/bid', body, {
    headers: { 'Content-Type': 'application/json' },
  });
  check(res, { 'valid status': (r) => r.status === 200 || r.status === 204 });
}
EOF

if [ -f "./loadtest/k6/k6.exe" ]; then
  ./loadtest/k6/k6.exe run scratch/overspend_k6.js
elif command -v k6 >/dev/null 2>&1; then
  k6 run scratch/overspend_k6.js
else
  MSYS_NO_PATHCONV=1 docker run --rm -i --network pulsebid_default grafana/k6:latest run - < scratch/overspend_k6.js
fi

# 5. Check budget API directly
budget_json=$(curl -sS -f -H "X-API-Key: ${API_KEY}" "${CAMPAIGN_URL}/v1/campaigns/${camp_id}/budget")
echo "Budget result: ${budget_json}" | tee "${REPORT_DIR}/overspend-proof.json"

spent_micros=$(echo "${budget_json}" | grep -o '"spent_micros":[0-9]*' | cut -d':' -f2)
daily_budget_micros=$(echo "${budget_json}" | grep -o '"daily_budget_micros":[0-9]*' | cut -d':' -f2)

if (( spent_micros > daily_budget_micros )); then
  echo "OVERSPEND DETECTED: spent ${spent_micros} > daily budget ${daily_budget_micros}!"
  exit 1
fi

echo "SUCCESS: 0 overspend confirmed! Spent ${spent_micros} <= Daily budget ${daily_budget_micros} micros."

# Run verify-no-overspend across all campaigns
bash scripts/verify-no-overspend.sh | tee -a "${REPORT_DIR}/overspend-proof.txt"
