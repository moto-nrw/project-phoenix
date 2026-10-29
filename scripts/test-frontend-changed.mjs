#!/usr/bin/env node
import { spawnSync } from "node:child_process";
import assert from "node:assert/strict";
import { readdirSync, readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { join, relative } from "node:path";

// Vitest's module graph cannot discover dependencies created by filesystem scans.
// Use its real changed-file selection, then run that set plus the scan guards once.
// Usage: node scripts/test-frontend-changed.mjs BASE [Vitest project/options]
const [base, ...options] = process.argv.slice(2);
if (!base) throw new Error("base ref required");
const cwd = fileURLToPath(new URL("../frontend/", import.meta.url));

// Input scopes (relative to frontend):
// src/app/**: route inventories, page metadata, loading boundaries and scaffolds.
// src/**/*.ts(x): personal-storage and browser-token source scans.
// src/components/{ui,timetable,staff}/**: planning design contracts.
// ../backend/modules/settings/internal/defaults/*.go: category labels.
// ../.github/workflows/e2e.yml: setup-tour target E2E path coverage.
// src/app/api/**/route.ts: BFF routes that forward a file's CSP (#5275).
// Always run every scan guard: this covers new/deleted files and directory renames,
// without duplicating each guard's traversal/exclusion logic in a path matcher.
const guards = [
  "src/app/api/proxy-routes.test.ts",
  "src/app/api/route-cache-policy.test.ts",
  "src/server/auth/route-handler-coverage.test.ts",
  "src/lib/app-route-invariants.test.ts",
  "src/lib/analytics-routes.test.ts",
  "src/lib/help-topics.test.ts",
  "src/lib/loading-boundaries.test.ts",
  "src/lib/navigation-sync.test.ts",
  "src/lib/personal-storage.test.ts",
  "src/lib/browser-token-ratchet.test.ts",
  "src/components/ui/tenant-page-scaffold.test.ts",
  "src/components/ui/plan-design-guards.test.ts",
  "src/components/settings/settings-filter.test.ts",
  "src/components/school-setup/setup-tour-targets.test.ts",
  "src/proxy.test.ts",
];

// Run in every frontend CI node job, including full-suite runs which do not
// use the incremental runner. A new filesystem scan must not silently opt out.
if (base === "--check-guards") {
  function scan(directory) {
    return readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
      const file = join(directory, entry.name);
      if (entry.isDirectory()) return scan(file);
      return /\.test\./.test(entry.name) &&
        readFileSync(file, "utf8").includes("readdirSync")
        ? [relative(cwd, file).split("\\").join("/")]
        : [];
    });
  }
  assert.deepEqual(
    [...guards].sort(),
    scan(join(cwd, "src")).sort(),
    "Filesystem guard inventory differs from frontend tests using readdirSync",
  );
  process.exit(0);
}

function vitest(args, capture = false) {
  const result = spawnSync("pnpm", ["exec", "vitest", ...args], {
    cwd,
    stdio: capture ? ["inherit", "pipe", "inherit"] : "inherit",
    encoding: "utf8",
  });
  if (result.error) throw result.error;
  if (result.status !== 0) process.exit(result.status ?? 1);
  return result.stdout;
}

const selected = JSON.parse(
  vitest(
    ["list", "--filesOnly", "--json", "--changed", base, ...options],
    true,
  ),
);
const files = new Set([
  ...guards,
  ...selected.map(({ file }) => relative(cwd, file)),
]);
vitest(["run", ...options, ...files]);
