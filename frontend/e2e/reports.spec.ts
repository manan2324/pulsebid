import { test, expect } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { readFile } from "node:fs/promises";
import { createSession } from "../src/lib/server/session";
import type { Bidder, ReportResponse } from "../src/lib/api/types";
import type { BidResponse } from "../src/lib/api/gateway";
test.use({ actionTimeout: 30000, navigationTimeout: 45000 });
test("deduplicated delivery in Reports, CSV, UTC filters and API-equal Dashboard", async ({
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
  const name = `e2e-report-${suffix}`;
  const placement = `e2e-report-placement-${suffix}`;
  const origin = { Origin: baseURL! };
  let advertiserId: string | undefined;
  let campaignId: string | undefined;
  const post = async (path: string, data: unknown, status = 201) => {
    const response = await page.request.post(path, { headers: origin, data });
    expect(response.status(), await response.text()).toBe(status);
    return response;
  };
  try {
    advertiserId = (
      await (
        await post("/api/campaign/advertisers", {
          name: `e2e-report-advertiser-${suffix}`,
        })
      ).json()
    ).id;
    const bidders: { data: Bidder[] } = await (
      await page.request.get("/api/campaign/bidders?limit=200")
    ).json();
    const bidder = bidders.data.find(
      (item) => item.enabled && new URL(item.endpoint_url).port === "9101",
    );
    expect(bidder).toBeDefined();
    campaignId = (
      await (
        await post("/api/campaign/campaigns", {
          name,
          advertiser_id: advertiserId,
          bidder_id: bidder!.id,
          daily_budget: "100",
          max_bid_cpm: "3",
          pacing_mode: "asap",
        })
      ).json()
    ).id;
    expect(
      (
        await page.request.put(
          `/api/campaign/campaigns/${campaignId}/targeting`,
          {
            headers: origin,
            data: {
              countries: ["IN"],
              device_types: ["mobile"],
              placement_ids: [placement],
              hours_of_day: [],
              timezone: "UTC",
            },
          },
        )
      ).status(),
    ).toBe(200);
    await post(`/api/campaign/campaigns/${campaignId}/creatives`, {
      name: `report-banner-${suffix}`,
      format: "banner",
      width: 300,
      height: 250,
      markup:
        '<div style="padding:20px">Reporting test ad</div><a href="{click_url}">Learn more</a><img src="{impression_url}">',
      landing_url: "https://example.test/report-landing",
      status: "active",
    });
    await post(`/api/campaign/campaigns/${campaignId}/activate`, {}, 200);
    const request = {
      id: `report-request-${suffix}`,
      imp: {
        id: "imp-1",
        placement_id: placement,
        w: 300,
        h: 250,
        bidfloor: 1,
        bidfloorcur: "USD",
      },
      device: { type: "mobile" },
      geo: { country: "IN" },
      tmax: 100,
    };
    await expect
      .poll(
        async () => {
          const response = await page.request.post("/api/gateway/bid", {
            headers: origin,
            data: request,
          });
          if (response.status() !== 200) return null;
          const result: BidResponse = await response.json();
          return { id: result.campaign_id, price: result.price };
        },
        { timeout: 15000, intervals: [250, 500, 1000] },
      )
      .toEqual({ id: campaignId, price: 1.01 });
    await page.goto("/simulator");
    await page.getByLabel("Placement ID", { exact: true }).fill(placement);
    await page.getByLabel("Bid floor (USD CPM)", { exact: true }).fill("1");
    const auction = page.waitForResponse(
      (response) =>
        response.url().endsWith("/api/gateway/bid") &&
        response.request().method() === "POST",
    );
    await page.getByRole("button", { name: "Send bid", exact: true }).click();
    const bid = await auction;
    expect(bid.status()).toBe(200);
    expect((await bid.json()).campaign_id).toBe(campaignId);
    const result = page.locator(".simulator-result");
    await result
      .getByRole("button", { name: "Fire impression", exact: true })
      .click();
    await expect(
      result.getByRole("button", { name: "Impression sent", exact: true }),
    ).toBeDisabled();
    for (let index = 0; index < 2; index++) {
      const duplicate = page.waitForResponse((response) =>
        response.url().includes("/api/gateway/impression?"),
      );
      await result
        .getByRole("button", {
          name: "Fire impression again (duplicate test)",
          exact: true,
        })
        .click();
      expect((await duplicate).status()).toBe(200);
    }
    await result
      .getByRole("button", { name: "Fire click", exact: true })
      .click();
    await expect(
      result.getByRole("link", {
        name: "https://example.test/report-landing",
        exact: true,
      }),
    ).toBeVisible();
    const start = new Date();
    start.setUTCHours(0, 0, 0, 0);
    const end = new Date(start.getTime() + 86400000);
    const query = new URLSearchParams({
      from: start.toISOString(),
      to: end.toISOString(),
      granularity: "total",
      campaign_id: campaignId!,
    });
    await expect
      .poll(
        async () => {
          const response = await page.request.get(
            `/api/reporting/reports/campaigns?${query}`,
          );
          expect(response.status()).toBe(200);
          const report: ReportResponse = await response.json();
          return {
            impressions: report.totals.impressions,
            clicks: report.totals.clicks,
            spend: report.totals.spend,
          };
        },
        { timeout: 20000 },
      )
      .toEqual({ impressions: 1, clicks: 1, spend: 0.00101 });
    await page.goto(`/reports?campaign_id=${campaignId}&granularity=total`);
    await expect(
      page.getByRole("heading", { name: "Reports", exact: true }),
    ).toBeVisible();
    await expect(page.getByTestId("report-impressions")).toHaveText("1");
    await expect(page.getByTestId("report-clicks")).toHaveText("1");
    await expect(page.getByTestId("report-spend")).toHaveText("$0.00101");
    await expect(page.getByRole("link", { name, exact: true })).toBeVisible();
    const downloadPromise = page.waitForEvent("download");
    await page.getByRole("button", { name: "Export CSV", exact: true }).click();
    const download = await downloadPromise;
    const csv = await readFile((await download.path())!, "utf8");
    expect(csv).toContain("campaign_id,campaign,bucket_utc");
    expect(csv).toContain(name);
    expect(csv).toContain(",1,1,1,0.00101,USD,");
    expect(csv).toContain(start.toISOString().slice(0, 19) + "Z");
    await page
      .getByRole("combobox", { name: "Granularity", exact: true })
      .selectOption("hour");
    await page
      .getByRole("button", { name: "Apply filters", exact: true })
      .click();
    await expect
      .poll(() => new URL(page.url()).searchParams.get("granularity") ?? "hour")
      .toBe("hour");
    await expect(
      page.getByRole("img", {
        name: "Delivery over time: Impressions",
        exact: true,
      }),
    ).toBeVisible();
    await page
      .getByRole("combobox", { name: "Chart metric", exact: true })
      .selectOption("spend");
    await expect(
      page.getByRole("img", { name: "Delivery over time: Spend", exact: true }),
    ).toBeVisible();
    await page.getByText("Show chart data", { exact: true }).click();
    await page
      .getByRole("combobox", { name: "Date range", exact: true })
      .selectOption("yesterday");
    await page
      .getByRole("button", { name: "Apply filters", exact: true })
      .click();
    await expect(page.getByTestId("report-impressions")).toHaveText("0");
    await expect(
      page.getByRole("heading", {
        name: "No events in this range",
        exact: true,
      }),
    ).toBeVisible();
    await page.goBack();
    await expect(page.getByTestId("report-impressions")).toHaveText("1");
    await expect(
      page.getByRole("combobox", { name: "Granularity", exact: true }),
    ).toHaveValue("hour");
    const fromInput = page.getByLabel("From (UTC, inclusive)", { exact: true });
    await fromInput.fill(
      new Date(start.getTime() - 32 * 86400000).toISOString().slice(0, 16),
    );
    await expect(
      page.getByRole("button", { name: "Apply filters", exact: true }),
    ).toBeDisabled();
    await expect(
      page.getByRole("option", { name: "Hour", exact: true }),
    ).toBeDisabled();
    await page
      .getByRole("combobox", { name: "Granularity", exact: true })
      .selectOption("day");
    await page
      .getByRole("button", { name: "Apply filters", exact: true })
      .click();
    await expect(page.getByTestId("report-impressions")).toHaveText("1");
    await expect
      .poll(() => new URL(page.url()).searchParams.get("preset"))
      .toBe("custom");
    await page.evaluate(() => window.scrollTo(0, 0));
    await page.screenshot({
      path: "docs/screenshots/fe-5-reports.png",
      fullPage: true,
    });
    const reportsAxe = await new AxeBuilder({ page }).analyze();
    expect(
      reportsAxe.violations.filter((item) =>
        ["serious", "critical"].includes(item.impact ?? ""),
      ),
    ).toEqual([]);
    let liveCalls = 0;
    page.on("response", (response) => {
      const url = new URL(response.url());
      if (
        url.pathname === "/api/prom/query" &&
        url.searchParams.get("name") === "bid_rps" &&
        response.status() === 200
      )
        liveCalls++;
    });
    const dashboardResponse = page.waitForResponse((response) => {
      const url = new URL(response.url());
      return (
        url.pathname === "/api/reporting/reports/campaigns" &&
        url.searchParams.get("granularity") === "total" &&
        !url.searchParams.has("campaign_id") &&
        Date.parse(url.searchParams.get("from") ?? "") === start.getTime()
      );
    });
    await page.goto("/");
    const totals: ReportResponse = await (await dashboardResponse).json();
    for (const metric of [
      "impressions",
      "clicks",
      "ctr",
      "spend",
      "win_rate",
    ] as const)
      await expect(page.getByTestId(`dashboard-${metric}`)).toHaveAttribute(
        "data-value",
        String(totals.totals[metric]),
      );
    await post("/api/gateway/bid", request, 200);
    await expect
      .poll(() => liveCalls, { timeout: 15000 })
      .toBeGreaterThanOrEqual(2);
    // A newly created counter needs two Prometheus scrapes before rate() is finite.
    await expect(
      page.getByText("Bid requests / second", { exact: true }),
    ).toBeVisible({ timeout: 60000 });
    await expect(
      page.getByRole("heading", { name: "Live exchange", exact: true }),
    ).toBeVisible();
    await expect(
      page.getByRole("heading", { name: "Budget alerts", exact: true }),
    ).toBeVisible();
    await expect(
      page.getByRole("heading", { name: "Service connectivity", exact: true }),
    ).toBeVisible();
    for (const panel of await page.locator(".deferred-panel").all()) { await panel.scrollIntoViewIfNeeded(); await expect(panel.locator("[data-slot=skeleton]")).toHaveCount(0); }
    const dashboardAxe = await new AxeBuilder({ page }).analyze();
    expect(
      dashboardAxe.violations.filter((item) =>
        ["serious", "critical"].includes(item.impact ?? ""),
      ),
    ).toEqual([]);
    expect(browserErrors).toEqual([]);
    await page.evaluate(() => window.scrollTo(0, 0));
    await page.screenshot({
      path: "docs/screenshots/fe-5-dashboard.png",
      fullPage: true,
    });
    await page.setViewportSize({ width: 768, height: 1024 });
    await page.evaluate(() => window.scrollTo(0, 0));
    await page.screenshot({
      path: "docs/screenshots/fe-5-dashboard-tablet.png",
      fullPage: true,
    });
  } finally {
    test.setTimeout(test.info().timeout + 30000);
    if (campaignId)
      expect(
        (
          await page.request.delete(`/api/campaign/campaigns/${campaignId}`, {
            headers: origin,
          })
        ).status(),
      ).toBe(204);
    if (advertiserId)
      expect(
        (
          await page.request.delete(
            `/api/campaign/advertisers/${advertiserId}`,
            { headers: origin },
          )
        ).status(),
      ).toBe(204);
  }
});
