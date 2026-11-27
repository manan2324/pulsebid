import { api, apiRequest, queryString } from "./client";
import type {
  Advertiser,
  Bidder,
  Campaign,
  Creative,
  Targeting,
  BudgetView,
  Page,
} from "./types";
import type { components } from "./generated/campaign-service";
export type CampaignCreate = components["schemas"]["CampaignCreate"];
export type CampaignPatch = Partial<components["schemas"]["CampaignPatch"]>;
export type CreativeCreate = components["schemas"]["CreativeCreate"];
export type CreativePatch = Partial<components["schemas"]["CreativePatch"]>;
export type TargetingPut = components["schemas"]["TargetingPut"];
export type Snapshot = components["schemas"]["CatalogSnapshot"];
export type CampaignFilters = {
  advertiser_id?: string;
  status?: Campaign["status"];
  cursor?: string;
  limit?: number;
};
const campaignPath = (id: string) =>
  `/api/campaign/campaigns/${encodeURIComponent(id)}`;
export async function allPages<T>(
  load: (cursor: string | null) => Promise<Page<T>>,
) {
  const data: T[] = [];
  let cursor: string | null = null;
  const seen = new Set<string>();
  do {
    const page = await load(cursor);
    data.push(...page.data);
    cursor = page.next_cursor;
    if (cursor && seen.has(cursor)) throw new Error("Repeated catalog cursor");
    if (cursor) seen.add(cursor);
  } while (cursor);
  return data;
}
export const campaignsApi = {
  advertisers: (
    params: { limit?: number; cursor?: string } = {},
    signal?: AbortSignal,
  ) =>
    api.get<Page<Advertiser>>(
      `/api/campaign/advertisers${queryString(params)}`,
      signal,
    ),
  bidders: (signal?: AbortSignal) =>
    api.get<Page<Bidder>>("/api/campaign/bidders", signal),
  campaigns: (params: CampaignFilters = {}, signal?: AbortSignal) =>
    api.get<Page<Campaign>>(
      `/api/campaign/campaigns${queryString(params)}`,
      signal,
    ),
  get: (id: string, signal?: AbortSignal) =>
    api.get<Campaign>(campaignPath(id), signal),
  create: (body: CampaignCreate) =>
    api.post<Campaign>("/api/campaign/campaigns", body),
  update: (id: string, body: CampaignPatch) =>
    apiRequest<Campaign>(campaignPath(id), {
      method: "PATCH",
      body: JSON.stringify(body),
    }),
  transition: (id: string, action: "activate" | "pause") =>
    api.post<Campaign>(`${campaignPath(id)}/${action}`, {}),
  archive: (id: string) =>
    apiRequest<void>(campaignPath(id), { method: "DELETE" }),
  targeting: (id: string, signal?: AbortSignal) =>
    api.get<Targeting>(`${campaignPath(id)}/targeting`, signal),
  saveTargeting: (id: string, body: TargetingPut) =>
    apiRequest<Targeting>(`${campaignPath(id)}/targeting`, {
      method: "PUT",
      body: JSON.stringify(body),
    }),
  creatives: (id: string, cursor: string | null = null, signal?: AbortSignal) =>
    api.get<Page<Creative>>(
      `${campaignPath(id)}/creatives${queryString({ limit: 200, cursor })}`,
      signal,
    ),
  createCreative: (id: string, body: CreativeCreate) =>
    api.post<Creative>(`${campaignPath(id)}/creatives`, body),
  updateCreative: (id: string, body: CreativePatch) =>
    apiRequest<Creative>(`/api/campaign/creatives/${encodeURIComponent(id)}`, {
      method: "PATCH",
      body: JSON.stringify(body),
    }),
  deleteCreative: (id: string) =>
    apiRequest<void>(`/api/campaign/creatives/${encodeURIComponent(id)}`, {
      method: "DELETE",
    }),
  budget: (id: string, signal?: AbortSignal) =>
    api.get<BudgetView>(`${campaignPath(id)}/budget`, signal),
  resetBudget: (id: string) =>
    api.post<BudgetView>(`${campaignPath(id)}/budget/reset`, {}),
  snapshot: (signal?: AbortSignal) =>
    api.get<Snapshot>("/api/campaign/snapshot", signal),
};
