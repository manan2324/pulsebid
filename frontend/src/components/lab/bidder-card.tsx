"use client";
import { useEffect, useState } from "react";
import { toast } from "sonner";
import { useBidderConfig, useBidderUpdate } from "@/hooks/use-lab";
import { useDirtyGuard } from "@/hooks/use-dirty-guard";
import { bidderConfigSchema, configDiff, type BidderConfig } from "@/lib/lab";
import { labCopy as c } from "@/lib/copy/lab";
import { ApiClientError } from "@/lib/api/client";
import { ErrorState } from "@/components/data/shared";
import { BidderHealth } from "@/components/catalog/bidder-health";
import { Skeleton } from "@/components/ui/skeleton";
import { Button } from "@/components/ui/button";
export function BidderCard({
  index,
  port,
  selected,
  onSelect,
  locked,
  remember,
}: {
  index: number;
  port: number;
  selected: boolean;
  onSelect: () => void;
  locked: boolean;
  remember: (index: number, config: BidderConfig) => void;
}) {
  const query = useBidderConfig(index),
    mutation = useBidderUpdate(index);
  const [draft, setDraft] = useState<BidderConfig>();
  const [error, setError] = useState("");
  useEffect(() => {
    if (query.data) setDraft(query.data);
  }, [query.data]);
  const changed = query.data && draft ? configDiff(query.data, draft) : {};
  useDirtyGuard(Object.keys(changed).length > 0);
  return (
    <section className="panel lab-card" aria-label={`Bidder ${index + 1}`}>
      <div className="section-heading">
        <h2>Bidder {index + 1}</h2>
        <label>
          <input
            type="checkbox"
            checked={selected}
            onChange={onSelect}
            disabled={locked}
          />{" "}
          {c.select} {index + 1}
        </label>
      </div>
      <p>
        {c.port}: {port} · <BidderHealth index={index} />
      </p>
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
      ) : (
        draft && (
          <form
            onSubmit={(event) => {
              event.preventDefault();
              const parsed = bidderConfigSchema.safeParse(draft);
              if (!parsed.success) {
                setError(c.configError);
                return;
              }
              remember(index, query.data!);
              setError("");
              mutation.mutate(changed, {
                onSuccess: () => toast.success(c.saved),
              });
            }}
          >
            <fieldset
              disabled={locked || mutation.isPending}
              className="lab-fields"
            >
              {(
                [
                  "latency_ms",
                  "jitter_ms",
                  "error_rate",
                  "timeout_rate",
                  "no_bid_rate",
                  "bid_shade",
                  "seed",
                ] as const
              ).map((key) => (
                <label key={key}>
                  {c[key]}
                  <input
                    className="form-input"
                    type="number"
                    value={Number.isNaN(draft[key]) ? "" : draft[key]}
                    min={key === "seed" ? undefined : 0}
                    max={
                      key === "latency_ms"
                        ? 1000
                        : key === "jitter_ms"
                          ? 500
                          : key === "seed"
                            ? undefined
                            : 1
                    }
                    step={
                      key.endsWith("rate")
                        ? 0.05
                        : key === "bid_shade"
                          ? 0.01
                          : 1
                    }
                    onChange={(event) =>
                      setDraft({
                        ...draft,
                        [key]:
                          event.target.value === ""
                            ? NaN
                            : Number(event.target.value),
                      })
                    }
                    required
                  />
                </label>
              ))}
              <label>
                {c.bid_mode}
                <select
                  className="form-input"
                  value={draft.bid_mode}
                  onChange={(event) =>
                    setDraft({
                      ...draft,
                      bid_mode: event.target.value as BidderConfig["bid_mode"],
                    })
                  }
                >
                  {["fixed", "shade", "random"].map((value) => (
                    <option key={value}>{value}</option>
                  ))}
                </select>
              </label>
            </fieldset>
            {error && (
              <p role="alert" className="field-error">
                {error}
              </p>
            )}
            {mutation.isError && (
              <ErrorState
                message={mutation.error.message}
                requestId={
                  mutation.error instanceof ApiClientError
                    ? mutation.error.requestId
                    : undefined
                }
              />
            )}
            <Button
              disabled={
                locked || mutation.isPending || !Object.keys(changed).length
              }
              type="submit"
            >
              {c.save}
            </Button>
          </form>
        )
      )}
    </section>
  );
}
