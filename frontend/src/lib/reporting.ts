import type { ReportResponse, ReportRow } from "./api/types";
import { reportingCopy as c } from "./copy/reporting";
export type Totals = ReportResponse["totals"];
export type ReportMetric =
  "impressions" | "clicks" | "ctr" | "spend" | "win_rate";
export const reportMetrics: ReportMetric[] = [
  "impressions",
  "clicks",
  "ctr",
  "spend",
  "win_rate",
];
export function relativeDelta(
  current: number,
  previous: number,
): number | null {
  if (
    !Number.isFinite(current) ||
    !Number.isFinite(previous) ||
    current < 0 ||
    previous < 0
  )
    return null;
  if (previous === 0) return current === 0 ? 0 : null;
  return (current - previous) / previous;
}
export function campaignName(id: string, names: ReadonlyMap<string, string>) {
  return names.get(id) ?? `${c.unknown} (${id.slice(0, 8)}…)`;
}
export interface ReportPoint {
  bucket: string;
  impressions: number;
  clicks: number;
  bids: number;
  wins: number;
  spendMicros: number;
  spend: number;
  ctr: number;
  win_rate: number;
}
function spendMicros(value: number) {
  const [whole, fraction = ""] = value.toFixed(6).split(".");
  return BigInt(whole!) * BigInt(1000000) + BigInt(fraction.padEnd(6, "0"));
}
export function chartPoints(report: ReportResponse): ReportPoint[] {
  if (report.granularity === "total" || !report.data.length) return [];
  const buckets = new Map<
    string,
    {
      impressions: number;
      clicks: number;
      bids: number;
      wins: number;
      micros: bigint;
    }
  >();
  const step = report.granularity === "hour" ? 3600000 : 86400000;
  const first =
    report.granularity === "day"
      ? Math.floor(Date.parse(report.from) / step) * step
      : Math.ceil(Date.parse(report.from) / step) * step;
  for (let time = first; time < Date.parse(report.to); time += step)
    buckets.set(new Date(time).toISOString(), {
      impressions: 0,
      clicks: 0,
      bids: 0,
      wins: 0,
      micros: BigInt(0),
    });
  for (const row of report.data) {
    if (!row.bucket) continue;
    const key = new Date(row.bucket).toISOString();
    const value = buckets.get(key) ?? {
      impressions: 0,
      clicks: 0,
      bids: 0,
      wins: 0,
      micros: BigInt(0),
    };
    value.impressions += row.impressions;
    value.clicks += row.clicks;
    value.bids += row.bids;
    value.wins += row.wins;
    value.micros += spendMicros(row.spend);
    buckets.set(key, value);
  }
  return [...buckets]
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([bucket, value]) => {
      if (value.micros > BigInt(Number.MAX_SAFE_INTEGER))
        throw new RangeError(c.dataError);
      const micros = Number(value.micros);
      return {
        bucket,
        impressions: value.impressions,
        clicks: value.clicks,
        bids: value.bids,
        wins: value.wins,
        spendMicros: micros,
        spend: micros / 1000000,
        ctr: value.impressions ? value.clicks / value.impressions : 0,
        win_rate: value.bids ? value.wins / value.bids : 0,
      };
    });
}
function csvCell(value: string | number) {
  let text = String(value);
  if (
    typeof value === "string" &&
    (/^[\t\r\n]/.test(text) || /^\s*[=+\-@]/.test(text))
  )
    text = "'" + text;
  return /[",\r\n]/.test(text) ? `"${text.replace(/"/g, '""')}"` : text;
}
export function reportCsv(
  report: ReportResponse,
  names: ReadonlyMap<string, string>,
) {
  const header = [
    "campaign_id",
    "campaign",
    "bucket_utc",
    "impressions",
    "clicks",
    "ctr",
    "spend_usd",
    "currency",
    "bids",
    "wins",
    "win_rate",
    "from_utc",
    "to_utc",
  ];
  const rows = report.data.map((row) => [
    row.campaign_id,
    campaignName(row.campaign_id, names),
    row.bucket ?? "",
    row.impressions,
    row.clicks,
    row.ctr,
    row.spend,
    row.currency,
    row.bids,
    row.wins,
    row.win_rate,
    report.from,
    report.to,
  ]);
  return (
    "\ufeff" +
    [header, ...rows].map((row) => row.map(csvCell).join(",")).join("\r\n") +
    "\r\n"
  );
}
export function topCampaigns(rows: readonly ReportRow[], limit = 5) {
  return [...rows]
    .sort(
      (a, b) => b.spend - a.spend || a.campaign_id.localeCompare(b.campaign_id),
    )
    .slice(0, limit);
}
