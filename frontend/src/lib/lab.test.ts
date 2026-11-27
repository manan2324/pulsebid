import { expect, it, vi } from "vitest";
import {
  bidderConfigSchema,
  configDiff,
  defaultBidderConfig as defaults,
  presetConfig,
  restoreBeacon,
  scenarioVerdict,
} from "./lab";
import type { BurstResult } from "./burst";
const summary: BurstResult = {
  completed: 60,
  requested: 60,
  started: 60,
  cancelled: false,
  counts: { filled: 60, "no-fill": 0, error: 0 },
  reasons: {},
  fillRate: 1,
  p50: 20,
  p95: 30,
  p99: 40,
  sampleCount: 60,
  histogram: [],
};
it("maps one slow bidder and keeps peers healthy", () => {
  expect(presetConfig("slow", 0).latency_ms).toBe(500);
  expect(presetConfig("slow", 1).latency_ms).toBe(10);
  expect(presetConfig("failing", 0).error_rate).toBe(1);
  expect(presetConfig("hanging", 0).timeout_rate).toBe(1);
  expect(presetConfig("allError", 7).error_rate).toBe(1);
  expect(presetConfig("allHang", 7).timeout_rate).toBe(1);
});
it("sends only changed config fields", () => {
  expect(configDiff(defaults, { ...defaults, latency_ms: 500 })).toEqual({
    latency_ms: 500,
  });
  expect(configDiff(defaults, defaults)).toEqual({});
});
it("rejects unsafe/invalid configs", () => {
  expect(
    bidderConfigSchema.safeParse({ ...defaults, error_rate: 1.1 }).success,
  ).toBe(false);
  expect(
    bidderConfigSchema.safeParse({ ...defaults, latency_ms: -1 }).success,
  ).toBe(false);
  expect(
    bidderConfigSchema.safeParse({ ...defaults, endpoint: "https://evil.test" })
      .success,
  ).toBe(false);
});
it("requires complete filled timed traffic for a resilience verdict", () => {
  expect(scenarioVerdict(summary, summary)).toContain("Resilient");
  expect(scenarioVerdict(summary, { ...summary, p99: 120 })).toBe("Degraded");
  for (const patch of [
    { p99: null },
    { cancelled: true },
    { sampleCount: 59 },
    { counts: { filled: 0, "no-fill": 60, error: 0 } },
  ])
    expect(scenarioVerdict(summary, { ...summary, ...patch })).toContain(
      "Inconclusive",
    );
});
it("sends bounded originals as JSON to the same-origin restoration endpoint", () => {
  const sendBeacon = vi.fn(() => true);
  vi.stubGlobal("navigator", { sendBeacon });
  expect(restoreBeacon([])).toBe(false);
  expect(restoreBeacon([{ index: 0, config: defaults }])).toBe(true);
  expect(sendBeacon).toHaveBeenCalledWith("/api/lab/reset", expect.any(Blob));
});
