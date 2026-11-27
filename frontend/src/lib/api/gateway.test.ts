import { it, expect, vi } from "vitest";
import { sendBid, fireEvent } from "./gateway";
import { presetRequest } from "@/lib/schemas/bid";
const request = presetRequest("mobile");
it("preserves no-fill diagnostics and authoritative gateway timing without parsing a body", async () => {
  const fetcher = vi.fn().mockResolvedValue(
    new Response(null, {
      status: 204,
      headers: {
        "x-upstream-server-timing": "total;dur=31.2",
        "x-upstream-x-pulsebid-reason": "budget",
        "x-upstream-x-pulsebid-debug": "eligible=2;responded=2",
        "x-request-id": "trace-1",
      },
    }),
  );
  vi.stubGlobal("fetch", fetcher);
  const result = await sendBid(request);
  expect(result).toMatchObject({
    kind: "no-fill",
    httpStatus: 204,
    gatewayMs: 31.2,
    reason: "budget",
    requestId: "trace-1",
    debug: { eligible: 2, responded: 2 },
  });
  expect(fetcher).toHaveBeenCalledTimes(1);
  expect(fetcher.mock.calls[0]?.[0]).toBe("/api/gateway/bid");
});
it("retains field errors and request IDs, without retrying failed bids", async () => {
  const fetcher = vi.fn().mockResolvedValue(
    Response.json(
      {
        error: {
          code: "invalid_request",
          message: "Invalid floor",
          details: [{ field: "imp.bidfloor", issue: "too high" }],
          request_id: "trace-2",
        },
      },
      { status: 400 },
    ),
  );
  vi.stubGlobal("fetch", fetcher);
  const result = await sendBid(request);
  expect(result.error).toMatchObject({
    code: "invalid_request",
    requestId: "trace-2",
    details: [{ field: "imp.bidfloor", issue: "too high" }],
  });
  expect(fetcher).toHaveBeenCalledTimes(1);
});
it("extracts event tokens into a same-origin BFF call and never navigates to the upstream URL", async () => {
  const fetcher = vi
    .fn()
    .mockResolvedValue(
      Response.json({ status: 302, location: "https://landing.test/ad" }),
    );
  vi.stubGlobal("fetch", fetcher);
  expect(
    await fireEvent(
      "click",
      "https://upstream.test/v1/events/click?t=signed-token",
    ),
  ).toEqual({ location: "https://landing.test/ad" });
  expect(fetcher.mock.calls[0]?.[0]).toBe("/api/gateway/click?t=signed-token");
  expect(fetcher).toHaveBeenCalledTimes(1);
});
it("propagates abort instead of reporting a false auction error", async () => {
  const controller = new AbortController();
  controller.abort();
  vi.stubGlobal(
    "fetch",
    vi.fn().mockRejectedValue(new DOMException("Aborted", "AbortError")),
  );
  await expect(
    sendBid(request, { signal: controller.signal }),
  ).rejects.toMatchObject({ name: "AbortError" });
});

it("sends tracking only to the BFF and displays a redirect without navigating", async () => {
  const fetcher = vi
    .fn()
    .mockResolvedValueOnce(
      new Response(new Uint8Array([71, 73, 70]), {
        status: 200,
        headers: { "content-type": "image/gif" },
      }),
    )
    .mockResolvedValueOnce(
      Response.json({ status: 302, location: "https://example.test/landing" }),
    );
  vi.stubGlobal("fetch", fetcher);
  await expect(
    fireEvent(
      "impression",
      "https://upstream.test/v1/events/impression?t=signed%2Btoken",
    ),
  ).resolves.toEqual({ location: null });
  expect(fetcher.mock.calls[0]?.[0]).toBe(
    "/api/gateway/impression?t=signed%2Btoken",
  );
  await expect(
    fireEvent("click", "https://upstream.test/v1/events/click?t=token"),
  ).resolves.toEqual({ location: "https://example.test/landing" });
  expect(fetcher).toHaveBeenCalledTimes(2);
  expect(fetcher.mock.calls[1]?.[0]).toBe("/api/gateway/click?t=token");
});
it("retains structured errors and never retries failed auctions", async () => {
  const fetcher = vi.fn().mockResolvedValue(
    Response.json(
      {
        error: {
          code: "invalid_request",
          message: "bad floor",
          details: [{ field: "imp.bidfloor", issue: "too precise" }],
          request_id: "error-123",
        },
      },
      { status: 400 },
    ),
  );
  vi.stubGlobal("fetch", fetcher);
  const outcome = await sendBid(request);
  expect(outcome.kind).toBe("error");
  expect(outcome.error).toMatchObject({
    code: "invalid_request",
    requestId: "error-123",
    details: [{ field: "imp.bidfloor", issue: "too precise" }],
  });
  expect(fetcher).toHaveBeenCalledTimes(1);
});
it("propagates cancellation instead of treating it as a completed error outcome", async () => {
  const controller = new AbortController();
  controller.abort();
  vi.stubGlobal(
    "fetch",
    vi.fn().mockRejectedValue(new DOMException("Aborted", "AbortError")),
  );
  await expect(
    sendBid(request, { signal: controller.signal }),
  ).rejects.toThrow();
});
