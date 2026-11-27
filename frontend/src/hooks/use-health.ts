"use client";
import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api/client";
import { healthSchema } from "@/lib/schemas/health";
import { queryKeys } from "@/lib/query-keys";
export function useHealth() {
  return useQuery({
    queryKey: queryKeys.health,
    queryFn: ({ signal }) =>
      api.get("/api/health", signal, (value) => healthSchema.parse(value)),
    staleTime: 5000,
    refetchInterval: 10000,
    refetchIntervalInBackground: false,
  });
}
