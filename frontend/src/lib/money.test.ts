import { describe, expect, it } from "vitest";
import { formatUsd, hasMaxDecimals, microsToUsd } from "./money";
import { formatNumber, formatPercent } from "./format";
import { formatTime, utcDay } from "./time";

describe("money display without float aggregation", () => {
  it.each([
    [0, "$0.00"],
    [1.51, "$1.51"],
    [1.23456, "$1.2346"],
    [1234567.5, "$1,234,567.50"],
    [-1.51, "-$1.51"],
    [0.000001, "$0.00"],
  ] as const)("formats %s", (input, output) =>
    expect(formatUsd(input)).toBe(output),
  );

  it.each([
    [1, 0.000001],
    [1510000, 1.51],
    [9090, 0.00909],
    [0, 0],
    [-1, -0.000001],
  ] as const)("converts %s micros", (input, output) =>
    expect(microsToUsd(input)).toBe(output),
  );

  it.each([0.1, NaN, Infinity, Number.MAX_SAFE_INTEGER + 1])(
    "rejects unsafe micros %s",
    (input) => expect(() => microsToUsd(input)).toThrow(RangeError),
  );

  it("preserves per-impression precision", () =>
    expect(formatUsd(0.00151, { maxFraction: 6 })).toBe("$0.00151"));

  it("handles rounding at display precision", () =>
    expect(formatUsd(1.005, { maxFraction: 2 })).toBe("$1.01"));

  it.each(["1", "0", "-1.123456", "+1.0", "1.000000", "999999999999.123456"])(
    "accepts decimal string %s",
    (value) => expect(hasMaxDecimals(value)).toBe(true),
  );

  it.each(["1.1234567", "1e-6", "1.", ".5", " 1.2", "NaN", "", "1,234.56"])(
    "rejects decimal string %s",
    (value) => expect(hasMaxDecimals(value)).toBe(false),
  );

  it("supports explicit decimal limit", () => {
    expect(hasMaxDecimals("1.01", 2)).toBe(true);
    expect(hasMaxDecimals("1.001", 2)).toBe(false);
    expect(hasMaxDecimals("1", -1)).toBe(false);
  });

  it.each([NaN, Infinity, -Infinity])("non-finite display %s", (value) => {
    expect(formatUsd(value)).toBe("—");
    expect(formatNumber(value)).toBe("—");
    expect(formatPercent(value)).toBe("—");
  });
});

describe("number formatting", () => {
  it.each([
    [0, "0"],
    [1000, "1,000"],
    [-1234, "-1,234"],
    [1.5, "2"],
  ] as const)("number %s", (input, output) =>
    expect(formatNumber(input)).toBe(output),
  );
  it.each([
    [0, "0%"],
    [0.03, "3%"],
    [0.12345, "12.35%"],
    [1, "100%"],
  ] as const)("percent %s", (input, output) =>
    expect(formatPercent(input)).toBe(output),
  );
});

describe("UTC-first time", () => {
  it.each([
    ["2026-11-24T23:59:59Z", "2026-11-24"],
    ["2026-11-24T01:00:00+05:30", "2026-11-23"],
    ["2024-02-29T00:00:00Z", "2024-02-29"],
  ] as const)("UTC day %s", (input, output) =>
    expect(utcDay(new Date(input))).toBe(output),
  );
  it("labels UTC and converts offsets", () => {
    expect(formatTime("2026-11-24T01:00:00+05:30")).toContain("23 Nov 2026");
    expect(formatTime("2026-11-24T01:00:00+05:30")).toContain("UTC");
  });
  it("supports local display", () =>
    expect(formatTime("2026-11-24T00:00:00Z", "Local")).not.toBe("—"));
  it("invalid timestamps", () => {
    expect(formatTime("invalid")).toBe("—");
    expect(() => utcDay(new Date("invalid"))).toThrow(RangeError);
  });
});
