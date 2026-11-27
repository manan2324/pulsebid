// @vitest-environment jsdom
import { act, cleanup, renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "@/mocks/server";
import { advertiserFixture, bidderFixture } from "@/mocks/fixtures/catalog";
import { useAdvertisers, useBidders } from "./use-catalog";
let client: QueryClient;
beforeEach(() => {
  client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  });
  const interceptedFetch = globalThis.fetch;
  vi.stubGlobal("fetch", (input: RequestInfo | URL, init?: RequestInit) =>
    interceptedFetch(
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
it("loads advertiser cursors with repeated symbols and stops at null", async () => {
  const cursors: (string | null)[] = [];
  server.use(
    http.get("http://localhost/api/campaign/advertisers", ({ request }) => {
      const url = new URL(request.url);
      expect(url.searchParams.get("limit")).toBe("20");
      const cursor = url.searchParams.get("cursor");
      cursors.push(cursor);
      return HttpResponse.json(
        cursor
          ? {
              data: [{ ...advertiserFixture, id: "second", name: "Second" }],
              next_cursor: null,
            }
          : { data: [advertiserFixture], next_cursor: "opaque+a/b=" },
      );
    }),
  );
  const { result } = renderHook(() => useAdvertisers(), { wrapper: Wrapper });
  await waitFor(() => expect(result.current.isSuccess).toBe(true));
  expect(result.current.hasNextPage).toBe(true);
  await act(async () => {
    await result.current.fetchNextPage();
  });
  expect(cursors).toEqual([null, "opaque+a/b="]);
  await waitFor(() =>
    expect(
      result.current.data?.pages.flatMap((page) => page.data),
    ).toHaveLength(2),
  );
  expect(result.current.hasNextPage).toBe(false);
});
it("loads bidder pages and rejects repeated cursors", async () => {
  server.use(
    http.get("http://localhost/api/campaign/bidders", ({ request }) =>
      HttpResponse.json({
        data: [
          {
            ...bidderFixture,
            id: new URL(request.url).searchParams.has("cursor")
              ? "second-dsp"
              : "first-dsp",
          },
        ],
        next_cursor: "same-cursor",
      }),
    ),
  );
  const { result } = renderHook(() => useBidders(), { wrapper: Wrapper });
  await waitFor(() => expect(result.current.isSuccess).toBe(true));
  expect(result.current.hasNextPage).toBe(true);
  await act(async () => {
    const fetched = await result.current.fetchNextPage();
    expect(fetched.error).toBeNull();
  });
  await waitFor(() => expect(result.current.data?.pages).toHaveLength(2));
  expect(result.current.hasNextPage).toBe(false);
});
it("retains loaded advertisers when the next page fails", async () => {
  server.use(
    http.get("http://localhost/api/campaign/advertisers", ({ request }) =>
      new URL(request.url).searchParams.has("cursor")
        ? HttpResponse.json(
            {
              error: {
                code: "unavailable",
                message: "Catalog unavailable",
                request_id: "page-failed",
              },
            },
            { status: 503 },
          )
        : HttpResponse.json({ data: [advertiserFixture], next_cursor: "next" }),
    ),
  );
  const { result } = renderHook(() => useAdvertisers(), { wrapper: Wrapper });
  await waitFor(() => expect(result.current.isSuccess).toBe(true));
  await act(async () => {
    await result.current.fetchNextPage();
  });
  expect(result.current.data?.pages[0]?.data).toEqual([advertiserFixture]);
  await waitFor(() =>
    expect(result.current.error).toMatchObject({
      requestId: "page-failed",
      status: 503,
    }),
  );
});
