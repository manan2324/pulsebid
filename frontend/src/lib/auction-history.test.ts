import { it, expect } from "vitest";
import {
  appendHistory,
  auctionHistorySchema,
  type AuctionHistoryEntry,
} from "./auction-history";
import { presetRequest } from "./schemas/bid";
it("retains the newest 50 entries and rejects malformed persisted requests", () => {
  const entries: AuctionHistoryEntry[] = Array.from(
    { length: 60 },
    (_, index) => ({
      id: String(index),
      at: new Date().toISOString(),
      request: presetRequest("mobile"),
      status: 204,
      outcome: "no-fill",
      price: null,
      reason: "no_eligible",
      gatewayMs: 31,
      roundTripMs: 40,
      campaignId: null,
    }),
  );
  const stored = appendHistory([], entries);
  expect(stored).toHaveLength(50);
  expect(stored[0]?.id).toBe("59");
  expect(auctionHistorySchema.safeParse(stored).success).toBe(true);
  expect(
    auctionHistorySchema.safeParse([{ ...entries[0], request: {} }]).success,
  ).toBe(false);
});
