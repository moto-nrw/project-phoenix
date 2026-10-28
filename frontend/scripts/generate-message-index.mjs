import { readFileSync, readdirSync, writeFileSync } from "node:fs";
import { join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { format, resolveConfig } from "prettier";
import {
  assertLocaleDirectories,
  composeMessages,
  readMessageSources,
  renderMessageFacade,
} from "./message-sources.mjs";

const defaultDirectory = fileURLToPath(
  new URL("../src/i18n/messages/", import.meta.url),
);
const defaultLocales = JSON.parse(
  readFileSync(
    new URL(
      "../../backend/modules/delivery/internal/adapters/localization/locales.json",
      import.meta.url,
    ),
    "utf8",
  ),
).map((locale) => locale.code);

export async function synchronizeMessageFacades({
  directory = defaultDirectory,
  locales = defaultLocales,
  check = false,
} = {}) {
  assertLocaleDirectories(directory, locales);
  const expectedFiles = new Set(
    locales.map((locale) => `${locale}.generated.ts`),
  );
  for (const file of readdirSync(directory)) {
    if (file.endsWith(".generated.ts") && !expectedFiles.has(file))
      throw new Error(`Unsupported generated locale facade: ${file}`);
  }
  const catalogs = new Map();
  for (const locale of locales) {
    const sources = readMessageSources(join(directory, locale));
    catalogs.set(locale, composeMessages(sources));
    const file = join(directory, `${locale}.generated.ts`);
    const generated = await format(renderMessageFacade(sources, locale), {
      ...(await resolveConfig(file)),
      plugins: [],
      filepath: file,
    });
    if (check) {
      let existing = "";
      try {
        existing = readFileSync(file, "utf8");
      } catch {
        /* Missing facade is stale. */
      }
      if (existing !== generated)
        throw new Error(
          `Message facade ${file} is stale. Run pnpm generate:messages.`,
        );
    } else writeFileSync(file, generated);
  }
  return catalogs;
}

if (
  process.argv[1] &&
  resolve(process.argv[1]) === fileURLToPath(import.meta.url)
) {
  await synchronizeMessageFacades({ check: process.argv.includes("--check") });
}
