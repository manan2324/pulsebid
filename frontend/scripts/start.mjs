import fs from "node:fs/promises";
import path from "node:path";
import { spawn } from "node:child_process";
import nextEnv from "@next/env";
nextEnv.loadEnvConfig(process.cwd(), false);
const directory = path.resolve(".next/standalone");
await fs.access(path.join(directory, "server.js"));
await fs.cp(
  path.resolve(".next/static"),
  path.join(directory, ".next/static"),
  { recursive: true },
);
await fs.cp(path.resolve("public"), path.join(directory, "public"), {
  recursive: true,
});
const child = spawn(process.execPath, [path.join(directory, "server.js")], {
  stdio: "inherit",
  windowsHide: true,
  env: {
    ...process.env,
    PORT: process.env.PORT ?? "3001",
    HOSTNAME: "0.0.0.0",
  },
});
for (const signal of ["SIGINT", "SIGTERM"])
  process.on(signal, () => child.kill(signal));
child.on("exit", (code) => {
  process.exitCode = code ?? 1;
});
