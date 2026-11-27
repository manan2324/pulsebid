import type { CreationJournal } from "./campaign-journal";
export { journalSchema, emptyJournal } from "./campaign-journal";
export type { CreationJournal } from "./campaign-journal";
import { campaignsApi, allPages } from "./api/campaigns";
import { ApiClientError } from "./api/client";
import { campaignPayload, type CampaignFormValues } from "./schemas/campaign";
import { creativePayload, type CreativeValues } from "./schemas/creative";
import type { TargetingValues } from "./schemas/targeting";
import { campaignCopy as c } from "./copy/campaign";
export interface CampaignDraft {
  values: CampaignFormValues;
  targeting: TargetingValues;
  creatives: CreativeValues[];
  activate: boolean;
}
const uncertain = (error: unknown) =>
  !(error instanceof ApiClientError) || error.code === "unavailable";
export async function runCampaignCreation(
  draft: CampaignDraft,
  initial: CreationJournal,
  onCheckpoint: (state: CreationJournal) => void,
  client = campaignsApi,
) {
  let state = { ...initial, creativeIds: { ...initial.creativeIds } };
  const commit = (patch: Partial<CreationJournal>) => {
    state = { ...state, ...patch };
    onCheckpoint(state);
  };
  if (!state.campaignId) {
    const existing = await allPages((cursor) =>
      client.campaigns({
        advertiser_id: draft.values.advertiser_id,
        limit: 200,
        cursor: cursor ?? undefined,
      }),
    );
    const matches = existing.filter(
      (item) =>
        item.name === draft.values.name &&
        item.bidder_id === draft.values.bidder_id,
    );
    if (state.pending?.kind === "campaign") {
      const recovered = matches.filter(
        (item) => !state.pending?.priorIds.includes(item.id),
      );
      if (recovered.length !== 1) throw new Error(c.uncertain);
      commit({ campaignId: recovered[0]!.id, pending: null });
    } else {
      commit({
        pending: {
          kind: "campaign",
          index: 0,
          priorIds: matches.map((item) => item.id),
        },
      });
      try {
        const campaign = await client.create(campaignPayload(draft.values));
        commit({ campaignId: campaign.id, pending: null });
      } catch (error) {
        if (!uncertain(error)) commit({ pending: null });
        throw error;
      }
    }
  }
  const id = state.campaignId!;
  if (!state.targetingSaved) {
    await client.saveTargeting(id, draft.targeting);
    commit({ targetingSaved: true });
  }
  for (const [index, creative] of draft.creatives.entries()) {
    if (state.creativeIds[index]) continue;
    const body = creativePayload(creative);
    const existing = await allPages((cursor) => client.creatives(id, cursor));
    if (state.pending?.kind === "creative" && state.pending.index === index) {
      const found = existing.filter(
        (item) =>
          !state.pending?.priorIds.includes(item.id) &&
          item.name === body.name &&
          item.markup === body.markup &&
          item.format === body.format &&
          item.landing_url === body.landing_url &&
          item.width === body.width &&
          item.height === body.height,
      );
      if (found.length !== 1) throw new Error(c.uncertain);
      commit({
        creativeIds: { ...state.creativeIds, [index]: found[0]!.id },
        pending: null,
      });
    } else {
      commit({
        pending: {
          kind: "creative",
          index,
          priorIds: existing.map((item) => item.id),
        },
      });
      try {
        const result = await client.createCreative(id, body);
        commit({
          creativeIds: { ...state.creativeIds, [index]: result.id },
          pending: null,
        });
      } catch (error) {
        if (!uncertain(error)) commit({ pending: null });
        throw error;
      }
    }
  }
  if (draft.activate && !state.activated) {
    await client.transition(id, "activate");
    commit({ activated: true });
  }
  return client.get(id);
}
