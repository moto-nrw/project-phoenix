import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { mkdtempSync, mkdirSync, readFileSync, writeFileSync, rmSync, symlinkSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const runner = readFileSync(new URL("./test-frontend-changed.mjs", import.meta.url), "utf8");

function fixture(t) {
  const root = mkdtempSync(join(tmpdir(), "phoenix-frontend-inputs-"));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  function write(path, content, mode) {
    const file = join(root, path);
    mkdirSync(dirname(file), { recursive: true });
    writeFileSync(file, content, { mode });
  }
  write("scripts/test-frontend-changed.mjs", runner);
  mkdirSync(join(root, "frontend/scripts"), { recursive: true });
  symlinkSync(new URL("../frontend/node_modules", import.meta.url), join(root, "frontend/node_modules"), "dir");
  // Seed the declared inventory, then add an unknown input consumer below.
  for (const [, file] of runner.matchAll(/"([^"\n]+\.(?:test|spec)\.[cm]?[jt]sx?)"/g)) {
    write(`frontend/${file}`, 'import { readdirSync } from "node:fs";');
  }
  write("bin/pnpm", `#!/bin/sh
if [ "$3" = list ]; then
  printf '[]'
else
  printf '%s\\n' "$@" > '${root}/args'
fi
`, 0o755);
  function git(...args) {
    const result = spawnSync("git", args, { cwd: root, encoding: "utf8" });
    assert.equal(result.status, 0, result.stderr);
    return result.stdout.trim();
  }
  write(".gitignore", "frontend/node_modules\n");
  git("init", "-q");
  git("config", "user.name", "selection-fixture");
  git("config", "user.email", "selection-fixture@example.invalid");
  git("config", "commit.gpgSign", "false");
  git("config", "core.hooksPath", "/dev/null");
  write("frontend/src/lib/product.ts", "export {};\n");
  git("add", ".");
  git("commit", "-qm", "baseline");
  write("frontend/src/lib/product.ts", "export const changed = true;\n");
  return {
    root, write, git,
    run: (...args) => spawnSync(process.execPath, [join(root, "scripts/test-frontend-changed.mjs"), ...args], { encoding: "utf8", env: { ...process.env, PATH: `${root}/bin:${process.env.PATH}` } }),
  };
}

for (const directory of ["src", "scripts"]) {
  for (const [api, module] of [
    ["readFileSync", "node:fs"], ["readdirSync", "node:fs"],
    ["readFile", "fs/promises"], ["readdir", "node:fs/promises"],
    ["readdir", "fs/promises"], ["glob", "glob"], ["globSync", "node:fs"],
    ["existsSync", "node:fs"], ["statSync", "node:fs"],
    ["copyFileSync", "node:fs"], ["createReadStream", "node:fs"],
    ["futureFileAPI", "node:fs"],
    ["spawnSync", "node:child_process"], ["execFileSync", "child_process"],
  ]) {
    test(`inventory covers ${directory} consumer of ${module}/${api}`, t => {
      const f = fixture(t);
      assert.equal(f.run("--check-guards").status, 0);
      f.write(`frontend/${directory}/probe.spec.ts`, `import { ${api} as input } from "${module}"; input("src/**");`);
      const result = f.run("--check-guards");
      if (directory === "src") {
        assert.equal(result.status, 1, result.stderr);
        assert.match(result.stderr, /probe\.spec\.ts/);
      } else {
        // External consumers need no registration; editing scripts runs in full.
        assert.equal(result.status, 0, result.stderr);
        const selected = f.run("HEAD");
        assert.equal(selected.status, 0, selected.stderr);
        const args = readFileSync(join(f.root, "args"), "utf8").split("\n");
        assert.deepEqual(args.filter(Boolean), ["exec", "vitest", "run"]);
      }
    });
  }
}

test("fixed inputs are selected once alongside Vitest's changed files", t => {
  const f = fixture(t);
  f.write("frontend/scripts/product-screenshots/shot-list.test.ts", "");
  f.git("add", "frontend/scripts");
  f.git("commit", "-qm", "external guard");
  f.write("bin/pnpm", `#!/bin/sh
if [ "$3" = list ]; then
  printf '[{"file":"%s/src/lib/service-worker.test.ts"}]' "$PWD"
else
  printf '%s\\n' "$@" > '${f.root}/args'
fi
`, 0o755);
  const result = spawnSync(process.execPath, [join(f.root, "scripts/test-frontend-changed.mjs"), "HEAD", "--project", "logic-node"], {
    encoding: "utf8", env: { ...process.env, PATH: `${f.root}/bin:${process.env.PATH}` },
  });
  assert.equal(result.status, 0, result.stderr);
  const args = readFileSync(join(f.root, "args"), "utf8").trim().split("\n");
  for (const file of [
    "src/lib/service-worker.test.ts", "src/lib/permission-labels.test.ts",
    "src/lib/modules.test.ts", "src/lib/role-labels.test.ts",
    "src/styles/moto-token-drift.test.ts", "src/server/node-header-limit.test.ts",
    "scripts/product-screenshots/shot-list.test.ts",
  ]) assert.equal(args.filter(arg => arg === file).length, 1, file);
  assert.deepEqual(args.slice(0, 5), ["exec", "vitest", "run", "--project", "logic-node"]);
});

test("CI starts frontend guards for backend-only inputs and the native launcher", () => {
  const workflow = readFileSync(new URL("../.github/workflows/main.yml", import.meta.url), "utf8");
  const filter = workflow.match(/            frontend-fs-inputs:\n([\s\S]*?)(?=            [\w-]+:)/)?.[1];
  assert.ok(filter, "frontend input job filter exists");
  assert.match(filter, /- 'backend\/\*\*'/);
  assert.doesNotMatch(filter, /frontend\//);
  assert.match(filter, /- 'scripts\/dev-native\.sh'/);
  assert.match(filter, /- '\.github\/workflows\/e2e\.yml'/);
  assert.match(workflow, /run-frontend:.*steps\.filter\.outputs\.frontend-fs-inputs == 'true'/);
  assert.match(workflow, /frontend-guards-only:.*frontend != 'true'.*frontend-test-infra != 'true'/);
  const testWorkflow = readFileSync(new URL("../.github/workflows/test.yml", import.meta.url), "utf8");
  assert.match(testWorkflow, /inputs.frontend-guards-only && '\["guards"\]'/);
  assert.match(testWorkflow, /matrix.project == 'node' && inputs.run-frontend-runner-tests/);
  assert.match(testWorkflow, /elif \[ "\$PROJECT" = guards \]; then[\s\S]*?test_args=\(\)/);
  assert.match(workflow, /frontend-runner-tests:\n\s+- 'scripts\/\*\*'/);
});

for (const input of [
  'import { existsSync } from "node:fs"; export const read = existsSync;',
  'export { statSync as read } from "fs";',
  'const fs = require("fs"); export const read = fs.statSync;',
  'const fs = await import("node:fs/promises"); export const read = fs.stat;',
  'import { execFileSync as read } from "node:child_process"; export { read };',
]) {
  test(`transitive local helper inputs are guard-required: ${input}`, t => {
    const f = fixture(t);
    f.write("frontend/src/probe.test.ts", 'import { read } from "~/helpers/first"; read();');
    f.write("frontend/src/helpers/first.ts", 'export { read } from "./second.js";');
    f.write("frontend/src/helpers/second.ts", input);
    const result = f.run("--check-guards");
    assert.equal(result.status, 1, result.stderr);
    assert.match(result.stderr, /src\/probe\.test\.ts/);
  });
}

test("pure helper cycles stay incremental; IO in the cycle is guard-required", t => {
  const f = fixture(t);
  f.write("frontend/src/probe.test.ts", 'import "@/helpers/first";');
  f.write("frontend/src/helpers/first.ts", 'export * from "./second";');
  f.write("frontend/src/helpers/second.ts", 'import "./first";');
  assert.equal(f.run("--check-guards").status, 0);
  f.write("frontend/src/helpers/second.ts", 'import "./first"; import "node:fs";');
  assert.equal(f.run("--check-guards").status, 1);
});

for (const edge of ['import "./missing";', 'import(variable);', 'require(variable);']) {
  test(`unresolved local or dynamic edges fail closed: ${edge}`, t => {
    const f = fixture(t);
    f.write("frontend/src/probe.test.ts", 'import "./helper";');
    f.write("frontend/src/helper.ts", edge);
    const result = f.run("--check-guards");
    assert.equal(result.status, 1, result.stderr);
    assert.match(result.stderr, /src\/probe\.test\.ts/);
  });
}

test("unchanged external consumers are guards even when product modules change", t => {
  const f = fixture(t);
  for (const file of ["scripts/new-class.test.mjs", "tools/nested/new-class.spec.ts", "root.test.ts"]) {
    f.write(`frontend/${file}`, 'import opaqueRunner from "a-new-package"; opaqueRunner();');
  }
  assert.equal(f.run("--check-guards").status, 0);
  f.git("add", "frontend/scripts", "frontend/tools", "frontend/root.test.ts");
  f.git("commit", "-qm", "external tests");
  const result = f.run("HEAD", "--project", "logic-node");
  assert.equal(result.status, 0, result.stderr);
  const args = readFileSync(join(f.root, "args"), "utf8").split("\n");
  for (const file of ["scripts/new-class.test.mjs", "tools/nested/new-class.spec.ts", "root.test.ts"]) {
    assert.equal(args.filter(arg => arg === file).length, 1, file);
  }
});

test("import-like fixture text and comments do not create IO edges", t => {
  const f = fixture(t);
  f.write("frontend/src/probe.test.ts", `
    // import fs from "node:fs";
    const sample = 'import { existsSync } from "node:fs";';
    const dynamicSample = 'import(variable);';
    export { sample, dynamicSample };
  `);
  assert.equal(f.run("--check-guards").status, 0);
});

test("the real repository inventory matches source and transitive helpers", () => {
  const result = spawnSync(process.execPath, [
    fileURLToPath(new URL("./test-frontend-changed.mjs", import.meta.url)), "--check-guards",
  ], { encoding: "utf8" });
  assert.equal(result.status, 0, result.stderr);
});

// Exercise the actual shared dispatcher, not a copy of its path expression.
for (const input of [
  "frontend/tools/zz-probe/reader.test.ts", "frontend/src/test/node-test-files.ts",
  "frontend/src/test/phosphor-imports-plugin.ts", "frontend/public/sw.js",
  "frontend/scripts/plugin.mjs", "frontend/.oxlintrc.json", "frontend/new.config.ts",
  "frontend/README.md", "frontend/src/lib/new.json", "frontend/src/lib/new.unknown",
  "frontend/src/lib/product.test.ts", "frontend/src/lib/product.spec.tsx",
  "frontend/src/types/new.d.ts", "frontend/src/new-area/module.ts",
  "frontend/src/sentry.config.ts",
]) {
  test(`non-product input fails closed to full: ${input}`, t => {
    const f = fixture(t);
    f.write(input, "");
    assert.equal(f.run("--mode", "HEAD").stdout, "full\n");
    const result = f.run("HEAD", "--project", "logic-node");
    assert.equal(result.status, 0, result.stderr);
    assert.deepEqual(readFileSync(join(f.root, "args"), "utf8").trim().split("\n"),
      ["exec", "vitest", "run", "--project", "logic-node"]);
  });
}

test("untracked, staged, committed, deleted and renamed inputs use the same fallback", t => {
  const f = fixture(t);
  let base = f.git("rev-parse", "HEAD");
  f.write("frontend/unknown.input", "fixed input");
  for (const state of ["untracked", "staged", "committed", "modified", "renamed", "deleted"]) {
    if (state === "staged") f.git("add", "frontend/unknown.input");
    if (state === "committed") f.git("commit", "-qm", "input");
    if (state === "modified") f.write("frontend/unknown.input", "changed");
    if (state === "renamed") f.git("mv", "frontend/unknown.input", "frontend/src/lib/moved.ts");
    if (state === "deleted") {
      f.git("add", "frontend/src/lib/moved.ts");
      f.git("commit", "-qm", "move");
      rmSync(join(f.root, "frontend/src/lib/moved.ts"));
    }
    assert.equal(f.run("--mode", base).stdout, "full\n", state);
    // Subsequent edits/renames target an input already present at the base.
    if (state === "committed") base = f.git("rev-parse", "HEAD");
  }
});

test("only product modules stay incremental; backend-only changes run guards without list", t => {
  const f = fixture(t);
  assert.equal(f.run("--mode", "HEAD").stdout, "incremental\n");
  f.git("checkout", "--", "frontend/src/lib/product.ts");
  assert.equal(f.run("--mode", "HEAD").stdout, "none\n");
  f.write("backend/auth/authorize/permissions/catalog.json", "[]");
  assert.equal(f.run("--mode", "HEAD").stdout, "guards\n");
  // Listing would exit 7. Guard-only dispatch must not traverse the import graph.
  f.write("bin/pnpm", `#!/bin/sh
if [ "$3" = list ]; then exit 7; fi
printf '%s\\n' "$@" > '${f.root}/args'
`, 0o755);
  assert.equal(f.run("HEAD").status, 0);
  assert.match(readFileSync(join(f.root, "args"), "utf8"), /permission-labels\.test\.ts/);
});
