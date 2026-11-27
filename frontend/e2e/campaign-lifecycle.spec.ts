import { test, expect } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import type { Bidder } from "../src/lib/api/types";
test.use({ actionTimeout: 30000, navigationTimeout: 45000 });
test("campaign wizard, detail, creatives, settings, pause, activate and archive", async ({
  page,
  baseURL,
}) => {
  test.setTimeout(240000);
  const suffix = `${Date.now()}-${Math.random().toString(36).slice(2, 7)}`;
  const name = `e2e-campaign-${suffix}`;
  const creativeName = `e2e-banner-${suffix}`;
  let advertiserId: string | undefined;
  let campaignId: string | undefined;
  let archived = false;
  const origin = { Origin: baseURL! };
  const trackers: string[] = [];
  page.on("request", (request) => {
    if (/\/(v1\/events\/|api\/gateway\/(impression|click))/.test(request.url()))
      trackers.push(request.url());
  });
  await page.goto("/login");
  await page.getByLabel("Password", { exact: true }).fill("e2e-password");
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await expect(page).toHaveURL(/\/$/, { timeout: 45000 });
  try {
    const adv = await page.request.post("/api/campaign/advertisers", {
      headers: origin,
      data: { name: `e2e-campaign-advertiser-${suffix}`, contact_email: null },
    });
    expect(adv.status()).toBe(201);
    advertiserId = (await adv.json()).id;
    const biddersResponse = await page.request.get(
      "/api/campaign/bidders?limit=200",
    );
    expect(biddersResponse.status()).toBe(200);
    const bidderPage: { data: Bidder[] } = await biddersResponse.json();
    const bidder = bidderPage.data.find(
      (item) => item.enabled && new URL(item.endpoint_url).port === "9101",
    );
    expect(bidder).toBeDefined();
    await page.goto("/campaigns/new");
    await page.getByLabel("Campaign name", { exact: true }).fill(name);
    await page
      .getByRole("combobox", { name: "Advertiser", exact: true })
      .selectOption(advertiserId!);
    await page.getByLabel("Bidder", { exact: true }).selectOption(bidder!.id);
    await page.getByRole("button", { name: "Next", exact: true }).click();
    await page
      .getByLabel("Daily budget (USD)", { exact: true })
      .fill("25.000001");
    await page
      .getByLabel("Max bid (USD CPM)", { exact: true })
      .fill("3.000001");
    await page.getByRole("button", { name: "Next", exact: true }).click();
    await expect(
      page.getByRole("heading", { name: "Targeting", exact: true }),
    ).toBeVisible();
    page.once("dialog", (dialog) => dialog.accept());
    await page.reload();
    await expect(
      page.getByRole("heading", { name: "Targeting", exact: true }),
    ).toBeVisible();
    await expect(
      page.getByText("Your saved wizard draft has been restored."),
    ).toBeVisible();
    await page.getByLabel("Search countries").fill("India");
    await page.getByRole("checkbox", { name: "India IN" }).check();
    await page.getByRole("checkbox", { name: "mobile", exact: true }).check();
    await page
      .getByLabel("Placement IDs", { exact: true })
      .fill("home-top-banner");
    await page
      .getByRole("button", { name: "Add placement", exact: true })
      .click();
    await page
      .getByRole("button", { name: "Business hours", exact: true })
      .click();
    await page.getByRole("button", { name: "Clear", exact: true }).click();
    await page.getByRole("button", { name: "Next", exact: true }).click();
    await page.getByLabel("Creative name", { exact: true }).fill(creativeName);
    await page
      .getByLabel("Landing URL", { exact: true })
      .fill("https://example.test/fe3");
    await page
      .getByLabel("Ad markup", { exact: true })
      .fill(
        '<div style="padding:20px;background-color:#eeeaff;color:#343467">E2E campaign ad</div><a href="{click_url}">Learn more</a><img src="{impression_url}"><img src="http://localhost:8080/v1/events/impression?t=preview-must-not-fire"><meta http-equiv="refresh" content="0;url=/api/gateway/click?t=blocked">',
      );
    await expect(page.getByTitle("Sandboxed ad preview")).toHaveAttribute(
      "sandbox",
      "",
    );
    await expect
      .poll(() =>
        page.getByTitle("Sandboxed ad preview").getAttribute("srcdoc"),
      )
      .not.toContain("preview-must-not-fire");
    await page.getByRole("button", { name: "Next", exact: true }).click();
    await expect(
      page.getByRole("heading", { name: "Review", exact: true }),
    ).toBeVisible();
    await expect(
      page.getByRole("checkbox", { name: "Activate immediately", exact: true }),
    ).toBeChecked();
    const created = page.waitForResponse(
      (response) =>
        response.url().endsWith("/api/campaign/campaigns") &&
        response.request().method() === "POST",
    );
    await page
      .getByRole("button", { name: "Create campaign", exact: true })
      .click();
    const response = await created;
    expect(response.status()).toBe(201);
    campaignId = (await response.json()).id;
    await expect(page).toHaveURL(new RegExp(`/campaigns/${campaignId}$`));
    await expect(
      page.getByRole("heading", { name, exact: true }),
    ).toBeVisible();
    await expect(
      page
        .locator(".campaign-detail-heading")
        .getByText("Active", { exact: true }),
    ).toBeVisible();
    const record = await page.request.get(
      `/api/campaign/campaigns/${campaignId}`,
    );
    expect(await record.json()).toMatchObject({
      daily_budget: 25.000001,
      max_bid_cpm: 3.000001,
      status: "active",
    });
    await expect
      .poll(
        async () => {
          const snapshot = await page.request.get("/api/campaign/snapshot");
          expect(snapshot.status()).toBe(200);
          const data: { campaigns: { id: string }[] } = await snapshot.json();
          return data.campaigns.some((item) => item.id === campaignId);
        },
        { timeout: 15000 },
      )
      .toBe(true);
    await page.getByRole("tab", { name: "Targeting", exact: true }).click();
    await expect(
      page.getByRole("checkbox", { name: "mobile", exact: true }),
    ).toBeChecked();
    await page.getByLabel("Search countries").fill("India");
    await expect(
      page.getByRole("checkbox", { name: "India IN" }),
    ).toBeChecked();
    await page.screenshot({
      path: "docs/screenshots/fe-3-targeting.png",
      fullPage: true,
    });
    await page.getByRole("tab", { name: "Creatives", exact: true }).click();
    await expect(
      page.getByRole("heading", { name: creativeName, exact: true }),
    ).toBeVisible();
    await expect(page.getByTitle("Sandboxed ad preview")).toHaveAttribute(
      "sandbox",
      "",
    );
    expect(trackers).toEqual([]);
    await page
      .getByRole("button", {
        name: `Edit creative ${creativeName}`,
        exact: true,
      })
      .click();
    const dialog = page.getByRole("dialog");
    await dialog
      .getByLabel("Creative name", { exact: true })
      .fill(`${creativeName}-edited`);
    await dialog
      .getByRole("button", { name: "Save changes", exact: true })
      .click();
    await expect(dialog).not.toBeVisible();
    await expect(
      page.getByRole("heading", {
        name: `${creativeName}-edited`,
        exact: true,
      }),
    ).toBeVisible();
    await page.getByRole("tab", { name: "Settings", exact: true }).click();
    await page
      .getByLabel("Daily budget (USD)", { exact: true })
      .fill("30.000001");
    await page
      .getByRole("button", { name: "Save changes", exact: true })
      .click();
    await expect(page.getByText("You have unsaved changes.")).not.toBeVisible();
    const updated = await page.request.get(
      `/api/campaign/campaigns/${campaignId}`,
    );
    expect(await updated.json()).toMatchObject({ daily_budget: 30.000001 });
    await page.getByRole("tab", { name: "Overview", exact: true }).click();
    await expect(
      page.getByText("Budget today (UTC)", { exact: true }),
    ).toBeVisible();
    await page.screenshot({
      path: "docs/screenshots/fe-3-campaign-detail.png",
      fullPage: true,
    });
    await page.getByRole("button", { name: "Pause", exact: true }).click();
    await expect(
      page
        .locator(".campaign-detail-heading")
        .getByText("Paused", { exact: true }),
    ).toBeVisible();
    await page.getByRole("button", { name: "Activate", exact: true }).click();
    await expect(
      page
        .locator(".campaign-detail-heading")
        .getByText("Active", { exact: true }),
    ).toBeVisible();
    await expect(
      page.getByRole("button", { name: "Pause", exact: true }),
    ).toBeEnabled();
    await expect(
      page.getByRole("button", { name: "Pause", exact: true }),
    ).toHaveCSS("opacity", "1");
    const axe = await new AxeBuilder({ page }).analyze();
    expect(
      axe.violations.filter((item) =>
        ["serious", "critical"].includes(item.impact ?? ""),
      ),
    ).toEqual([]);
    await page.getByRole("button", { name: "Archive", exact: true }).click();
    await page
      .getByRole("dialog")
      .getByLabel("Type campaign name", { exact: true })
      .fill(name);
    await page
      .getByRole("dialog")
      .getByRole("button", { name: "Archive", exact: true })
      .click();
    await expect(page.getByRole("dialog")).not.toBeVisible();
    await expect(
      page
        .locator(".campaign-detail-heading")
        .getByText("Archived", { exact: true }),
    ).toBeVisible();
    archived = true;
    await page.goto("/campaigns");
    await page
      .getByLabel("Search loaded campaigns", { exact: true })
      .fill(name);
    await page
      .getByRole("combobox", { name: "Advertiser", exact: true })
      .selectOption(advertiserId!);
    await page.getByText("Status filters (0)", { exact: true }).click();
    await page.getByRole("checkbox", { name: "Archived", exact: true }).check();
    await expect(page.getByRole("link", { name, exact: true })).toBeVisible();
    await expect
      .poll(() => new URL(page.url()).searchParams.get("status"))
      .toBe("archived");
    await page
      .getByRole("button", { name: `Actions ${name}`, exact: true })
      .click();
    await expect(
      page.getByRole("menuitem", { name: "Open", exact: true }),
    ).toBeVisible();
    await page.keyboard.press("Escape");
    await expect(page.getByRole("menu")).not.toBeVisible();
    await expect(page.locator(".console-shell")).not.toHaveAttribute(
      "aria-hidden",
      "true",
    );
    const listAxe = await new AxeBuilder({ page }).analyze();
    expect(
      listAxe.violations.filter((item) =>
        ["serious", "critical"].includes(item.impact ?? ""),
      ),
    ).toEqual([]);
    await page.getByText("Status filters (1)", { exact: true }).click();
    await page.getByLabel("Search loaded campaigns", { exact: true }).focus();
    await page.locator(".table-scroll").evaluate((element) => {
      element.scrollLeft = 0;
    });
    await page.screenshot({
      path: "docs/screenshots/fe-3-campaigns.png",
      fullPage: true,
    });
    await page.setViewportSize({ width: 768, height: 1024 });
    await expect(page.getByRole("link", { name, exact: true })).toBeVisible();
    await page.locator(".table-scroll").evaluate((element) => {
      element.scrollLeft = 0;
    });
    await page.screenshot({
      path: "docs/screenshots/fe-3-campaigns-tablet.png",
      fullPage: true,
    });
  } finally {
    if (campaignId && !archived)
      await page.request.delete(`/api/campaign/campaigns/${campaignId}`, {
        headers: origin,
      });
    if (advertiserId)
      await page.request.delete(`/api/campaign/advertisers/${advertiserId}`, {
        headers: origin,
      });
  }
});
