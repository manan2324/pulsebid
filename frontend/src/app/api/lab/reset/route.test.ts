import { beforeEach, expect, it, vi } from "vitest";
import { POST } from "./route";
import { defaultBidderConfig } from "@/lib/lab";
beforeEach(() => {
  vi.stubEnv("PULSEBID_API_KEY", "test-key");
  vi.stubEnv("ADMIN_PASSWORD", "test-password");
  vi.stubEnv("SESSION_SECRET", "test-session-secret-at-least-32-characters");
});
const entry = { index: 0, config: defaultBidderConfig };
const request = (body: unknown, origin = "http://localhost:3001") =>
  new Request("http://localhost:3001/api/lab/reset", {
    method: "POST",
    headers: { Origin: origin, "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
it("rejects foreign Origin before reaching bidders", async () => {
  const fetcher = vi.fn();
  vi.stubGlobal("fetch", fetcher);
  expect(
    (await POST(request({ entries: [entry] }, "https://evil.test"))).status,
  ).toBe(403);
  expect(fetcher).not.toHaveBeenCalled();
});
it("rejects out-of-range, duplicate indices and arbitrary fields", async () => {
  const fetcher = vi.fn();
  vi.stubGlobal("fetch", fetcher);
  for (const entries of [
    [{ ...entry, index: 8 }],
    [entry, entry],
    [{ ...entry, config: { ...entry.config, endpoint: "https://evil.test" } }],
  ])
    expect((await POST(request({ entries }))).status).toBe(400);
  expect(fetcher).not.toHaveBeenCalled();
});
it("restores valid entries and reports partial failures", async () => {
  const fetcher = vi
    .fn()
    .mockResolvedValueOnce(Response.json(defaultBidderConfig))
    .mockRejectedValueOnce(new Error("offline"));
  vi.stubGlobal("fetch", fetcher);
  const response = await POST(
    request({ entries: [entry, { ...entry, index: 1 }] }),
  );
  expect(response.status).toBe(503);
  expect(await response.json()).toMatchObject({
    error: {
      code: "unavailable",
      request_id: response.headers.get("X-Request-Id"),
      details: [{ field: "bidders.1", issue: "Restoration failed" }],
    },
    results: [
      { index: 0, restored: true },
      { index: 1, restored: false },
    ],
  });
  expect(fetcher.mock.calls[0]?.[0]).toBe("http://localhost:9101/admin/config");
  expect(response.headers.get("X-Request-Id")).toBeTruthy();
});
