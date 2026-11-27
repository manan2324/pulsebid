import { describe, expect, it, vi } from "vitest";
import { apiRequest, ApiClientError, queryString } from "./client";
describe("typed API client", () => {
  it("parses standard envelope", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        Response.json(
          {
            error: {
              code: "invalid_request",
              message: "Fix name",
              request_id: "trace-1",
              details: [{ field: "name", issue: "required" }],
            },
          },
          { status: 400 },
        ),
      ),
    );
    await expect(apiRequest("/api/campaign/advertisers")).rejects.toMatchObject(
      {
        code: "invalid_request",
        message: "Fix name",
        requestId: "trace-1",
        status: 400,
        details: [{ field: "name", issue: "required" }],
      },
    );
  });
  it("normalizes non-JSON errors", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        new Response("bad", {
          status: 502,
          headers: { "x-request-id": "fallback" },
        }),
      ),
    );
    await expect(apiRequest("/api/health")).rejects.toBeInstanceOf(
      ApiClientError,
    );
  });
  it("handles 204 and abort signal", async () => {
    const fn = vi.fn().mockResolvedValue(new Response(null, { status: 204 }));
    vi.stubGlobal("fetch", fn);
    const controller = new AbortController();
    expect(
      await apiRequest("/api/campaign/advertisers/a", {
        method: "DELETE",
        signal: controller.signal,
      }),
    ).toBeUndefined();
    expect(fn.mock.calls[0]![1].signal).toBe(controller.signal);
  });
  it("validates response with supplied parser", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(Response.json({ ok: true })),
    );
    expect(await apiRequest("/api/health", {}, (value) => Boolean(value))).toBe(
      true,
    );
  });
  it.each(["https://evil.test", "//evil.test", "/api/\\evil.test"])(
    "rejects nonlocal path",
    async (path) => expect(apiRequest(path)).rejects.toThrow("same-origin"),
  );
  it("preserves repeated query values and encodes cursors", () =>
    expect(
      queryString({
        campaign_id: ["a", "b"],
        cursor: "a+b",
        limit: 20,
        none: null,
      }),
    ).toBe("?campaign_id=a&campaign_id=b&cursor=a%2Bb&limit=20"));
});

it("keeps request correlation when a response parser rejects the body", async () => {
  vi.stubGlobal(
    "fetch",
    vi
      .fn()
      .mockResolvedValue(
        Response.json(
          { invalid: true },
          { headers: { "X-Request-Id": "invalid-response-123" } },
        ),
      ),
  );
  await expect(
    apiRequest("/api/reporting/reports/campaigns", {}, () => {
      throw new Error("Invalid report");
    }),
  ).rejects.toMatchObject({
    code: "invalid_response",
    requestId: "invalid-response-123",
    status: 200,
    message: "Invalid report",
  });
});
