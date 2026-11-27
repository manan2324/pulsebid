"use client";
import { useMemo } from "react";
import Link from "next/link";
import { useReport, useReportCatalog } from "@/hooks/use-reports";
import type { ReportRange } from "@/lib/reportRange";
import { topCampaigns, campaignName } from "@/lib/reporting";
import { reportValue } from "@/components/reports/report-kpis";
import { Skeleton } from "@/components/ui/skeleton";
import { ErrorState } from "@/components/data/shared";
import { ApiClientError } from "@/lib/api/client";
import { reportingCopy as c } from "@/lib/copy/reporting";
export function TopCampaigns({ range }: { range: ReportRange }) {
  const query = useReport(range),
    catalog = useReportCatalog();
  const names = useMemo(
    () => new Map(catalog.data?.map((row) => [row.id, row.name]) ?? []),
    [catalog.data],
  );
  return (
    <section className="panel dashboard-top">
      <div className="section-heading">
        <h2>{c.top}</h2>
        <Link href="/reports">{c.allReports}</Link>
      </div>
      <p className="field-help">{c.topHelp}</p>
      {query.isPending ? (
        <Skeleton className="h-64 w-full" />
      ) : query.isError ? (
        <ErrorState
          message={query.error.message}
          requestId={
            query.error instanceof ApiClientError
              ? query.error.requestId
              : undefined
          }
          retry={() => {
            void query.refetch();
          }}
        />
      ) : !query.data.data.length ? (
        <p>{c.topEmpty}</p>
      ) : (
        <div className="table-scroll">
          <table className="simulator-simple-table" data-testid="top-campaigns">
            <thead>
              <tr>
                <th scope="col">{c.campaign}</th>
                {(["impressions", "spend", "ctr", "win_rate"] as const).map(
                  (metric) => (
                    <th scope="col" key={metric}>
                      {c[metric]}
                    </th>
                  ),
                )}
              </tr>
            </thead>
            <tbody>
              {topCampaigns(query.data.data).map((row) => (
                <tr key={row.campaign_id}>
                  <th scope="row">
                    <Link
                      href={`/campaigns/${encodeURIComponent(row.campaign_id)}`}
                    >
                      {campaignName(row.campaign_id, names)}
                    </Link>
                  </th>
                  {(["impressions", "spend", "ctr", "win_rate"] as const).map(
                    (metric) => (
                      <td key={metric}>{reportValue(metric, row[metric])}</td>
                    ),
                  )}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </section>
  );
}
