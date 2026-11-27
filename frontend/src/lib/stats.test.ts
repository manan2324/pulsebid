import { it, expect } from "vitest";
import { percentile, latencyHistogram } from "./stats";
it("uses nearest rank on unsorted arrays, ties and boundary percentiles without mutation", () => {
  const values = [50, 10, 40, 20, 30];
  expect(percentile(values, 50)).toBe(30);
  expect(percentile(values, 95)).toBe(50);
  expect(percentile(values, 99)).toBe(50);
  expect(percentile(values, 0)).toBe(10);
  expect(percentile(values, 100)).toBe(50);
  expect(values).toEqual([50, 10, 40, 20, 30]);
  expect(percentile([10, 10, 10], 99)).toBe(10);
});
it("keeps missing/invalid samples out of latency statistics", () => {
  expect(percentile([], 99)).toBeNull();
  expect(percentile([NaN, Infinity, -1], 99)).toBeNull();
  expect(() => percentile([10], 101)).toThrow();
  expect(
    latencyHistogram([0, 19.9, 20, 119.9, 120, 140]).map((row) => row.count),
  ).toEqual([2, 1, 0, 0, 0, 1, 2]);
});
