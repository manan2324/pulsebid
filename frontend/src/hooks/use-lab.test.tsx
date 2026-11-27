// @vitest-environment jsdom
import { act, cleanup, renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, expect, it, vi } from "vitest";
import { defaultBidderConfig } from "@/lib/lab";
import { useBidderUpdate } from "./use-lab";
afterEach(cleanup);
it("optimistically applies only the patch and rolls back a rejected update", async () => {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: Infinity } },
  });
  client.setQueryData(["lab-config", 0], defaultBidderConfig);
  let respond: (response: Response) => void = () => {};
  const fetcher = vi.fn((_input: RequestInfo | URL, options?: RequestInit) => {
    expect(JSON.parse(String(options?.body))).toEqual({ latency_ms: 500 });
    return new Promise<Response>((resolve) => {
      respond = resolve;
    });
  });
  vi.stubGlobal("fetch", fetcher);
  const { result } = renderHook(() => useBidderUpdate(0), {
    wrapper: ({ children }) => (
      <QueryClientProvider client={client}>{children}</QueryClientProvider>
    ),
  });
  act(() => result.current.mutate({ latency_ms: 500 }));
  await waitFor(() =>
    expect(client.getQueryData(["lab-config", 0])).toEqual({
      ...defaultBidderConfig,
      latency_ms: 500,
    }),
  );
  await act(async () => {
    respond(
      Response.json(
        {
          error: {
            code: "unavailable",
            message: "Bidder offline",
            request_id: "rollback-trace",
          },
        },
        { status: 503 },
      ),
    );
  });
  await waitFor(() => expect(result.current.isError).toBe(true));
  expect(client.getQueryData(["lab-config", 0])).toEqual(defaultBidderConfig);
  expect(fetcher).toHaveBeenCalledOnce();
  client.clear();
});
