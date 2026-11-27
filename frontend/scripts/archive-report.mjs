import fs from "node:fs/promises";
import path from "node:path";
import { createHash } from "node:crypto";
import { execFileSync } from "node:child_process";
import { chromium } from "@playwright/test";
const results = JSON.parse(
  await fs.readFile("test-results/results.json", "utf8"),
);
const date = new Date().toISOString().slice(0, 10);
const destination = path.resolve(`../docs/test-reports/${date}/frontend`);
await fs.mkdir(destination, { recursive: true });
for (const [source, target] of [
  ["playwright-report", "html"],
  ["docs/screenshots", "screenshots"],
  ["docs/performance", "performance"],
  ["coverage", "coverage"],
]) {
  try {
    await fs.access(source);
    await fs.cp(source, path.join(destination, target), { recursive: true });
  } catch (error) {
    if (error.code !== "ENOENT") throw error;
  }
}
for (const file of ["junit.xml", "results.json"])
  await fs.copyFile(`test-results/${file}`, path.join(destination, file));
const hash = createHash("sha256");
async function digest(directory) {
  for (const entry of (
    await fs.readdir(directory, { withFileTypes: true })
  ).sort((a, b) => a.name.localeCompare(b.name))) {
    const file = path.join(directory, entry.name);
    if (entry.isDirectory()) await digest(file);
    else {
      hash.update(file.replaceAll("\\", "/"));
      hash.update(await fs.readFile(file));
    }
  }
}
await digest("src");
await digest("scripts");
await digest("e2e");
for (const file of [
  "package.json",
  "package-lock.json",
  "next.config.ts",
  "Dockerfile",
  "playwright.config.ts",
  "vitest.config.ts",
  "eslint.config.mjs",
  "../docker-compose.yml",
  "../.github/workflows/frontend.yml",
])
  hash.update(await fs.readFile(file));
const git = (directory) => {
  try {
    return execFileSync("git", ["-C", directory, "rev-parse", "HEAD"], {
      encoding: "utf8",
      stdio: ["ignore", "pipe", "ignore"],
      windowsHide: true,
    }).trim();
  } catch {
    return null;
  }
};
const browser = await chromium.launch({ headless: true });
const browserVersion = browser.version();
await browser.close();
const metadata = {
  date,
  frontendGitSha: git("."),
  backendGitSha: git("../Backend"),
  frontendSourceSha256: hash.digest("hex"),
  browser: `Chromium ${browserVersion}`,
  node: process.version,
  stats: results.stats,
};
await fs.writeFile(
  path.join(destination, "metadata.json"),
  JSON.stringify(metadata, null, 2),
);
const { expected = 0, unexpected = 0, skipped = 0, flaky = 0 } = results.stats;
await fs.writeFile(
  path.join(destination, "README.md"),
  `# Frontend acceptance — ${date}\n\n${expected} passed, ${unexpected} failed, ${skipped} skipped, ${flaky} flaky. ${metadata.browser}; Node ${metadata.node}.\n\nFrontend Git SHA: ${metadata.frontendGitSha ?? "unavailable — workspace root/frontend is not a Git checkout"}. Backend Git SHA: ${metadata.backendGitSha ?? "unavailable"}. Frontend source SHA-256: ${metadata.frontendSourceSha256}.\n\n- [Interactive HTML report](./html/index.html)\n- [JUnit](./junit.xml) and [JSON](./results.json)\n- [Screenshots](./screenshots/)\n- [Lighthouse and bundle measurements](./performance/)\n- [Library coverage](./coverage/index.html)\n\nRun against the real backend stack with the authenticated production console. Tests use isolated fixtures, archive/delete their own catalog records and restore mock bidder settings. API credentials remain server-side. Lighthouse uses provided local desktop conditions; saved authentication headers are redacted. CI workflow is included in the workspace; this report records local execution, not a remote CI run.\n`,
);
console.log(`Archived frontend acceptance at ${destination}`);
if (unexpected || flaky || !expected) process.exitCode = 1;
