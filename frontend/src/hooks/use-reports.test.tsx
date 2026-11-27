// @vitest-environment jsdom
import { cleanup, renderHook, waitFor, act } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, afterEach, it, expect, vi } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "@/mocks/server";
import {
  reportFixture,
  reportCampaignId,
  emptyReport,
} from "@/mocks/fixtures/reporting";
import { useReport } from "./use-reports";
import type { ReportRange } from "@/lib/reportRange";
let client: QueryClient;
beforeEach(() => {
  client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  });
  const fetcher = globalThis.fetch;
  vi.stubGlobal("fetch", (input: RequestInfo | URL, init?: RequestInit) =>
    fetcher(
      typeof input === "string" && input.startsWith("/")
        ? `http://localhost${input}`
        : input,
      init,
    ),
  );
});
afterEach(() => {
  cleanup();
  client.clear();
  vi.unstubAllGlobals();
});
function Wrapper({ children }: { children: React.ReactNode }) {
  return <QueryClientProvider client={client}>{children}</QueryClientProvider>;
}
const range: ReportRange = {
  from: "2026-11-25T00:00:00Z",
  to: "2026-11-26T00:00:00Z",
  granularity: "hour",
  campaignIds: [reportCampaignId, "019aab00-0000-7000-8000-000000000002"],
};
it("loads hour/day/total MSW contracts and repeats campaign_id filters", async () => {
  const calls: string[] = [];
  server.use(
    http.get(
      "http://localhost/api/reporting/reports/campaigns",
      ({ request }) => {
        const url = new URL(request.url);
        expect(url.searchParams.getAll("campaign_id")).toEqual(
          range.campaignIds,
        );
        expect(url.searchParams.get("from")).toBe(range.from);
        const granularity = url.searchParams.get(
          "granularity",
        ) as ReportRange["granularity"];
        calls.push(granularity);
        return HttpResponse.json(reportFixture(granularity));
      },
    ),
  );
  const { result, rerender } = renderHook(({ value }) => useReport(value), {
    wrapper: Wrapper,
    initialProps: { value: range },
  });
  await waitFor(() => expect(result.current.isSuccess).toBe(true));
  expect(result.current.data?.totals.impressions).toBe(100);
  for (const granularity of ["day", "total"] as const) {
    rerender({ value: { ...range, granularity } });
    await waitFor(() =>
      expect(result.current.data?.granularity).toBe(granularity),
    );
  }
  expect(calls).toEqual(["hour", "day", "total"]);
});
it("does not fetch invalid ranges and renders a real zero response as success", async () => {
  let calls = 0;
  server.use(
    http.get("http://localhost/api/reporting/reports/campaigns", () => {
      calls++;
      return HttpResponse.json(emptyReport);
    }),
  );
  const { result, rerender } = renderHook(({ value }) => useReport(value), {
    wrapper: Wrapper,
    initialProps: { value: { ...range, to: range.from } },
  });
  await act(async () => {});
  expect(result.current.fetchStatus).toBe("idle");
  expect(calls).toBe(0);
  rerender({ value: range });
  await waitFor(() => expect(result.current.isSuccess).toBe(true));
  expect(result.current.data?.data).toEqual([]);
  expect(result.current.data?.totals.spend).toBe(0);
});
it("preserves error request IDs for malformed successful responses", async () => {
  server.use(
    http.get("http://localhost/api/reporting/reports/campaigns", () =>
      HttpResponse.json(
        { invalid: true },
        { headers: { "X-Request-Id": "bad-report-123" } },
      ),
    ),
  );
  const { result } = renderHook(() => useReport(range), { wrapper: Wrapper });
  await waitFor(() => expect(result.current.isError).toBe(true));
  expect(result.current.error).toMatchObject({
    code: "invalid_response",
    requestId: "bad-report-123",
  });
});
