import { expect, it, vi } from "vitest";
import { serviceHealth } from "./health";
import { healthSchema } from "@/lib/schemas/health";
it("keeps healthy probes beside degraded, HTTP and transport failures", async () => {
  vi.stubEnv("BIDDER_COUNT", "2");
  const fetcher = vi
    .fn()
    .mockResolvedValueOnce(new Response("ok"))
    .mockResolvedValueOnce(new Response("degraded"))
    .mockResolvedValueOnce(new Response("unavailable", { status: 503 }))
    .mockRejectedValueOnce(new Error("timeout"))
    .mockResolvedValueOnce(new Response("ok"))
    .mockResolvedValueOnce(new Response("ok"));
  vi.stubGlobal("fetch", fetcher);
  const result = healthSchema.parse(await serviceHealth("probe-trace"));
  expect(result.services.map((row) => row.status)).toEqual([
    "healthy",
    "degraded",
    "unavailable",
    "unavailable",
    "healthy",
    "healthy",
  ]);
  expect(result.services[3]?.status_code).toBeNull();
  expect(result.services.every((row) => row.latency_ms >= 0)).toBe(true);
  expect(fetcher.mock.calls[0]?.[1]).toMatchObject({
    redirect: "manual",
    cache: "no-store",
    headers: { "X-Request-Id": "probe-trace" },
  });
});
