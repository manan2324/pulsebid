"use client";
import {
  useInfiniteQuery,
  useMutation,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import { toast } from "sonner";
import {
  advertisersApi,
  type AdvertiserCreate,
  type AdvertiserPatch,
} from "@/lib/api/advertisers";
import {
  biddersApi,
  type BidderCreate,
  type BidderPatch,
} from "@/lib/api/bidders";
import type { Advertiser, Bidder, Page } from "@/lib/api/types";
import { queryKeys } from "@/lib/query-keys";
import { catalogCopy as c } from "@/lib/copy/catalog";
export function nextCatalogCursor<T>(
  last: Page<T>,
  _pages: Page<T>[],
  _lastParam: string | null,
  params: (string | null)[],
) {
  return last.next_cursor && !params.includes(last.next_cursor)
    ? last.next_cursor
    : undefined;
}
export function useAdvertisers() {
  return useInfiniteQuery({
    queryKey: queryKeys.advertisers(),
    initialPageParam: null as string | null,
    queryFn: ({ pageParam, signal }) => advertisersApi.list(pageParam, signal),
    getNextPageParam: nextCatalogCursor,
    staleTime: 15000,
  });
}
export function useBidders() {
  return useInfiniteQuery({
    queryKey: queryKeys.bidders,
    initialPageParam: null as string | null,
    queryFn: ({ pageParam, signal }) => biddersApi.list(pageParam, signal),
    getNextPageParam: nextCatalogCursor,
    staleTime: 15000,
  });
}
export type AdvertiserAction =
  | { type: "create"; body: AdvertiserCreate }
  | { type: "update"; id: string; body: AdvertiserPatch }
  | { type: "archive"; id: string };
export type BidderAction =
  | { type: "create"; body: BidderCreate }
  | { type: "update"; id: string; body: BidderPatch }
  | { type: "delete"; id: string };
export function useAdvertiserMutation() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (action: AdvertiserAction): Promise<Advertiser | void> =>
      action.type === "create"
        ? advertisersApi.create(action.body)
        : action.type === "update"
          ? advertisersApi.update(action.id, action.body)
          : advertisersApi.archive(action.id),
    onSuccess: async (_data, action) => {
      toast.success(
        action.type === "create"
          ? c.advertiserCreated
          : action.type === "update"
            ? c.advertiserUpdated
            : c.advertiserArchived,
        { description: c.gatewayHint },
      );
      await Promise.all([
        client.invalidateQueries({ queryKey: ["advertisers"] }),
        client.invalidateQueries({ queryKey: ["campaigns"] }),
        client.invalidateQueries({ queryKey: queryKeys.snapshot }),
      ]);
    },
  });
}
export function useBidderMutation() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (action: BidderAction): Promise<Bidder | void> =>
      action.type === "create"
        ? biddersApi.create(action.body)
        : action.type === "update"
          ? biddersApi.update(action.id, action.body)
          : biddersApi.delete(action.id),
    onSuccess: async (_data, action) => {
      toast.success(
        action.type === "create"
          ? c.bidderCreated
          : action.type === "update"
            ? c.bidderUpdated
            : c.bidderDeleted,
        { description: c.gatewayHint },
      );
      await Promise.all([
        client.invalidateQueries({ queryKey: queryKeys.bidders }),
        client.invalidateQueries({ queryKey: ["campaigns"] }),
        client.invalidateQueries({ queryKey: queryKeys.snapshot }),
        client.invalidateQueries({ queryKey: ["bidder-health"] }),
      ]);
    },
  });
}
export function useBidderHealth(index: number | null) {
  return useQuery({
    queryKey: ["bidder-health", index],
    queryFn: ({ signal }) => biddersApi.health(index!, signal),
    enabled: index !== null,
    staleTime: 5000,
    refetchInterval: 10000,
    refetchIntervalInBackground: false,
    retry: false,
    meta: { quiet: true },
  });
}
