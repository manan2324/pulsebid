import { api, queryString } from "./client";
import { parseReport } from "@/lib/schemas/report";
import type { Granularity, ReportRange } from "@/lib/reportRange";
export const reportingApi = {
  list: (range: ReportRange, signal?: AbortSignal) =>
    api.get(
      `/api/reporting/reports/campaigns${queryString({ from: range.from, to: range.to, granularity: range.granularity, campaign_id: range.campaignIds.length ? range.campaignIds : undefined })}`,
      signal,
      parseReport,
    ),
  campaign: (
    id: string,
    from: string,
    to: string,
    granularity: Granularity,
    signal?: AbortSignal,
  ) =>
    api.get(
      `/api/reporting/reports/campaigns/${encodeURIComponent(id)}${queryString({ from, to, granularity })}`,
      signal,
      parseReport,
    ),
};
