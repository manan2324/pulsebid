import { z } from "zod";
import { catalogCopy as c } from "@/lib/copy/catalog";
export const nameSchema = z
  .string()
  .refine(
    (value) => value.trim().length > 0 && Array.from(value).length <= 200,
    c.nameValidation,
  );
export const advertiserSchema = z.object({
  name: nameSchema,
  contact_email: z.union([z.literal(""), z.email(c.emailValidation)]),
});
export type AdvertiserFormValues = z.infer<typeof advertiserSchema>;
