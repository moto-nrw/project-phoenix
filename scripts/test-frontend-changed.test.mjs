import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { mkdtempSync, mkdirSync, readFileSync, writeFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join } from "node:path";
import test from "node:test";

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
  // Seed the declared inventory, then add an unknown input consumer below.
  for (const [, file] of runner.matchAll(/"([^"\n]+\.(?:test|spec)\.[cm]?[jt]sx?)"/g)) {
    write(`frontend/${file}`, 'import { readdirSync } from "node:fs";');
  }
  return {
    root, write,
    run: (...args) => spawnSync(process.execPath, [join(root, "scripts/test-frontend-changed.mjs"), ...args], { encoding: "utf8" }),
  };
}

for (const directory of ["src", "scripts"]) {
  for (const [api, module] of [
    ["readFileSync", "node:fs"], ["readdirSync", "node:fs"],
    ["readFile", "fs/promises"], ["readdir", "node:fs/promises"],
    ["readdir", "fs/promises"], ["glob", "glob"], ["globSync", "node:fs"],
  ]) {
    test(`inventory rejects unlisted ${directory} consumer of ${module}/${api}`, t => {
      const f = fixture(t);
      assert.equal(f.run("--check-guards").status, 0);
      f.write(`frontend/${directory}/probe.spec.ts`, `import { ${api} as input } from "${module}"; input("src/**");`);
      const result = f.run("--check-guards");
      assert.equal(result.status, 1, result.stderr);
      assert.match(result.stderr, /probe\.spec\.ts/);
    });
  }
}

test("fixed inputs are selected once alongside Vitest's changed files", t => {
  const f = fixture(t);
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
  assert.match(filter, /- 'scripts\/dev-native\.sh'/);
  assert.match(filter, /- '\.github\/workflows\/e2e\.yml'/);
  assert.match(workflow, /run-frontend:.*steps\.filter\.outputs\.frontend-fs-inputs == 'true'/);
});
