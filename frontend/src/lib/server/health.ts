import "server-only";
import { getEnv } from "./env";
import type { Health } from "@/lib/schemas/health";
export async function serviceHealth(id: string): Promise<Health> {
  const env = getEnv();
  const endpoints: [string, string][] = [
    ["Gateway", `${env.GATEWAY_ADMIN_URL}/readyz`],
    ["Campaign", `${env.CAMPAIGN_ADMIN_URL}/readyz`],
    ["Aggregator", `${env.AGGREGATOR_ADMIN_URL}/readyz`],
    ["Prometheus", `${env.PROMETHEUS_URL}/-/ready`],
    ...Array.from({ length: env.BIDDER_COUNT }, (_, i): [string, string] => [
      `Bidder ${i + 1}`,
      `http://${env.BIDDER_HOST}:${env.BIDDER_BASE_PORT + i}/healthz`,
    ]),
  ];
  const services = await Promise.all(
    endpoints.map(async ([name, url]) => {
      const start = performance.now();
      try {
        const response = await fetch(url, {
          signal: AbortSignal.timeout(2000),
          cache: "no-store",
          redirect: "manual",
          headers: { "X-Request-Id": id },
        });
        const body = await response.text();
        return {
          name,
          status: response.ok
            ? /degraded/i.test(body)
              ? ("degraded" as const)
              : ("healthy" as const)
            : ("unavailable" as const),
          status_code: response.status,
          latency_ms: Math.round(performance.now() - start),
        };
      } catch {
        return {
          name,
          status: "unavailable" as const,
          status_code: null,
          latency_ms: Math.round(performance.now() - start),
        };
      }
    }),
  );
  return { services, checked_at: new Date().toISOString() };
}
