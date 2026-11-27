"use client";
import { useReportClock } from "@/hooks/use-reports";
import { utcPreset } from "@/lib/reportRange";
import { ReportView } from "@/components/reports/report-view";
import { ReportWorkspace } from "@/components/reports/report-workspace";
import { campaignCopy as c } from "@/lib/copy/campaign";
export function CampaignReport({
  id,
  range = "today",
  compact = false,
  filters = false,
}: {
  id: string;
  range?: "today" | "7d";
  compact?: boolean;
  filters?: boolean;
}) {
  const now = useReportClock();
  if (filters) return <ReportWorkspace fixedCampaignId={id} />;
  return (
    <ReportView
      title={range === "7d" ? c.last7 : c.today}
      compact={compact}
      range={{
        ...utcPreset(range, now),
        granularity: compact ? "total" : "hour",
        campaignIds: [id],
      }}
    />
  );
}
