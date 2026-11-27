"use client";
import {
  ResponsiveContainer,
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ReferenceLine,
} from "recharts";
import { reportingCopy as c } from "@/lib/copy/reporting";
import { formatTime, type TimeMode } from "@/lib/time";
export function PromLatency({
  data,
  mode,
}: {
  data: { time: number; ms: number }[];
  mode: TimeMode;
}) {
  return (
    <div className="campaign-chart" role="img" aria-label={c.latencyChart}>
      <ResponsiveContainer width="100%" height="100%">
        <LineChart data={data}>
          <CartesianGrid strokeDasharray="3 3" stroke="var(--border)" />
          <XAxis
            dataKey="time"
            tick={{ fill: "var(--muted-foreground)", fontSize: 10 }}
            tickFormatter={(value) =>
              new Date(Number(value)).toLocaleTimeString("en-GB", {
                hour: "2-digit",
                minute: "2-digit",
                timeZone: mode === "UTC" ? "UTC" : undefined,
              })
            }
          />
          <YAxis
            tick={{ fill: "var(--muted-foreground)", fontSize: 10 }}
            unit=" ms"
            domain={[0, "auto"]}
          />
          <Tooltip
            labelFormatter={(value) =>
              formatTime(new Date(Number(value)), mode)
            }
            formatter={(value) => `${Number(value).toFixed(1)} ms`}
            contentStyle={{
              background: "var(--card)",
              color: "var(--foreground)",
              borderColor: "var(--border)",
            }}
          />
          <ReferenceLine
            ifOverflow="extendDomain"
            y={120}
            stroke="var(--destructive)"
            strokeDasharray="4 4"
            label={{
              value: c.latencyBudget,
              position: "insideTopRight",
              fill: "var(--foreground)",
              fontSize: 10,
            }}
          />
          <Line
            type="monotone"
            dataKey="ms"
            name={c.p99}
            stroke="var(--primary)"
            dot={false}
            isAnimationActive={false}
          />
        </LineChart>
      </ResponsiveContainer>
    </div>
  );
}
