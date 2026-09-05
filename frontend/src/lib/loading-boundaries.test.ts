import { existsSync, readdirSync, statSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const APP_DIR = path.join(
  path.dirname(fileURLToPath(import.meta.url)),
  "..",
  "app",
);
const PROTECTED_DIR = path.join(APP_DIR, "[tenant]", "(protected)");

function collect(directory: string, file: string): string[] {
  return [
    ...(existsSync(path.join(directory, file)) ? [directory] : []),
    ...readdirSync(directory).flatMap((entry) => {
      const entryPath = path.join(directory, entry);
      return statSync(entryPath).isDirectory() ? collect(entryPath, file) : [];
    }),
  ];
}

const collectPages = (directory: string) => collect(directory, "page.tsx");

function loadingBoundary(pageDirectory: string): string | null {
  for (
    let directory = pageDirectory;
    directory.startsWith(APP_DIR);
    directory = path.dirname(directory)
  ) {
    const loadingFile = path.join(directory, "loading.tsx");
    if (existsSync(loadingFile)) return loadingFile;
  }

  return null;
}

const ALLOWED_SHARED_BOUNDARIES = new Set([
  "loading.tsx",
  "[tenant]/(protected)/database/loading.tsx",
]);

/**
 * Die Regel im Personal-Portal (#2828): eine Seite hat entweder eine eigene,
 * seitenförmige Ladehülle — oder gar keine. Ohne Hülle bleibt beim
 * Seitenwechsel die vorherige Seite stehen, bis die neue bereit ist; der
 * Fortschrittsbalken der Hülle zeigt, dass etwas läuft.
 *
 * Was es nicht mehr geben darf, ist eine geteilte Hülle über mehreren Seiten.
 * Die alte `(protected)/loading.tsx` zeigte einen allgemeinen Ladekringel in
 * Inhaltshöhe: erst sprang das Layout auf Kringelhöhe zusammen, dann auf das
 * Skelett der Zielseite, dann auf den Inhalt.
 * Gemessen (`pnpm run perf:nav`, 150 ms RTT) trat das auf /settings und
 * /statistics bei jedem Wechsel auf.
 *
 * Ausnahme bleibt `database/loading.tsx`: die Unterseiten der
 * Datenverwaltung teilen sich ein Layout mit fester Form, und die Hülle hat
 * genau diese Form.
 */
describe("App Router loading boundaries", () => {
  it.each(["rooms", "staff"])(
    "uses a page-shaped loading boundary for /%s",
    (route) => {
      const pageDirectory = path.join(PROTECTED_DIR, route);

      expect(path.dirname(loadingBoundary(pageDirectory) ?? "")).toBe(
        pageDirectory,
      );
    },
  );

  it("keeps every loading boundary of the staff portal next to a page", () => {
    const orphaned = collect(PROTECTED_DIR, "loading.tsx")
      .filter((directory) => !existsSync(path.join(directory, "page.tsx")))
      .map((directory) => path.relative(APP_DIR, directory));

    expect(orphaned).toEqual([]);
  });

  it("has no shared loading boundary between the shell and a staff page", () => {
    const shared = collectPages(PROTECTED_DIR)
      .map((pageDirectory) => ({
        pageDirectory,
        file: loadingBoundary(pageDirectory),
      }))
      .filter(
        ({ pageDirectory, file }) =>
          file !== null &&
          path.dirname(file) !== pageDirectory &&
          !ALLOWED_SHARED_BOUNDARIES.has(path.relative(APP_DIR, file)),
      )
      .map(
        ({ pageDirectory, file }) =>
          `${path.relative(APP_DIR, pageDirectory)}/page.tsx inherits ${path.relative(APP_DIR, file!)}`,
      );

    expect(shared).toEqual([]);
  });

  it("gives every page outside the staff portal a loading boundary", () => {
    const invalidPages = collectPages(APP_DIR)
      .filter((pageDirectory) => !pageDirectory.startsWith(PROTECTED_DIR))
      .flatMap((pageDirectory) => {
        const loadingFile = loadingBoundary(pageDirectory);
        if (!loadingFile) {
          return [
            `${path.relative(APP_DIR, pageDirectory)}/page.tsx has no loading boundary`,
          ];
        }

        if (path.dirname(loadingFile) === pageDirectory) return [];

        const relativeLoadingFile = path.relative(APP_DIR, loadingFile);
        if (ALLOWED_SHARED_BOUNDARIES.has(relativeLoadingFile)) return [];

        return [
          `${path.relative(APP_DIR, pageDirectory)}/page.tsx inherits ${relativeLoadingFile}`,
        ];
      });

    expect(invalidPages).toEqual([]);
  });
});
