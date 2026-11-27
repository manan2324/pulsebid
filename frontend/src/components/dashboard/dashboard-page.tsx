"use client";
import dynamic from "next/dynamic";
import Link from "next/link";
import { PageHeader } from "@/components/layout/page-header";
import { OnVisible } from "@/components/layout/on-visible";
import { HealthPanel } from "@/components/data/health-panel";
import { ReportKpis } from "@/components/reports/report-kpis";
import { Skeleton } from "@/components/ui/skeleton";
import { Button } from "@/components/ui/button";
import { EmptyState, ErrorState } from "@/components/data/shared";
import { useReport, useReportClock } from "@/hooks/use-reports";
import { utcPreset, last24Hours } from "@/lib/reportRange";
import { ApiClientError } from "@/lib/api/client";
import { reportingCopy as c } from "@/lib/copy/reporting";

const Chart = dynamic(
  () =>
    import("@/components/reports/report-chart").then(
      (module) => module.ReportChart,
    ),
  { ssr: false, loading: () => <Skeleton className="h-64 w-full" /> },
);
const Live = dynamic(
  () => import("./live-exchange").then((module) => module.LiveExchange),
  { ssr: false, loading: () => <Skeleton className="h-96 w-full" /> },
);
const Top = dynamic(
  () => import("./top-campaigns").then((module) => module.TopCampaigns),
  { ssr: false, loading: () => <Skeleton className="h-64 w-full" /> },
);
const Budgets = dynamic(
  () => import("./budget-alerts").then((module) => module.BudgetAlerts),
  { ssr: false, loading: () => <Skeleton className="h-64 w-full" /> },
);
export function DashboardPage() {
  const now = useReportClock();
  const todayRange = {
    ...utcPreset("today", now),
    granularity: "total" as const,
    campaignIds: [],
  };
  const today = useReport(todayRange);
  const yesterday = useReport({
    ...utcPreset("yesterday", now),
    granularity: "total",
    campaignIds: [],
  });
  const hours = useReport({
    ...last24Hours(now),
    granularity: "hour",
    campaignIds: [],
  });
  return (
    <>
      <PageHeader title={c.dashboard} description={c.dashboardHelp} />
      <section className="dashboard-today" aria-label={c.today}>
        <div className="section-heading">
          <h2>{c.today}</h2>
          <Button
            variant="outline"
            disabled={
              today.isFetching || yesterday.isFetching || hours.isFetching
            }
            onClick={() => {
              void today.refetch();
              void yesterday.refetch();
              void hours.refetch();
            }}
          >
            {c.refresh}
          </Button>
        </div>
        {today.isError ? (
          <ErrorState
            message={today.error.message}
            requestId={
              today.error instanceof ApiClientError
                ? today.error.requestId
                : undefined
            }
            retry={() => {
              void today.refetch();
            }}
          />
        ) : (
          <ReportKpis
            totals={today.data?.totals}
            previous={yesterday.data?.totals}
            compare
            prefix="dashboard"
            metrics={["impressions", "clicks", "ctr", "spend", "win_rate"]}
          />
        )}
        <p className="field-help">
          {c.rangeSummary}: {todayRange.from} → {todayRange.to}
        </p>
      </section>
      {today.data && !today.data.data.length && (
        <EmptyState
          title={c.noTraffic}
          description={c.noTrafficHelp}
          action={
            <Button asChild>
              <Link href="/simulator">{c.simulator}</Link>
            </Button>
          }
        />
      )}
      <div className="dashboard-main-grid">
        <section className="panel dashboard-delivery">
          {hours.isPending ? (
            <Skeleton className="h-96 w-full" />
          ) : hours.isError ? (
            <ErrorState
              message={hours.error.message}
              requestId={
                hours.error instanceof ApiClientError
                  ? hours.error.requestId
                  : undefined
              }
              retry={() => {
                void hours.refetch();
              }}
            />
          ) : hours.data.data.length ? (
            <Chart
              report={hours.data}
              title={c.last24}
              metrics={["impressions", "clicks", "spend"]}
            />
          ) : (
            <>
              <h2>{c.last24}</h2>
              <p>{c.empty}</p>
            </>
          )}
        </section>
        <Live />
      </div>
      <OnVisible title={c.top}>
        <Top range={todayRange} />
      </OnVisible>
      <div className="dashboard-main-grid dashboard-support">
        <HealthPanel />
        <OnVisible title={c.budgetAlerts}>
          <Budgets />
        </OnVisible>
      </div>
    </>
  );
}
