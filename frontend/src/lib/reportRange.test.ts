import { it, expect } from "vitest";
import {
  utcPreset,
  validateReportRange,
  validTimestamp,
  resolveReportRange,
  fromUtcInput,
  utcInput,
  last24Hours,
  dayMs,
} from "./reportRange";
import { reportCampaignId } from "@/mocks/fixtures/reporting";
it("uses UTC calendar boundaries across leap days, offsets and year rollover", () => {
  expect(utcPreset("today", new Date("2026-11-25T02:30:00+05:30"))).toEqual({
    from: "2026-11-24T00:00:00.000Z",
    to: "2026-11-25T00:00:00.000Z",
  });
  expect(utcPreset("7d", new Date("2024-03-01T23:00:00Z"))).toEqual({
    from: "2024-02-24T00:00:00.000Z",
    to: "2024-03-02T00:00:00.000Z",
  });
  expect(utcPreset("yesterday", new Date("2026-01-01T00:00:00Z"))).toEqual({
    from: "2025-12-31T00:00:00.000Z",
    to: "2026-01-01T00:00:00.000Z",
  });
  expect(last24Hours(new Date("2026-11-25T10:34:00Z"))).toEqual({
    from: "2026-11-24T11:00:00.000Z",
    to: "2026-11-25T11:00:00.000Z",
  });
});
it.each([
  ["hour", 31, true],
  ["hour", 31 + 1 / dayMs, false],
  ["day", 366, true],
  ["total", 366, true],
  ["total", 367, false],
])("enforces %s %s-day range", (granularity, days, valid) => {
  const from = "2026-01-01T00:00:00Z",
    to = new Date(Date.parse(from) + Number(days) * dayMs).toISOString();
  expect(
    validateReportRange({ from, to, granularity: String(granularity) }) ===
      null,
  ).toBe(valid);
});
it.each([
  "2026-02-30T00:00:00Z",
  "1900-02-29T00:00:00Z",
  "2026-13-01T00:00:00Z",
  "2026-01-01T24:00:00Z",
  "2026-01-01T00:00:00",
  "invalid",
])("rejects invalid calendar timestamp %s", (value) =>
  expect(validTimestamp(value)).toBe(false),
);
it("validates order, count, UUIDs and supported granularity", () => {
  const value = {
    from: "2026-11-25T00:00:00Z",
    to: "2026-11-26T00:00:00Z",
    granularity: "hour",
    campaignIds: [reportCampaignId],
  };
  expect(validateReportRange(value)).toBeNull();
  expect(validateReportRange({ ...value, to: value.from })).not.toBeNull();
  expect(
    validateReportRange({
      ...value,
      campaignIds: Array(101).fill(reportCampaignId),
    }),
  ).not.toBeNull();
  expect(
    validateReportRange({ ...value, campaignIds: ["invalid"] }),
  ).not.toBeNull();
  expect(
    validateReportRange({ ...value, granularity: "minute" }),
  ).not.toBeNull();
  expect(validTimestamp("2000-02-29T10:20:30.123Z")).toBe(true);
});
it("roundtrips precise UTC inputs and keeps invalid custom URL filters invalid", () => {
  const value = "2026-11-25T10:20:30.123Z";
  expect(fromUtcInput(utcInput(value))).toBe(value);
  expect(
    resolveReportRange(
      {
        preset: "custom",
        from: "bad",
        to: value,
        granularity: "hour",
        campaignIds: [],
      },
      new Date(),
    ).range,
  ).toBeNull();
  expect(
    resolveReportRange(
      {
        preset: "invalid",
        from: "",
        to: "",
        granularity: "hour",
        campaignIds: [],
      },
      new Date(),
    ).range,
  ).toBeNull();
});

it("rejects a one-nanosecond excess at the backend duration limit", () => {
  expect(
    validateReportRange({
      from: "2026-01-01T00:00:00Z",
      to: "2026-02-01T00:00:00.000000001Z",
      granularity: "hour",
    }),
  ).not.toBeNull();
});
