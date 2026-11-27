"use client";
import { RefreshCw, Server } from "lucide-react";
import { useHealth } from "@/hooks/use-health";
import { useTimeMode } from "@/components/providers";
import { formatTime } from "@/lib/time";
import { copy } from "@/lib/copy/app";
import { ApiClientError } from "@/lib/api/client";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState, ErrorState, StatusBadge } from "./shared";
export function HealthPanel() {
  const health = useHealth();
  const { mode } = useTimeMode();
  return (
    <section className="panel health-panel">
      <div className="panel-heading">
        <div>
          <h2>
            <Server size={17} />
            {copy.healthTitle}
          </h2>
          <p>{copy.healthDescription}</p>
        </div>
        <Button
          variant="ghost"
          size="icon"
          aria-label={copy.retry}
          disabled={health.isFetching}
          onClick={() => {
            void health.refetch();
          }}
        >
          <RefreshCw
            size={15}
            className={health.isFetching ? "animate-spin" : ""}
          />
        </Button>
      </div>
      {health.isPending ? (
        <div className="health-loading" aria-label={copy.loading} role="status">
          {[0, 1, 2, 3].map((key) => (
            <Skeleton key={key} className="h-12 w-full" />
          ))}
        </div>
      ) : health.isError ? (
        <ErrorState
          message={health.error.message}
          requestId={
            health.error instanceof ApiClientError
              ? health.error.requestId
              : undefined
          }
          retry={() => {
            void health.refetch();
          }}
        />
      ) : health.data.services.length === 0 ? (
        <EmptyState description={copy.noServices} />
      ) : (
        <>
          <div className="service-grid">
            {health.data.services.map((service) => (
              <div className="service-row" key={service.name}>
                <span>{service.name}</span>
                <StatusBadge status={service.status} />
                <code>{service.latency_ms} ms</code>
              </div>
            ))}
          </div>
          <div className="health-checked">
            {formatTime(health.data.checked_at, mode)}
          </div>
        </>
      )}
    </section>
  );
}
