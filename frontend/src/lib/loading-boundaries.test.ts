import { existsSync, readdirSync, statSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const APP_DIR = path.join(
  path.dirname(fileURLToPath(import.meta.url)),
  "..",
  "app",
);

function collectPages(directory: string): string[] {
  return [
    ...(existsSync(path.join(directory, "page.tsx")) ? [directory] : []),
    ...readdirSync(directory).flatMap((entry) => {
      const entryPath = path.join(directory, entry);
      return statSync(entryPath).isDirectory() ? collectPages(entryPath) : [];
    }),
  ];
}

function inheritedLoadingBoundary(pageDirectory: string): string | null {
  if (existsSync(path.join(pageDirectory, "loading.tsx"))) return null;

  for (
    let directory = path.dirname(pageDirectory);
    directory.startsWith(APP_DIR);
    directory = path.dirname(directory)
  ) {
    const loadingFile = path.join(directory, "loading.tsx");
    if (existsSync(loadingFile)) return loadingFile;
  }

  return null;
}

describe("App Router loading boundaries", () => {
  it("never shows one route's skeleton while a different route loads", () => {
    const inheritedBoundaries = collectPages(APP_DIR).flatMap(
      (pageDirectory) => {
        const loadingFile = inheritedLoadingBoundary(pageDirectory);
        if (!loadingFile) return [];

        return [
          `${path.relative(APP_DIR, pageDirectory)}/page.tsx inherits ${path.relative(APP_DIR, loadingFile)}`,
        ];
      },
    );

    expect(inheritedBoundaries).toEqual([]);
  });
});
