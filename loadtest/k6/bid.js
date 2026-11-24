import http from 'k6/http';
import { check } from 'k6';
import { Counter, Trend } from 'k6/metrics';

const fills = new Counter('fills');
const nofills = new Counter('nofills');
const gatewayErrors = new Counter('gateway_errors');
const noBids = new Counter('no_bids');
const serverDuration = new Trend('server_duration', true);

const COUNTRIES = ['IN', 'US', 'GB', 'DE'];
const DEVICES = ['mobile', 'desktop', 'tablet'];
const RATE = Number(__ENV.RATE || 5000);
const DURATION = __ENV.DURATION || '2m';
const GATEWAY = __ENV.GATEWAY || 'http://127.0.0.1:8080';

export const options = {
  // HTTP latency includes the complete response; unused bodies are not retained
  // in JavaScript heaps on the machine that also runs the exchange.
  discardResponseBodies: true,
  summaryTrendStats: ['avg', 'min', 'med', 'max', 'p(90)', 'p(95)', 'p(99)'],
  scenarios: {
    bids: {
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

export default function () {
  const iter = __ITER;
  const body = JSON.stringify({
    id: `${__VU}-${iter}`,
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
  if (res.status === 204) {
    nofills.add(1);
    const reason = res.headers['X-Pulsebid-Reason'];
    if (reason === 'error') gatewayErrors.add(1);
    if (reason === 'no_bids') noBids.add(1);
  }
  if ((__ITER + __VU) % 100 === 0) {
    const timing = (res.headers['Server-Timing'] || '').match(/total;dur=([\d.]+)/);
    if (timing) serverDuration.add(Number(timing[1]));
  }

  check(res, {
    'status 200 or 204': (r) => r.status === 200 || r.status === 204,
  });
}
