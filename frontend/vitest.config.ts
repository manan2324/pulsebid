import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";
import { fileURLToPath } from "node:url";
export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./src", import.meta.url)),
      "server-only": fileURLToPath(
        new URL("./src/test/server-only.ts", import.meta.url),
      ),
    },
  },
  test: {
    environment: "node",
    pool: "threads",
    maxWorkers: 1,
    setupFiles: ["./src/test/setup.ts"],
    include: ["src/**/*.test.{ts,tsx}"],
    coverage: {
      provider: "v8",
      include: ["src/lib/**/*.ts"],
      exclude: [
        "src/lib/**/*.test.ts",
        "src/lib/api/generated/**",
        "src/lib/copy/**",
        "src/lib/copy.ts",
        "src/lib/api/types.ts",
      ],
      reporter: ["text", "json-summary", "html"],
      thresholds: { lines: 85, statements: 85 },
    },
    clearMocks: true,
    restoreMocks: true,
    unstubGlobals: true,
  },
});
