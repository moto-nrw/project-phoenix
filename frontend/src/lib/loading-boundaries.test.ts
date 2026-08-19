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

function collectPages(directory: string): string[] {
  return [
    ...(existsSync(path.join(directory, "page.tsx")) ? [directory] : []),
    ...readdirSync(directory).flatMap((entry) => {
      const entryPath = path.join(directory, entry);
      return statSync(entryPath).isDirectory() ? collectPages(entryPath) : [];
    }),
  ];
}

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
  "[tenant]/(protected)/loading.tsx",
  "[tenant]/(protected)/database/loading.tsx",
]);

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

  it("keeps protected-route loading inside the persistent app shell", () => {
    const rootLoadingFile = path.join(APP_DIR, "loading.tsx");
    const pagesUsingRootLoading = collectPages(PROTECTED_DIR)
      .filter(
        (pageDirectory) => loadingBoundary(pageDirectory) === rootLoadingFile,
      )
      .map((pageDirectory) => path.relative(APP_DIR, pageDirectory));

    expect(pagesUsingRootLoading).toEqual([]);
  });

  it("gives every page a colocated or approved shared loading boundary", () => {
    const invalidPages = collectPages(APP_DIR).flatMap((pageDirectory) => {
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
