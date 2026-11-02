import { readFile } from "node:fs/promises";
import { synchronizeMessageFacades } from "./generate-message-index.mjs";
import { assertMessageParity } from "./message-sources.mjs";

const backendLocalesUrl = new URL(
  "../../backend/modules/delivery/internal/adapters/localization/locales.json",
  import.meta.url,
);
const generatedLocalesUrl = new URL(
  "../src/i18n/locales.generated.ts",
  import.meta.url,
);

const backendLocales = JSON.parse(await readFile(backendLocalesUrl, "utf8"));
const expected = `export const localeDefinitions = ${JSON.stringify(
  backendLocales,
  null,
  2,
)
  .replaceAll('"code"', "code")
  .replaceAll('"label"', "label")
  .replaceAll('"fallback"', "fallback")} as const;\n`;

const actual = await readFile(generatedLocalesUrl, "utf8");

if (actual !== expected) {
  console.error(
    "frontend/src/i18n/locales.generated.ts is out of sync with backend/modules/delivery/internal/adapters/localization/locales.json",
  );
  console.error(
    "Update the generated frontend locale mirror before running checks.",
  );
  process.exit(1);
}

// Every supported locale must have exactly the fallback's global leaf keys.
// Generated facades are static type-preserving derivatives, never hand lists.
const catalogs = await synchronizeMessageFacades({
  locales: backendLocales.map((locale) => locale.code),
  check: true,
});
const fallback = backendLocales.find((locale) => locale.fallback)?.code ?? "de";
assertMessageParity(catalogs, fallback);
