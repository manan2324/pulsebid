import { z } from "zod";
import type { components } from "./generated/bid-gateway";
import { ApiClientError } from "./client";
import type { BidRequest } from "@/lib/schemas/bid";
import {
  parseDebug,
  parseServerTiming,
  eventToken,
  safeLanding,
} from "@/lib/simulator";
import { simulatorCopy as c } from "@/lib/copy/simulator";
export type BidResponse = components["schemas"]["BidResponse"];
const responseSchema: z.ZodType<BidResponse> = z.object({
  request_id: z.string(),
  bid_id: z.string(),
  campaign_id: z.string(),
  creative_id: z.string(),
  advertiser_id: z.string(),
  bidder_id: z.string(),
  price: z.number().nonnegative(),
  currency: z.string(),
  adm: z.string(),
  impression_url: z.string().url(),
  click_url: z.string().url(),
  w: z.number().nonnegative(),
  h: z.number().nonnegative(),
});
const errorSchema = z.object({
  error: z.object({
    code: z.string(),
    message: z.string(),
    request_id: z.string().optional(),
    details: z
      .array(z.object({ field: z.string(), issue: z.string() }))
      .optional(),
  }),
});
function apiError(body: unknown, headers: Headers, status: number) {
  const parsed = errorSchema.safeParse(body);
  return parsed.success
    ? new ApiClientError(
        parsed.data.error.code,
        parsed.data.error.message,
        parsed.data.error.details ?? [],
        parsed.data.error.request_id ??
          headers.get("x-request-id") ??
          "unknown",
        status,
      )
    : new ApiClientError(
        "internal",
        c.protocolError,
        [],
        headers.get("x-request-id") ?? "unknown",
        status,
      );
}
export interface AuctionResult {
  id: string;
  at: string;
  request: BidRequest;
  testTime?: string;
  httpStatus: number;
  kind: "filled" | "no-fill" | "error";
  response: BidResponse | null;
  error: ApiClientError | null;
  reason: string | null;
  gatewayMs: number | null;
  roundTripMs: number;
  requestId: string;
  debug: Record<string, number> | null;
}
export async function sendBid(
  request: BidRequest,
  options: { signal?: AbortSignal; testTime?: string } = {},
): Promise<AuctionResult> {
  const start = performance.now();
  const base = {
    id: crypto.randomUUID(),
    at: new Date().toISOString(),
    request,
    testTime: options.testTime,
  };
  try {
    const headers = new Headers({
      Accept: "application/json",
      "Content-Type": "application/json",
    });
    if (options.testTime) headers.set("X-PulseBid-Test-Time", options.testTime);
    const response = await fetch("/api/gateway/bid", {
      method: "POST",
      headers,
      body: JSON.stringify(request),
      credentials: "same-origin",
      cache: "no-store",
      signal: options.signal,
    });
    const body: unknown =
      response.status === 204 ? null : await response.json().catch(() => null);
    const shared = {
      ...base,
      httpStatus: response.status,
      roundTripMs: performance.now() - start,
      gatewayMs: parseServerTiming(
        response.headers.get("x-upstream-server-timing"),
      ),
      requestId: response.headers.get("x-request-id") ?? "unknown",
      debug: parseDebug(response.headers.get("x-upstream-x-pulsebid-debug")),
      reason: response.headers.get("x-upstream-x-pulsebid-reason"),
    };
    if (response.status === 204)
      return { ...shared, kind: "no-fill", response: null, error: null };
    const parsed = responseSchema.safeParse(body);
    if (response.status === 200 && parsed.success)
      return { ...shared, kind: "filled", response: parsed.data, error: null };
    return {
      ...shared,
      kind: "error",
      response: null,
      error: apiError(body, response.headers, response.status),
    };
  } catch (error) {
    if (
      options.signal?.aborted ||
      (error instanceof Error && error.name === "AbortError")
    )
      throw error;
    return {
      ...base,
      httpStatus: 0,
      kind: "error",
      response: null,
      error: new ApiClientError(
        "unavailable",
        c.networkError,
        [],
        "unknown",
        0,
      ),
      reason: null,
      gatewayMs: null,
      roundTripMs: performance.now() - start,
      requestId: "unknown",
      debug: null,
    };
  }
}
export async function fireEvent(
  kind: "impression" | "click",
  url: string,
  options: { testTime?: string } = {},
) {
  const token = eventToken(url);
  const headers = new Headers();
  if (options.testTime) headers.set("X-PulseBid-Test-Time", options.testTime);
  const response = await fetch(
    `/api/gateway/${kind}?t=${encodeURIComponent(token)}`,
    { headers, credentials: "same-origin", cache: "no-store" },
  );
  if (!response.ok)
    throw apiError(
      await response.json().catch(() => null),
      response.headers,
      response.status,
    );
  if (kind === "impression") {
    await response.arrayBuffer();
    return { location: null };
  }
  const data: unknown = await response.json();
  const parsed = z
    .object({
      status: z.number().int().min(300).max(399),
      location: z.unknown(),
    })
    .safeParse(data);
  const location = parsed.success ? safeLanding(parsed.data.location) : null;
  if (!location) throw new Error(c.badLanding);
  return { location };
}
