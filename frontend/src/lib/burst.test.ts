import { it, expect, vi } from "vitest";
import { runBurst, summarizeBurst } from "./burst";
import { presetRequest } from "./schemas/bid";
import type { AuctionResult } from "./api/gateway";
const request = presetRequest("mobile");
function result(index: number): AuctionResult {
  return {
    id: String(index),
    at: new Date().toISOString(),
    request,
    httpStatus: 204,
    kind: "no-fill",
    response: null,
    error: null,
    reason: "no_eligible",
    gatewayMs: index,
    roundTripMs: index + 10,
    requestId: String(index),
    debug: null,
  };
}
it("runs exactly N requests with bounded concurrency and reports gateway percentiles", async () => {
  let active = 0,
    max = 0,
    index = 0;
  const send = vi.fn(async () => {
    active++;
    max = Math.max(max, active);
    await Promise.resolve();
    active--;
    return result(++index);
  });
  const output = await runBurst(
    request,
    { count: 100, concurrency: 4, mode: "fixed" },
    { signal: new AbortController().signal },
    send,
  );
  expect(send).toHaveBeenCalledTimes(100);
  expect(max).toBeLessThanOrEqual(4);
  expect(output).toMatchObject({
    completed: 100,
    started: 100,
    requested: 100,
    cancelled: false,
    p50: 50,
    p95: 95,
    p99: 99,
    sampleCount: 100,
    counts: { "no-fill": 100 },
    reasons: { no_eligible: 100 },
  });
});
it("cancels in-flight work and never schedules the remaining requests", async () => {
  const controller = new AbortController();
  const send = vi.fn(
    async (
      _request: typeof request,
      options: { signal?: AbortSignal; testTime?: string } = {},
    ) => {
      controller.abort();
      options?.signal?.throwIfAborted();
      return result(1);
    },
  );
  const output = await runBurst(
    request,
    { count: 100, concurrency: 3, mode: "fixed" },
    { signal: controller.signal },
    send,
  );
  expect(send).toHaveBeenCalledTimes(1);
  expect(output).toMatchObject({
    completed: 0,
    started: 1,
    cancelled: true,
    p99: null,
  });
});
it("does not count missing timing as zero or include it in a passing verdict", () => {
  expect(summarizeBurst([{ ...result(1), gatewayMs: null }])).toMatchObject({
    p99: null,
    sampleCount: 0,
  });
});
it("validates burst limits before sending", async () => {
  const send = vi.fn();
  await expect(
    runBurst(
      request,
      { count: 201, concurrency: 1, mode: "fixed" },
      { signal: new AbortController().signal },
      send,
    ),
  ).rejects.toThrow();
  expect(send).not.toHaveBeenCalled();
});
