"use client";
import type { UseFormReturn } from "react-hook-form";
import { Input } from "@/components/ui/input";
import { campaignCopy as c } from "@/lib/copy/campaign";
import type { CampaignFormValues } from "@/lib/schemas/campaign";
import type { Advertiser, Bidder } from "@/lib/api/types";
export function CampaignFields({
  form,
  section = "all",
  advertisers = [],
  bidders = [],
  partners = true,
  disabled = false,
}: {
  form: UseFormReturn<CampaignFormValues>;
  section?: "basics" | "budget" | "all";
  advertisers?: Advertiser[];
  bidders?: Bidder[];
  partners?: boolean;
  disabled?: boolean;
}) {
  const {
    register,
    formState: { errors },
  } = form;
  const field = (
    key: "name" | "daily_budget" | "max_bid_cpm" | "start_at" | "end_at",
    label: string,
    type = "text",
  ) => (
    <div className="form-field">
      <label htmlFor={`campaign-${key}`}>{label}</label>
      <Input
        id={`campaign-${key}`}
        type={type}
        {...register(key)}
        aria-invalid={Boolean(errors[key])}
        aria-describedby={errors[key] ? `${key}-error` : undefined}
      />
      {errors[key] && (
        <p id={`${key}-error`} className="field-error" role="alert">
          {errors[key]?.message}
        </p>
      )}
    </div>
  );
  return (
    <fieldset disabled={disabled} className="campaign-fields">
      {section !== "budget" && (
        <>
          {field("name", c.name)}
          {partners && (
            <div className="field-columns">
              <label className="form-field">
                {c.advertiser}
                <select
                  aria-label={c.advertiser}
                  {...register("advertiser_id")}
                  aria-invalid={Boolean(errors.advertiser_id)}
                >
                  <option value="">{c.selectAdvertiser}</option>
                  {advertisers.map((item) => (
                    <option
                      key={item.id}
                      value={item.id}
                      disabled={item.status !== "active"}
                    >
                      {item.name}
                      {item.status === "archived" ? ` (${c.archived})` : ""}
                    </option>
                  ))}
                </select>
                {errors.advertiser_id && (
                  <span role="alert" className="field-error">
                    {errors.advertiser_id.message}
                  </span>
                )}
              </label>
              <label className="form-field">
                {c.bidder}
                <select
                  aria-label={c.bidder}
                  {...register("bidder_id")}
                  aria-invalid={Boolean(errors.bidder_id)}
                >
                  <option value="">{c.selectBidder}</option>
                  {bidders.map((item) => (
                    <option
                      key={item.id}
                      value={item.id}
                      disabled={!item.enabled}
                    >
                      {item.name}
                      {!item.enabled ? ` (${c.paused})` : ""}
                    </option>
                  ))}
                </select>
                {errors.bidder_id && (
                  <span role="alert" className="field-error">
                    {errors.bidder_id.message}
                  </span>
                )}
              </label>
            </div>
          )}
          <div className="field-columns">
            {field("start_at", c.start, "datetime-local")}
            {field("end_at", c.end, "datetime-local")}
          </div>
          <p className="field-help">{c.scheduleHelp}</p>
        </>
      )}
      {section !== "basics" && (
        <>
          <div className="field-columns">
            {field("daily_budget", c.budget)}
            {field("max_bid_cpm", c.maxBid)}
          </div>
          <p className="field-help">{c.moneyHelp}</p>
          <label className="form-field">
            {c.pacing}
            <select {...register("pacing_mode")}>
              <option value="even">{c.even}</option>
              <option value="asap">{c.asap}</option>
            </select>
          </label>
          <p className="field-help">
            {c.evenHelp} {c.asapHelp}
          </p>
        </>
      )}
    </fieldset>
  );
}
