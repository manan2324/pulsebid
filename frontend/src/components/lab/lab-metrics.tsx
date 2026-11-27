"use client";
import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api/client";
import { z } from "zod";
import { labCopy as c } from "@/lib/copy/lab";
import { formatNumber, formatPercent } from "@/lib/format";
const schema = z.object({
  status: z.literal("success"),
  data: z.object({
    result: z.array(
      z.object({
        metric: z.record(z.string(), z.string()),
        value: z.tuple([z.number(), z.string()]),
      }),
    ),
  }),
});
export function LabMetrics() {
  const timeout = useQuery({
    queryKey: ["lab-metrics", "timeout"],
    queryFn: ({ signal }) =>
      api.get("/api/prom/query?name=bidder_timeout_ratio", signal, (value) =>
        schema.parse(value),
      ),
    refetchInterval: 5000,
    retry: false,
    meta: { quiet: true },
  });
  const latency = useQuery({
    queryKey: ["lab-metrics", "latency"],
    queryFn: ({ signal }) =>
      api.get("/api/prom/query?name=bidder_p99", signal, (value) =>
        schema.parse(value),
      ),
    refetchInterval: 5000,
    retry: false,
    meta: { quiet: true },
  });
  return (
    <section className="panel lab-card">
      <h2>{c.metrics}</h2>
      <p className="field-help">{c.metricsHelp}</p>
      {[timeout, latency].map((query, i) => (
        <div key={i}>
          <h3>{i ? c.p99 : c.timeoutRatio}</h3>
          {query.isPending ? (
            <p>{c.loading}</p>
          ) : query.isError || !query.data?.data.result.length ? (
            <p>{c.unavailable}</p>
          ) : (
            <dl className="debug-counters">
              {query.data.data.result.map((row, j) => {
                const value = Number(row.value[1]);
                return (
                  <div key={j}>
                    <dt>{row.metric.bidder_id ?? String(j + 1)}</dt>
                    <dd>
                      {!row.value[1].trim() ||
                      !Number.isFinite(value) ||
                      value < 0
                        ? c.unavailable
                        : i
                          ? `${formatNumber(value * 1000, 1)} ms`
                          : formatPercent(value)}
                    </dd>
                  </div>
                );
              })}
            </dl>
          )}
        </div>
      ))}
    </section>
  );
}
