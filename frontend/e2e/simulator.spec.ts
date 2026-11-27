import { test, expect } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { createSession } from "../src/lib/server/session";
import type { Bidder } from "../src/lib/api/types";
import type { BidResponse } from "../src/lib/api/gateway";
test.use({ actionTimeout: 30000, navigationTimeout: 45000 });
test("matching auction, explicit events, no-fill, replay and 100-request burst", async ({
  page,
  baseURL,
}) => {
  test.setTimeout(240000);
  const browserErrors: string[] = [];
  page.on("pageerror", (error) => browserErrors.push(error.message));
  page.on("console", (message) => {
    if (
      message.type() === "error" &&
      message.text().includes("/_next/static/") &&
      message.text().includes("Content Security Policy")
    )
      browserErrors.push(message.text());
  });
  // Authentication itself is covered by auth.spec; avoid consuming its shared login limiter.
  if (process.env.E2E_AUTH_DISABLED !== "true")
    await page.context().addCookies([
      {
        name: "pb_session",
        value: await createSession("e2e-secret-with-at-least-32-characters"),
        url: baseURL!,
        httpOnly: true,
        sameSite: "Lax",
      },
    ]);
  const suffix = `${Date.now()}-${Math.random().toString(36).slice(2, 7)}`;
  const placement = `e2e-sim-${suffix}`;
  const campaignIds: string[] = [];
  let advertiserId: string | undefined;
  const origin = { Origin: baseURL! };
  const eventRequests: string[] = [];
  page.on("request", (request) => {
    if (/\/(v1\/events\/|api\/gateway\/(impression|click))/.test(request.url()))
      eventRequests.push(request.url());
  });
  const post = async (path: string, data: unknown, status = 201) => {
    const response = await page.request.post(path, { headers: origin, data });
    expect(response.status(), await response.text()).toBe(status);
    return response;
  };
  try {
    const advertiser = await post("/api/campaign/advertisers", {
      name: `e2e-simulator-advertiser-${suffix}`,
    });
    advertiserId = (await advertiser.json()).id;
    const bidders: { data: Bidder[] } = await (
      await page.request.get("/api/campaign/bidders?limit=200")
    ).json();
    const bidder = bidders.data.find(
      (item) => item.enabled && new URL(item.endpoint_url).port === "9101",
    );
    expect(bidder).toBeDefined();
    for (const [index, max] of [2, 1.5].entries()) {
      const created = await post("/api/campaign/campaigns", {
        name: `e2e-simulator-${suffix}-${index}`,
        advertiser_id: advertiserId,
        bidder_id: bidder!.id,
        daily_budget: "100",
        max_bid_cpm: String(max),
        pacing_mode: "asap",
      });
      const id: string = (await created.json()).id;
      campaignIds.push(id);
      expect(
        (
          await page.request.put(`/api/campaign/campaigns/${id}/targeting`, {
            headers: origin,
            data: {
              countries: ["IN"],
              device_types: ["mobile"],
              placement_ids: [placement],
              hours_of_day: [],
              timezone: "UTC",
            },
          })
        ).status(),
      ).toBe(200);
      await post(`/api/campaign/campaigns/${id}/creatives`, {
        name: `simulator-banner-${suffix}-${index}`,
        format: "banner",
        width: 300,
        height: 250,
        markup:
          '<div id="e2e-live-ad" style="padding:20px;background-color:#eeeaff;color:#30304a">Simulator test ad</div><a href="{click_url}">Learn more</a><img src="{impression_url}">',
        landing_url: "https://example.test/simulator-landing",
        status: "active",
      });
      await post(`/api/campaign/campaigns/${id}/activate`, {}, 200);
    }
    await expect
      .poll(async () => {
        const data = await (
          await page.request.get("/api/campaign/snapshot")
        ).json();
        return campaignIds.every((id) =>
          data.campaigns.some((item: { id: string }) => item.id === id),
        );
      })
      .toBe(true);
    // The gateway refreshes asynchronously; wait for both isolated bids before UI assertions.
    await expect
      .poll(
        async () => {
          const response = await page.request.post("/api/gateway/bid", {
            headers: origin,
            data: {
              id: `e2e-cache-${suffix}`,
              imp: {
                id: "imp-cache",
                placement_id: placement,
                w: 300,
                h: 250,
                bidfloor: 1,
                bidfloorcur: "USD",
              },
              device: { type: "mobile" },
              geo: { country: "IN" },
              tmax: 100,
            },
          });
          if (response.status() !== 200) return null;
          const bid: BidResponse = await response.json();
          return { campaign_id: bid.campaign_id, price: bid.price };
        },
        { timeout: 15000, intervals: [250, 500, 1000] },
      )
      .toEqual({ campaign_id: campaignIds[0], price: 1.51 });
    await page.goto("/simulator");
    await expect(
      page.getByRole("heading", { name: "Auction Simulator", exact: true }),
    ).toBeVisible();
    expect(await page.getByLabel("Test time (RFC 3339)").count()).toBe(0);
    await page.getByLabel("Placement ID", { exact: true }).fill(placement);
    await page.getByLabel("Bid floor (USD CPM)", { exact: true }).fill("1");
    await page.getByRole("button", { name: "JSON", exact: true }).click();
    const json = page.getByLabel("Bid request JSON");
    const request = JSON.parse(await json.inputValue());
    request.id = `e2e-request-${suffix}`;
    await json.fill(JSON.stringify(request, null, 2));
    await page.getByRole("button", { name: "Form", exact: true }).click();
    await expect(page.getByLabel("Request ID", { exact: true })).toHaveValue(
      request.id,
    );
    const nextBid = () =>
      page.waitForResponse(
        (response) =>
          response.url().endsWith("/api/gateway/bid") &&
          response.request().method() === "POST",
      );
    const pending = nextBid();
    await page.getByRole("button", { name: "Send bid", exact: true }).click();
    const auction = await pending;
    expect(auction.status()).toBe(200);
    const winner: BidResponse = await auction.json();
    expect(winner).toMatchObject({
      campaign_id: campaignIds[0],
      price: 1.51,
      currency: "USD",
    });
    const result = page.locator(".simulator-result");
    await expect(result.getByText("200 Filled", { exact: true })).toBeVisible();
    await expect(result.getByTestId("gateway-latency")).not.toHaveText(
      "Not supplied",
    );
    await expect(page.getByTitle("Sandboxed ad preview")).toHaveAttribute(
      "sandbox",
      "",
    );
    await expect(
      page
        .frameLocator('iframe[title="Sandboxed ad preview"]')
        .getByText("Simulator test ad"),
    ).toBeVisible();
    expect(await page.locator("#e2e-live-ad").count()).toBe(0);
    expect(eventRequests).toEqual([]);
    await page.evaluate(() => window.scrollTo(0, 0));
    await page.screenshot({
      path: "docs/screenshots/fe-4-filled.png",
      fullPage: true,
    });
    const impression = page.waitForResponse((response) =>
      response.url().includes("/api/gateway/impression?"),
    );
    await result
      .getByRole("button", { name: "Fire impression", exact: true })
      .click();
    expect((await impression).status()).toBe(200);
    await expect(
      result.getByRole("button", { name: "Impression sent", exact: true }),
    ).toBeDisabled();
    for (let index = 0; index < 2; index++) {
      const repeated = page.waitForResponse((response) =>
        response.url().includes("/api/gateway/impression?"),
      );
      await result
        .getByRole("button", {
          name: "Fire impression again (duplicate test)",
          exact: true,
        })
        .click();
      expect((await repeated).status()).toBe(200);
    }
    await result
      .getByRole("button", { name: "Fire click", exact: true })
      .click();
    await expect(
      result.getByRole("link", {
        name: "https://example.test/simulator-landing",
        exact: true,
      }),
    ).toBeVisible();
    await expect(page).toHaveURL(/\/simulator$/);
    await expect
      .poll(
        async () => {
          const now = new Date();
          const from = new Date(now.getTime() - 3600000).toISOString();
          const query = new URLSearchParams({
            from,
            to: now.toISOString(),
            granularity: "total",
          });
          const response = await page.request.get(
            `/api/reporting/reports/campaigns/${winner.campaign_id}?${query}`,
          );
          expect(response.status()).toBe(200);
          const data = await response.json();
          return {
            impressions: data.totals.impressions,
            clicks: data.totals.clicks,
          };
        },
        { timeout: 20000 },
      )
      .toEqual({ impressions: 1, clicks: 1 });
    await page
      .getByRole("combobox", { name: "Request preset", exact: true })
      .selectOption("no-match");
    const noFill = nextBid();
    await page.getByRole("button", { name: "Send bid", exact: true }).click();
    expect((await noFill).status()).toBe(204);
    await expect(
      result.getByText("No eligible campaigns", { exact: true }),
    ).toBeVisible();
    await page.evaluate(() => window.scrollTo(0, 0));
    await page.screenshot({
      path: "docs/screenshots/fe-4-no-fill.png",
      fullPage: true,
    });
    const replay = nextBid();
    await page
      .getByRole("button", {
        name: `Replay request ${request.id}`,
        exact: true,
      })
      .click();
    expect((await replay).status()).toBe(200);
    await expect(page.getByLabel("Placement ID", { exact: true })).toHaveValue(
      placement,
    );
    const history = await page.evaluate(() =>
      localStorage.getItem("pulsebid.auction-history.v1"),
    );
    expect(history).not.toContain("impression_url");
    expect(history).not.toContain("Simulator test ad");
    await page.getByText("Demo burst", { exact: true }).click();
    await page.getByLabel("Requests (1–200)", { exact: true }).fill("100");
    await page.getByLabel("Concurrency (1–10)", { exact: true }).fill("4");
    await page.getByRole("button", { name: "Run burst", exact: true }).click();
    await expect(
      page.getByRole("heading", { name: "Burst summary", exact: true }),
    ).toBeVisible({ timeout: 45000 });
    const summary = page.locator(".simulator-summary");
    await expect(summary.getByRole("status")).toHaveText(
      "Completed: 100/100 · Started: 100",
    );
    await expect(
      summary.getByText("Gateway timing samples: 100/100", { exact: true }),
    ).toBeVisible();
    await expect(
      summary.getByRole("img", {
        name: "Gateway latency histogram",
        exact: true,
      }),
    ).toBeVisible();
    expect(
      await page.evaluate(
        () =>
          JSON.parse(
            localStorage.getItem("pulsebid.auction-history.v1") ?? "[]",
          ).length,
      ),
    ).toBe(50);
    expect(browserErrors).toEqual([]);
    // The advertiser iframe deliberately forbids scripts. Scan the operator DOM;
    // sandbox/title/passive-tracking assertions above verify the frame separately.
    const axe = await new AxeBuilder({ page })
      .setLegacyMode()
      .options({ iframes: false })
      .analyze();
    expect(
      axe.violations.filter((item) =>
        ["serious", "critical"].includes(item.impact ?? ""),
      ),
    ).toEqual([]);
    await page.evaluate(() => window.scrollTo(0, 0));
    await page.screenshot({
      path: "docs/screenshots/fe-4-burst.png",
      fullPage: true,
    });
    await page.setViewportSize({ width: 768, height: 1024 });
    await expect(
      page.getByRole("heading", { name: "Auction Simulator", exact: true }),
    ).toBeVisible();
    await page.getByTitle("Sandboxed ad preview").scrollIntoViewIfNeeded();
    await expect(
      page
        .frameLocator('iframe[title="Sandboxed ad preview"]')
        .getByText("Simulator test ad"),
    ).toBeVisible();
    await page.screenshot({
      path: "docs/screenshots/fe-4-tablet.png",
      fullPage: false,
    });
    page.once("dialog", (dialog) => dialog.accept());
    await page
      .getByRole("button", { name: "Clear history", exact: true })
      .click();
    await expect(
      page.getByText("No auctions saved yet.", { exact: true }),
    ).toBeVisible();
  } finally {
    test.setTimeout(test.info().timeout + 30000);
    for (const id of campaignIds)
      await page.request.delete(`/api/campaign/campaigns/${id}`, {
        headers: origin,
      });
    if (advertiserId)
      await page.request.delete(`/api/campaign/advertisers/${advertiserId}`, {
        headers: origin,
      });
  }
});
