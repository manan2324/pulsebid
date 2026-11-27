import { describe, expect, it, vi } from "vitest";
import { allowedCampaign, bidderIndex, proxy } from "./proxy";
import { mutationGuard } from "./http";
import { prometheusPath } from "./promQueries";
import { envSchema } from "./env";
const base = "http://localhost:3001";
describe("proxy boundary", () => {
  it.each([
    "unknown",
    "advertisers/../snapshot",
    "advertisers/http://evil.test",
    "campaigns/id/../../secret",
    "campaigns/id/unknown",
    "snapshot/anything",
    "//evil.test",
  ])("rejects %s", (path) => expect(allowedCampaign(path, "GET")).toBe(false));
  it.each([
    ["advertisers", "GET"],
    ["campaigns/id/targeting", "PUT"],
    ["campaigns/id/creatives", "POST"],
    ["campaigns/id/activate", "POST"],
    ["creatives/id", "PATCH"],
    ["snapshot", "GET"],
  ] as const)("permits %s %s", (path, method) =>
    expect(allowedCampaign(path, method)).toBe(true),
  );
  it.each([
    ["snapshot", "POST"],
    ["advertisers", "DELETE"],
    ["campaigns/id/activate", "GET"],
    ["bidders/id", "PUT"],
  ] as const)("rejects method on %s", (path, method) =>
    expect(allowedCampaign(path, method)).toBe(false),
  );
  it("gates test reset", () => {
    expect(allowedCampaign("campaigns/id/budget/reset", "POST")).toBe(false);
    expect(allowedCampaign("campaigns/id/budget/reset", "POST", true)).toBe(
      true,
    );
  });
  it.each(["-1", "8", "1.0", "01", "1e0", "http://evil", "10000000000"])(
    "rejects bidder index %s",
    (index) => expect(bidderIndex(index, 8)).toBe(false),
  );
  it.each(["0", "7"])("accepts bidder index %s", (index) =>
    expect(bidderIndex(index, 8)).toBe(true),
  );
  it("injects secret server-side and strips browser identity", async () => {
    const fetcher = vi
      .fn()
      .mockResolvedValue(Response.json({ data: [], next_cursor: null }));
    vi.stubGlobal("fetch", fetcher);
    const response = await proxy(
      new Request(`${base}/api/campaign/advertisers?cursor=a%2Bb`, {
        headers: {
          cookie: "private=1",
          Authorization: "Bearer browser-secret",
          "x-request-id": "known-id",
        },
      }),
      {
        base: "http://localhost:8081",
        path: "/v1/advertisers?cursor=a%2Bb",
        service: "Campaign",
      },
    );
    const [url, init] = fetcher.mock.calls[0]! as [URL, RequestInit];
    const headers = new Headers(init.headers);
    expect(url.toString()).toBe(
      "http://localhost:8081/v1/advertisers?cursor=a%2Bb",
    );
    expect(headers.get("x-api-key")).toBe("test-server-key-never-public");
    expect(headers.has("cookie")).toBe(false);
    expect(headers.has("authorization")).toBe(false);
    expect(headers.get("x-request-id")).toBe("known-id");
    expect(response.headers.get("x-request-id")).toBe("known-id");
  });
  it.each([
    new DOMException("Timeout", "TimeoutError"),
    new TypeError("Network error"),
  ])("normalizes transport failures", async (error) => {
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(error));
    const response = await proxy(new Request(base), {
      base: "http://localhost:8081",
      path: "/v1/advertisers",
      service: "Campaign",
    });
    expect(response.status).toBe(503);
    expect(await response.json()).toMatchObject({
      error: {
        code: "unavailable",
        message: "Campaign unreachable",
        request_id: expect.any(String),
      },
    });
  });
  it("sets default and bid timeouts", async () => {
    const spy = vi.spyOn(AbortSignal, "timeout");
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(new Response(null, { status: 204 })),
    );
    await proxy(new Request(base), {
      base: "http://localhost:8080",
      path: "/v1/bid",
      service: "Gateway",
      timeout: 3000,
    });
    expect(spy).toHaveBeenCalledWith(3000);
  });
  it("enforces Origin and JSON", () => {
    expect(
      mutationGuard(new Request(base, { method: "POST", body: "{}" }), "id")
        ?.status,
    ).toBe(403);
    expect(
      mutationGuard(
        new Request(base, {
          method: "POST",
          headers: { Origin: base },
          body: "{}",
        }),
        "id",
      )?.status,
    ).toBe(415);
    expect(
      mutationGuard(
        new Request(base, {
          method: "POST",
          headers: {
            Origin: base,
            "Content-Type": "application/json; charset=utf-8",
          },
          body: "{}",
        }),
        "id",
      ),
    ).toBe(null);
  });
  it("does not fetch a blocked mutation", async () => {
    const fn = vi.fn();
    vi.stubGlobal("fetch", fn);
    expect(
      (
        await proxy(new Request(base, { method: "DELETE" }), {
          base: "http://localhost:8081",
          path: "/v1/advertisers/a",
          service: "Campaign",
        })
      ).status,
    ).toBe(403);
    expect(fn).not.toHaveBeenCalled();
  });
  it("preserves gateway diagnostics and no-fill", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        new Response(null, {
          status: 204,
          headers: {
            "server-timing": "total;dur=21",
            "x-pulsebid-reason": "budget",
            "x-pulsebid-debug": "eligible=1",
          },
        }),
      ),
    );
    const result = await proxy(new Request(base), {
      base: "http://localhost:8080",
      path: "/v1/bid",
      service: "Gateway",
      gateway: true,
    });
    expect(result.status).toBe(204);
    expect(result.headers.get("x-upstream-x-pulsebid-reason")).toBe("budget");
    expect(result.headers.get("x-upstream-server-timing")).toBe("total;dur=21");
    expect(await result.text()).toBe("");
  });
  it("never follows click redirects", async () => {
    const fn = vi.fn().mockResolvedValue(
      new Response(null, {
        status: 302,
        headers: { Location: "https://landing.test" },
      }),
    );
    vi.stubGlobal("fetch", fn);
    const result = await proxy(new Request(base), {
      base: "http://localhost:8080",
      path: "/v1/events/click?t=token",
      service: "Gateway",
      click: true,
    });
    expect(await result.json()).toEqual({
      status: 302,
      location: "https://landing.test",
    });
    expect(fn.mock.calls[0]![1].redirect).toBe("manual");
  });
  it("forwards test-time only when explicitly enabled", async () => {
    const fn = vi
      .fn()
      .mockImplementation(() =>
        Promise.resolve(new Response(null, { status: 204 })),
      );
    vi.stubGlobal("fetch", fn);
    for (const mode of ["false", "true"]) {
      vi.stubEnv("TEST_MODE_UI", mode);
      await proxy(
        new Request(base, {
          headers: { "X-PulseBid-Test-Time": "2026-01-01T00:00:00Z" },
        }),
        {
          base: "http://localhost:8080",
          path: "/v1/bid",
          service: "Gateway",
          gateway: true,
        },
      );
      const headers = new Headers(fn.mock.calls.at(-1)![1].headers);
      expect(headers.has("x-pulsebid-test-time")).toBe(mode === "true");
    }
    vi.unstubAllEnvs();
  });
});
it("checks configured browser origin independently of the internal bind host", () => {
  vi.stubEnv("APP_ORIGIN", "https://console.example.test");
  const request = (origin: string) =>
    new Request("http://0.0.0.0:3001/api/auth/login", {
      method: "POST",
      headers: { Origin: origin, "Content-Type": "application/json" },
      body: "{}",
    });
  expect(
    mutationGuard(request("https://console.example.test"), "id"),
  ).toBeNull();
  expect(mutationGuard(request("http://0.0.0.0:3001"), "id")?.status).toBe(403);
  vi.unstubAllEnvs();
});
describe("named metrics and env", () => {
  it("allows range queries", () =>
    expect(
      prometheusPath(
        new URLSearchParams({ name: "bid_rps", range: "15m" }),
        1_000_000,
      ),
    ).toContain("step=15"));
  it.each([
    { name: "raw" },
    { name: "outcomes", range: "1h" },
    { name: "bid_rps", range: "forever" },
    { name: "bid_rps", query: "up" },
  ] as Record<string, string>[])(
    "rejects arbitrary metric parameters",
    (params) => expect(prometheusPath(new URLSearchParams(params))).toBeNull(),
  );
  it("refuses production auth bypass", () => {
    expect(
      envSchema.safeParse({
        ...process.env,
        NODE_ENV: "production",
        AUTH_DISABLED: "true",
      }).success,
    ).toBe(false);
  });
  it("requires secrets and safe bidder host", () => {
    expect(envSchema.safeParse({}).success).toBe(false);
    expect(
      envSchema.safeParse({ ...process.env, BIDDER_HOST: "evil.test/path" })
        .success,
    ).toBe(false);
  });
});

it("allows a bodyless DELETE represented by an empty stream", async () => {
  const fn = vi.fn().mockResolvedValue(new Response(null, { status: 204 }));
  vi.stubGlobal("fetch", fn);
  const request = new Request(`${base}/api/campaign/bidders/test-dsp`, {
    method: "DELETE",
    headers: { Origin: base },
    body: new ArrayBuffer(0),
  });
  expect(request.body).not.toBeNull();
  const response = await proxy(request, {
    base: "http://localhost:8081",
    path: "/v1/bidders/test-dsp",
    service: "Campaign",
  });
  expect(response.status).toBe(204);
  expect(fn.mock.calls[0]![1].body).toBeUndefined();
});
it("still rejects non-JSON data in a DELETE body", async () => {
  const fn = vi.fn();
  vi.stubGlobal("fetch", fn);
  const response = await proxy(
    new Request(base, {
      method: "DELETE",
      headers: { Origin: base },
      body: "text",
    }),
    {
      base: "http://localhost:8081",
      path: "/v1/bidders/test-dsp",
      service: "Campaign",
    },
  );
  expect(response.status).toBe(415);
  expect(fn).not.toHaveBeenCalled();
});
