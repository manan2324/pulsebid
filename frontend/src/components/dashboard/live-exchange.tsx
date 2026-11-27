"use client";
import dynamic from "next/dynamic";
import { usePrometheus } from "@/hooks/use-prometheus";
import { promScalar, promSeries } from "@/lib/api/prometheus";
import { formatNumber, formatPercent } from "@/lib/format";
import { useTimeMode } from "@/components/providers";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { ApiClientError } from "@/lib/api/client";
import { copy } from "@/lib/copy/app";
import { StatCard, CopyButton } from "@/components/data/shared";
import { formatTime } from "@/lib/time";
import { reportingCopy as c } from "@/lib/copy/reporting";
const Chart = dynamic(
  () =>
    import("@/components/charts/prom-latency").then(
      (module) => module.PromLatency,
    ),
  { ssr: false, loading: () => <Skeleton className="h-64 w-full" /> },
);
export function LiveExchange() {
  const rps = usePrometheus("bid_rps"),
    fill = usePrometheus("fill_rate"),
    p99 = usePrometheus("bid_p99"),
    series = usePrometheus("bid_p99", "15m");
  const { mode } = useTimeMode();
  const values = [
    {
      query: rps,
      title: c.rps,
      format: (value: number) => formatNumber(value, 2),
    },
    {
      query: fill,
      title: c.fill,
      format: (value: number) => formatPercent(value),
    },
    {
      query: p99,
      title: c.p99,
      format: (value: number) => `${formatNumber(value * 1000, 1)} ms`,
    },
  ];
  const points = series.isError ? [] : promSeries(series.data);
  const failure = [rps, fill, p99, series].find(
    (query) => query.isError,
  )?.error;
  return (
    <section className="panel dashboard-live" aria-label={c.live}>
      <div className="section-heading">
        <h2>{c.live}</h2>
        <Button
          variant="ghost"
          size="sm"
          onClick={() => {
            for (const query of [rps, fill, p99, series]) void query.refetch();
          }}
        >
          {c.refresh}
        </Button>
      </div>
      <p className="field-help">{c.liveHelp}</p>
      <div className="metric-grid">
        {values.map(({ query, title, format }) => {
          const value = query.isError ? null : promScalar(query.data);
          return query.isPending ? (
            <Skeleton key={title} className="h-24 w-full" />
          ) : value !== null ? (
            <StatCard
              key={title}
              title={title}
              value={format(value)}
              description={c.live}
            />
          ) : null;
        })}
      </div>
      {values.every(
        ({ query }) =>
          !query.isPending &&
          (query.isError || promScalar(query.data) === null),
      ) && (
        <p role="status" className="notice">
          {c.liveUnavailable}
          {failure instanceof ApiClientError && (
            <span>
              {" "}
              {copy.requestId}: <code>{failure.requestId}</code>
              <CopyButton value={failure.requestId} />
            </span>
          )}
        </p>
      )}
      {series.isPending ? (
        <Skeleton className="h-64 w-full" />
      ) : points.length ? (
        <>
          <p className="field-help">
            {c.latencyChart} · {mode} · {c.latencyBudget}
          </p>
          <Chart data={points} mode={mode} />
          <details>
            <summary>{c.tableToggle}</summary>
            <p className="field-help">
              {c.chartSamples}: {points.length}. {c.latest}:{" "}
              {formatNumber(points.at(-1)!.ms, 1)} ms.
            </p>
            <div className="table-scroll">
              <table className="simulator-simple-table">
                <thead>
                  <tr>
                    <th scope="col">
                      {c.bucket} ({mode})
                    </th>
                    <th scope="col">{c.p99}</th>
                  </tr>
                </thead>
                <tbody>
                  {points.slice(-100).map((point) => (
                    <tr key={point.time}>
                      <td>{formatTime(new Date(point.time), mode)}</td>
                      <td>{formatNumber(point.ms, 1)} ms</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </details>
        </>
      ) : null}
    </section>
  );
}
