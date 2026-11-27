import { z } from "zod";
import { labCopy as c } from "./copy/lab";
import type { BurstResult } from "./burst";
export const bidderConfigSchema = z.strictObject({
  latency_ms: z.number().int().min(0).max(1000),
  jitter_ms: z.number().int().min(0).max(500),
  error_rate: z.number().min(0).max(1),
  timeout_rate: z.number().min(0).max(1),
  no_bid_rate: z.number().min(0).max(1),
  bid_mode: z.enum(["fixed", "shade", "random"]),
  bid_shade: z.number().min(0).max(1),
  seed: z.number().int().safe(),
});
export type BidderConfig = z.infer<typeof bidderConfigSchema>;
export const defaultBidderConfig: BidderConfig = {
  latency_ms: 10,
  jitter_ms: 15,
  error_rate: 0,
  timeout_rate: 0,
  no_bid_rate: 0.05,
  bid_mode: "fixed",
  bid_shade: 0.9,
  seed: 1,
};
export const presets = [
  "healthy",
  "slow",
  "failing",
  "hanging",
  "allError",
  "allHang",
] as const;
export type LabPreset = (typeof presets)[number];
export function presetConfig(
  preset: LabPreset,
  position: number,
): Partial<BidderConfig> {
  const healthy = {
    latency_ms: 10,
    jitter_ms: 0,
    error_rate: 0,
    timeout_rate: 0,
    no_bid_rate: 0,
  };
  if (preset === "allError" || (position === 0 && preset === "failing"))
    return { ...healthy, error_rate: 1 };
  if (preset === "allHang" || (position === 0 && preset === "hanging"))
    return { ...healthy, timeout_rate: 1 };
  return {
    ...healthy,
    latency_ms: position === 0 && preset === "slow" ? 500 : 10,
  };
}
export function configDiff(
  before: BidderConfig,
  after: BidderConfig,
): Partial<BidderConfig> {
  return Object.fromEntries(
    Object.entries(after).filter(
      ([key, value]) => before[key as keyof BidderConfig] !== value,
    ),
  );
}
export function scenarioVerdict(baseline: BurstResult, scenario: BurstResult) {
  if (
    [baseline, scenario].some(
      (result) =>
        result.cancelled ||
        result.completed !== result.requested ||
        !result.counts.filled ||
        result.sampleCount !== result.completed ||
        result.p99 === null,
    )
  )
    return c.inconclusive;
  return scenario.p99! < 120 ? c.resilient : c.degraded;
}
export const restoreSchema = z.strictObject({
  entries: z
    .array(
      z.strictObject({
        index: z.number().int().min(0).max(99),
        config: bidderConfigSchema,
      }),
    )
    .min(1)
    .max(100),
});
export type RestoreEntry = z.infer<typeof restoreSchema>["entries"][number];
export function restoreBeacon(entries: RestoreEntry[]) {
  if (!entries.length) return false;
  return navigator.sendBeacon(
    "/api/lab/reset",
    new Blob([JSON.stringify({ entries })], { type: "application/json" }),
  );
}
