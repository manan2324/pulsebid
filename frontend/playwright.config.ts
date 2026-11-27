import { defineConfig, devices } from "@playwright/test";
const port = 3001;

export default defineConfig({
  testDir: "./e2e",
  timeout: 60000,
  expect: { timeout: 15000 },
  fullyParallel: false,
  workers: 1,
  retries: process.env.CI ? 1 : 0,
  reporter: [
    ["list"],
    ["html", { open: "never" }],
    ["junit", { outputFile: "test-results/junit.xml" }],
    ["json", { outputFile: "test-results/results.json" }],
  ],
  use: {
    baseURL: `http://localhost:${port}`,
    trace: "on-first-retry",
    screenshot: "only-on-failure",
  },
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }],
  webServer:
    process.env.E2E_EXTERNAL_SERVER === "true"
      ? undefined
      : {
          command:
            process.env.E2E_PRODUCTION === "true"
              ? "npm run start"
              : "npm run dev",
          url: `http://localhost:${port}/login`,
          reuseExistingServer: false,
          stdout: "pipe",
          timeout: 300000,
          env: {
            AUTH_DISABLED: process.env.E2E_AUTH_DISABLED ?? "false",
            PULSEBID_API_KEY: process.env.PULSEBID_API_KEY ?? "dev-key",
            ADMIN_PASSWORD: "e2e-password",
            SESSION_SECRET: "e2e-secret-with-at-least-32-characters",
          },
        },
});
