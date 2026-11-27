import { z } from "zod";
import { nameSchema } from "./advertiser";
import { catalogCopy as c } from "@/lib/copy/catalog";
export const bidderSchema = z.object({
  id: z.string().regex(/^[a-z0-9][a-z0-9-]{1,62}$/, c.idValidation),
  name: nameSchema,
  endpoint_url: z.string().refine((value) => {
    try {
      const url = new URL(value);
      return (
        /^https?:\/\//.test(value) &&
        value === value.trim() &&
        /^https?:$/.test(url.protocol) &&
        Boolean(url.hostname) &&
        !url.username &&
        !url.password
      );
    } catch {
      return false;
    }
  }, c.urlValidation),
  timeout_ms: z
    .string()
    .refine(
      (value) =>
        value === "" ||
        (/^\d+$/.test(value) && Number(value) >= 10 && Number(value) <= 1000),
      c.timeoutValidation,
    ),
  enabled: z.boolean(),
});
export type BidderFormValues = z.infer<typeof bidderSchema>;
export { getBidderIndex } from "@/lib/bidder-index";
