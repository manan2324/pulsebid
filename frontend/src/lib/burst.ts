import { sendBid, type AuctionResult } from "./api/gateway";
import { randomizeRequest, type BidRequest } from "./schemas/bid";
import { percentile, latencyHistogram } from "./stats";
import { simulatorCopy as c } from "./copy/simulator";
export type BurstConfig = {
  count: number;
  concurrency: number;
  mode: "fixed" | "random";
};
export function summarizeBurst(results: readonly AuctionResult[]) {
  const samples = results.flatMap((result) =>
    result.gatewayMs === null ? [] : [result.gatewayMs],
  );
  const counts = { filled: 0, "no-fill": 0, error: 0 };
  const reasons: Record<string, number> = {};
  for (const result of results) {
    counts[result.kind]++;
    if (result.kind === "no-fill") {
      const reason = result.reason ?? "unknown";
      reasons[reason] = (reasons[reason] ?? 0) + 1;
    }
  }
  return {
    counts,
    reasons,
    completed: results.length,
    fillRate: results.length ? counts.filled / results.length : 0,
    p50: percentile(samples, 50),
    p95: percentile(samples, 95),
    p99: percentile(samples, 99),
    sampleCount: samples.length,
    histogram: latencyHistogram(samples),
  };
}
export type BurstSummary = ReturnType<typeof summarizeBurst>;
export async function runBurst(
  request: BidRequest,
  config: BurstConfig,
  options: {
    signal: AbortSignal;
    testTime?: string;
    onProgress?: (completed: number, started: number) => void;
    onResult?: (result: AuctionResult) => void;
  },
  send = sendBid,
) {
  if (
    !Number.isInteger(config.count) ||
    config.count < 1 ||
    config.count > 200 ||
    !Number.isInteger(config.concurrency) ||
    config.concurrency < 1 ||
    config.concurrency > 10
  )
    throw new RangeError(c.burstError);
  const results: AuctionResult[] = [];
  let started = 0;
  await Promise.all(
    Array.from(
      { length: Math.min(config.count, config.concurrency) },
      async () => {
        while (!options.signal.aborted && started < config.count) {
          started++;
          const current =
            config.mode === "random" ? randomizeRequest(request) : request;
          try {
            const result = await send(current, {
              signal: options.signal,
              testTime: options.testTime,
            });
            results.push(result);
            options.onResult?.(result);
            options.onProgress?.(results.length, started);
          } catch (error) {
            if (options.signal.aborted) break;
            throw error;
          }
        }
      },
    ),
  );
  return {
    ...summarizeBurst(results),
    started,
    requested: config.count,
    cancelled: options.signal.aborted,
  };
}
export type BurstResult = Awaited<ReturnType<typeof runBurst>>;
