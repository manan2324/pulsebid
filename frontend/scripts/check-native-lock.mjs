import fs from "node:fs/promises";
const lock = JSON.parse(await fs.readFile("package-lock.json", "utf8"));
const binary =
  /^(lightningcss-|@img\/sharp-|@rolldown\/binding-|@tailwindcss\/oxide-|@next\/swc-|@rollup\/rollup-|@unrs\/resolver-binding-|@oxc-parser\/binding-)/;
const names = new Set(
  Object.values(lock.packages).flatMap((entry) =>
    Object.keys(entry.optionalDependencies ?? {}).filter((name) =>
      binary.test(name),
    ),
  ),
);
const missing = [...names].filter(
  (name) => !lock.packages[`node_modules/${name}`]?.integrity,
);
if (missing.length) {
  console.error(
    `Missing native lock metadata: ${missing.join(", ")}. Regenerate optional platform entries from a clean install without changing tested versions.`,
  );
  process.exitCode = 1;
} else
  console.log(
    `PASS: complete native lock metadata for ${names.size} platform packages`,
  );
