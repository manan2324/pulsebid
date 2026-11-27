export const promQueries = {
  bid_rps: { query: "sum(rate(bid_requests_total[1m]))", range: true },
  fill_rate: {
    query:
      'sum(rate(bid_requests_total{outcome="filled"}[5m])) / sum(rate(bid_requests_total[5m]))',
    range: true,
  },
  bid_p99: {
    query:
      "histogram_quantile(0.99, sum by (le) (rate(bid_duration_seconds_bucket[5m])))",
    range: true,
  },
  bid_p50: {
    query:
      "histogram_quantile(0.50, sum by (le) (rate(bid_duration_seconds_bucket[5m])))",
    range: true,
  },
  outcomes: {
    query: "sum by (outcome) (rate(bid_requests_total[1m]))",
    range: false,
  },
  bidder_timeout_ratio: {
    query:
      'sum by (bidder_id) (rate(bidder_requests_total{outcome="timeout"}[1m])) / sum by (bidder_id) (rate(bidder_requests_total[1m]))',
    range: false,
  },
  bidder_p99: {
    query:
      "histogram_quantile(0.99, sum by (le, bidder_id) (rate(bidder_latency_seconds_bucket[5m])))",
    range: false,
  },
  budget_results: {
    query: "sum by (result) (rate(budget_reserve_total[1m]))",
    range: false,
  },
  snapshot_age: { query: "max(snapshot_age_seconds)", range: false },
} as const;
export function prometheusPath(params: URLSearchParams, now = Date.now()) {
  const name = params.get("name");
  if (
    !name ||
    !Object.hasOwn(promQueries, name) ||
    [...params.keys()].some((key) => !["name", "range"].includes(key))
  )
    return null;
  const entry = promQueries[name as keyof typeof promQueries];
  const range = params.get("range");
  const query = new URLSearchParams({ query: entry.query });
  if (!range) return `/api/v1/query?${query}`;
  if (!entry.range || !["15m", "1h", "6h"].includes(range)) return null;
  const [seconds, step] =
    range === "15m" ? [900, 15] : range === "1h" ? [3600, 30] : [21600, 60];
  query.set("start", String(Math.floor(now / 1000) - seconds!));
  query.set("end", String(Math.floor(now / 1000)));
  query.set("step", String(step));
  return `/api/v1/query_range?${query}`;
}
