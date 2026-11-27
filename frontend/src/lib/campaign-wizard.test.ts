import { expect, it, vi } from "vitest";
import {
  runCampaignCreation,
  emptyJournal,
  type CreationJournal,
  type CampaignDraft,
} from "./campaign-wizard";
import { campaignsApi } from "./api/campaigns";
import { ApiClientError } from "./api/client";
import { starterCreative, creativePayload } from "./schemas/creative";
import { emptyTargeting } from "./schemas/targeting";
import type { Campaign, Creative } from "./api/types";
const campaign: Campaign = {
  id: "019aab00-0000-7000-8000-000000000002",
  name: "Test campaign",
  advertiser_id: "019aab00-0000-7000-8000-000000000001",
  bidder_id: "bidder-1",
  daily_budget: 50,
  max_bid_cpm: 3,
  pacing_mode: "even",
  start_at: null,
  end_at: null,
  status: "active",
  created_at: "2026-11-24T00:00:00Z",
  updated_at: "2026-11-24T00:00:00Z",
};
const creative = (id: string, name: string): Creative => ({
  ...creativePayload({ ...starterCreative, name }),
  id,
  campaign_id: campaign.id,
  created_at: campaign.created_at,
  updated_at: campaign.updated_at,
});
const draft: CampaignDraft = {
  values: {
    name: campaign.name,
    advertiser_id: campaign.advertiser_id,
    bidder_id: campaign.bidder_id,
    daily_budget: "50",
    max_bid_cpm: "3",
    pacing_mode: "even",
    start_at: "",
    end_at: "",
  },
  targeting: emptyTargeting,
  creatives: [
    { ...starterCreative, name: "First" },
    { ...starterCreative, name: "Second" },
  ],
  activate: true,
};
function client() {
  return {
    ...campaignsApi,
    campaigns: vi
      .fn<typeof campaignsApi.campaigns>()
      .mockResolvedValue({ data: [], next_cursor: null }),
    create: vi.fn<typeof campaignsApi.create>().mockResolvedValue(campaign),
    saveTargeting: vi
      .fn<typeof campaignsApi.saveTargeting>()
      .mockResolvedValue({
        ...emptyTargeting,
        campaign_id: campaign.id,
        updated_at: campaign.updated_at,
      }),
    creatives: vi
      .fn<typeof campaignsApi.creatives>()
      .mockResolvedValue({ data: [], next_cursor: null }),
    createCreative: vi.fn<typeof campaignsApi.createCreative>(),
    transition: vi
      .fn<typeof campaignsApi.transition>()
      .mockResolvedValue(campaign),
    get: vi.fn<typeof campaignsApi.get>().mockResolvedValue(campaign),
  };
}
it("retries a failed creative step without creating another campaign or first creative", async () => {
  const api = client();
  api.createCreative
    .mockResolvedValueOnce(creative("first-id", "First"))
    .mockRejectedValueOnce(
      new ApiClientError(
        "internal",
        "Database failure",
        [],
        "failed-step-3",
        500,
      ),
    )
    .mockResolvedValueOnce(creative("second-id", "Second"));
  let journal: CreationJournal = emptyJournal;
  const checkpoint = (next: CreationJournal) => {
    journal = next;
  };
  await expect(
    runCampaignCreation(draft, journal, checkpoint, api),
  ).rejects.toMatchObject({ requestId: "failed-step-3" });
  expect(journal.campaignId).toBe(campaign.id);
  expect(journal.creativeIds[0]).toBe("first-id");
  await runCampaignCreation(draft, journal, checkpoint, api);
  expect(api.create).toHaveBeenCalledTimes(1);
  expect(api.saveTargeting).toHaveBeenCalledTimes(1);
  expect(api.createCreative).toHaveBeenCalledTimes(3);
  expect(api.transition).toHaveBeenCalledTimes(1);
});
it("reconciles an ambiguous creation reply before retrying", async () => {
  const api = client();
  api.create.mockRejectedValueOnce(
    new ApiClientError("unavailable", "Network timeout", [], "unknown", 503),
  );
  let journal: CreationJournal = emptyJournal;
  const checkpoint = (next: CreationJournal) => {
    journal = next;
  };
  await expect(
    runCampaignCreation(draft, journal, checkpoint, api),
  ).rejects.toBeInstanceOf(ApiClientError);
  api.campaigns.mockResolvedValue({ data: [campaign], next_cursor: null });
  api.createCreative
    .mockResolvedValueOnce(creative("first", "First"))
    .mockResolvedValueOnce(creative("second", "Second"));
  await runCampaignCreation(draft, journal, checkpoint, api);
  expect(api.create).toHaveBeenCalledTimes(1);
  expect(journal.campaignId).toBe(campaign.id);
});
it("does not blindly repeat an unconfirmed POST", async () => {
  const api = client();
  api.create.mockRejectedValue(
    new ApiClientError("unavailable", "Timeout", [], "unknown", 503),
  );
  let journal: CreationJournal = emptyJournal;
  const checkpoint = (next: CreationJournal) => {
    journal = next;
  };
  await expect(
    runCampaignCreation(draft, journal, checkpoint, api),
  ).rejects.toBeInstanceOf(ApiClientError);
  await expect(
    runCampaignCreation(draft, journal, checkpoint, api),
  ).rejects.toThrow("could not be confirmed");
  expect(api.create).toHaveBeenCalledTimes(1);
});
