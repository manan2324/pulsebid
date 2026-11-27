"use client";
import { useEffect } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import type { Campaign } from "@/lib/api/types";
import {
  campaignSchema,
  campaignPayload,
  type CampaignFormValues,
} from "@/lib/schemas/campaign";
import { useCampaignMutation } from "@/hooks/use-campaigns";
import { useDirtyGuard } from "@/hooks/use-dirty-guard";
import { applyApiErrors } from "@/lib/api/apply-api-errors";
import { CampaignFields } from "./campaign-fields";
import { CopyButton } from "@/components/data/shared";
import { Button } from "@/components/ui/button";
import { formatTime } from "@/lib/time";
import { useTimeMode } from "@/components/providers";
import { campaignCopy as c } from "@/lib/copy/campaign";
export function CampaignSettings({
  campaign,
  onDirty,
}: {
  campaign: Campaign;
  onDirty: (dirty: boolean) => void;
}) {
  const form = useForm<CampaignFormValues>({
    resolver: zodResolver(campaignSchema),
    defaultValues: {
      name: campaign.name,
      advertiser_id: campaign.advertiser_id,
      bidder_id: campaign.bidder_id,
      daily_budget: String(campaign.daily_budget),
      max_bid_cpm: String(campaign.max_bid_cpm),
      pacing_mode: campaign.pacing_mode,
      start_at: campaign.start_at?.slice(0, 16) ?? "",
      end_at: campaign.end_at?.slice(0, 16) ?? "",
    },
  });
  const mutation = useCampaignMutation();
  const { mode } = useTimeMode();
  const { isDirty, dirtyFields, errors } = form.formState;
  useDirtyGuard(isDirty);
  useEffect(() => {
    onDirty(isDirty);
    return () => onDirty(false);
  }, [isDirty, onDirty]);
  return (
    <section className="panel detail-panel">
      <form
        noValidate
        onSubmit={form.handleSubmit(async (values) => {
          const payload = campaignPayload(values);
          try {
            await mutation.mutateAsync({
              type: "update",
              id: campaign.id,
              body: {
                ...(dirtyFields.name ? { name: payload.name } : {}),
                ...(dirtyFields.daily_budget
                  ? { daily_budget: payload.daily_budget }
                  : {}),
                ...(dirtyFields.max_bid_cpm
                  ? { max_bid_cpm: payload.max_bid_cpm }
                  : {}),
                ...(dirtyFields.pacing_mode
                  ? { pacing_mode: payload.pacing_mode }
                  : {}),
                ...(dirtyFields.start_at ? { start_at: payload.start_at } : {}),
                ...(dirtyFields.end_at ? { end_at: payload.end_at } : {}),
              },
            });
            form.reset(values);
          } catch (error) {
            applyApiErrors(form, error, [
              "name",
              "daily_budget",
              "max_bid_cpm",
              "pacing_mode",
              "start_at",
              "end_at",
            ]);
          }
        })}
      >
        <CampaignFields
          form={form}
          partners={false}
          disabled={campaign.status === "archived" || mutation.isPending}
        />
        {isDirty && <p className="notice">{c.unsaved}</p>}
        {errors.root?.server?.message && (
          <p role="alert" className="field-error">
            {errors.root.server.message}
          </p>
        )}
        <Button
          disabled={
            !isDirty || mutation.isPending || campaign.status === "archived"
          }
        >
          {mutation.isPending ? c.saving : c.save}
        </Button>
      </form>
      <dl className="metadata-grid">
        <div>
          <dt>{c.id}</dt>
          <dd>
            <code>{campaign.id}</code>
            <CopyButton value={campaign.id} />
          </dd>
        </div>
        <div>
          <dt>{c.advertiser}</dt>
          <dd>
            <code>{campaign.advertiser_id}</code>
            <CopyButton value={campaign.advertiser_id} />
          </dd>
        </div>
        <div>
          <dt>{c.bidder}</dt>
          <dd>
            <code>{campaign.bidder_id}</code>
          </dd>
        </div>
        <div>
          <dt>{c.createdAt}</dt>
          <dd>{formatTime(campaign.created_at, mode)}</dd>
        </div>
        <div>
          <dt>{c.updatedAt}</dt>
          <dd>{formatTime(campaign.updated_at, mode)}</dd>
        </div>
      </dl>
      <details>
        <summary>{c.raw}</summary>
        <pre className="raw-json">{JSON.stringify(campaign, null, 2)}</pre>
      </details>
    </section>
  );
}
