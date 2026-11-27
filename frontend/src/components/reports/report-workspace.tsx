"use client";
import { useQueryStates, parseAsArrayOf, parseAsString } from "nuqs";
import dynamic from "next/dynamic";
import { useReportClock } from "@/hooks/use-reports";
import { resolveReportRange, utcPreset } from "@/lib/reportRange";
import { ReportFilters, type ReportSelection } from "./report-filters";
import { ErrorState } from "@/components/data/shared";
import { Skeleton } from "@/components/ui/skeleton";
const View = dynamic(
  () => import("./report-view").then((module) => module.ReportView),
  { ssr: false, loading: () => <Skeleton className="h-96 w-full" /> },
);
export function ReportWorkspace({
  fixedCampaignId,
}: {
  fixedCampaignId?: string;
}) {
  const now = useReportClock();
  const [filters, setFilters] = useQueryStates({
    preset: parseAsString.withDefault("today"),
    from: parseAsString.withDefault(""),
    to: parseAsString.withDefault(""),
    granularity: parseAsString.withDefault("hour"),
    campaign_id: parseAsArrayOf(parseAsString).withDefault([]),
  });
  const selection = {
    preset: filters.preset,
    from: filters.from,
    to: filters.to,
    granularity: filters.granularity,
    campaignIds: fixedCampaignId ? [fixedCampaignId] : filters.campaign_id,
  };
  const resolved = resolveReportRange(selection, now);
  const bounds =
    resolved.range ??
    (filters.preset === "custom"
      ? { from: filters.from, to: filters.to }
      : utcPreset("today", now));
  const apply = (value: ReportSelection) => {
    void setFilters(
      {
        preset: value.preset,
        from: value.preset === "custom" ? value.from : null,
        to: value.preset === "custom" ? value.to : null,
        granularity: value.granularity,
        campaign_id: fixedCampaignId
          ? null
          : [...new Set(value.campaignIds)].sort(),
      },
      { history: "push" },
    );
  };
  return (
    <>
      <ReportFilters
        key={JSON.stringify({ ...selection, from: bounds.from, to: bounds.to })}
        initial={{ ...selection, from: bounds.from, to: bounds.to }}
        now={now}
        fixedCampaignId={fixedCampaignId}
        onApply={apply}
      />
      {resolved.error ? (
        <ErrorState message={resolved.error} />
      ) : (
        resolved.range && (
          <div className="panel report-results">
            <View range={resolved.range} />
          </div>
        )
      )}
    </>
  );
}
