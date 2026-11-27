import { api, apiRequest } from "./client";
import { bidderConfigSchema, type BidderConfig } from "@/lib/lab";
export const labApi = {
  config: (index: number, signal?: AbortSignal) =>
    api.get(`/api/bidder/${index}/config`, signal, (value) =>
      bidderConfigSchema.parse(value),
    ),
  update: (index: number, patch: Partial<BidderConfig>) =>
    apiRequest(
      `/api/bidder/${index}/config`,
      { method: "POST", body: JSON.stringify(patch) },
      (value) => bidderConfigSchema.parse(value),
    ),
};
