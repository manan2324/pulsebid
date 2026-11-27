import { z } from "zod";
import { simulatorCopy as c } from "./copy/simulator";
export function parseServerTiming(header: string | null): number | null {
  if (!header) return null;
  // Quoted descriptions can contain commas and semicolons.
  const metrics = header.match(/(?:[^,"\r\n]|"(?:\\.|[^"\\])*")+/g) ?? [];
  for (const metric of metrics) {
    const parts = metric.match(/(?:[^;"\r\n]|"(?:\\.|[^"\\])*")+/g) ?? [];
    if (parts.shift()?.trim().toLowerCase() !== "total") continue;
    for (const part of parts) {
      const match = /^\s*dur\s*=\s*(\d+(?:\.\d+)?|\.\d+)\s*$/i.exec(part);
      if (match && Number.isFinite(Number(match[1]))) return Number(match[1]);
    }
  }
  return null;
}
export const noFillReasons = [
  "no_eligible",
  "no_bids",
  "below_floor",
  "budget",
  "error",
] as const;
export function reasonText(reason: string | null) {
  return reason !== null && Object.hasOwn(c.reasons, reason)
    ? c.reasons[reason as keyof typeof c.reasons]
    : c.unknownReason;
}
export function parseDebug(
  header: string | null,
): Record<string, number> | null {
  if (header === null) return null;
  const counters: Record<string, number> = {};
  for (const part of header.split(";")) {
    const match = /^([a-z_]+)=(\d+)$/.exec(part.trim());
    if (match && Number.isSafeInteger(Number(match[2])))
      counters[match[1]!] = Number(match[2]);
  }
  return counters;
}
export function eventToken(url: string): string {
  const parsed = new URL(url);
  if (!/^https?:$/.test(parsed.protocol)) throw new Error(c.tokenError);
  const token = parsed.searchParams.get("t");
  if (
    !token ||
    parsed.searchParams.getAll("t").length !== 1 ||
    token.length > 16384
  )
    throw new Error(c.tokenError);
  return token;
}
export function safeLanding(value: unknown): string | null {
  if (typeof value !== "string") return null;
  try {
    const url = new URL(value);
    return /^https?:$/.test(url.protocol) && !url.username && !url.password
      ? url.href
      : null;
  } catch {
    return null;
  }
}
const quote = (value: string) => `'${value.replace(/'/g, `'"'"'`)}'`;
export function buildCurl(
  gatewayUrl: string,
  request: unknown,
  testTime?: string,
) {
  const url = new URL("/v1/bid", gatewayUrl);
  if (!/^https?:$/.test(url.protocol) || url.username || url.password)
    throw new Error(c.gatewayUrlError);
  return `curl --request POST ${quote(url.href)} --header 'Content-Type: application/json'${testTime ? ` --header ${quote(`X-PulseBid-Test-Time: ${testTime}`)}` : ""} --data-raw ${quote(JSON.stringify(request))}`;
}
export const historyEntrySchema = z.object({
  id: z.string(),
  at: z.string().datetime(),
  request: z.unknown(),
  status: z.number().int(),
  outcome: z.enum(["filled", "no-fill", "error"]),
  price: z.number().nullable(),
  reason: z.string().nullable(),
  gatewayMs: z.number().nonnegative().nullable(),
  roundTripMs: z.number().nonnegative(),
  campaignId: z.string().nullable(),
  testTime: z.string().optional(),
});
