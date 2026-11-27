import { z } from "zod";
import { nameSchema } from "./advertiser";
import { hasMaxDecimals } from "@/lib/money";
import { campaignCopy as c } from "@/lib/copy/campaign";
export function decimalMoney(max: number) {
  return z
    .string()
    .refine(
      (value) =>
        hasMaxDecimals(value, 6) && Number(value) > 0 && Number(value) <= max,
      c.moneyError,
    );
}
export const campaignFieldsSchema = z.object({
  name: nameSchema,
  advertiser_id: z.uuid(c.advertiserRequired),
  bidder_id: z.string().regex(/^[a-z0-9][a-z0-9-]{1,62}$/, c.bidderRequired),
  daily_budget: decimalMoney(1000000),
  max_bid_cpm: decimalMoney(1000),
  pacing_mode: z.enum(["asap", "even"]),
  start_at: z.string(),
  end_at: z.string(),
});
export const campaignSchema = campaignFieldsSchema.superRefine((value, ctx) => {
  for (const key of ["start_at", "end_at"] as const)
    if (value[key] && !Number.isFinite(new Date(`${value[key]}Z`).getTime()))
      ctx.addIssue({ code: "custom", path: [key], message: c.dateError });
  if (
    value.start_at &&
    value.end_at &&
    new Date(`${value.end_at}Z`) <= new Date(`${value.start_at}Z`)
  )
    ctx.addIssue({ code: "custom", path: ["end_at"], message: c.endError });
});
export type CampaignFormValues = z.infer<typeof campaignSchema>;
export function campaignPayload(values: CampaignFormValues) {
  return {
    ...values,
    start_at: values.start_at
      ? new Date(`${values.start_at}Z`).toISOString()
      : null,
    end_at: values.end_at ? new Date(`${values.end_at}Z`).toISOString() : null,
  };
}
