import { api, apiRequest, queryString } from "./client";
import type { Advertiser, Page } from "./types";
import type { components } from "./generated/campaign-service";
export type AdvertiserCreate = components["schemas"]["AdvertiserCreate"];
export type AdvertiserPatch = components["schemas"]["AdvertiserPatch"];
export const advertisersApi = {
  list: (cursor: string | null, signal?: AbortSignal) =>
    api.get<Page<Advertiser>>(
      `/api/campaign/advertisers${queryString({ limit: 20, cursor })}`,
      signal,
    ),
  create: (body: AdvertiserCreate) =>
    api.post<Advertiser>("/api/campaign/advertisers", body),
  update: (id: string, body: AdvertiserPatch) =>
    apiRequest<Advertiser>(
      `/api/campaign/advertisers/${encodeURIComponent(id)}`,
      { method: "PATCH", body: JSON.stringify(body) },
    ),
  archive: (id: string) =>
    apiRequest<void>(`/api/campaign/advertisers/${encodeURIComponent(id)}`, {
      method: "DELETE",
    }),
};
