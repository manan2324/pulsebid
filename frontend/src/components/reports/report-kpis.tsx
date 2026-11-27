"use client";
import { Skeleton } from "@/components/ui/skeleton";
import { formatNumber, formatPercent } from "@/lib/format";
import { formatUsd } from "@/lib/money";
import { relativeDelta, type Totals } from "@/lib/reporting";
import { reportingCopy as c } from "@/lib/copy/reporting";
export const kpiMetrics = [
  "impressions",
  "clicks",
  "ctr",
  "spend",
  "bids",
  "wins",
  "win_rate",
] as const;
export function reportValue(
  metric: (typeof kpiMetrics)[number],
  value: number,
) {
  return metric === "spend"
    ? formatUsd(value, { maxFraction: 6 })
    : metric === "ctr" || metric === "win_rate"
      ? formatPercent(value)
      : formatNumber(value);
}
export function ReportKpis({
  totals,
  previous,
  compare = false,
  prefix = "report",
  metrics = kpiMetrics,
}: {
  totals?: Totals;
  previous?: Totals;
  compare?: boolean;
  prefix?: string;
  metrics?: readonly (typeof kpiMetrics)[number][];
}) {
  return (
    <div className="metric-grid report-kpis">
      {metrics.map((metric) => {
        const delta =
          totals && previous
            ? relativeDelta(totals[metric], previous[metric])
            : null;
        return (
          <div className="stat-card" key={metric}>
            <span>
              {c[metric]}
              {metric === "spend" ? ` (${c.usd})` : ""}
            </span>
            {totals ? (
              <strong
                data-testid={`${prefix}-${metric}`}
                data-value={totals[metric]}
              >
                {reportValue(metric, totals[metric])}
              </strong>
            ) : (
              <Skeleton className="h-7 w-20" />
            )}
            {compare && (
              <small className="metric-delta">
                {previous
                  ? delta === null
                    ? c.noBaseline
                    : `${delta > 0 ? "+" : ""}${formatPercent(delta)}`
                  : c.deltaUnavailable}{" "}
                · {c.yesterday}
              </small>
            )}
          </div>
        );
      })}
    </div>
  );
}
