import { test, expect } from "@playwright/test";

test("Jaeger serves the real trace search UI and its assets", async ({
  page,
}) => {
  const url = process.env.NEXT_PUBLIC_JAEGER_URL ?? "http://localhost:16686";
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  page.on("response", (response) => {
    if (
      ["script", "stylesheet"].includes(response.request().resourceType()) &&
      !response.ok()
    )
      errors.push(`UI asset returned ${response.status()}`);
  });

  const response = await page.goto(url);
  expect(response?.status()).toBe(200);
  await expect(page).toHaveTitle(/Jaeger UI/);
  await expect(
    page.getByRole("button", { name: "Find Traces", exact: true }),
  ).toBeVisible();
  await expect(
    page.getByText("This is not the Jaeger UI you are looking for!", {
      exact: false,
    }),
  ).toHaveCount(0);
  const services = await page.request.get(
    new URL("api/services", `${url.replace(/\/$/, "")}/`).toString(),
  );
  expect(services.status()).toBe(200);
  expect(Array.isArray((await services.json()).data)).toBe(true);
  expect(errors).toEqual([]);
});
