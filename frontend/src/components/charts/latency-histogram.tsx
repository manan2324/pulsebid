"use client";
import {
  ResponsiveContainer,
  BarChart,
  Bar,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
} from "recharts";
import { simulatorCopy as c } from "@/lib/copy/simulator";
export function LatencyHistogram({
  data,
}: {
  data: { label: string; count: number }[];
}) {
  return (
    <div className="campaign-chart" role="img" aria-label={c.histogram}>
      <ResponsiveContainer width="100%" height="100%">
        <BarChart data={data}>
          <CartesianGrid strokeDasharray="3 3" stroke="var(--border)" />
          <XAxis dataKey="label" fontSize={10} />
          <YAxis allowDecimals={false} fontSize={10} />
          <Tooltip
            contentStyle={{
              background: "var(--card)",
              color: "var(--foreground)",
              borderColor: "var(--border)",
            }}
          />
          <Bar dataKey="count" name={c.completed} fill="var(--primary)" />
        </BarChart>
      </ResponsiveContainer>
    </div>
  );
}
