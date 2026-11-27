import { z } from "zod";
export const healthSchema = z.object({
  checked_at: z.string(),
  services: z.array(
    z.object({
      name: z.string(),
      status: z.enum(["healthy", "degraded", "unavailable"]),
      latency_ms: z.number(),
      status_code: z.number().nullable(),
    }),
  ),
});
export type Health = z.infer<typeof healthSchema>;
