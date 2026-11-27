import { z } from "zod";
import { nameSchema } from "./advertiser";
import { campaignCopy as c } from "@/lib/copy/campaign";
export const httpUrlSchema = z.string().refine((value) => {
  try {
    const url = new URL(value);
    return (
      /^https?:\/\//.test(value) &&
      value === value.trim() &&
      Boolean(url.hostname) &&
      !url.username &&
      !url.password
    );
  } catch {
    return false;
  }
}, c.urlError);
const size = z
  .string()
  .refine(
    (value) =>
      value === "" ||
      (/^\d+$/.test(value) &&
        Number.isSafeInteger(Number(value)) &&
        Number(value) > 0 &&
        Number(value) <= 2147483647),
    c.sizeError,
  );
export const creativeSchema = z.object({
  name: nameSchema,
  format: z.enum(["banner", "native", "video"]),
  width: size,
  height: size,
  markup: z
    .string()
    .refine(
      (value) =>
        new TextEncoder().encode(value).length > 0 &&
        new TextEncoder().encode(value).length <= 65536,
      c.markupError,
    ),
  landing_url: httpUrlSchema,
  status: z.enum(["active", "paused"]),
});
export type CreativeValues = z.infer<typeof creativeSchema>;
export const starterCreative: CreativeValues = {
  name: "Banner 300x250",
  format: "banner",
  width: "300",
  height: "250",
  markup:
    '<a href="{click_url}" style="display:block;padding:32px;background:#eeeaff;color:#343467;font:20px sans-serif">Your campaign starts here</a><img src="{impression_url}" width="1" height="1" alt="">',
  landing_url: "https://example.com",
  status: "active",
};
export function creativePayload(value: CreativeValues) {
  return {
    ...value,
    width: value.width ? Number(value.width) : null,
    height: value.height ? Number(value.height) : null,
  };
}
