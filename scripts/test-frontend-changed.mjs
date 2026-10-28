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

// Fixed files and directory scans have no import edges. Always run this small
// inventory rather than duplicate each test's input paths and traversal rules.
// Include fixture-only filesystem tests too: conservative selection needs no
// exception list and cannot silently lose a new repository input in those tests.
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
  "src/lib/hooks/use-sse.test.ts",
  "src/lib/service-worker.test.ts",
  "src/styles/moto-token-drift.test.ts",
  "src/server/node-header-limit.test.ts",
  "src/lib/permission-labels.test.ts",
  "src/lib/modules.test.ts",
  "src/lib/role-labels.test.ts",
  "scripts/oxlint-plugin-bauart.test.ts",
  "scripts/product-screenshots/shot-list.test.ts",
  "scripts/product-screenshots/pipeline.test.ts",
  "scripts/product-screenshots/publish.test.ts",
];

// Run in every frontend CI node job, including full-suite runs which do not
// use the incremental runner. A new filesystem scan must not silently opt out.
if (base === "--check-guards") {
  function scan(directory) {
    return readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
      const file = join(directory, entry.name);
      if (entry.isDirectory()) return scan(file);
      return /\.(test|spec)\.[cm]?[jt]sx?$/.test(entry.name) &&
        /\b(readFileSync|readFile|readdirSync|readdir|globSync|glob)\b/.test(
          readFileSync(file, "utf8"),
        )
        ? [relative(cwd, file).split("\\").join("/")]
        : [];
    });
  }
  assert.deepEqual(
    [...guards].sort(),
    [...scan(join(cwd, "src")), ...scan(join(cwd, "scripts"))].sort(),
    "Filesystem guard inventory differs from frontend tests using file inputs",
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
