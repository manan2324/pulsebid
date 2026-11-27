import { spawn } from "node:child_process";
import { createRequire } from "node:module";
const require = createRequire(import.meta.url);
const child = spawn(
  process.execPath,
  [require.resolve("next/dist/bin/next"), "build"],
  {
    stdio: "inherit",
    windowsHide: true,
    env: { ...process.env, ANALYZE: "true" },
  },
);
child.on("exit", (code) => {
  process.exitCode = code ?? 1;
});
