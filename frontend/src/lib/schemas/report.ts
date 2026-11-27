import { z } from "zod";
import type { components } from "@/lib/api/generated/reporting";
import { reportingCopy as c } from "@/lib/copy/reporting";
const count = z.number().int().nonnegative().max(Number.MAX_SAFE_INTEGER);
const finite = z.number().finite().nonnegative();
const totals = {
  impressions: count,
  clicks: count,
  ctr: finite,
  spend: finite,
  bids: count,
  wins: count,
  win_rate: finite,
};
export const reportSchema: z.ZodType<components["schemas"]["ReportResponse"]> =
  z.object({
    from: z.iso.datetime({ offset: true }),
    to: z.iso.datetime({ offset: true }),
    granularity: z.enum(["hour", "day", "total"]),
    data: z
      .array(
        z.object({
          ...totals,
          campaign_id: z.string(),
          bucket: z.iso.datetime({ offset: true }).optional(),
          currency: z.literal("USD"),
        }),
      )
      .max(50000),
    totals: z.object(totals),
  });
export function parseReport(value: unknown) {
  const parsed = reportSchema.safeParse(value);
  if (!parsed.success) throw new Error(c.dataError);
  return parsed.data;
}
