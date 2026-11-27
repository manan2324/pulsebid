import fs from "node:fs/promises";
import path from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";
import openapiTS, { astToString } from "openapi-typescript";
const root = fileURLToPath(new URL("../", import.meta.url));
const directory = path.resolve(root, "../Backend/api/openapi");
const output = path.resolve(root, "src/lib/api/generated");
await fs.mkdir(output, { recursive: true });
const files = (await fs.readdir(directory))
  .filter((file) => file.endsWith(".yaml"))
  .sort();
if (!files.length)
  console.log(
    "No backend OpenAPI files found; use the Zod schemas until specs exist.",
  );
for (const file of files) {
  const ast = await openapiTS(pathToFileURL(path.join(directory, file)));
  const destination = path.join(output, file.replace(/\.yaml$/, ".ts"));
  const content = astToString(ast);
  if (process.argv.includes("--check")) {
    const existing = await fs.readFile(destination, "utf8").catch(() => "");
    if (
      existing.replaceAll("\r\n", "\n") !== content.replaceAll("\r\n", "\n")
    ) {
      console.error(`API contract drift: ${file}`);
      process.exitCode = 1;
    } else console.log(`Verified ${file}`);
  } else {
    await fs.writeFile(destination, content);
    console.log(`Generated ${file}`);
  }
}
