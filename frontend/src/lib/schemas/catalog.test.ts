import { describe, expect, it } from "vitest";
import { advertiserSchema } from "./advertiser";
import { bidderSchema, getBidderIndex } from "./bidder";
const advertiser = { name: "Acme", contact_email: "" };
const bidder = {
  id: "ab",
  name: "Test DSP",
  endpoint_url: "http://localhost:9101/bid",
  timeout_ms: "",
  enabled: true,
};
describe("catalog validation parity", () => {
  it.each(["a", " ", "A".repeat(201)])("validates advertiser name %s", (name) =>
    expect(advertiserSchema.safeParse({ ...advertiser, name }).success).toBe(
      name === "a",
    ),
  );
  it("accepts 200 Unicode code points", () =>
    expect(
      advertiserSchema.safeParse({ ...advertiser, name: "😀".repeat(200) })
        .success,
    ).toBe(true));
  it.each(["", "ads@example.com"])(
    "accepts optional email %s",
    (contact_email) =>
      expect(
        advertiserSchema.safeParse({ ...advertiser, contact_email }).success,
      ).toBe(true),
  );
  it("rejects invalid email", () =>
    expect(
      advertiserSchema.safeParse({ ...advertiser, contact_email: "bad" })
        .success,
    ).toBe(false));
  it.each(["ab", "a-b", "a".repeat(63)])("accepts bidder slug %s", (id) =>
    expect(bidderSchema.safeParse({ ...bidder, id }).success).toBe(true),
  );
  it.each(["a", "-ab", "Ab", "a_b", "a".repeat(64)])(
    "rejects bidder slug %s",
    (id) =>
      expect(bidderSchema.safeParse({ ...bidder, id }).success).toBe(false),
  );
  it.each(["", "10", "1000"])("accepts timeout %s", (timeout_ms) =>
    expect(bidderSchema.safeParse({ ...bidder, timeout_ms }).success).toBe(
      true,
    ),
  );
  it.each(["9", "1001", "10.5", "-10", "1e2"])(
    "rejects timeout %s",
    (timeout_ms) =>
      expect(bidderSchema.safeParse({ ...bidder, timeout_ms }).success).toBe(
        false,
      ),
  );
  it.each([
    "ftp://localhost/bid",
    "javascript:alert(1)",
    "http:localhost",
    "http://user:password@localhost:9101/bid",
    " http://localhost:9101/bid",
  ])("rejects endpoint %s", (endpoint_url) =>
    expect(bidderSchema.safeParse({ ...bidder, endpoint_url }).success).toBe(
      false,
    ),
  );
  it.each([
    ["http://localhost:9101/bid", 0],
    ["http://mock-bidders:9108/bid", 7],
    ["http://localhost:9100/bid", null],
    ["http://localhost:9109/bid", null],
    ["https://external.test/bid", null],
    ["invalid", null],
  ] as const)("maps health port %s", (endpoint, index) =>
    expect(getBidderIndex(endpoint, { basePort: 9101, count: 8 })).toBe(index),
  );
});
