"use client";
import { z } from "zod";
import { useQuery } from "@tanstack/react-query";
import Link from "next/link";
import { api, ApiClientError } from "@/lib/api/client";
import { systemCopy as c } from "@/lib/copy/system";
import { formatTime } from "@/lib/time";
import { useTimeMode } from "@/components/providers";
import { PageHeader } from "@/components/layout/page-header";
import { HealthPanel } from "@/components/data/health-panel";
import { ErrorState, EmptyState, StatCard } from "@/components/data/shared";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Sheet,
  SheetContent,
  SheetTrigger,
  SheetTitle,
  SheetDescription,
} from "@/components/ui/sheet";
const schema = z
  .object({
    version: z.string(),
    generated_at: z.string().datetime({ offset: true }),
    bidders: z.array(z.object({ id: z.string() }).passthrough()),
    campaigns: z.array(
      z.object({ creatives: z.array(z.unknown()) }).passthrough(),
    ),
  })
  .passthrough();
export function SystemPage() {
  const { mode } = useTimeMode();
  const query = useQuery({
    queryKey: ["system-snapshot"],
    queryFn: ({ signal }) =>
      api.get("/api/campaign/snapshot", signal, (value) => {
        const parsed = schema.safeParse(value);
        if (!parsed.success) throw new Error(c.snapshotError);
        return parsed.data;
      }),
    staleTime: 10000,
    refetchInterval: 10000,
    meta: { quiet: true },
  });
  const grafana =
    process.env.NEXT_PUBLIC_GRAFANA_URL ?? "http://localhost:3000";
  return (
    <div className="lab-workspace">
      <PageHeader title={c.title} description={c.description} />
      <Sheet>
        <SheetTrigger asChild>
          <Button variant="outline">{c.help}</Button>
        </SheetTrigger>
        <SheetContent>
          <SheetTitle>{c.help}</SheetTitle>
          <SheetDescription className="system-help">
            {c.helpText}
          </SheetDescription>
        </SheetContent>
      </Sheet>
      <HealthPanel />
      <section className="panel lab-card">
        <div className="section-heading">
          <h2>{c.snapshot}</h2>
          <Button
            variant="outline"
            disabled={query.isFetching}
            onClick={() => {
              void query.refetch();
            }}
          >
            {c.refresh}
          </Button>
        </div>
        {query.isPending ? (
          <Skeleton className="h-48 w-full" />
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
            <p>
              {c.version}: <code>{query.data.version}</code>
            </p>
            <p>
              {c.generated}: {formatTime(query.data.generated_at, mode)}
            </p>
            <div className="metric-grid">
              <StatCard
                title={c.bidders}
                value={query.data.bidders.length}
                description={c.snapshot}
              />
              <StatCard
                title={c.campaigns}
                value={query.data.campaigns.length}
                description={c.snapshot}
              />
              <StatCard
                title={c.creatives}
                value={query.data.campaigns.reduce(
                  (sum, row) => sum + row.creatives.length,
                  0,
                )}
                description={c.snapshot}
              />
            </div>
            {!query.data.campaigns.length && (
              <EmptyState
                title={c.empty}
                description={c.emptyHelp}
                action={
                  <Button asChild>
                    <Link href="/campaigns/new">{c.create}</Link>
                  </Button>
                }
              />
            )}
            <details>
              <summary>{c.json}</summary>
              <textarea
                className="system-json"
                aria-label={c.json}
                readOnly
                rows={14}
                value={JSON.stringify(query.data, null, 2)}
              />
            </details>
          </>
        )}
      </section>
      <section className="panel lab-card">
        <h2>{c.dashboards}</h2>
        <div className="report-actions">
          {[
            ["pulsebid-exchange-overview", c.exchange],
            ["pulsebid-bidders", c.bidderDashboard],
            ["pulsebid-budget-and-events", c.budgets],
          ].map(([uid, label]) => (
            <a
              key={uid}
              href={`${grafana}/d/${uid}`}
              target="_blank"
              rel="noreferrer"
            >
              {label} ↗
            </a>
          ))}
          <a
            href={
              process.env.NEXT_PUBLIC_JAEGER_URL ?? "http://localhost:16686"
            }
            target="_blank"
            rel="noreferrer"
          >
            {c.jaeger} ↗
          </a>
        </div>
      </section>
    </div>
  );
}
