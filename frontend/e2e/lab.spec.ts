import { test, expect } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { createSession } from "../src/lib/server/session";
import { defaultBidderConfig, type BidderConfig } from "../src/lib/lab";
import type { Bidder } from "../src/lib/api/types";
test.use({ actionTimeout: 30000, navigationTimeout: 45000 });
test("slow bidder isolation, scenario exports, safety and restoration", async ({
  page,
  baseURL,
}) => {
  test.setTimeout(240000);
  await page.context().addCookies([
    {
      name: "pb_session",
      value: await createSession("e2e-secret-with-at-least-32-characters"),
      url: baseURL!,
      httpOnly: true,
      sameSite: "Lax",
    },
  ]);
  const headers = { Origin: baseURL! };
  const originals: BidderConfig[] = [];
  const campaigns: string[] = [];
  let advertiser: string | undefined;
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  const post = async (path: string, data: unknown, status = 201) => {
    const r = await page.request.post(path, { headers, data });
    expect(r.status(), await r.text()).toBe(status);
    return r.json();
  };
  for (let index = 0; index < 8; index++)
    originals.push(
      await (await page.request.get(`/api/bidder/${index}/config`)).json(),
    );
  const suffix = `${Date.now()}-${Math.random().toString(36).slice(2, 6)}`,
    placement = `e2e-lab-${suffix}`;
  try {
    advertiser = (
      await post("/api/campaign/advertisers", {
        name: `e2e-lab-advertiser-${suffix}`,
      })
    ).id;
    const bidders: { data: Bidder[] } = await (
      await page.request.get("/api/campaign/bidders?limit=200")
    ).json();
    for (let index = 0; index < 2; index++) {
      const bidder = bidders.data.find(
        (item) =>
          item.enabled &&
          new URL(item.endpoint_url).port === String(9101 + index),
      );
      expect(bidder).toBeDefined();
      const campaign = await post("/api/campaign/campaigns", {
        name: `e2e-lab-${suffix}-${index}`,
        advertiser_id: advertiser,
        bidder_id: bidder!.id,
        daily_budget: "100",
        max_bid_cpm: String(3 - index),
        pacing_mode: "asap",
      });
      campaigns.push(campaign.id);
      expect(
        (
          await page.request.put(
            `/api/campaign/campaigns/${campaign.id}/targeting`,
            {
              headers,
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
      await post(`/api/campaign/campaigns/${campaign.id}/creatives`, {
        name: `e2e-lab-ad-${suffix}-${index}`,
        format: "banner",
        width: 300,
        height: 250,
        markup: "<p>Lab test</p>",
        landing_url: "https://example.test/lab",
        status: "active",
      });
      await post(`/api/campaign/campaigns/${campaign.id}/activate`, {}, 200);
    }
    const request = {
      id: `lab-${suffix}`,
      imp: {
        id: "1",
        placement_id: placement,
        w: 300,
        h: 250,
        bidfloor: 0.5,
        bidfloorcur: "USD",
      },
      device: { type: "mobile" },
      geo: { country: "IN" },
      tmax: 100,
    };
    await expect
      .poll(
        async () => {
          const r = await page.request.post("/api/gateway/bid", {
            headers,
            data: request,
          });
          return r.status() === 200 ? (await r.json()).campaign_id : null;
        },
        { timeout: 20000 },
      )
      .toBe(campaigns[0]);
    await page.goto("/lab");
    await expect(
      page.getByRole("heading", { name: "Resilience Lab", exact: true }),
    ).toBeVisible();
    await page
      .getByRole("button", { name: "Reset all bidders", exact: true })
      .click();
    await expect
      .poll(
        async () =>
          (await (await page.request.get("/api/bidder/0/config")).json())
            .latency_ms,
      )
      .toBe(10);
    await page
      .getByRole("combobox", { name: "Scenario preset", exact: true })
      .selectOption("slow");
    await page
      .getByRole("button", { name: "Apply preset", exact: true })
      .click();
    await expect
      .poll(
        async () =>
          (await (await page.request.get("/api/bidder/0/config")).json())
            .latency_ms,
      )
      .toBe(500);
    await page
      .getByRole("button", { name: "Restore original settings", exact: true })
      .click();
    await expect
      .poll(
        async () =>
          (await (await page.request.get("/api/bidder/0/config")).json())
            .latency_ms,
      )
      .toBe(10);
    await page
      .getByRole("textbox", { name: "Auction request JSON", exact: true })
      .fill(JSON.stringify(request));
    await page
      .getByRole("button", { name: "Run comparison", exact: true })
      .click();
    await expect(page.getByTestId("scenario-verdict")).toHaveText(
      "Resilient: p99 stayed under 120 ms",
      { timeout: 90000 },
    );
    expect(
      parseFloat(await page.getByTestId("scenario-p99").innerText()),
    ).toBeLessThan(120);
    await expect(
      page.getByRole("button", { name: "Run comparison", exact: true }),
    ).toBeEnabled();
    for (let index = 0; index < 8; index++)
      expect(
        await (await page.request.get(`/api/bidder/${index}/config`)).json(),
      ).toEqual(defaultBidderConfig);
    for (const label of ["Export JSON", "Export Markdown"]) {
      const download = page.waitForEvent("download");
      await page.getByRole("button", { name: label, exact: true }).click();
      expect((await download).suggestedFilename()).toContain(
        "pulsebid-resilience",
      );
    }
    await page
      .getByRole("combobox", { name: "Scenario preset", exact: true })
      .selectOption("allHang");
    await page
      .getByRole("button", { name: "Apply preset", exact: true })
      .click();
    await expect(page.getByRole("dialog")).toBeVisible();
    await page.getByRole("button", { name: "Cancel", exact: true }).click();
    await expect(page.getByRole("dialog")).not.toBeVisible();
    await page.evaluate(() => window.scrollTo(0, 0));
    await page.screenshot({
      path: "docs/screenshots/fe-6-lab.png",
      fullPage: true,
    });
    const axe = await new AxeBuilder({ page }).analyze();
    expect(
      axe.violations.filter((v) =>
        ["serious", "critical"].includes(v.impact ?? ""),
      ),
    ).toEqual([]);
    await page
      .getByRole("combobox", { name: "Scenario preset", exact: true })
      .selectOption("slow");
    await page
      .getByRole("button", { name: "Apply preset", exact: true })
      .click();
    await expect
      .poll(
        async () =>
          (await (await page.request.get("/api/bidder/0/config")).json())
            .latency_ms,
      )
      .toBe(500);
    const restoration = page.waitForResponse(
      (response) =>
        response.url().endsWith("/api/lab/reset") &&
        response.request().method() === "POST",
    );
    await page
      .getByRole("navigation")
      .getByRole("link", { name: "System", exact: true })
      .click();
    expect((await restoration).status()).toBe(200);
    await expect
      .poll(
        async () =>
          (await (await page.request.get("/api/bidder/0/config")).json())
            .latency_ms,
      )
      .toBe(10);
    await expect(
      page.getByRole("heading", { name: "Catalog snapshot", exact: true }),
    ).toBeVisible();
    await expect(
      page.getByRole("button", { name: "Refresh snapshot", exact: true }),
    ).toBeEnabled();
    await page.getByText("Inspect snapshot JSON", { exact: true }).click();
    await expect(page.locator(".system-json")).toHaveValue(
      new RegExp(campaigns[0]!),
    );
    const systemAxe = await new AxeBuilder({ page }).analyze();
    expect(
      systemAxe.violations.filter((v) =>
        ["serious", "critical"].includes(v.impact ?? ""),
      ),
    ).toEqual([]);
    expect(errors).toEqual([]);
    await page.evaluate(() => window.scrollTo(0, 0));
    await page.screenshot({
      path: "docs/screenshots/fe-6-system.png",
      fullPage: true,
    });
  } finally {
    test.setTimeout(test.info().timeout + 60000);
    for (const [index, config] of originals.entries())
      expect(
        (
          await page.request.post(`/api/bidder/${index}/config`, {
            headers,
            data: config,
          })
        ).status(),
      ).toBe(200);
    for (const id of campaigns)
      expect(
        (
          await page.request.delete(`/api/campaign/campaigns/${id}`, {
            headers,
          })
        ).status(),
      ).toBe(204);
    if (advertiser)
      expect(
        (
          await page.request.delete(`/api/campaign/advertisers/${advertiser}`, {
            headers,
          })
        ).status(),
      ).toBe(204);
  }
});
