import type { ReportResponse, ReportRow } from "@/lib/api/types";
export const reportCampaignId = "019aab00-0000-7000-8000-000000000001";
export const reportRow: ReportRow = {
  campaign_id: reportCampaignId,
  bucket: "2026-11-25T10:00:00Z",
  impressions: 100,
  clicks: 3,
  ctr: 0.03,
  spend: 0.151,
  bids: 200,
  wins: 100,
  win_rate: 0.5,
  currency: "USD",
};
export function reportFixture(
  granularity: "hour" | "day" | "total" = "hour",
): ReportResponse {
  const totals = {
    impressions: reportRow.impressions,
    clicks: reportRow.clicks,
    ctr: reportRow.ctr,
    spend: reportRow.spend,
    bids: reportRow.bids,
    wins: reportRow.wins,
    win_rate: reportRow.win_rate,
  };
  const row = { ...reportRow };
  if (granularity === "total") delete row.bucket;
  else if (granularity === "day") row.bucket = "2026-11-25T00:00:00Z";
  return {
    from: "2026-11-25T00:00:00Z",
    to: "2026-11-26T00:00:00Z",
    granularity,
    data: [row],
    totals,
  };
}
export const emptyReport: ReportResponse = {
  from: "2026-11-25T00:00:00Z",
  to: "2026-11-26T00:00:00Z",
  granularity: "hour",
  data: [],
  totals: {
    impressions: 0,
    clicks: 0,
    ctr: 0,
    spend: 0,
    bids: 0,
    wins: 0,
    win_rate: 0,
  },
};
