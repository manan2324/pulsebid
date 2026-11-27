import http from 'k6/http';
import { check } from 'k6';
import { Counter } from 'k6/metrics';

const fills = new Counter('fills');

const COUNTRIES = ['IN', 'US', 'GB', 'DE'];
const DEVICES = ['mobile', 'desktop', 'tablet'];
const RATE = Number(__ENV.RATE || 1000);
const DURATION = __ENV.DURATION || '30s';
const GATEWAY = __ENV.GATEWAY || 'http://127.0.0.1:8080';
const BIDDER_URL = __ENV.BIDDER_URL || 'http://127.0.0.1:9101';

export const options = {
  discardResponseBodies: true,
  summaryTrendStats: ['avg', 'min', 'med', 'max', 'p(90)', 'p(95)', 'p(99)'],
  scenarios: {
    slow_bidder_isolation: {
      executor: 'constant-arrival-rate',
      rate: RATE,
      timeUnit: '1s',
      duration: DURATION,
      preAllocatedVUs: Number(__ENV.PRE_VUS || 500),
      maxVUs: Number(__ENV.MAX_VUS || 1000),
    },
  },
  thresholds: {
    'http_req_duration{name:bid}': ['p(99)<120'],
    'http_req_failed': ['rate<0.01'],
    'checks': ['rate>0.99'],
    'dropped_iterations': ['count==0'],
    'fills': ['count>0'],
  },
};

export function setup() {
  const original = http.get(`${BIDDER_URL}/admin/config`, { responseType: 'text' });
  if (original.status !== 200) throw new Error('Could not read bidder configuration');
  // Tune bidder-1 (e.g. port 9101) to have 500ms latency to test isolation under load
  const payload = JSON.stringify({
    latency_ms: 500,
    jitter_ms: 0,
    error_rate: 0,
    timeout_rate: 0,
  });
  const res = http.post(`${BIDDER_URL}/admin/config`, payload, {
    headers: { 'Content-Type': 'application/json' },
  });
  console.log(`Setup slow bidder at ${BIDDER_URL}: status=${res.status}`);
  if (res.status !== 200) throw new Error('Could not configure slow bidder');
  return original.json();
}

export default function () {
  const iter = __ITER;
  const body = JSON.stringify({
    id: `slow-${__VU}-${iter}`,
    imp: {
      id: '1',
      placement_id: `slot-${iter % 10}`,
      w: 300,
      h: 250,
      bidfloor: Number((0.2 + (iter % 5) * 0.1).toFixed(2)),
    },
    device: {
      type: DEVICES[iter % DEVICES.length],
    },
    geo: {
      country: COUNTRIES[iter % COUNTRIES.length],
    },
    tmax: 100,
  });

  const res = http.post(`${GATEWAY}/v1/bid`, body, {
    headers: { 'Content-Type': 'application/json' },
    tags: { name: 'bid' },
  });
  if (res.status === 200) fills.add(1);

  check(res, {
    'status 200 or 204': (r) => r.status === 200 || r.status === 204,
  });
}

export function teardown(original) {
  const payload = JSON.stringify(original);
  const res = http.post(`${BIDDER_URL}/admin/config`, payload, {
    headers: { 'Content-Type': 'application/json' },
  });
  console.log(`Teardown: reset bidder-1 at ${BIDDER_URL} to normal latency: status=${res.status}`);
  if (res.status !== 200) throw new Error('Could not restore bidder configuration');
}
