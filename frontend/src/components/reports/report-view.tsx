"use client";
import { useMemo, useState } from "react";
import dynamic from "next/dynamic";
import Link from "next/link";
import { toast } from "sonner";
import { useReport, useReportCatalog } from "@/hooks/use-reports";
import type { ReportRange } from "@/lib/reportRange";
import { reportCsv } from "@/lib/reporting";
import { ApiClientError } from "@/lib/api/client";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState, ErrorState } from "@/components/data/shared";
import { useTimeMode } from "@/components/providers";
import { formatTime } from "@/lib/time";
import { ReportKpis } from "./report-kpis";
import { reportingCopy as c } from "@/lib/copy/reporting";
const Chart = dynamic(
  () => import("./report-chart").then((module) => module.ReportChart),
  { ssr: false, loading: () => <Skeleton className="h-64 w-full" /> },
);
const Table = dynamic(
  () => import("./report-table").then((module) => module.ReportTable),
  { ssr: false, loading: () => <Skeleton className="h-64 w-full" /> },
);
export function ReportView({
  range,
  compact = false,
  title,
}: {
  range: ReportRange;
  compact?: boolean;
  title?: string;
}) {
  const query = useReport(range);
  const catalog = useReportCatalog(!compact);
  const names = useMemo(
    () => new Map(catalog.data?.map((row) => [row.id, row.name]) ?? []),
    [catalog.data],
  );
  const { mode } = useTimeMode();
  const [exportError, setExportError] = useState("");
  const download = () => {
    if (!query.data) return;
    try {
      const url = URL.createObjectURL(
        new Blob([reportCsv(query.data, names)], {
          type: "text/csv;charset=utf-8",
        }),
      );
      const link = document.createElement("a");
      link.href = url;
      link.download = `pulsebid-report-${query.data.from.slice(0, 10)}-${query.data.to.slice(0, 10)}-${query.data.granularity}.csv`;
      document.body.append(link);
      link.click();
      link.remove();
      window.setTimeout(() => URL.revokeObjectURL(url), 1000);
      setExportError("");
      toast.success(c.exported);
    } catch {
      setExportError(c.exportFailed);
    }
  };
  return (
    <section className="report-view" aria-label={title ?? c.reports}>
      {title && <h2>{title}</h2>}
      {!compact && (
        <div className="report-actions">
          <Button
            variant="outline"
            disabled={!query.data?.data.length || query.isError}
            onClick={download}
          >
            {c.export}
          </Button>
          <Button
            variant="outline"
            disabled={query.isFetching}
            onClick={() => {
              void query.refetch();
            }}
          >
            {c.refresh}
          </Button>
          {query.data && (
            <small>
              {c.updated}: {formatTime(new Date(query.dataUpdatedAt), mode)}
            </small>
          )}
        </div>
      )}
      {exportError && (
        <p role="alert" className="field-error">
          {exportError}
        </p>
      )}
      {query.isPending ? (
        <ReportKpis />
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
      ) : (
        <>
          <ReportKpis totals={query.data.totals} />
          {!compact && (
            <>
              {catalog.isError && (
                <p className="notice">{c.namesUnavailable}</p>
              )}
              <p className="field-help">
                {c.rangeSummary}: <time>{query.data.from}</time> →{" "}
                <time>{query.data.to}</time>
              </p>
              {query.data.data.length ? (
                <>
                  <Chart report={query.data} />
                  <h2>{c.table}</h2>
                  <p className="field-help">{c.exportHelp}</p>
                  <Table data={query.data.data} names={names} />
                </>
              ) : (
                <EmptyState
                  title={c.empty}
                  description={c.emptyHelp}
                  action={
                    <Button asChild variant="outline">
                      <Link href="/simulator">{c.simulator}</Link>
                    </Button>
                  }
                />
              )}
            </>
          )}
        </>
      )}
    </section>
  );
}
