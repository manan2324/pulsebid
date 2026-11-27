import fs from "node:fs/promises";
import path from "node:path";
import nextEnv from "@next/env";
nextEnv.loadEnvConfig(process.cwd());
const secrets = [
  process.env.PULSEBID_API_KEY,
  process.env.ADMIN_PASSWORD,
  process.env.SESSION_SECRET,
].filter(Boolean);
if (secrets.length !== 3)
  throw new Error("Set all three server secrets before running the leak check");
async function scan(directory) {
  for (const item of await fs.readdir(directory, { withFileTypes: true })) {
    const filename = path.join(directory, item.name);
    if (item.isDirectory()) await scan(filename);
    else {
      const content = await fs.readFile(filename, "utf8");
      if (secrets.some((value) => content.includes(value)))
        throw new Error(`Secret found in browser asset: ${filename}`);
    }
  }
}
await scan(".next/static");
console.log("PASS: no server secrets found in browser assets");
