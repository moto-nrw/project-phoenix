import { readFile } from "node:fs/promises";

const backendLocalesUrl = new URL(
  "../../backend/localization/locales.json",
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
    "frontend/src/i18n/locales.generated.ts is out of sync with backend/localization/locales.json",
  );
  console.error("Update the generated frontend locale mirror before running checks.");
  process.exit(1);
}

// Guard message-catalog parity. Every supported locale must define exactly the
// same key set as the German fallback — a missing key would silently render
// German via the runtime merge, and an extra key is dead translation.
function leafKeys(obj, prefix = "") {
  if (obj && typeof obj === "object" && !Array.isArray(obj)) {
    return Object.entries(obj).flatMap(([key, value]) =>
      leafKeys(value, prefix ? `${prefix}.${key}` : key),
    );
  }
  return [prefix];
}

const messagesDirUrl = new URL("../src/i18n/messages/", import.meta.url);
const fallback = backendLocales.find((locale) => locale.fallback)?.code ?? "de";
const fallbackKeys = new Set(
  leafKeys(
    JSON.parse(
      await readFile(new URL(`${fallback}.json`, messagesDirUrl), "utf8"),
    ),
  ),
);

let parityFailed = false;
for (const { code } of backendLocales) {
  if (code === fallback) continue;
  const catalog = JSON.parse(
    await readFile(new URL(`${code}.json`, messagesDirUrl), "utf8"),
  );
  const keys = new Set(leafKeys(catalog));
  const missing = [...fallbackKeys].filter((key) => !keys.has(key));
  const extra = [...keys].filter((key) => !fallbackKeys.has(key));
  if (missing.length || extra.length) {
    parityFailed = true;
    console.error(`src/i18n/messages/${code}.json is out of sync with ${fallback}.json`);
    if (missing.length) console.error(`  missing keys: ${missing.join(", ")}`);
    if (extra.length) console.error(`  unexpected keys: ${extra.join(", ")}`);
  }
}

if (parityFailed) {
  process.exit(1);
}
