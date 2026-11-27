"use client";
import {
  useInfiniteQuery,
  useMutation,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import { toast } from "sonner";
import {
  allPages,
  campaignsApi,
  type CampaignCreate,
  type CampaignPatch,
  type CreativeCreate,
  type CreativePatch,
  type TargetingPut,
} from "@/lib/api/campaigns";
import { advertisersApi } from "@/lib/api/advertisers";
import { biddersApi } from "@/lib/api/bidders";

import { nextCatalogCursor } from "./use-catalog";
import { campaignCopy as c } from "@/lib/copy/campaign";
import { catalogCopy } from "@/lib/copy/catalog";
export function useCampaigns(advertiser_id: string) {
  return useInfiniteQuery({
    queryKey: ["campaigns", { advertiser_id }],
    initialPageParam: null as string | null,
    queryFn: ({ pageParam, signal }) =>
      campaignsApi.campaigns(
        {
          advertiser_id: advertiser_id || undefined,
          limit: 20,
          cursor: pageParam ?? undefined,
        },
        signal,
      ),
    getNextPageParam: nextCatalogCursor,
    staleTime: 15000,
  });
}
export function useCatalogOptions() {
  const advertisers = useQuery({
    queryKey: ["advertiser-options"],
    queryFn: () => allPages((cursor) => advertisersApi.list(cursor)),
    staleTime: 15000,
  });
  const bidders = useQuery({
    queryKey: ["bidder-options"],
    queryFn: () => allPages((cursor) => biddersApi.list(cursor)),
    staleTime: 15000,
  });
  return { advertisers, bidders };
}
export function useCampaign(id: string) {
  return useQuery({
    queryKey: ["campaign", id],
    queryFn: ({ signal }) => campaignsApi.get(id, signal),
    staleTime: 15000,
  });
}
export function useTargeting(id: string) {
  return useQuery({
    queryKey: ["targeting", id],
    queryFn: ({ signal }) => campaignsApi.targeting(id, signal),
    staleTime: 15000,
  });
}
export function useCreatives(id: string, enabled = true) {
  return useQuery({
    queryKey: ["creatives", id],
    enabled,
    queryFn: () => allPages((cursor) => campaignsApi.creatives(id, cursor)),
    staleTime: 15000,
  });
}
export function useBudget(id: string) {
  return useQuery({
    queryKey: ["budget", id],
    queryFn: ({ signal }) => campaignsApi.budget(id, signal),
    staleTime: 10000,
    refetchInterval: 10000,
    refetchIntervalInBackground: false,
    meta: { quiet: true },
  });
}
export type CampaignAction =
  | { type: "create"; body: CampaignCreate }
  | { type: "update"; id: string; body: CampaignPatch }
  | { type: "activate" | "pause" | "archive" | "reset"; id: string }
  | { type: "targeting"; id: string; body: TargetingPut }
  | { type: "creative-create"; id: string; body: CreativeCreate }
  | { type: "creative-update"; id: string; body: CreativePatch }
  | { type: "creative-delete"; id: string };
export function useCampaignMutation() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: async (action: CampaignAction): Promise<unknown> => {
      switch (action.type) {
        case "create":
          return campaignsApi.create(action.body);
        case "update":
          return campaignsApi.update(action.id, action.body);
        case "activate":
        case "pause":
          return campaignsApi.transition(action.id, action.type);
        case "archive":
          return campaignsApi.archive(action.id);
        case "reset":
          return campaignsApi.resetBudget(action.id);
        case "targeting":
          return campaignsApi.saveTargeting(action.id, action.body);
        case "creative-create":
          return campaignsApi.createCreative(action.id, action.body);
        case "creative-update":
          return campaignsApi.updateCreative(action.id, action.body);
        case "creative-delete":
          return campaignsApi.deleteCreative(action.id);
      }
    },
    onSuccess: async () => {
      toast.success(c.saved, { description: catalogCopy.gatewayHint });
      await Promise.all(
        [
          "campaigns",
          "campaign-options",
          "campaign",
          "targeting",
          "creatives",
          "budget",
          "snapshot",
          "bidder-options",
          "advertiser-options",
        ].map((key) => client.invalidateQueries({ queryKey: [key] })),
      );
    },
  });
}
