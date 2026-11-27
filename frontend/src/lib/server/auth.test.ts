import { describe, expect, it, vi } from "vitest";
import { NextRequest } from "next/server";
import { createSession, verifySession } from "./session";
import { allowLogin } from "./rateLimit";
import { middleware } from "@/middleware";
import { POST as login } from "@/app/api/auth/login/route";
import { POST as logout } from "@/app/api/auth/logout/route";
const secret = "test-session-secret-at-least-32-characters";
describe("operator sessions", () => {
  it("signs and verifies a 12-hour operator JWT", async () => {
    const token = await createSession(secret);
    expect(await verifySession(token, secret)).toBe(true);
    expect(await verifySession(token, secret + "wrong")).toBe(false);
    expect(await verifySession(undefined, secret)).toBe(false);
    expect(await verifySession("invalid", secret)).toBe(false);
  });
  it("rejects expired sessions", async () => {
    vi.useFakeTimers();
    const token = await createSession(secret);
    vi.setSystemTime(Date.now() + 13 * 60 * 60 * 1000);
    expect(await verifySession(token, secret)).toBe(false);
    vi.useRealTimers();
  });
  it("redirects pages with a safe next path", async () => {
    vi.stubEnv("AUTH_DISABLED", "false");
    const response = await middleware(
      new NextRequest("http://localhost:3001/campaigns?status=active"),
    );
    expect(response.status).toBe(307);
    expect(response.headers.get("location")).toBe(
      "http://localhost:3001/login?next=%2Fcampaigns%3Fstatus%3Dactive",
    );
    vi.unstubAllEnvs();
  });
  it("returns standard 401 for unauthenticated API", async () => {
    vi.stubEnv("AUTH_DISABLED", "false");
    const response = await middleware(
      new NextRequest("http://localhost:3001/api/campaign/advertisers"),
    );
    expect(response.status).toBe(401);
    expect(await response.json()).toMatchObject({
      error: { code: "unauthorized" },
    });
    vi.unstubAllEnvs();
  });
  it("accepts signed cookie", async () => {
    const token = await createSession(secret);
    const response = await middleware(
      new NextRequest("http://localhost:3001/", {
        headers: { cookie: `pb_session=${token}` },
      }),
    );
    expect(response.status).toBe(200);
    expect(response.headers.get("Content-Security-Policy")).toContain(
      "'nonce-",
    );
  });
  it("allows login page", async () =>
    expect(
      (await middleware(new NextRequest("http://localhost:3001/login"))).status,
    ).toBe(200));
  it("does not honor AUTH_DISABLED in production", async () => {
    vi.stubEnv("NODE_ENV", "production");
    vi.stubEnv("AUTH_DISABLED", "true");
    expect(
      (await middleware(new NextRequest("http://localhost:3001/"))).status,
    ).toBe(307);
    vi.unstubAllEnvs();
  });
  it("limits per IP to 5 attempts in one minute", () => {
    for (let i = 0; i < 5; i++) expect(allowLogin("test-ip", 1000)).toBe(true);
    expect(allowLogin("test-ip", 1000)).toBe(false);
    expect(allowLogin("other-ip", 1000)).toBe(true);
    expect(allowLogin("test-ip", 61001)).toBe(true);
  });
  it("login sets protected cookie and logout clears it", async () => {
    const response = await login(
      new NextRequest("http://localhost:3001/api/auth/login", {
        method: "POST",
        headers: {
          Origin: "http://localhost:3001",
          "Content-Type": "application/json",
        },
        body: JSON.stringify({ password: "test-password" }),
      }),
    );
    expect(response.status).toBe(200);
    expect(response.headers.get("set-cookie")).toMatch(
      /pb_session=.*HttpOnly.*SameSite=lax/i,
    );
    expect(response.headers.get("set-cookie")).toContain("Max-Age=43200");
    const result = await logout(
      new Request("http://localhost:3001/api/auth/logout", {
        method: "POST",
        headers: { Origin: "http://localhost:3001", cookie: "pb_session=x" },
      }),
    );
    expect(result.headers.get("set-cookie")).toContain("Max-Age=0");
  });
  it("rejects incorrect password and foreign origin", async () => {
    const req = (password: string, origin: string) =>
      new NextRequest("http://localhost:3001/api/auth/login", {
        method: "POST",
        headers: { Origin: origin, "Content-Type": "application/json" },
        body: JSON.stringify({ password }),
      });
    expect((await login(req("wrong", "http://localhost:3001"))).status).toBe(
      401,
    );
    expect(
      (await login(req("test-password", "https://evil.test"))).status,
    ).toBe(403);
  });
});

it("allows logout with an empty framework request stream", async () => {
  const response = await logout(
    new Request("http://localhost:3001/api/auth/logout", {
      method: "POST",
      headers: { Origin: "http://localhost:3001", cookie: "pb_session=old" },
      body: new ArrayBuffer(0),
    }),
  );
  expect(response.status).toBe(200);
  expect(response.headers.get("set-cookie")).toContain("Max-Age=0");
});
