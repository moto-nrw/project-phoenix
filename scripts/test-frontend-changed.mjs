#!/usr/bin/env node
import { spawnSync } from "node:child_process";
import assert from "node:assert/strict";
import { existsSync, readdirSync, readFileSync, statSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { createRequire } from "node:module";
import { dirname, join, relative, resolve } from "node:path";

// Vitest's module graph cannot discover dependencies created by filesystem scans.
// Use its real changed-file selection, then run that set plus the scan guards once.
// Usage: node scripts/test-frontend-changed.mjs BASE [Vitest project/options]
const [base, ...options] = process.argv.slice(2);
if (!base) throw new Error("base ref required");
const cwd = fileURLToPath(new URL("../frontend/", import.meta.url));

// Script tests execute tools with opaque config/plugin inputs. Include the whole
// group unconditionally, including new tests and unknown IO APIs. Other tests
// stay incremental unless their local module closure performs IO (checked below).
function testsIn(directory) {
  return readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
    const file = join(directory, entry.name);
    if (entry.isDirectory()) {
      return ["node_modules", "e2e"].includes(entry.name) ? [] : testsIn(file);
    }
    return /\.(test|spec)\.[cm]?[jt]sx?$/.test(entry.name)
      ? [relative(cwd, file).split("\\").join("/")]
      : [];
  }).sort();
}
const scriptTests = testsIn(join(cwd, "scripts"));
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
  "src/proxy.test.ts",
  "src/sentry.config.test.ts",
  "src/test/phosphor-imports-plugin.test.ts",
  "src/test/proxy-routes.table.test.ts",
  ...scriptTests,
];

// This is a conservative local-import inventory, not a second test selector.
// Match module boundaries rather than method names, so new fs/process APIs and
// aliased calls cannot silently escape. Cache parsed modules, not cyclic closures.
if (base === "--check-guards") {
  // TypeScript is already a frontend dependency. Parse real syntax rather than
  // treating import-shaped text in comments and test fixtures as module edges.
  const ts = createRequire(new URL("../frontend/package.json", import.meta.url))("typescript");
  const modules = new Map();
  const extensions = [".ts", ".tsx", ".mts", ".cts", ".js", ".jsx", ".mjs", ".cjs", ".json"];
  function localFile(importer, specifier) {
    const target = specifier.startsWith(".")
      ? resolve(dirname(importer), specifier)
      : resolve(cwd, "src", specifier.slice(2));
    const stem = target.replace(/\.[cm]?jsx?$/, "");
    return [target, ...extensions.map(ext => stem + ext), ...extensions.map(ext => join(target, "index" + ext))]
      .find(file => existsSync(file) && statSync(file).isFile());
  }
  function moduleInputs(file) {
    if (modules.has(file)) return modules.get(file);
    if (!/\.(?:[cm]?[jt]sx?|json)$/.test(file)) return { io: false, dependencies: [] };
    const source = readFileSync(file, "utf8");
    const dependencies = [];
    let io = false;
    function dependency(expression) {
      if (!expression || !ts.isStringLiteralLike(expression)) {
        io = true; // Expression-valued imports cannot be proven graph-only.
        return;
      }
      const specifier = expression.text;
      if (/^(?:node:)?(?:fs(?:\/.*)?|child_process)$|^(?:glob|fast-glob|globby)(?:\/|$)/.test(specifier)) {
        io = true;
      } else if (/^(?:\.|~\/|@\/)/.test(specifier)) {
        const imported = localFile(file, specifier);
        if (imported) dependencies.push(imported);
        else io = true; // Unresolved local imports must never opt a test out.
      }
    }
    function visit(node) {
      if (ts.isImportDeclaration(node) || ts.isExportDeclaration(node)) {
        if (node.moduleSpecifier) dependency(node.moduleSpecifier);
      } else if (ts.isImportEqualsDeclaration(node) && ts.isExternalModuleReference(node.moduleReference)) {
        dependency(node.moduleReference.expression);
      } else if (ts.isCallExpression(node) &&
        (node.expression.kind === ts.SyntaxKind.ImportKeyword ||
          (ts.isIdentifier(node.expression) && node.expression.text === "require"))) {
        dependency(node.arguments[0]);
      }
      ts.forEachChild(node, visit);
    }
    visit(ts.createSourceFile(file, source, ts.ScriptTarget.Latest, true));
    const inputs = { io, dependencies };
    modules.set(file, inputs);
    return inputs;
  }
  function usesInputs(file, visited = new Set()) {
    if (visited.has(file)) return false;
    visited.add(file);
    const { io, dependencies } = moduleInputs(file);
    return io || dependencies.some(dependency => usesInputs(dependency, visited));
  }
  assert.deepEqual(
    [...guards].sort(),
    [...testsIn(join(cwd, "src")).filter(file => usesInputs(resolve(cwd, file))), ...scriptTests].sort(),
    "Filesystem/process guard inventory differs from tests and their local helpers",
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
