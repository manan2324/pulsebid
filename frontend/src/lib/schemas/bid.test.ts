import { it, expect } from "vitest";
import {
  bidSchema,
  fieldsToRequest,
  requestToFields,
  presetRequest,
  parseBidJson,
} from "./bid";
it("roundtrips complete optional context without discarding JSON fields", () => {
  const request = {
    ...presetRequest("mobile"),
    device: { type: "mobile" as const, os: "android", ua: "test" },
    geo: { country: "IN", region: "GJ", city: "Ahmedabad" },
    site: { id: "site-1", domain: "example.test" },
    user: { id: "u-1" },
  };
  expect(bidSchema.parse(fieldsToRequest(requestToFields(request)))).toEqual({
    ...request,
    imp: { ...request.imp, bidfloor: "0.5" },
  });
});
it("all presets validate and preserve an explicit request ID", () => {
  for (const preset of ["mobile", "desktop", "tablet", "no-match", "floor"])
    expect(bidSchema.parse(presetRequest(preset, "req-fixed")).id).toBe(
      "req-fixed",
    );
});
it.each(["0.0000001", "1.0000000", "1e-7", "-1", "1000.000001"])(
  "rejects floor %s in form and numeric JSON source",
  (floor) => {
    const request = presetRequest("mobile");
    request.imp.bidfloor = floor;
    expect(bidSchema.safeParse(request).success).toBe(false);
    expect(() =>
      parseBidJson(
        JSON.stringify(presetRequest("mobile")).replace(
          '"bidfloor":0.5',
          `"bidfloor":${floor}`,
        ),
      ),
    ).toThrow();
  },
);
it("validates missing fields, timeout, dimensions, country and unsupported JSON keys", () => {
  const value = presetRequest("mobile");
  expect(bidSchema.safeParse({ ...value, id: " " }).success).toBe(false);
  expect(bidSchema.safeParse({ ...value, tmax: 1001 }).success).toBe(false);
  expect(
    bidSchema.safeParse({ ...value, geo: { country: "IND" } }).success,
  ).toBe(false);
  expect(
    bidSchema.safeParse({ ...value, imp: { ...value.imp, w: 0 } }).success,
  ).toBe(false);
  expect(() =>
    parseBidJson(JSON.stringify({ ...value, extra: true })),
  ).toThrow();
  expect(() => parseBidJson("{")).toThrow();
});
