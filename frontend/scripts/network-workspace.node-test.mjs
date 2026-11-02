import assert from "node:assert/strict";
import {
  existsSync,
  mkdirSync,
  mkdtempSync,
  readFileSync,
  readdirSync,
  realpathSync,
  rmSync,
  writeFileSync,
} from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join, relative, sep } from "node:path";
import { pathToFileURL } from "node:url";
import test from "node:test";
import { createAppWorkspace } from "../e2e/network/app-workspace.mjs";

async function fixture(run) {
  const repository = mkdtempSync(join(tmpdir(), "moto-network-workspace-"));
  const frontend = join(repository, "frontend");
  try {
    for (const name of ["src", "public", "node_modules", ".next/dev/cache"]) {
      mkdirSync(join(frontend, name), { recursive: true });
    }
    writeFileSync(
      join(frontend, "package.json"),
      JSON.stringify({ type: "module" }),
    );
    writeFileSync(
      join(frontend, "next.config.js"),
      'export default { experimental: { example: true }, turbopack: { resolveAlias: { example: "./src/input.ts" } } };',
    );
    for (const name of [
      "postcss.config.mjs",
      "tailwind.config.ts",
      "tsconfig.json",
    ]) {
      writeFileSync(join(frontend, name), `original ${name}`);
    }
    writeFileSync(join(frontend, "src", "input.ts"), "shared input");
    writeFileSync(join(frontend, "public", "asset.txt"), "shared asset");
    writeFileSync(
      join(frontend, "node_modules", "dependency.txt"),
      "warm dependency",
    );
    writeFileSync(
      join(frontend, ".next/dev/cache", "native"),
      "warm native cache",
    );
    writeFileSync(join(frontend, "next-env.d.ts"), "native type imports");
    writeFileSync(join(frontend, ".env.local"), "native-only configuration");
    return await run(frontend);
  } finally {
    rmSync(repository, { recursive: true, force: true });
  }
}

test("runs isolate compilation, generated files and source snapshots from native development", () =>
  fixture(async (frontend) => {
    const first = createAppWorkspace(frontend);
    const second = createAppWorkspace(frontend);
    assert.notEqual(first.directory, second.directory);
    for (const workspace of [first, second]) {
      assert.ok(relative(frontend, workspace.directory).startsWith(`..${sep}`));
      assert.notEqual(
        realpathSync(join(workspace.directory, "src")),
        realpathSync(join(frontend, "src")),
      );
      assert.equal(
        readFileSync(join(workspace.directory, "src", "input.ts"), "utf8"),
        "shared input",
      );
      assert.equal(
        realpathSync(join(workspace.directory, "node_modules")),
        realpathSync(join(frontend, "node_modules")),
      );
      const { default: config } = await import(
        pathToFileURL(join(workspace.directory, "next.config.js")).href
      );
      assert.deepEqual(config.experimental, { example: true });
      assert.deepEqual(config.turbopack.resolveAlias, {
        example: "./src/input.ts",
      });
      assert.equal(config.turbopack.root, dirname(frontend));
      assert.equal(existsSync(join(workspace.directory, ".env.local")), false);
      assert.equal(existsSync(join(workspace.directory, ".next")), false);
    }

    // These are the writes a Next dev process performs in its app root.
    writeFileSync(join(first.directory, "src", "input.ts"), "isolated source");
    mkdirSync(join(first.directory, ".next/dev/cache"), { recursive: true });
    writeFileSync(
      join(first.directory, ".next/dev/cache", "compiled-env"),
      "first run's hosts",
    );
    writeFileSync(
      join(first.directory, "tsconfig.json"),
      "first run's type globs",
    );
    writeFileSync(
      join(first.directory, "next-env.d.ts"),
      "first run's type imports",
    );
    assert.equal(
      readFileSync(join(second.directory, "tsconfig.json"), "utf8"),
      "original tsconfig.json",
    );
    assert.equal(
      readFileSync(join(frontend, "tsconfig.json"), "utf8"),
      "original tsconfig.json",
    );
    assert.equal(
      readFileSync(join(frontend, "next-env.d.ts"), "utf8"),
      "native type imports",
    );
    assert.equal(
      readFileSync(join(frontend, ".next/dev/cache", "native"), "utf8"),
      "warm native cache",
    );

    first.cleanup();
    assert.equal(existsSync(first.directory), false);
    assert.equal(
      readFileSync(join(second.directory, "src", "input.ts"), "utf8"),
      "shared input",
    );
    assert.equal(
      readFileSync(join(frontend, "src", "input.ts"), "utf8"),
      "shared input",
    );
    second.cleanup();
    second.cleanup();
    assert.equal(
      readFileSync(join(frontend, "public", "asset.txt"), "utf8"),
      "shared asset",
    );
    assert.equal(
      readFileSync(join(frontend, "node_modules", "dependency.txt"), "utf8"),
      "warm dependency",
    );
    assert.deepEqual(readdirSync(join(frontend, "../tmp/network-e2e")), []);
  }));

test("partial initialization removes only its own workspace", () =>
  fixture((frontend) => {
    const active = createAppWorkspace(frontend);
    rmSync(join(frontend, "postcss.config.mjs"));
    assert.throws(() => createAppWorkspace(frontend), { code: "ENOENT" });
    assert.deepEqual(readdirSync(join(frontend, "../tmp/network-e2e")), [
      active.directory.split("/").at(-1),
    ]);
    assert.equal(
      readFileSync(join(frontend, ".next/dev/cache", "native"), "utf8"),
      "warm native cache",
    );
    active.cleanup();
  }));
