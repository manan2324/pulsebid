import { dirname } from "node:path";
import { fileURLToPath } from "node:url";
import { FlatCompat } from "@eslint/eslintrc";
import tailwind from "eslint-plugin-tailwindcss";

const compat = new FlatCompat({
  baseDirectory: dirname(fileURLToPath(import.meta.url)),
});

const eslintConfig = [
  {
    ignores: [
      "node_modules/**",
      ".next/**",
      ".next-dev/**",
      "out/**",
      "build/**",
      "next-env.d.ts",
      "public/mockServiceWorker.js",
      "playwright-report/**",
      "test-results/**",
      "coverage/**",
    ],
  },
  ...compat.extends(
    "next/core-web-vitals",
    "next/typescript",
    "plugin:jsx-a11y/recommended",
  ),
  {
    plugins: { tailwindcss: tailwind },
    rules: {
      "@typescript-eslint/no-explicit-any": "error",
      "tailwindcss/no-contradicting-classname": "error",
    },
    settings: { tailwindcss: { cssConfigPath: "./src/app/globals.css" } },
  },
];

export default eslintConfig;
