"use client";
import type { State } from "./campaign-wizard";
import type { Advertiser } from "@/lib/api/types";
import { campaignCopy as c } from "@/lib/copy/campaign";
export function WizardReview({
  state,
  advertisers,
  bidderName,
  locked,
  onActivate,
}: {
  state: State;
  advertisers: Advertiser[];
  bidderName?: string;
  locked: boolean;
  onActivate: (value: boolean) => void;
}) {
  return (
    <>
      <dl className="review-grid">
        <div>
          <dt>{c.name}</dt>
          <dd>{state.values.name}</dd>
        </div>
        <div>
          <dt>{c.advertiser}</dt>
          <dd>
            {
              advertisers.find((item) => item.id === state.values.advertiser_id)
                ?.name
            }
          </dd>
        </div>
        <div>
          <dt>{c.bidder}</dt>
          <dd>{bidderName}</dd>
        </div>
        <div>
          <dt>{c.budget}</dt>
          <dd>${state.values.daily_budget}</dd>
        </div>
        <div>
          <dt>{c.maxBid}</dt>
          <dd>${state.values.max_bid_cpm} CPM</dd>
        </div>
        <div>
          <dt>{c.pacing}</dt>
          <dd>{state.values.pacing_mode}</dd>
        </div>
        <div>
          <dt>{c.start}</dt>
          <dd>{state.values.start_at || c.notScheduled}</dd>
        </div>
        <div>
          <dt>{c.end}</dt>
          <dd>{state.values.end_at || c.notScheduled}</dd>
        </div>
      </dl>
      <h3>{c.targetingSummary}</h3>
      <p className="field-help">
        {state.targeting.countries.join(", ") || c.anyCountry} ·{" "}
        {state.targeting.device_types.join(", ") || c.anyDevice} ·{" "}
        {state.targeting.placement_ids.join(", ") || c.anyPlacement} ·{" "}
        {state.targeting.hours_of_day.join(", ") || c.anyHour} ·{" "}
        {state.targeting.timezone}
      </p>
      <h3>{c.creativeSummary}</h3>
      <ul>
        {state.creatives.map((value, index) => (
          <li key={index}>
            {value.name} · {value.format} · {value.width}×{value.height} ·{" "}
            {value.status}
          </li>
        ))}
      </ul>
      <label className="checkbox-label">
        <input
          type="checkbox"
          checked={state.activate}
          disabled={locked}
          onChange={(event) => onActivate(event.target.checked)}
        />
        {c.activateImmediately}
      </label>
      {state.activate &&
        !state.creatives.some((value) => value.status === "active") && (
          <p className="field-error">{c.activateRequired}</p>
        )}
      <ol className="creation-progress">
        <li>
          {c.checkpointCreate}:{" "}
          {state.journal.campaignId ? c.complete : c.pending}
        </li>
        <li>
          {c.checkpointTargeting}:{" "}
          {state.journal.targetingSaved ? c.complete : c.pending}
        </li>
        {state.creatives.map((item, index) => (
          <li key={index}>
            {c.checkpointCreative} — {item.name}:{" "}
            {state.journal.creativeIds[index] ? c.complete : c.pending}
          </li>
        ))}
        {state.activate && (
          <li>
            {c.checkpointActivate}:{" "}
            {state.journal.activated ? c.complete : c.pending}
          </li>
        )}
      </ol>
      {locked && <p className="field-help">{c.freezeHelp}</p>}
    </>
  );
}
