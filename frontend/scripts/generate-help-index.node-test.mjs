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
