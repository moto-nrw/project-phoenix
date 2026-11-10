// Reine Logik des Kit-Pixel-Vergleichs (#6562): Story-Auswahl, Argumente,
// Browser-Abgleich. Der Befehl steht in scripts/kit-pixel.mjs, der Lauf in
// e2e/kit-pixel/stories.spec.ts.
import { parseArgs } from "node:util";

/** Stories unter diesem Pfad gehören zum Kit; alles andere nur mit `--all-stories`. */
const KIT_IMPORT_PREFIX = "./src/components/ui/";

/** @type {readonly string[]} */
const COMMANDS = ["baseline", "compare"];

/**
 * @typedef {{ type: string, id: string, importPath: string }} IndexEntry
 * @typedef {{ v?: number, entries?: Record<string, IndexEntry> }} StorybookIndex
 * @typedef {{ id: string, importPath: string }} SelectedStory
 */

/**
 * Stories aus Storybooks `index.json` (Format v5), nach id sortiert.
 * `filter` ist ein regulärer Ausdruck auf die Story-id
 * (z. B. `components-ui-modal--default`).
 *
 * @param {StorybookIndex} index
 * @param {{ filter?: string, allStories?: boolean }} [options]
 * @returns {SelectedStory[]}
 */
export function selectStories(index, { filter, allStories = false } = {}) {
  if (index?.v !== 5 || typeof index.entries !== "object") {
    throw new Error(
      `Unbekanntes Format der Storybook-index.json (v=${index?.v}); erwartet v=5.`,
    );
  }
  let pattern = null;
  if (filter) {
    try {
      pattern = new RegExp(filter);
    } catch (error) {
      const reason = error instanceof Error ? error.message : String(error);
      throw new Error(`Ungültiger Filter "${filter}": ${reason}`, {
        cause: error,
      });
    }
  }
  return Object.values(index.entries)
    .filter((entry) => entry.type === "story")
    .filter(
      (entry) => allStories || entry.importPath.startsWith(KIT_IMPORT_PREFIX),
    )
    .filter((entry) => !pattern || pattern.test(entry.id))
    .map(({ id, importPath }) => ({ id, importPath }))
    .sort((a, b) => a.id.localeCompare(b.id));
}

/**
 * `baseline [--ref <rev>] [--filter <re>] [--all-stories]` | `compare [--filter <re>] [--all-stories]`
 *
 * @param {string[]} argv
 */
export function parseKitPixelArgs(argv) {
  const args = argv[0] === "--" ? argv.slice(1) : argv;
  const { positionals, values } = parseArgs({
    args,
    allowPositionals: true,
    strict: true,
    options: {
      ref: { type: "string" },
      filter: { type: "string" },
      "all-stories": { type: "boolean", default: false },
    },
  });
  const command = positionals[0];
  if (
    positionals.length !== 1 ||
    command === undefined ||
    !COMMANDS.includes(command)
  ) {
    throw new Error(
      "Aufruf: pnpm run kit-pixel baseline [--ref <commit>] [--filter <regex>] [--all-stories]\n" +
        "        pnpm run kit-pixel compare [--filter <regex>] [--all-stories]",
    );
  }
  if (command === "compare" && values.ref !== undefined) {
    throw new Error(
      "--ref gilt nur für baseline; compare vergleicht immer den Arbeitsstand.",
    );
  }
  return {
    command,
    ref: command === "baseline" ? (values.ref ?? "HEAD") : undefined,
    filter: values.filter,
    allStories: values["all-stories"],
  };
}

/**
 * Meldung, wenn Chrome seit der Baseline gewechselt hat, sonst `null`.
 *
 * @param {string} baselineVersion
 * @param {string} currentVersion
 * @returns {string | null}
 */
export function browserMismatch(baselineVersion, currentVersion) {
  if (baselineVersion === currentVersion) return null;
  return (
    `Die Baseline stammt aus Chrome ${baselineVersion}, dieser Lauf nutzt Chrome ${currentVersion}.\n` +
    "Ein anderer Browser rendert anders; Abweichungen wären nicht aussagekräftig.\n" +
    "Baseline neu erzeugen: pnpm run kit-pixel baseline --ref <commit>"
  );
}

/**
 * Story-ids, für die die Baseline ein Bild hat, der Build aber keine Story
 * mehr. Ein Umbau, der eine Story löscht oder umbenennt, darf ihre Abweichung
 * nicht verstecken.
 *
 * @param {readonly string[]} baselineFiles PNG-Namen eines Breiten-Ordners
 * @param {readonly string[]} storyIds Auswahl im aktuellen Build
 * @returns {string[]}
 */
export function orphanedBaselines(baselineFiles, storyIds) {
  const current = new Set(storyIds);
  return baselineFiles
    .filter((file) => file.endsWith(".png"))
    .map((file) => file.slice(0, -".png".length))
    .filter((id) => !current.has(id))
    .sort();
}
