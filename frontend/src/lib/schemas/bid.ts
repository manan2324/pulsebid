import { z } from "zod";
import { hasMaxDecimals } from "@/lib/money";
import { simulatorCopy as c } from "@/lib/copy/simulator";
const id = z
  .string()
  .min(1, c.idError)
  .max(128, c.idError)
  .refine((value) => value.trim().length > 0, c.idError);
const dimension = z
  .number()
  .int()
  .positive(c.dimensionError)
  .max(2147483647)
  .optional();
const floor = z
  .union([z.string(), z.number()])
  .refine(
    (value) =>
      hasMaxDecimals(String(value), 6) &&
      Number(value) >= 0 &&
      Number(value) <= 1000,
    c.floorError,
  );
export const bidSchema = z.strictObject({
  id,
  imp: z.strictObject({
    id,
    placement_id: id,
    w: dimension,
    h: dimension,
    bidfloor: floor.optional(),
    bidfloorcur: z.literal("USD").optional(),
  }),
  device: z
    .strictObject({
      type: z.enum(["mobile", "desktop", "tablet", "ctv"]).optional(),
      os: z.string().optional(),
      ua: z.string().optional(),
    })
    .optional(),
  geo: z
    .strictObject({
      country: z
        .string()
        .regex(/^[A-Za-z]{2}$/, c.countryError)
        .optional(),
      region: z.string().optional(),
      city: z.string().optional(),
    })
    .optional(),
  site: z
    .strictObject({ id: z.string().optional(), domain: z.string().optional() })
    .optional(),
  user: z.strictObject({ id: z.string().optional() }).optional(),
  tmax: z
    .number()
    .int()
    .min(10, c.timeoutError)
    .max(1000, c.timeoutError)
    .optional(),
});
export type BidRequest = z.infer<typeof bidSchema>;
export type BidFields = {
  id: string;
  impression: string;
  placement: string;
  width: string;
  height: string;
  floor: string;
  currency: string;
  device: string;
  country: string;
  timeout: string;
  os: string;
  ua: string;
  region: string;
  city: string;
  siteId: string;
  domain: string;
  userId: string;
};
export function requestToFields(request: BidRequest): BidFields {
  return {
    id: request.id,
    impression: request.imp.id,
    placement: request.imp.placement_id,
    width: String(request.imp.w ?? ""),
    height: String(request.imp.h ?? ""),
    floor: String(request.imp.bidfloor ?? ""),
    currency: request.imp.bidfloorcur ?? "USD",
    device: request.device?.type ?? "",
    country: request.geo?.country ?? "",
    timeout: String(request.tmax ?? ""),
    os: request.device?.os ?? "",
    ua: request.device?.ua ?? "",
    region: request.geo?.region ?? "",
    city: request.geo?.city ?? "",
    siteId: request.site?.id ?? "",
    domain: request.site?.domain ?? "",
    userId: request.user?.id ?? "",
  };
}
export function fieldsToRequest(value: BidFields): unknown {
  const number = (text: string) => (text === "" ? undefined : Number(text));
  return {
    id: value.id,
    imp: {
      id: value.impression,
      placement_id: value.placement,
      w: number(value.width),
      h: number(value.height),
      bidfloor: value.floor || undefined,
      bidfloorcur: value.currency,
    },
    device: {
      type: value.device || undefined,
      os: value.os || undefined,
      ua: value.ua || undefined,
    },
    geo: {
      country: value.country || undefined,
      region: value.region || undefined,
      city: value.city || undefined,
    },
    site: { id: value.siteId || undefined, domain: value.domain || undefined },
    user: { id: value.userId || undefined },
    tmax: number(value.timeout),
  };
}
export function parseBidJson(text: string): BidRequest {
  const raw: unknown = JSON.parse(text);
  // Validate numeric source precision before JSON.parse can normalize trailing zeros/exponents.
  for (const match of text.matchAll(
    /"bidfloor"\s*:\s*(-?\d+(?:\.\d+)?(?:[eE][+-]?\d+)?)/g,
  )) {
    if (!hasMaxDecimals(match[1]!, 6))
      throw new Error(`imp.bidfloor: ${c.floorError}`);
  }
  return bidSchema.parse(raw);
}
export function presetRequest(
  preset: string,
  requestId = "sim-request",
): BidRequest {
  const request: BidRequest = {
    id: requestId,
    imp: {
      id: "imp-1",
      placement_id: "home-top-banner",
      w: 300,
      h: 250,
      bidfloor: 0.5,
      bidfloorcur: "USD",
    },
    device: { type: "mobile" },
    geo: { country: "IN" },
    tmax: 100,
  };
  if (preset === "desktop") {
    request.device = { type: "desktop" };
    request.geo = { country: "US" };
  }
  if (preset === "tablet") {
    request.device = { type: "tablet" };
    request.geo = { country: "GB" };
  }
  if (preset === "no-match")
    request.imp.placement_id = "unknown-placement-simulator";
  if (preset === "floor") request.imp.bidfloor = 1000;
  return request;
}
export function randomizeRequest(request: BidRequest): BidRequest {
  const variants = [
    { type: "mobile", country: "IN" },
    { type: "desktop", country: "US" },
    { type: "tablet", country: "GB" },
  ] as const;
  const variant = variants[Math.floor(Math.random() * variants.length)]!;
  return {
    ...request,
    id: `sim-${crypto.randomUUID()}`,
    imp: { ...request.imp, id: `imp-${crypto.randomUUID()}` },
    device: { ...request.device, type: variant.type },
    geo: { ...request.geo, country: variant.country },
  };
}
