import { z } from "zod";
import { countryCodes, timezones } from "@/lib/countries";
import { campaignCopy as c } from "@/lib/copy/campaign";
export const deviceTypes = ["mobile", "desktop", "tablet", "ctv"] as const;
export const targetingSchema = z.object({
  countries: z.array(
    z.string().refine((value) => countryCodes.includes(value), c.countryError),
  ),
  device_types: z.array(z.enum(deviceTypes)),
  placement_ids: z.array(
    z.string().refine((value) => value.trim().length > 0, c.placementError),
  ),
  hours_of_day: z
    .array(z.number().int().min(0).max(23))
    .refine((value) => new Set(value).size === value.length, c.hoursError),
  timezone: z
    .string()
    .refine((value) => timezones.includes(value), c.timezoneError),
});
export type TargetingValues = z.infer<typeof targetingSchema>;
export const emptyTargeting: TargetingValues = {
  countries: [],
  device_types: [],
  placement_ids: [],
  hours_of_day: [],
  timezone: "UTC",
};
export function placementTags(value: string) {
  return [
    ...new Set(
      value
        .split(/[\n,]/)
        .map((item) => item.trim())
        .filter(Boolean),
    ),
  ];
}
