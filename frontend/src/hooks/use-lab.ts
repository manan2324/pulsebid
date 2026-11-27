"use client";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { labApi } from "@/lib/api/lab";
import type { BidderConfig } from "@/lib/lab";
export function useBidderConfig(index: number) {
  return useQuery({
    queryKey: ["lab-config", index],
    queryFn: ({ signal }) => labApi.config(index, signal),
    staleTime: 5000,
    meta: { quiet: true },
  });
}
export function useBidderUpdate(index: number) {
  const client = useQueryClient();
  const key = ["lab-config", index];
  return useMutation({
    mutationFn: (patch: Partial<BidderConfig>) => labApi.update(index, patch),
    retry: false,
    onMutate: async (patch) => {
      await client.cancelQueries({ queryKey: key });
      const previous = client.getQueryData<BidderConfig>(key);
      if (previous) client.setQueryData(key, { ...previous, ...patch });
      return { previous };
    },
    onError: (_error, _patch, context) => {
      if (context?.previous) client.setQueryData(key, context.previous);
    },
    onSuccess: (config) => client.setQueryData(key, config),
    onSettled: () => {
      void client.invalidateQueries({ queryKey: key });
    },
  });
}
