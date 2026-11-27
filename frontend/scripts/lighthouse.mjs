import fs from "node:fs/promises";
import { spawn } from "node:child_process";

import lighthouse, { generateReport } from "lighthouse";
import { launch } from "chrome-launcher";
import { chromium } from "@playwright/test";
import { SignJWT } from "jose";
import nextEnv from "@next/env";
nextEnv.loadEnvConfig(process.cwd(), false);
await fs.cp(".next/static", ".next/standalone/.next/static", {
  recursive: true,
});
await fs.cp("public", ".next/standalone/public", { recursive: true });
const occupied = await fetch("http://localhost:3001/login", {
  signal: AbortSignal.timeout(1000),
}).then(
  () => true,
  () => false,
);
if (occupied)
  throw new Error("Stop the console on port 3001 before running Lighthouse");
const secret = "lighthouse-session-secret-at-least-32-characters";
const server = spawn(process.execPath, [".next/standalone/server.js"], {
  windowsHide: true,
  stdio: "ignore",
  env: {
    ...process.env,
    PORT: "3001",
    HOSTNAME: "127.0.0.1",
    AUTH_DISABLED: "false",
    ADMIN_PASSWORD: "lighthouse-password",
    SESSION_SECRET: secret,
  },
});
let chrome;
try {
  for (let attempt = 0; ; attempt++) {
    try {
      if ((await fetch("http://localhost:3001/login")).ok) break;
    } catch {}
    if (attempt >= 120) throw new Error("Production server did not start");
    await new Promise((resolve) => setTimeout(resolve, 500));
  }
  const token = await new SignJWT({ role: "operator" })
    .setProtectedHeader({ alg: "HS256" })
    .setSubject("operator")
    .setIssuer("pulsebid-console")
    .setAudience("pulsebid-console")
    .setIssuedAt()
    .setExpirationTime("10m")
    .sign(new TextEncoder().encode(secret));
  chrome = await launch({
    chromePath: chromium.executablePath(),
    chromeFlags: ["--headless=new", "--no-sandbox", "--disable-gpu"],
    logLevel: "silent",
  });
  await fs.mkdir("docs/performance", { recursive: true });
  for (const [name, route] of [
    ["dashboard", "/"],
    ["campaigns", "/campaigns"],
  ]) {
    const result = await lighthouse(`http://localhost:3001${route}`, {
      port: chrome.port,
      logLevel: "error",
      onlyCategories: ["performance", "accessibility"],
      formFactor: "desktop",
      screenEmulation: {
        mobile: false,
        width: 1440,
        height: 900,
        deviceScaleFactor: 1,
        disabled: false,
      },
      throttlingMethod: "provided",
      extraHeaders: { Cookie: `pb_session=${token}` },
    });
    if (!result || result.lhr.runtimeError)
      throw new Error(
        result?.lhr.runtimeError?.message ?? "No Lighthouse result",
      );
    result.lhr.configSettings.extraHeaders = { Cookie: "[redacted]" };
    await fs.writeFile(
      `docs/performance/${name}.json`,
      JSON.stringify(result.lhr, null, 2),
    );
    await fs.writeFile(
      `docs/performance/${name}.html`,
      generateReport(result.lhr, "html"),
    );
    const performance = Math.round(
        result.lhr.categories.performance.score * 100,
      ),
      accessibility = Math.round(
        result.lhr.categories.accessibility.score * 100,
      ),
      lcp = result.lhr.audits["largest-contentful-paint"].numericValue;
    console.log(
      `${name}: performance=${performance} accessibility=${accessibility} LCP=${Math.round(lcp)}ms (desktop, provided local conditions)`,
    );
    if (
      performance < 90 ||
      accessibility < 95 ||
      (name === "dashboard" && lcp >= 2000)
    )
      process.exitCode = 1;
  }
} finally {
  if (chrome) await chrome.kill();
  server.kill();
}
