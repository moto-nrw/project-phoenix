#!/usr/bin/env node
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";
import { relative } from "node:path";

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
// Always run all 13: this also covers new/deleted files and directory renames,
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
];

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
  vitest(["list", "--filesOnly", "--json", "--changed", base, ...options], true),
);
const files = new Set([
  ...guards,
  ...selected.map(({ file }) => relative(cwd, file)),
]);
vitest(["run", ...options, ...files]);
