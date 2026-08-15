// Einmalwerkzeug fuer den Eltern-App-Umbau: fuegt Katalogeintraege in alle
// vier Sprachdateien ein, ohne die Formatierung zu veraendern.
// Aufruf: node scripts/apply-parent-locales.mjs <patch.json>
// Der Patch ist { de: {...}, en: {...}, ru: {...}, sq: {...} } und wird je
// Sprache flach in die Wurzel des Katalogs gemischt (tiefes Merge pro Namespace).
import { readFile, writeFile } from "node:fs/promises";

const patchPath = process.argv[2];
if (!patchPath) {
  console.error("Usage: node scripts/apply-parent-locales.mjs <patch.json>");
  process.exit(1);
}

const patch = JSON.parse(await readFile(patchPath, "utf8"));

function deepMerge(target, source) {
  for (const [key, value] of Object.entries(source)) {
    if (value === null) {
      delete target[key];
      continue;
    }
    if (
      typeof value === "object" &&
      !Array.isArray(value) &&
      typeof target[key] === "object" &&
      target[key] !== null &&
      !Array.isArray(target[key])
    ) {
      deepMerge(target[key], value);
    } else {
      target[key] = value;
    }
  }
  return target;
}

for (const [locale, entries] of Object.entries(patch)) {
  const url = new URL(`../src/i18n/messages/${locale}.json`, import.meta.url);
  const catalog = JSON.parse(await readFile(url, "utf8"));
  deepMerge(catalog, entries);
  await writeFile(url, JSON.stringify(catalog, null, 2) + "\n", "utf8");
  console.log(`updated ${locale}.json`);
}
