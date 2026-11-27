import { test, expect } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { createSession } from "../src/lib/server/session";
test.use({ actionTimeout: 30000, navigationTimeout: 45000 });
test("all console pages, tablet layout, themes and keyboard commands", async ({
  page,
  baseURL,
}) => {
  test.setTimeout(300000);
  await page.context().addCookies([
    {
      name: "pb_session",
      value: await createSession("e2e-secret-with-at-least-32-characters"),
      url: baseURL!,
      httpOnly: true,
      sameSite: "Lax",
    },
  ]);
  const origin = { Origin: baseURL! };
  const suffix = Date.now();
  let advertiser: string | undefined;
  let campaign: string | undefined;
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  try {
    const advertiserResponse = await page.request.post(
      "/api/campaign/advertisers",
      { headers: origin, data: { name: `e2e-a11y-advertiser-${suffix}` } },
    );
    expect(advertiserResponse.status()).toBe(201);
    advertiser = (await advertiserResponse.json()).id;
    const bidders = await (
      await page.request.get("/api/campaign/bidders?limit=200")
    ).json();
    const response = await page.request.post("/api/campaign/campaigns", {
      headers: origin,
      data: {
        name: `e2e-keyboard-${suffix}`,
        advertiser_id: advertiser,
        bidder_id: bidders.data[0].id,
        daily_budget: "10",
        max_bid_cpm: "2",
        pacing_mode: "asap",
      },
    });
    expect(response.status()).toBe(201);
    campaign = (await response.json()).id;
    const routes = [
      "/",
      "/campaigns",
      `/campaigns/${campaign}`,
      "/campaigns/new",
      "/advertisers",
      "/bidders",
      "/reports",
      "/simulator",
      "/lab",
      "/system",
      "/login",
    ];
    for (const theme of ["light", "dark"]) {
      await page.setViewportSize(
        theme === "light"
          ? { width: 768, height: 1024 }
          : { width: 1440, height: 1000 },
      );
      await page.goto("/");
      await page
        .getByRole("combobox", { name: "Change theme: System", exact: true })
        .selectOption(theme);
      for (const route of routes) {
        await page.goto(route);
        await expect(page.locator("h1")).toBeVisible();
        if (route === "/") {
          for (const panel of await page.locator(".deferred-panel").all()) { await panel.scrollIntoViewIfNeeded(); await expect(panel.locator("[data-slot=skeleton]")).toHaveCount(0); }
          await page.evaluate(() => window.scrollTo(0, 0));
        }
        await expect(page.locator('[data-slot="skeleton"]')).toHaveCount(0, {
          timeout: 30000,
        });
        const results = await new AxeBuilder({ page })
          .exclude('iframe[title="Sandboxed ad preview"]')
          .analyze();
        expect(
          results.violations.filter((v) =>
            ["serious", "critical"].includes(v.impact ?? ""),
          ),
          `${theme} ${route}`,
        ).toEqual([]);
        expect(
          await page.evaluate(
            () => document.documentElement.scrollWidth <= window.innerWidth + 1,
          ),
          `${theme} ${route} overflow`,
        ).toBe(true);
        if (["/lab", "/system", "/campaigns"].includes(route))
          await page.screenshot({
            path: `docs/screenshots/fe-7-${route.slice(1)}-${theme}.png`,
            fullPage: true,
          });
      }
    }
    await page.goto("/");
    await page.keyboard.press("Control+k");
    const search = page.getByRole("combobox", {
      name: "Search pages, campaigns and actions",
      exact: true,
    });
    await expect(search).toBeVisible();
    await search.fill(`e2e-keyboard-${suffix}`);
    await expect(
      page.getByRole("option", { name: `e2e-keyboard-${suffix}`, exact: true }),
    ).toBeVisible();
    await search.press("Enter");
    await expect(page).toHaveURL(new RegExp(`/campaigns/${campaign}$`));
    await page.keyboard.press("Control+k");
    await search.fill("Create campaign");
    await search.press("Enter");
    await expect(page).toHaveURL(/\/campaigns\/new$/);
    await page.keyboard.press("Control+k");
    await search.fill("Toggle theme");
    await search.press("Enter");
    await expect(page.getByRole("dialog")).not.toBeVisible();
    await page.locator("h1").click();
    await page.keyboard.press("?");
    await expect(page.getByRole("dialog")).toContainText("Ctrl/Cmd+K");
    await page.keyboard.press("Escape");
    expect(errors).toEqual([]);
  } finally {
    test.setTimeout(test.info().timeout + 30000);
    if (campaign)
      expect(
        (
          await page.request.delete(`/api/campaign/campaigns/${campaign}`, {
            headers: origin,
          })
        ).status(),
      ).toBe(204);
    if (advertiser)
      expect(
        (
          await page.request.delete(`/api/campaign/advertisers/${advertiser}`, {
            headers: origin,
          })
        ).status(),
      ).toBe(204);
  }
});
