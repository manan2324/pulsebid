"use client";
import { useMemo, useState } from "react";
import dynamic from "next/dynamic";
import { Skeleton } from "@/components/ui/skeleton";
import { ErrorState } from "@/components/data/shared";
import type { ReportResponse } from "@/lib/api/types";
import { chartPoints, reportMetrics, type ReportMetric } from "@/lib/reporting";
import { formatNumber } from "@/lib/format";
import { useTimeMode } from "@/components/providers";
import { reportingCopy as c } from "@/lib/copy/reporting";
const Series = dynamic(
  () =>
    import("@/components/charts/report-series").then(
      (module) => module.ReportSeries,
    ),
  { ssr: false, loading: () => <Skeleton className="h-64 w-full" /> },
);
const Table = dynamic(() =>
  import("./chart-data-table").then((module) => module.ChartDataTable),
);
export function ReportChart({
  report,
  title = c.chart,
  metrics = reportMetrics,
}: {
  report: ReportResponse;
  title?: string;
  metrics?: ReportMetric[];
}) {
  const [metric, setMetric] = useState<ReportMetric>(
    metrics[0] ?? "impressions",
  );
  const [showTable, setShowTable] = useState(false);
  const { mode } = useTimeMode();
  const mapped = useMemo(() => {
    try {
      return { points: chartPoints(report), error: "" };
    } catch (error) {
      return {
        points: [],
        error: error instanceof Error ? error.message : c.dataError,
      };
    }
  }, [report]);
  return (
    <section className="report-chart">
      <div className="section-heading">
        <h2>{title}</h2>
        <label>
          {c.metric}
          <select
            aria-label={c.metric}
            value={metric}
            onChange={(event) => setMetric(event.target.value as ReportMetric)}
          >
            {metrics.map((key) => (
              <option key={key} value={key}>
                {c[key]}
              </option>
            ))}
          </select>
        </label>
      </div>
      <p className="field-help">
        {c.chartHelp} {formatNumber(report.totals.impressions)} {c.impressions},{" "}
        {formatNumber(report.totals.clicks)} {c.clicks}.
      </p>
      {mapped.error ? (
        <ErrorState message={mapped.error} />
      ) : report.granularity === "total" ? (
        <p className="notice">{c.totalHint}</p>
      ) : mapped.points.length ? (
        <>
          <Series data={mapped.points} metric={metric} mode={mode} />
          <details onToggle={(event) => setShowTable(event.currentTarget.open)}>
            <summary>{c.tableToggle}</summary>
            {showTable && <Table data={mapped.points} metric={metric} />}
          </details>
        </>
      ) : (
        <p>{c.chartEmpty}</p>
      )}
    </section>
  );
}
