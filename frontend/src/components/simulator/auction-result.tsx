"use client";
import { useState } from "react";
import dynamic from "next/dynamic";
import Link from "next/link";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import {
  CopyButton,
  EmptyState,
  ErrorState,
  MoneyText,
} from "@/components/data/shared";
import { useGatewayEvent } from "@/hooks/use-simulator";
import { ApiClientError } from "@/lib/api/client";
import type { AuctionResult as Result } from "@/lib/api/gateway";
import { reasonText } from "@/lib/simulator";
import { formatNumber } from "@/lib/format";
import { simulatorCopy as c } from "@/lib/copy/simulator";
const AdPreview = dynamic(
  () =>
    import("@/components/campaigns/ad-preview").then(
      (module) => module.AdPreview,
    ),
  { loading: () => <Skeleton className="h-64 w-full" /> },
);
function EventAction({
  kind,
  url,
  testTime,
  disabled,
}: {
  kind: "impression" | "click";
  url: string;
  testTime?: string;
  disabled: boolean;
}) {
  const mutation = useGatewayEvent(kind, url, testTime);
  const [used, setUsed] = useState(false);
  const fire = () =>
    mutation.mutate(undefined, { onSuccess: () => setUsed(true) });
  return (
    <div className="event-action">
      <Button
        variant="outline"
        disabled={disabled || used || mutation.isPending}
        onClick={fire}
      >
        {used
          ? kind === "impression"
            ? c.impressionSent
            : c.clickSent
          : kind === "impression"
            ? c.impression
            : c.click}
      </Button>
      {used && (
        <Button
          variant="ghost"
          disabled={disabled || mutation.isPending}
          onClick={fire}
        >
          {kind === "impression" ? c.duplicateImpression : c.duplicateClick}
        </Button>
      )}
      {mutation.error && (
        <ErrorState
          message={mutation.error.message}
          requestId={
            mutation.error instanceof ApiClientError
              ? mutation.error.requestId
              : undefined
          }
        />
      )}
      {mutation.data?.location && (
        <p className="landing-url">
          {c.landing}:{" "}
          <a
            href={mutation.data.location}
            target="_blank"
            rel="noopener noreferrer"
          >
            {mutation.data.location}
          </a>
          <CopyButton value={mutation.data.location} />
        </p>
      )}
    </div>
  );
}
export function AuctionResult({
  result,
  busy,
}: {
  result: Result | null;
  busy: boolean;
}) {
  if (!result)
    return (
      <section className="panel simulator-result" aria-busy={busy}>
        <h2>{c.result}</h2>
        {busy ? (
          <Skeleton className="h-64 w-full" />
        ) : (
          <EmptyState title={c.emptyResult} description={c.emptyResultHelp} />
        )}
      </section>
    );
  const response = result.response;
  const ms = (value: number | null) =>
    value === null ? c.unavailable : `${formatNumber(value, 1)} ms`;
  return (
    <section className="panel simulator-result" aria-busy={busy}>
      <h2>{c.result}</h2>
      <div className="auction-outcome" role="status">
        <span
          className={`campaign-status ${result.kind === "filled" ? "active" : result.kind === "error" ? "auction-error" : "draft"}`}
        >
          {result.httpStatus || "—"}{" "}
          {result.kind === "filled"
            ? c.filled
            : result.kind === "no-fill"
              ? c.noFill
              : c.error}
        </span>
        {result.kind === "no-fill" && (
          <strong>{reasonText(result.reason)}</strong>
        )}
        {response && (
          <strong>
            <MoneyText value={response.price} maxFraction={6} unit="CPM" />
          </strong>
        )}
      </div>
      <dl className="latency-grid">
        <div>
          <dt>{c.gateway}</dt>
          <dd>
            <span data-testid="gateway-latency">{ms(result.gatewayMs)}</span>
            <small
              className={
                result.gatewayMs === null
                  ? ""
                  : result.gatewayMs < 120
                    ? "latency-good"
                    : "latency-bad"
              }
            >
              {result.gatewayMs === null
                ? c.unavailable
                : result.gatewayMs < 120
                  ? c.latencyPass
                  : c.latencyFail}
            </small>
          </dd>
        </div>
        <div>
          <dt>{c.roundTrip}</dt>
          <dd>{ms(result.roundTripMs)}</dd>
        </div>
      </dl>
      <p className="field-help">{c.gatewayHelp}</p>
      {result.error && (
        <ErrorState
          message={`${result.error.code}: ${result.error.message}`}
          requestId={result.error.requestId}
        />
      )}
      {response && (
        <>
          <dl className="metadata-grid">
            <div>
              <dt>{c.bidder}</dt>
              <dd>
                <code>{response.bidder_id}</code>
              </dd>
            </div>
            <div>
              <dt>{c.campaign}</dt>
              <dd>
                <Link
                  href={`/campaigns/${encodeURIComponent(response.campaign_id)}`}
                >
                  {response.campaign_id}
                </Link>
              </dd>
            </div>
            <div>
              <dt>{c.creative}</dt>
              <dd>
                <code>{response.creative_id}</code>
              </dd>
            </div>
            <div>
              <dt>{c.bidId}</dt>
              <dd>
                <code>{response.bid_id}</code>
              </dd>
            </div>
          </dl>
          <AdPreview
            markup={response.adm}
            width={response.w || 300}
            height={response.h || 250}
          />
          <p className="field-help">{c.eventsHelp}</p>
          <div className="event-actions">
            <EventAction
              kind="impression"
              url={response.impression_url}
              testTime={result.testTime}
              disabled={busy}
            />
            <EventAction
              kind="click"
              url={response.click_url}
              testTime={result.testTime}
              disabled={busy}
            />
          </div>
          <p className="field-help">{c.duplicateHelp}</p>
        </>
      )}
      {result.debug !== null && (
        <details open>
          <summary>{c.debug}</summary>
          <dl className="debug-counters">
            {Object.entries(result.debug).map(([name, value]) => (
              <div key={name}>
                <dt>{name}</dt>
                <dd>{value}</dd>
              </div>
            ))}
          </dl>
        </details>
      )}
      <p className="request-correlation">
        {c.requestId}: <code>{result.requestId}</code>
        <CopyButton value={result.requestId} />
      </p>
      <details>
        <summary>{c.rawRequest}</summary>
        <pre className="raw-json">
          {JSON.stringify(result.request, null, 2)}
        </pre>
      </details>
      <details>
        <summary>{c.rawResponse}</summary>
        <pre className="raw-json">
          {JSON.stringify(
            response ??
              (result.error
                ? {
                    error: {
                      code: result.error.code,
                      message: result.error.message,
                      details: result.error.details,
                      request_id: result.error.requestId,
                    },
                  }
                : { status: 204, reason: result.reason }),
            null,
            2,
          )}
        </pre>
      </details>
    </section>
  );
}
