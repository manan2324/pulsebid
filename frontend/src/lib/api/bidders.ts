import { api, apiRequest, queryString } from "./client";
import type { Bidder, Page } from "./types";
import type { components } from "./generated/campaign-service";
export type BidderCreate = components["schemas"]["BidderCreate"];
// The backend PATCH contract is partial; OpenAPI defaults make enabled appear required in generated types.
export type BidderPatch = Partial<components["schemas"]["BidderPatch"]>;
export const biddersApi = {
  list: (cursor: string | null, signal?: AbortSignal) =>
    api.get<Page<Bidder>>(
      `/api/campaign/bidders${queryString({ limit: 20, cursor })}`,
      signal,
    ),
  create: (body: BidderCreate) =>
    api.post<Bidder>("/api/campaign/bidders", body),
  update: (id: string, body: BidderPatch) =>
    apiRequest<Bidder>(`/api/campaign/bidders/${encodeURIComponent(id)}`, {
      method: "PATCH",
      body: JSON.stringify(body),
    }),
  delete: (id: string) =>
    apiRequest<void>(`/api/campaign/bidders/${encodeURIComponent(id)}`, {
      method: "DELETE",
    }),
  health: (index: number, signal?: AbortSignal) =>
    api.get(`/api/bidder/${index}/health`, signal, () => true),
};
