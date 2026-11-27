"use client";
import dynamic from "next/dynamic";
import { Skeleton } from "@/components/ui/skeleton";
import { StatCard } from "@/components/data/shared";
import type { BurstResult } from "@/lib/burst";
import { formatNumber, formatPercent } from "@/lib/format";
import { reasonText } from "@/lib/simulator";
import { simulatorCopy as c } from "@/lib/copy/simulator";
const Histogram = dynamic(
  () =>
    import("@/components/charts/latency-histogram").then(
      (module) => module.LatencyHistogram,
    ),
  { loading: () => <Skeleton className="h-64 w-full" /> },
);
export function BurstSummary({ summary }: { summary: BurstResult }) {
  const ms = (value: number | null) =>
    value === null ? c.unavailable : `${formatNumber(value, 1)} ms`;
  return (
    <section className="panel simulator-summary">
      <h2>{c.burstSummary}</h2>
      <p role="status">
        {summary.cancelled ? c.partial : c.completed}: {summary.completed}/
        {summary.requested} · {c.started}: {summary.started}
      </p>
      <div className="metric-grid">
        <StatCard
          title={c.filled}
          value={summary.counts.filled}
          description={c.outcomes}
        />
        <StatCard
          title={c.noFill}
          value={summary.counts["no-fill"]}
          description={c.outcomes}
        />
        <StatCard
          title={c.error}
          value={summary.counts.error}
          description={c.outcomes}
        />
        <StatCard
          title={c.fillRate}
          value={formatPercent(summary.fillRate)}
          description={c.completed}
        />
        <StatCard
          title={c.p50}
          value={ms(summary.p50)}
          description={c.gateway}
        />
        <StatCard
          title={c.p95}
          value={ms(summary.p95)}
          description={c.gateway}
        />
        <StatCard
          title={c.p99}
          value={ms(summary.p99)}
          description={c.gateway}
        />
      </div>
      <p
        className={
          summary.p99 === null
            ? "notice"
            : summary.p99 < 120
              ? "latency-good"
              : "latency-bad"
        }
      >
        {summary.p99 === null
          ? c.noTiming
          : summary.p99 < 120
            ? c.pass
            : c.fail}
      </p>
      <p className="field-help">
        {c.timingCoverage}: {summary.sampleCount}/{summary.completed}
      </p>
      {Object.keys(summary.reasons).length > 0 && (
        <dl className="debug-counters">
          {Object.entries(summary.reasons).map(([reason, count]) => (
            <div key={reason}>
              <dt>{reasonText(reason)}</dt>
              <dd>{count}</dd>
            </div>
          ))}
        </dl>
      )}
      {summary.completed === 0 ? (
        <p>{c.noSamples}</p>
      ) : (
        <>
          <h3>{c.histogram}</h3>
          <Histogram data={summary.histogram} />
          <details>
            <summary>{c.histogramTable}</summary>
            <table className="simulator-simple-table">
              <thead>
                <tr>
                  <th scope="col">{c.latencyBucket}</th>
                  <th scope="col">{c.count}</th>
                </tr>
              </thead>
              <tbody>
                {summary.histogram.map((row) => (
                  <tr key={row.label}>
                    <th scope="row">{row.label}</th>
                    <td>{row.count}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </details>
        </>
      )}
    </section>
  );
}
