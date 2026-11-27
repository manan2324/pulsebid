"use client";
import { useQuery } from "@tanstack/react-query";
import { prometheusApi, type PromName } from "@/lib/api/prometheus";
export function usePrometheus(name: PromName, range?: "15m" | "1h" | "6h") {
  return useQuery({
    queryKey: ["prom", name, range ?? "instant"],
    queryFn: ({ signal }) => prometheusApi.query(name, range, signal),
    staleTime: 2000,
    refetchInterval: 5000,
    refetchIntervalInBackground: false,
    retry: false,
    meta: { quiet: true },
  });
}
