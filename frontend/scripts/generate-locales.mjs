import { readFile, writeFile } from "node:fs/promises";

// Generates frontend/src/i18n/locales.generated.ts from the single source of
// truth (backend/localization/locales.json). Run after adding a language:
//   pnpm run generate:locales
// verify-locales.mjs (part of `pnpm run check`) guards that the mirror matches.

const backendLocalesUrl = new URL(
  "../../backend/localization/locales.json",
  import.meta.url,
);
const generatedLocalesUrl = new URL(
  "../src/i18n/locales.generated.ts",
  import.meta.url,
);

const backendLocales = JSON.parse(await readFile(backendLocalesUrl, "utf8"));
const generated = `export const localeDefinitions = ${JSON.stringify(
  backendLocales,
  null,
  2,
)
  .replaceAll('"code"', "code")
  .replaceAll('"label"', "label")
  .replaceAll('"fallback"', "fallback")} as const;\n`;

await writeFile(generatedLocalesUrl, generated, "utf8");
console.log("Wrote src/i18n/locales.generated.ts from backend locales.json");
