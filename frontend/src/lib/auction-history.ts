import { z } from "zod";
import { historyEntrySchema } from "./simulator";
import { bidSchema } from "./schemas/bid";
import type { AuctionResult } from "./api/gateway";
export const auctionHistorySchema = z
  .array(historyEntrySchema.extend({ request: bidSchema }))
  .max(50);
export type AuctionHistoryEntry = z.infer<typeof auctionHistorySchema>[number];
export const historyKey = "pulsebid.auction-history.v1";
export function historyEntry(result: AuctionResult): AuctionHistoryEntry {
  return {
    id: result.id,
    at: result.at,
    request: result.request,
    status: result.httpStatus,
    outcome: result.kind,
    price: result.response?.price ?? null,
    reason: result.reason,
    gatewayMs: result.gatewayMs,
    roundTripMs: result.roundTripMs,
    campaignId: result.response?.campaign_id ?? null,
    testTime: result.testTime,
  };
}
export function appendHistory(
  previous: readonly AuctionHistoryEntry[],
  entries: readonly AuctionHistoryEntry[],
) {
  return [...entries].reverse().concat(previous).slice(0, 50);
}
