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
import type { ReportRow } from "@/lib/api/types";
import { campaignCopy as c } from "@/lib/copy/campaign";
export function CampaignTimeSeries({ data }: { data: ReportRow[] }) {
  return (
    <div className="campaign-chart" role="img" aria-label={c.chart}>
      <ResponsiveContainer width="100%" height="100%">
        <LineChart data={data}>
          <CartesianGrid strokeDasharray="3 3" stroke="var(--border)" />
          <XAxis
            dataKey="bucket"
            tickFormatter={(value: string) => value.slice(11, 16)}
            fontSize={10}
          />
          <YAxis fontSize={10} />
          <Tooltip />
          <Line
            type="monotone"
            dataKey="impressions"
            name={c.impressions}
            stroke="var(--primary)"
            dot={false}
          />
        </LineChart>
      </ResponsiveContainer>
    </div>
  );
}
