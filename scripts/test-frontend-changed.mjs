#!/usr/bin/env node
import { spawnSync } from "node:child_process";
import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { copyFileSync, existsSync, mkdtempSync, readdirSync, readFileSync, rmSync, statSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { fileURLToPath } from "node:url";
import { createRequire } from "node:module";
import { dirname, isAbsolute, join, relative, resolve, sep } from "node:path";

// Vitest's module graph cannot discover dependencies created by filesystem scans.
// Only allowlisted source paths use changed-file selection. Except Markdown
// documentation (guards only), other frontend inputs fail closed to full.
// Usage: node scripts/test-frontend-changed.mjs BASE [Vitest project/options]
const modeFiles = process.argv[2] === "--mode-files";
const modeOnly = modeFiles || process.argv[2] === "--mode";
const [base, ...options] = process.argv.slice(modeOnly ? 3 : 2);
if (!base && !modeFiles) throw new Error("base ref required");
const cwd = fileURLToPath(new URL("../frontend/", import.meta.url));

// Shared by local dispatch and CI. Use NUL-separated paths and disable rename
// detection so moving an input INTO src cannot hide its old, full-run path.
function git(args) {
  const result = spawnSync("git", args, { cwd: resolve(cwd, ".."), encoding: "utf8" });
  if (result.error) throw result.error;
  if (result.status !== 0) throw new Error(result.stderr);
  return result.stdout;
}
function selectionMode(files) {
  let changed;
  if (files !== undefined) {
    assert.ok(Array.isArray(files) && files.every(file => typeof file === "string"), "changed paths must be a JSON string array");
    changed = new Set(files);
  } else {
    const mergeBase = git(["merge-base", "HEAD", base]).trim();
    changed = new Set([
      ...git(["diff", "--no-renames", "--name-only", "-z", mergeBase]).split("\0"),
      ...git(["ls-files", "--others", "--exclude-standard", "-z"]).split("\0"),
    ]);
  }
  // Tests in these source areas use Vitest's changed graph too. Declaration
  // files have no runtime emission; source-scanning guards still run with them.
  // Markdown skips lint/build, but tests may read it. Keep its guard-only
  // route aligned with the workflow filters; never silently skip consumers.
  // Every other path/type outside this allowlist defaults to the full suite.
  const sourceModule = /^frontend\/src\/(?:app|components|contexts|hooks|i18n|lib|server)\/.+\.(?:[cm]?[jt]s|[jt]sx)$/;
  const frontend = [...changed].filter(file => file.startsWith("frontend/") && !file.endsWith(".md"));
  if (frontend.some(file => !sourceModule.test(file)) ||
    ["scripts/test-frontend-changed.mjs", "scripts/test-frontend-changed.test.mjs",
      "scripts/test-changed.sh", "scripts/test-changed_test.sh",
      ".github/workflows/main.yml", ".github/workflows/test.yml"].some(file => changed.has(file))) return "full";
  if (frontend.length) return "incremental";
  if ([...changed].some(file => file.startsWith("frontend/") && file.endsWith(".md") ||
    file.startsWith("backend/") && !file.endsWith(".md") ||
    file === "scripts/dev-native.sh" || file === ".github/workflows/e2e.yml" ||
    file === ".github/workflows/lint.yml")) return "guards";
  return "none";
}
const mode = base === "--check-guards" ? undefined : selectionMode(modeFiles ? JSON.parse(readFileSync(0, "utf8")) : undefined);
if (modeOnly) {
  process.stdout.write(mode + "\n");
  process.exit(0);
}

if (mode === "none") process.exit(0);
if (mode === "full") {
  console.log("==> vitest full (input outside source-path allowlist)");
  vitest(["run", ...options]);
  process.exit(0);
}

// Tests outside src may execute opaque tools or scan product modules. Keep
// all of them, regardless of location or IO API. src consumers are inventoried.
function testsIn(directory, externalOnly = false) {
  return readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
    const file = join(directory, entry.name);
    if (entry.isDirectory()) {
      return ["node_modules", "e2e"].includes(entry.name) ||
        (externalOnly && directory === cwd && entry.name === "src") ? [] : testsIn(file);
    }
    return /\.(test|spec)\.[cm]?[jt]sx?$/.test(entry.name)
      ? [relative(cwd, file).split("\\").join("/")]
      : [];
  }).sort();
}
const externalTests = testsIn(cwd, true);
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
  ...externalTests,
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
        if (imported) {
          const path = relative(cwd, imported);
          // Backend/catalog imports are outside Vitest's frontend graph.
          if (isAbsolute(path) || path === ".." || path.startsWith(".." + sep)) io = true;
          else dependencies.push(imported);
        } else io = true; // Unresolved local imports must never opt a test out.
      }
    }
    function isImportMeta(expression) {
      if (ts.isMetaProperty(expression)) return expression.keywordToken === ts.SyntaxKind.ImportKeyword;
      if (ts.isParenthesizedExpression(expression)) return isImportMeta(expression.expression);
      return (ts.isPropertyAccessExpression(expression) || ts.isElementAccessExpression(expression)) &&
        isImportMeta(expression.expression);
    }
    function visit(node) {
      if (ts.isCallExpression(node) && isImportMeta(node.expression)) {
        io = true; // Vite glob/resolve calls can read files beyond its module graph.
      } else if (ts.isImportDeclaration(node) || ts.isExportDeclaration(node)) {
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
    [...testsIn(join(cwd, "src")).filter(file => usesInputs(resolve(cwd, file))), ...externalTests].sort(),
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

// Guards read files beyond Vitest's module graph, so a guards-only run depends
// on the whole checkout. Locally, reuse a pass only for byte-identical inputs:
// every non-ignored file (untracked included, via a scratch index), installed
// dependencies, local env files, Node and the options. Any change misses.
function guardsKey() {
  const scratch = mkdtempSync(join(tmpdir(), "phoenix-guards-"));
  try {
    const index = join(scratch, "index");
    const realIndex = resolve(cwd, "..", git(["rev-parse", "--git-path", "index"]).trim());
    if (existsSync(realIndex)) copyFileSync(realIndex, index);
    const scratchGit = args => {
      const result = spawnSync("git", args, {
        cwd: resolve(cwd, ".."), env: { ...process.env, GIT_INDEX_FILE: index }, encoding: "utf8",
      });
      if (result.error) throw result.error;
      if (result.status !== 0) throw new Error(result.stderr);
      return result.stdout.trim();
    };
    scratchGit(["add", "-A"]);
    const tree = scratchGit(["write-tree"]);
    const hash = createHash("sha256").update(JSON.stringify({
      tree, options, node: process.version, platform: process.platform, arch: process.arch,
      env: Object.entries(process.env).filter(([name]) =>
        /^(NEXT_|NEXTAUTH_|AUTH_|API_URL$|TENANT_DOMAIN$|NODE_OPTIONS$|NODE_ENV$|TZ$|CI$|VITEST)/.test(name)).sort(),
    }));
    for (const file of [".env", ".env.local", ".env.development", ".env.development.local",
      "node_modules/.modules.yaml", "node_modules/.pnpm/lock.yaml"]) {
      hash.update(file).update(existsSync(join(cwd, file)) ? readFileSync(join(cwd, file)) : "<absent>");
    }
    return hash.digest("hex");
  } finally {
    rmSync(scratch, { recursive: true, force: true });
  }
}
const cacheGuards = mode === "guards" && !process.env.GITHUB_ACTIONS && process.env.PHX_GUARDS_CACHE !== "off";
const guardsRecord = cacheGuards && resolve(cwd, "..", git(["rev-parse", "--git-path", "test-frontend-guards.pass"]).trim());
const key = cacheGuards && guardsKey();
if (cacheGuards && existsSync(guardsRecord) && readFileSync(guardsRecord, "utf8") === key) {
  console.log("==> vitest guards: same inputs as the last passing run, skipped (PHX_GUARDS_CACHE=off reruns)");
  process.exit(0);
}
console.log(`==> vitest ${mode}`);
const selected = mode === "guards" ? [] : JSON.parse(
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
// vitest() exits on failure. Inputs edited during the run must not be recorded.
if (cacheGuards && guardsKey() === key) writeFileSync(guardsRecord, key);
