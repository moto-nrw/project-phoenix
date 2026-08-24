import { readdirSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

function collectDynamicSiblingConflicts(
  directory: string,
  appRoot = directory,
): string[] {
  const entries = readdirSync(directory, { withFileTypes: true });
  const directories = entries.filter((entry) => entry.isDirectory());
  const dynamicDirectories = directories
    .filter((entry) => entry.name.startsWith("[") && entry.name.endsWith("]"))
    .map((entry) => entry.name)
    .sort();
  const conflicts =
    dynamicDirectories.length > 1
      ? [
          `${path.relative(appRoot, directory) || "."}: ${dynamicDirectories.join(", ")}`,
        ]
      : [];

  for (const entry of directories) {
    conflicts.push(
      ...collectDynamicSiblingConflicts(
        path.join(directory, entry.name),
        appRoot,
      ),
    );
  }

  return conflicts;
}

describe("Next.js app route filesystem invariants", () => {
  it("uses at most one dynamic child directory per filesystem parent", () => {
    const appRoot = path.resolve(
      path.dirname(fileURLToPath(import.meta.url)),
      "../app",
    );

    expect(collectDynamicSiblingConflicts(appRoot)).toEqual([]);
  });
});
