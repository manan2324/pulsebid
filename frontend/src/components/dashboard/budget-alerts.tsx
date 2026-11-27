"use client";
import { useState } from "react";
import { useQueries } from "@tanstack/react-query";
import Link from "next/link";
import { useReportCatalog } from "@/hooks/use-reports";
import { campaignsApi } from "@/lib/api/campaigns";
import { safeBudget, budgetPercent } from "@/lib/dashboard";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { MoneyText, ErrorState } from "@/components/data/shared";
import { ApiClientError } from "@/lib/api/client";
import { formatPercent } from "@/lib/format";
import { reportingCopy as c } from "@/lib/copy/reporting";
export function BudgetAlerts() {
  const catalog = useReportCatalog();
  const [page, setPage] = useState(0);
  const active = (catalog.data ?? []).filter((row) => row.status === "active");
  const offset = Math.min(
    page * 20,
    Math.max(0, Math.floor((active.length - 1) / 20) * 20),
  );
  const visible = active.slice(offset, offset + 20);
  const budgets = useQueries({
    queries: visible.map((campaign) => ({
      queryKey: ["budget", campaign.id],
      queryFn: ({ signal }: { signal: AbortSignal }) =>
        campaignsApi.budget(campaign.id, signal),
      staleTime: 5000,
      refetchInterval: 10000,
      refetchIntervalInBackground: false,
      meta: { quiet: true },
    })),
  });
  const loading = catalog.isPending || budgets.some((query) => query.isPending);
  const error = budgets.find((query) => query.isError)?.error;
  const invalid = budgets.some(
    (query) => query.isSuccess && !safeBudget(query.data),
  );
  const alerts = visible.flatMap((campaign, index) => {
    const budget = budgets[index]?.data;
    return safeBudget(budget) && budgetPercent(budget) >= 80
      ? [{ campaign, budget }]
      : [];
  });
  return (
    <section className="panel dashboard-budgets">
      <h2>{c.budgetAlerts}</h2>
      <p className="field-help">{c.budgetHelp}</p>
      {catalog.isError ? (
        <ErrorState
          message={c.budgetCatalogError}
          requestId={
            catalog.error instanceof ApiClientError
              ? catalog.error.requestId
              : undefined
          }
          retry={() => {
            void catalog.refetch();
          }}
        />
      ) : loading ? (
        <Skeleton className="h-24 w-full" />
      ) : !active.length ? (
        <p>{c.noActive}</p>
      ) : (
        <>
          {(error || invalid) && (
            <div className="notice" role="status">
              {c.budgetUnavailable}
              {error instanceof ApiClientError && (
                <code> {error.requestId}</code>
              )}
              <Button
                variant="ghost"
                onClick={() => {
                  for (const query of budgets)
                    if (query.isError) void query.refetch();
                }}
              >
                {c.retry}
              </Button>
            </div>
          )}
          {alerts.length ? (
            <ul className="budget-alert-list">
              {alerts.map(({ campaign, budget }) => (
                <li key={campaign.id}>
                  <Link href={`/campaigns/${encodeURIComponent(campaign.id)}`}>
                    {campaign.name}
                  </Link>
                  <span
                    className={
                      budgetPercent(budget) >= 100
                        ? "latency-bad"
                        : "budget-near"
                    }
                  >
                    {budgetPercent(budget) >= 100 ? c.exhausted : c.near}
                  </span>
                  <span>
                    {formatPercent(budgetPercent(budget) / 100)} {c.spent} ·{" "}
                    <MoneyText
                      micros={budget.remaining_micros}
                      maxFraction={6}
                    />{" "}
                    {c.remaining}
                  </span>
                </li>
              ))}
            </ul>
          ) : !error && !invalid ? (
            <p>{c.noBudgetAlerts}</p>
          ) : null}
          <p className="field-help">
            {c.coverage}: {offset + 1}–{offset + visible.length} /{" "}
            {active.length}
          </p>
          <div className="report-actions">
            <Button
              variant="outline"
              disabled={offset === 0}
              onClick={() => setPage(Math.max(0, page - 1))}
            >
              {c.previous}
            </Button>
            <Button
              variant="outline"
              disabled={offset + visible.length >= active.length}
              onClick={() => setPage(Math.floor(offset / 20) + 1)}
            >
              {c.inspectMore}
            </Button>
          </div>
        </>
      )}
    </section>
  );
}
