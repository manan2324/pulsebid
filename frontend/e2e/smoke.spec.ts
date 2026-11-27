import { test, expect } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
test("shell navigation, theme, timezone, tablet and accessibility", async ({
  page,
}) => {
  if (process.env.E2E_AUTH_DISABLED !== "true") {
    await page.goto("/login");
    await page.getByLabel("Password", { exact: true }).fill("e2e-password");
    await page.getByRole("button", { name: "Sign in", exact: true }).click();
    await expect(page).toHaveURL(/\/$/);
  }
  await page.goto("/");
  await expect(
    page.getByRole("heading", { name: "Dashboard", exact: true }),
  ).toBeVisible();
  for (const name of [
    "Dashboard",
    "Campaigns",
    "Advertisers",
    "Bidders",
    "Reports",
    "Simulator",
    "Resilience Lab",
    "System",
  ])
    await expect(
      page.getByRole("navigation").getByRole("link", { name, exact: true }),
    ).toBeVisible();
  await page.getByRole("button", { name: "Change theme", exact: true }).click();
  await page.getByRole("button", { name: "Change display timezone" }).click();
  await expect(
    page.getByRole("button", { name: "Change display timezone" }),
  ).toHaveText("Local");
  await page
    .getByRole("navigation")
    .getByRole("link", { name: "Campaigns", exact: true })
    .click();
  await expect(
    page.getByRole("heading", { name: "Campaigns", exact: true }),
  ).toBeVisible();
  await page.goto("/");
  await expect(
    page.getByRole("heading", { name: "Service connectivity" }),
  ).toBeVisible();
  await page.screenshot({
    path: "docs/screenshots/fe-0-desktop.png",
    fullPage: true,
  });
  const results = await new AxeBuilder({ page }).analyze();
  expect(
    results.violations.filter((item) =>
      ["serious", "critical"].includes(item.impact ?? ""),
    ),
  ).toEqual([]);
  await page.setViewportSize({ width: 768, height: 1024 });
  await page.getByRole("button", { name: "Toggle navigation" }).click();
  await page
    .getByRole("navigation")
    .getByRole("link", { name: "Advertisers", exact: true })
    .click();
  await expect(
    page.getByRole("heading", { name: "Advertisers", exact: true }),
  ).toBeVisible();
  await page.screenshot({
    path: "docs/screenshots/fe-0-tablet.png",
    fullPage: true,
  });
});
