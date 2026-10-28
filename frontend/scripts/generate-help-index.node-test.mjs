import assert from "node:assert/strict";
import {
  mkdirSync,
  mkdtempSync,
  readFileSync,
  rmSync,
  writeFileSync,
} from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";
import ts from "typescript";
import { synchronizeHelpIndex } from "./generate-help-index.mjs";

async function fixture(run) {
  const directory = mkdtempSync(join(tmpdir(), "moto-help-index-"));
  mkdirSync(join(directory, "articles"));
  try {
    await run(directory);
  } finally {
    rmSync(directory, { recursive: true, force: true });
  }
}

test("new articles are discovered without a hand-edited import list or product-order mutation", () =>
  fixture(async (directory) => {
    const facade = "// Explicit product order stays author-owned.\n";
    writeFileSync(join(directory, "help-content.ts"), facade);
    writeFileSync(
      join(directory, "articles", "first.ts"),
      "export function firstTopic() { return {title: 'Erstes Thema'}; }",
    );
    await synchronizeHelpIndex({ directory });
    await synchronizeHelpIndex({ directory, check: true });
    writeFileSync(
      join(directory, "articles", "second.ts"),
      "export function secondTopic() { return {title: 'Zweites Thema'}; }",
    );
    await assert.rejects(
      () => synchronizeHelpIndex({ directory, check: true }),
      /stale/,
    );
    await synchronizeHelpIndex({ directory });
    const generated = readFileSync(
      join(directory, "help-articles.generated.ts"),
      "utf8",
    );
    assert.match(generated, /firstTopic/);
    assert.match(generated, /secondTopic/);
    const emitted = ts.transpileModule(generated, {
      compilerOptions: { module: ts.ModuleKind.CommonJS },
    }).outputText;
    assert.doesNotMatch(emitted, /require\(|from ["']\.\/articles\//);
    const emittedModule = { exports: {} };
    new Function("module", "exports", emitted)(
      emittedModule,
      emittedModule.exports,
    );
    assert.deepEqual(emittedModule.exports.firstTopic(), {
      title: "Erstes Thema",
    });
    assert.deepEqual(emittedModule.exports.secondTopic(), {
      title: "Zweites Thema",
    });
    assert.match(generated, /satisfies typeof authored_firstTopic/);
    assert.match(generated, /firstTopic as authored_firstTopic/);
    assert.match(generated, /import type/);
    assert.equal(
      readFileSync(join(directory, "help-content.ts"), "utf8"),
      facade,
    );
  }));

test("duplicate article factory names fail instead of silently shadowing an article", () =>
  fixture(async (directory) => {
    writeFileSync(
      join(directory, "articles", "one.ts"),
      "export function repeatedTopic() {}",
    );
    writeFileSync(
      join(directory, "articles", "two.ts"),
      "export function repeatedTopic() {}",
    );
    await assert.rejects(
      () => synchronizeHelpIndex({ directory }),
      /Duplicate/,
    );
  }));

test("empty article sources or ambiguous exports cannot produce a green empty index", () =>
  fixture(async (directory) => {
    await assert.rejects(() => synchronizeHelpIndex({ directory }), /empty/);
    writeFileSync(
      join(directory, "articles", "ambiguous.ts"),
      "export function one() {} export function two() {}",
    );
    await assert.rejects(
      () => synchronizeHelpIndex({ directory }),
      /exactly one/,
    );
  }));

test("source changes invalidate the runtime derivative, without a new export", () =>
  fixture(async (directory) => {
    const article = join(directory, "articles", "first.ts");
    writeFileSync(
      article,
      "export function firstTopic() { return {title: 'Before'}; }",
    );
    await synchronizeHelpIndex({ directory });
    writeFileSync(
      article,
      "export function firstTopic() { return {title: 'After'}; }",
    );
    await assert.rejects(
      () => synchronizeHelpIndex({ directory, check: true }),
      /stale/,
    );
    await synchronizeHelpIndex({ directory });
    assert.match(
      readFileSync(join(directory, "help-articles.generated.ts"), "utf8"),
      /After/,
    );
  }));

test("module-local declarations and side effects cannot disappear during compaction", () =>
  fixture(async (directory) => {
    const article = join(directory, "articles", "first.ts");
    writeFileSync(
      article,
      "const lost = 1; export function firstTopic() { return lost; }",
    );
    await assert.rejects(
      () => synchronizeHelpIndex({ directory }),
      /imports only/,
    );
    writeFileSync(
      article,
      "import './side-effect'; export function firstTopic() { return 1; }",
    );
    await assert.rejects(
      () => synchronizeHelpIndex({ directory }),
      /named bindings/,
    );
  }));

test("empty and inline-type-only value imports cannot disappear behind a green freshness check", () =>
  fixture(async (directory) => {
    const article = join(directory, "articles", "first.ts");
    const factory = "export function firstTopic() { return {title: 'First'}; }";
    writeFileSync(article, factory);
    await synchronizeHelpIndex({ directory });
    const target = join(directory, "help-articles.generated.ts");
    const before = readFileSync(target, "utf8");
    for (const declaration of [
      "import {} from '../side-effect';",
      "import { type Thing } from '../side-effect';",
    ]) {
      writeFileSync(article, declaration + factory);
      await assert.rejects(
        () => synchronizeHelpIndex({ directory, check: true }),
        /value imports must use import type/,
      );
      assert.equal(readFileSync(target, "utf8"), before);
    }
  }));
