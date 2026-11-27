import { test, expect } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
test("advertiser and bidder CRUD against the real BFF", async ({
  page,
  baseURL,
}) => {
  test.setTimeout(120000);
  const suffix = `${Date.now()}-${Math.random().toString(36).slice(2, 7)}`;
  const advertiserName = `e2e-advertiser-${suffix}`;
  const updatedAdvertiser = `${advertiserName}-updated`;
  const bidderId = `e2e-dsp-${suffix}`;
  const bidderName = `e2e-bidder-${suffix}`;
  const updatedBidder = `${bidderName}-updated`;
  let advertiserId: string | undefined;
  let advertiserArchived = false;
  let bidderCreated = false;
  let bidderDeleted = false;
  const checkA11y = async () => {
    const result = await new AxeBuilder({ page }).analyze();
    expect(
      result.violations.filter((item) =>
        ["serious", "critical"].includes(item.impact ?? ""),
      ),
    ).toEqual([]);
  };
  if (process.env.E2E_AUTH_DISABLED !== "true") {
    await page.goto("/login");
    await page.getByLabel("Password", { exact: true }).fill("e2e-password");
    await page.getByRole("button", { name: "Sign in", exact: true }).click();
    await expect(page).toHaveURL(/\/$/);
  }
  try {
    await page.goto("/advertisers");
    await page
      .getByRole("button", { name: "New advertiser", exact: true })
      .click();
    const dialog = page.getByRole("dialog");
    await dialog.getByLabel("Name", { exact: true }).fill(advertiserName);
    await dialog.getByLabel("Contact email").fill("ads@example.test");
    const created = page.waitForResponse(
      (response) =>
        response.url().endsWith("/api/campaign/advertisers") &&
        response.request().method() === "POST",
    );
    await dialog
      .getByRole("button", { name: "Create advertiser", exact: true })
      .click();
    const creation = await created;
    expect(creation.status()).toBe(201);
    advertiserId = (await creation.json()).id;
    await expect(dialog).not.toBeVisible();
    const advertiserRow = () =>
      page.getByRole("row").filter({
        has: page.getByRole("button", { name: advertiserName, exact: true }),
      });
    await expect(advertiserRow()).toBeVisible();
    await advertiserRow()
      .getByRole("button", { name: `Edit ${advertiserName}`, exact: true })
      .click();
    await dialog.getByLabel("Name", { exact: true }).fill(updatedAdvertiser);
    await dialog.getByLabel("Contact email").fill("");
    await dialog
      .getByRole("button", { name: "Save changes", exact: true })
      .click();
    await expect(dialog).not.toBeVisible();
    await page.getByLabel("Search loaded records").fill(updatedAdvertiser);
    const updatedAdvertiserRow = () =>
      page.getByRole("row").filter({
        has: page.getByRole("button", {
          name: updatedAdvertiser,
          exact: true,
        }),
      });
    await expect(updatedAdvertiserRow()).toContainText("No contact email");
    await checkA11y();
    await page.screenshot({
      path: "docs/screenshots/fe-2-advertisers.png",
      fullPage: true,
    });
    await page
      .getByRole("button", { name: "New advertiser", exact: true })
      .click();
    await dialog.getByLabel("Name", { exact: true }).fill(updatedAdvertiser);
    await dialog
      .getByRole("button", { name: "Create advertiser", exact: true })
      .click();
    await expect(dialog.locator("#form-server-error")).toContainText(
      "Request ID",
    );
    page.once("dialog", (alert) => alert.accept());
    await dialog.getByRole("button", { name: "Cancel", exact: true }).click();
    await expect(dialog).not.toBeVisible();
    await page.goto("/bidders");
    await page.getByRole("button", { name: "New bidder", exact: true }).click();
    await dialog.getByLabel("Bidder ID", { exact: true }).fill(bidderId);
    await dialog.getByLabel("Name", { exact: true }).fill(bidderName);
    await dialog
      .getByLabel("Endpoint URL", { exact: true })
      .fill("http://localhost:9101/bid");
    const bidderResponse = page.waitForResponse(
      (response) =>
        response.url().endsWith("/api/campaign/bidders") &&
        response.request().method() === "POST",
    );
    await dialog
      .getByRole("button", { name: "Create bidder", exact: true })
      .click();
    expect((await bidderResponse).status()).toBe(201);
    bidderCreated = true;
    await expect(dialog).not.toBeVisible();
    await page.getByLabel("Search loaded records").fill(bidderId);
    const bidderRow = () =>
      page.getByRole("row").filter({
        has: page.getByRole("button", { name: bidderName, exact: true }),
      });
    await expect(bidderRow()).toContainText("Gateway default");
    await expect(bidderRow()).toContainText("Healthy");
    await bidderRow()
      .getByRole("button", { name: `Edit ${bidderName}`, exact: true })
      .click();
    await expect(
      dialog.getByLabel("Bidder ID", { exact: true }),
    ).toHaveAttribute("readonly");
    await dialog.getByLabel("Name", { exact: true }).fill(updatedBidder);
    await dialog.getByLabel("Timeout (ms)").fill("75");
    await dialog
      .getByRole("button", { name: "Save changes", exact: true })
      .click();
    await expect(dialog).not.toBeVisible();
    const updatedBidderRow = () =>
      page.getByRole("row").filter({
        has: page.getByRole("button", { name: updatedBidder, exact: true }),
      });
    await expect(updatedBidderRow()).toContainText("75 ms");
    await updatedBidderRow()
      .getByRole("button", { name: `Disable ${updatedBidder}`, exact: true })
      .click();
    await expect(
      updatedBidderRow().getByText("Disabled", { exact: true }),
    ).toBeVisible();
    await updatedBidderRow()
      .getByRole("button", { name: `Enable ${updatedBidder}`, exact: true })
      .click();
    await expect(
      updatedBidderRow().getByText("Enabled", { exact: true }),
    ).toBeVisible();
    await expect(
      updatedBidderRow().getByRole("link", { name: "Quick tune" }),
    ).toHaveAttribute("href", "/lab?bidder=0");
    await checkA11y();
    await page.screenshot({
      path: "docs/screenshots/fe-2-bidders.png",
      fullPage: true,
    });
    await page.setViewportSize({ width: 768, height: 1024 });
    await page.screenshot({
      path: "docs/screenshots/fe-2-bidders-tablet.png",
      fullPage: true,
    });
    await page.setViewportSize({ width: 1280, height: 720 });
    await updatedBidderRow()
      .getByRole("button", { name: `Delete ${updatedBidder}`, exact: true })
      .click();
    await dialog.getByRole("button", { name: "Delete", exact: true }).click();
    await expect(dialog).not.toBeVisible();
    await expect(updatedBidderRow()).not.toBeVisible();
    bidderDeleted = true;
    expect(
      (await page.request.get(`/api/campaign/bidders/${bidderId}`)).status(),
    ).toBe(404);
    await page.goto("/advertisers");
    await page.getByLabel("Search loaded records").fill(updatedAdvertiser);
    await updatedAdvertiserRow()
      .getByRole("button", {
        name: `Archive ${updatedAdvertiser}`,
        exact: true,
      })
      .click();
    await dialog.getByRole("button", { name: "Archive", exact: true }).click();
    await expect(dialog).not.toBeVisible();
    await expect(
      updatedAdvertiserRow().getByText("Archived", { exact: true }),
    ).toBeVisible();
    advertiserArchived = true;
    const result = await page.request.get(
      `/api/campaign/advertisers/${advertiserId}`,
    );
    expect(result.status()).toBe(200);
    expect(await result.json()).toMatchObject({
      status: "archived",
      contact_email: null,
      name: updatedAdvertiser,
    });
  } finally {
    const headers = { Origin: baseURL! };
    if (bidderCreated && !bidderDeleted)
      await page.request.delete(`/api/campaign/bidders/${bidderId}`, {
        headers,
      });
    if (advertiserId && !advertiserArchived)
      await page.request.delete(`/api/campaign/advertisers/${advertiserId}`, {
        headers,
      });
  }
});
