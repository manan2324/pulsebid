"use client";
import { useEffect, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { reportingApi } from "@/lib/api/reporting";
import { allPages, campaignsApi } from "@/lib/api/campaigns";
import { validateReportRange, type ReportRange } from "@/lib/reportRange";
export function useReport(range: ReportRange | null) {
  return useQuery({
    queryKey: ["report", range],
    enabled: Boolean(range && !validateReportRange(range)),
    queryFn: ({ signal }) => reportingApi.list(range!, signal),
    staleTime: 10000,
    refetchInterval: 30000,
    refetchIntervalInBackground: false,
    gcTime: 60000,
    meta: { quiet: true },
  });
}
export function useReportCatalog(enabled = true) {
  return useQuery({
    enabled,
    queryKey: ["campaign-options"],
    queryFn: ({ signal }) =>
      allPages((cursor) =>
        campaignsApi.campaigns(
          { limit: 200, cursor: cursor ?? undefined },
          signal,
        ),
      ),
    staleTime: 15000,
    meta: { quiet: true },
  });
}
export function useReportClock() {
  const [now, setNow] = useState(() => new Date());
  useEffect(() => {
    const refresh = () => {
      if (document.visibilityState !== "hidden") setNow(new Date());
    };
    const timer = window.setInterval(refresh, 30000);
    document.addEventListener("visibilitychange", refresh);
    return () => {
      window.clearInterval(timer);
      document.removeEventListener("visibilitychange", refresh);
    };
  }, []);
  return now;
}
