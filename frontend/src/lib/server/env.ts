import "server-only";
import { z } from "zod";

const httpUrl = z
  .string()
  .url()
  .refine((value) => /^https?:\/\//.test(value), "HTTP(S) URL required");
export const envSchema = z
  .object({
    NODE_ENV: z
      .enum(["development", "test", "production"])
      .default("development"),
    APP_ORIGIN: httpUrl.default("http://localhost:3001"),
    CAMPAIGN_API_URL: httpUrl.default("http://localhost:8081"),
    REPORTING_API_URL: httpUrl.default("http://localhost:8083"),
    GATEWAY_URL: httpUrl.default("http://localhost:8080"),
    GATEWAY_PUBLIC_URL: httpUrl.default("http://localhost:8080"),
    GATEWAY_ADMIN_URL: httpUrl.default("http://localhost:8090"),
    CAMPAIGN_ADMIN_URL: httpUrl.default("http://localhost:8091"),
    AGGREGATOR_ADMIN_URL: httpUrl.default("http://localhost:8093"),
    PROMETHEUS_URL: httpUrl.default("http://localhost:9090"),
    BIDDER_HOST: z
      .string()
      .regex(/^[a-zA-Z0-9.-]+$/)
      .default("localhost"),
    BIDDER_BASE_PORT: z.coerce.number().int().min(1).max(65535).default(9101),
    BIDDER_COUNT: z.coerce.number().int().min(1).max(100).default(8),
    PULSEBID_API_KEY: z.string().min(1),
    ADMIN_PASSWORD: z.string().min(1),
    SESSION_SECRET: z.string().min(32),
    AUTH_DISABLED: z.enum(["true", "false"]).default("false"),
    TEST_MODE_UI: z.enum(["true", "false"]).default("false"),
  })
  .superRefine((env, ctx) => {
    if (env.NODE_ENV === "production" && env.AUTH_DISABLED === "true")
      ctx.addIssue({
        code: "custom",
        path: ["AUTH_DISABLED"],
        message: "AUTH_DISABLED is forbidden in production",
      });
    if (env.BIDDER_BASE_PORT + env.BIDDER_COUNT - 1 > 65535)
      ctx.addIssue({
        code: "custom",
        path: ["BIDDER_COUNT"],
        message: "Bidder ports exceed 65535",
      });
  });
export function getEnv() {
  return envSchema.parse(process.env);
}
