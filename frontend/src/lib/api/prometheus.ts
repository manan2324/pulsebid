import { z } from "zod";
import { api, queryString } from "./client";
import { reportingCopy as c } from "@/lib/copy/reporting";
export type PromName = "bid_rps" | "fill_rate" | "bid_p99";
const sample = z.tuple([z.number().finite(), z.string()]);
const schema = z.object({
  status: z.literal("success"),
  data: z.object({
    resultType: z.enum(["vector", "matrix"]),
    result: z.array(
      z.object({
        metric: z.record(z.string(), z.string()),
        value: sample.optional(),
        values: z.array(sample).optional(),
      }),
    ),
  }),
});
export type PromResponse = z.infer<typeof schema>;
export const prometheusApi = {
  query: (name: PromName, range?: "15m" | "1h" | "6h", signal?: AbortSignal) =>
    api.get(
      `/api/prom/query${queryString({ name, range })}`,
      signal,
      (value) => {
        const parsed = schema.safeParse(value);
        if (!parsed.success) throw new Error(c.metricsError);
        return parsed.data;
      },
    ),
};
export function promScalar(response: PromResponse | undefined) {
  if (
    response?.data.resultType !== "vector" ||
    response.data.result.length !== 1
  )
    return null;
  const raw = response.data.result[0]?.value?.[1];
  if (raw === undefined || raw.trim() === "") return null;
  const value = Number(raw);
  return Number.isFinite(value) && value >= 0 ? value : null;
}
export function promSeries(response: PromResponse | undefined) {
  if (
    response?.data.resultType !== "matrix" ||
    response.data.result.length !== 1
  )
    return [];
  return (response.data.result[0]?.values ?? []).flatMap(([time, value]) => {
    const number = Number(value);
    return value.trim() !== "" && Number.isFinite(number) && number >= 0
      ? [{ time: time * 1000, ms: number * 1000 }]
      : [];
  });
}
