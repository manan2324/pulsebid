"use client";
import { RotateCw } from "lucide-react";
import { useBidderHealth } from "@/hooks/use-catalog";
import { StatusBadge } from "@/components/data/shared";
import { Button } from "@/components/ui/button";
import { catalogCopy as c } from "@/lib/copy/catalog";
import { ApiClientError } from "@/lib/api/client";
export function BidderHealth({ index }: { index: number | null }) {
  const query = useBidderHealth(index);
  if (index === null)
    return (
      <span className="muted-text" title={c.healthUnknown}>
        {c.notApplicable}
      </span>
    );
  if (query.isPending)
    return (
      <span role="status" className="muted-text">
        {c.checking}
      </span>
    );
  if (query.isError)
    return (
      <span
        className="bidder-health-error"
        title={`${query.error.message}${query.error instanceof ApiClientError ? ` (${c.requestId}: ${query.error.requestId})` : ""}`}
      >
        <StatusBadge status="unavailable" />
        <Button
          variant="ghost"
          size="icon"
          aria-label={c.retryHealth}
          disabled={query.isFetching}
          onClick={() => {
            void query.refetch();
          }}
        >
          <RotateCw size={12} />
        </Button>
      </span>
    );
  return <StatusBadge status="healthy" />;
}
