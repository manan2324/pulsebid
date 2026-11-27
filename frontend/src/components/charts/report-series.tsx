"use client";
import {
  ResponsiveContainer,
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
} from "recharts";
import type { ReportMetric, ReportPoint } from "@/lib/reporting";
import { formatNumber, formatPercent } from "@/lib/format";
import { formatUsd } from "@/lib/money";
import { formatTime, type TimeMode } from "@/lib/time";
import { reportingCopy as c } from "@/lib/copy/reporting";
export function ReportSeries({
  data,
  metric,
  mode,
}: {
  data: ReportPoint[];
  metric: ReportMetric;
  mode: TimeMode;
}) {
  const format = (value: number) =>
    metric === "spend"
      ? formatUsd(value, { maxFraction: 6 })
      : metric === "ctr" || metric === "win_rate"
        ? formatPercent(value)
        : formatNumber(value);
  return (
    <div
      className="campaign-chart"
      role="img"
      aria-label={`${c.chart}: ${c[metric]}`}
    >
      <ResponsiveContainer width="100%" height="100%">
        <LineChart data={data}>
          <CartesianGrid strokeDasharray="3 3" stroke="var(--border)" />
          <XAxis
            dataKey="bucket"
            minTickGap={35}
            tick={{ fill: "var(--muted-foreground)", fontSize: 10 }}
            tickFormatter={(value: string) =>
              new Intl.DateTimeFormat("en-GB", {
                month: "short",
                day: "numeric",
                hour: "2-digit",
                minute: "2-digit",
                ...(mode === "UTC" ? { timeZone: "UTC" } : {}),
              }).format(new Date(value))
            }
          />
          <YAxis
            tick={{ fill: "var(--muted-foreground)", fontSize: 10 }}
            tickFormatter={format}
            allowDecimals={metric !== "impressions" && metric !== "clicks"}
            tickCount={metric === "spend" ? 2 : 5}
            width={70}
          />
          <Tooltip
            labelFormatter={(label) => formatTime(String(label), mode)}
            formatter={(value) => format(Number(value))}
            contentStyle={{
              background: "var(--card)",
              color: "var(--foreground)",
              borderColor: "var(--border)",
            }}
          />
          <Line
            type="monotone"
            dataKey={metric}
            name={c[metric]}
            stroke="var(--primary)"
            dot={false}
            isAnimationActive={false}
          />
        </LineChart>
      </ResponsiveContainer>
    </div>
  );
}
