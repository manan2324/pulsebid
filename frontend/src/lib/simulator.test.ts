import { describe, it, expect } from "vitest";
import {
  buildCurl,
  eventToken,
  parseDebug,
  parseServerTiming,
  reasonText,
  safeLanding,
} from "./simulator";
import { presetRequest } from "./schemas/bid";
import { simulatorCopy as c } from "./copy/simulator";
describe("gateway diagnostics", () => {
  it.each([
    ["total;dur=31.4", 31.4],
    ['cache;dur=2, total;desc="includes, proxy; time";dur=12.5', 12.5],
    ["total ; dur = .5", 0.5],
    ["total;dur=-2", null],
    ["db;dur=3", null],
    ["total;dur=Infinity", null],
    [null, null],
  ])("parses %s", (value, expected) =>
    expect(parseServerTiming(value)).toBe(expected),
  );
  it("maps every backend reason and handles unknown/inherited keys", () => {
    for (const reason of [
      "no_eligible",
      "no_bids",
      "below_floor",
      "budget",
      "error",
    ] as const)
      expect(reasonText(reason)).toBe(c.reasons[reason]);
    expect(reasonText(null)).toBe(c.unknownReason);
    expect(reasonText("constructor")).toBe(c.unknownReason);
  });
  it("only exposes numeric debug counters when supplied", () => {
    expect(parseDebug(null)).toBeNull();
    expect(parseDebug("eligible=3;responded=2;timed_out=1;errors=bad")).toEqual(
      { eligible: 3, responded: 2, timed_out: 1 },
    );
  });
});
it("cURL targets the public gateway, preserves JSON, safely quotes shell metacharacters, and has no key", () => {
  const request = presetRequest("mobile");
  request.id = "operator's $(echo unsafe)";
  const command = buildCurl(
    "https://gateway.example.test/internal",
    request,
    "2026-11-25T12:00:00Z",
  );
  expect(command).toContain("'https://gateway.example.test/v1/bid'");
  expect(command).toContain(`operator'"'"'s $(echo unsafe)`);
  expect(command).toContain("X-PulseBid-Test-Time: 2026-11-25T12:00:00Z");
  expect(command).not.toContain("API-Key");
  expect(command).not.toContain("/api/gateway");
  expect(() => buildCurl("https://user:pass@example.test", request)).toThrow();
});
it("extracts only the token and permits only deliberate HTTP(S) landing navigation", () => {
  expect(eventToken("https://upstream.test/v1/events/impression?t=a%2Bb")).toBe(
    "a+b",
  );
  expect(() => eventToken("javascript:alert(1)")).toThrow();
  expect(() => eventToken("https://upstream.test/?t=a&t=b")).toThrow();
  expect(safeLanding("javascript:alert(1)")).toBeNull();
  expect(safeLanding("https://user:password@example.test")).toBeNull();
  expect(safeLanding("https://example.test/landing")).toBe(
    "https://example.test/landing",
  );
});
