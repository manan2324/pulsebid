import { expect, it } from "vitest";
import { campaignSchema, campaignPayload } from "./campaign";
import { creativeSchema, starterCreative } from "./creative";
import { targetingSchema, emptyTargeting } from "./targeting";
import { countryCodes } from "@/lib/countries";
const valid = {
  name: "Test",
  advertiser_id: "019aab00-0000-7000-8000-000000000001",
  bidder_id: "bidder-1",
  daily_budget: "0.000001",
  max_bid_cpm: "1.000001",
  pacing_mode: "even" as const,
  start_at: "",
  end_at: "",
};
it.each(["0", "-1", "1.1234567", "1e-3", "1000001"])(
  "rejects invalid budget %s",
  (daily_budget) =>
    expect(campaignSchema.safeParse({ ...valid, daily_budget }).success).toBe(
      false,
    ),
);
it("preserves exact decimal strings and emits UTC timestamps", () => {
  expect(campaignSchema.safeParse(valid).success).toBe(true);
  const payload = campaignPayload({ ...valid, start_at: "2026-11-24T12:00" });
  expect(payload.daily_budget).toBe("0.000001");
  expect(payload.start_at).toBe("2026-11-24T12:00:00.000Z");
  expect(payload.end_at).toBeNull();
});
it("rejects reversed schedules and excessive CPM", () => {
  expect(
    campaignSchema.safeParse({
      ...valid,
      start_at: "2026-11-24T12:00",
      end_at: "2026-11-24T11:00",
    }).success,
  ).toBe(false);
  expect(
    campaignSchema.safeParse({ ...valid, max_bid_cpm: "1001" }).success,
  ).toBe(false);
});
it("validates countries, unique hours, device and timezone selections", () => {
  expect(new Set(countryCodes).size).toBe(249);
  expect(targetingSchema.safeParse(emptyTargeting).success).toBe(true);
  for (const patch of [
    { countries: ["XX"] },
    { countries: ["in"] },
    { hours_of_day: [1, 1] },
    { hours_of_day: [24] },
    { timezone: "Local" },
    { device_types: ["watch"] },
  ])
    expect(
      targetingSchema.safeParse({ ...emptyTargeting, ...patch }).success,
    ).toBe(false);
});
it("enforces creative UTF-8 byte and size/URL rules", () => {
  expect(creativeSchema.safeParse(starterCreative).success).toBe(true);
  expect(
    creativeSchema.safeParse({ ...starterCreative, markup: "😀".repeat(16385) })
      .success,
  ).toBe(false);
  expect(
    creativeSchema.safeParse({ ...starterCreative, width: "0" }).success,
  ).toBe(false);
  expect(
    creativeSchema.safeParse({
      ...starterCreative,
      landing_url: "javascript:alert(1)",
    }).success,
  ).toBe(false);
});
