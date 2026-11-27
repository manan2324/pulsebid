import fs from "node:fs/promises";
import path from "node:path";
import utils from "next/dist/build/utils.js";
const dist = path.resolve(".next");
const build = JSON.parse(
  await fs.readFile(path.join(dist, "build-manifest.json"), "utf8"),
);
const app = JSON.parse(
  await fs.readFile(path.join(dist, "app-build-manifest.json"), "utf8"),
);
const routes = [
  "/",
  "/advertisers",
  "/bidders",
  "/campaigns",
  "/campaigns/new",
  "/campaigns/[id]",
  "/reports",
  "/simulator",
  "/lab",
  "/system",
  "/login",
];
const sizes = {};
for (const route of routes) {
  const [, bytes] = await utils.getJsPageSizeInKb(
    "app",
    route,
    dist,
    build,
    app,
    true,
  );
  sizes[route] = bytes;
  console.log(`${route}: ${bytes} gzip bytes`);
}
await fs.mkdir("docs/performance", { recursive: true });
await fs.writeFile(
  "docs/performance/bundles.json",
  JSON.stringify(
    { at: new Date().toISOString(), limit: 200000, sizes },
    null,
    2,
  ),
);
if (Object.values(sizes).some((bytes) => bytes >= 200000)) {
  console.error("First-load JavaScript exceeds the 200,000-byte budget.");
  process.exitCode = 1;
}
