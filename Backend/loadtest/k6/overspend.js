import http from 'k6/http';
import { check } from 'k6';
import { Counter } from 'k6/metrics';

const fills = new Counter('fills');
const errorNoFills = new Counter('error_no_fills');
const bidderNoFills = new Counter('bidder_no_fills');
export const options = {
  scenarios: { hammer: { executor: 'per-vu-iterations', vus: 50, iterations: 40, maxDuration: '60s' } },
  // A timed-out Redis response can leave a charged reservation. The safety
  // guarantee is at most 49 fills; Python independently verifies exact spend.
  thresholds: { checks: ['rate==1'], fills: ['count>0', 'count<=49'], http_req_failed: ['rate==0'] },
};

export default function () {
  const response = http.post(`${__ENV.GATEWAY}/v1/bid`, JSON.stringify({
    id: `overspend-${__VU}-${__ITER}`,
    imp: { id: '1', placement_id: __ENV.PLACEMENT, w: 300, h: 250, bidfloor: 1 },
    device: { type: 'mobile' }, geo: { country: 'IN' }, tmax: 100,
  }), { headers: { 'Content-Type': 'application/json' } });
  if (response.status === 200) {
    fills.add(1);
    check(response, {
      'isolated campaign and exact clearing price': r => r.json('campaign_id') === __ENV.CAMPAIGN_ID && r.json('price') === 1.01,
    });
  } else {
    const reason = response.headers['X-Pulsebid-Reason'];
    if (reason === 'error') errorNoFills.add(1);
    if (reason === 'no_bids') bidderNoFills.add(1);
    check(response, {
      'documented no-fill outcome': r => r.status === 204 && ['budget', 'no_eligible', 'error', 'no_bids'].includes(reason),
    });
  }
}
