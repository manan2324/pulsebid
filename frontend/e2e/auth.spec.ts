import { test, expect } from "@playwright/test";
test.skip(process.env.E2E_AUTH_DISABLED === "true", "Auth-disabled smoke run");
test("auth gate, cookie, BFF session, logout", async ({ page, request }) => {
  await page.goto("/campaigns");
  await expect(page).toHaveURL(/\/login\?next=/);
  const anonymous = await request.get("/api/campaign/advertisers");
  expect(anonymous.status()).toBe(401);
  expect(await anonymous.json()).toMatchObject({
    error: { code: "unauthorized" },
  });
  await page.getByLabel("Password", { exact: true }).fill("wrong-password");
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await expect(page.locator("#login-error")).toContainText(
    "Incorrect password",
  );
  await page.getByLabel("Password", { exact: true }).fill("e2e-password");
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await expect(page).toHaveURL(/\/campaigns$/);
  const cookie = (await page.context().cookies()).find(
    (cookie) => cookie.name === "pb_session",
  );
  expect(cookie?.httpOnly).toBe(true);
  expect(cookie?.sameSite).toBe("Lax");
  const response = await page.request.get("/api/campaign/advertisers");
  if (process.env.E2E_REAL_BACKEND === "true") {
    expect(response.status()).toBe(200);
    expect(await response.json()).toHaveProperty("data");
  } else {
    expect([200, 503]).toContain(response.status());
  }
  expect(response.headers()["x-request-id"]).toBeTruthy();
  const blocked = await page.request.post("/api/campaign/advertisers", {
    data: { name: "blocked" },
    headers: { Origin: "https://evil.test" },
  });
  expect(blocked.status()).toBe(403);
  await page.screenshot({
    path: "docs/screenshots/fe-1-authenticated.png",
    fullPage: true,
  });
  await page.getByRole("button", { name: "Sign out" }).click();
  await expect(page).toHaveURL(/\/login$/);
  expect((await page.request.get("/api/health")).status()).toBe(401);
});
