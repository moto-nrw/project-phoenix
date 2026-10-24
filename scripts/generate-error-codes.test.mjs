import { mkdirSync, mkdtempSync, readFileSync, rmSync, symlinkSync, unlinkSync, writeFileSync } from "node:fs";
import { test } from "node:test";
import { tmpdir } from "node:os";
import { spawnSync } from "node:child_process";
import { join } from "node:path";
import assert from "node:assert/strict";
import { generate, validateRegistry, writeGenerated } from "./generate-error-codes.mjs";

const registry = JSON.parse(readFileSync(new URL("../error-registry.json", import.meta.url), "utf8"));

test("every registered code generates one Go constant and one TypeScript union member", () => {
  const entries = validateRegistry(registry);
  const generated = generate(registry);
  assert.equal((Object.values(generated.goFiles).join("\n").match(/ = "/g) ?? []).length, entries.length);
  assert.equal((Object.values(generated.goFiles).join("\n").match(/^\t"[a-z][a-z0-9_]*\.[a-z][a-z0-9_]*": +"/gm) ?? []).length, entries.length);
  assert.equal((generated.ts.match(/^  "[a-z][a-z0-9_]*\.[a-z][a-z0-9_]*",$/gm) ?? []).length, entries.length);
});

test("Go error code shards preserve every registered class mapping", () => {
  const entries = validateRegistry(registry);
  const generated = generate(registry);
  assert.match(generated.goFiles["error_codes.generated.go"], /var errorClassByCode = make\(map\[string\]string\)/);
  const source = Object.values(generated.goFiles).join("\n");
  const mappings = [...source.matchAll(/^\s*"([^"]+)":\s*"([^"]+)",$/gm)]
    .map(([, code, errorClass]) => [code, errorClass]);
  assert.deepEqual(mappings, entries.map((entry) => [entry.code, entry.class]));
});

test("duplicate codes are rejected", () => {
  const entry = registry.codes[0];
  assert.ok(entry);
  assert.throws(() => validateRegistry({ codes: [entry, { ...entry, legacy: "another_legacy_code" }] }), /duplicate error code/);
});

test("duplicate legacy mappings are rejected", () => {
  const entry = registry.codes[0];
  assert.ok(entry);
  assert.throws(() => validateRegistry({ codes: [entry, { ...entry, code: "other.code" }] }), /duplicate legacy code/);
});

test("unknown error classes are rejected", () => {
  const entry = registry.codes[0];
  assert.ok(entry);
  assert.throws(() => validateRegistry({ codes: [{ ...entry, class: "other" }] }), /invalid class/);
});

for (const count of [0, 300, 301, 2000]) {
  test(`${count} codes keep all Go output bounded and preserve every constant and class`, () => {
    const codes = Array.from({ length: count }, (_, index) => ({
      code: `probe.code_${index}`, class: index % 2 ? "input" : "server",
      parameters: [], legacy: `probe.code_${index}`,
    }));
    const { goFiles } = generate({ codes });
    for (const [name, source] of Object.entries(goFiles)) {
      assert.ok(source.split("\n").length - 1 <= 800, `${name} exceeded the ratchet`);
    }
    const source = Object.values(goFiles).join("\n");
    const constants = [...source.matchAll(/\t(Code\w+) += "([^"]+)"/g)];
    assert.equal(constants.length, count);
    assert.deepEqual(new Set(constants.map(([, , code]) => code)), new Set(codes.map(({ code }) => code)));
    const classes = [...source.matchAll(/\t"([^"]+)": +"([^"]+)",/g)];
    assert.equal(classes.length, count);
    assert.deepEqual(Object.fromEntries(classes.map(([, code, errorClass]) => [code, errorClass])),
      Object.fromEntries(codes.map(({ code, class: errorClass }) => [code, errorClass])));
  });
}

test("freshness check rejects missing, changed and obsolete shards without repairing them", (t) => {
  const root = mkdtempSync(join(tmpdir(), "error-code-generation-"));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  const directory = join(root, "backend/api/common");
  mkdirSync(directory, { recursive: true });
  mkdirSync(join(root, "frontend/src/lib"), { recursive: true });
  const generated = generate(registry);
  writeGenerated(root, generated);
  assert.doesNotThrow(() => writeGenerated(root, generated, { check: true }));
  const shard = join(directory, "error_codes_000.generated.go");
  unlinkSync(shard);
  assert.throws(() => writeGenerated(root, generated, { check: true }), /error_codes_000/);
  writeGenerated(root, generated);
  writeFileSync(shard, "// changed\n");
  assert.throws(() => writeGenerated(root, generated, { check: true }), /error_codes_000/);
  assert.equal(readFileSync(shard, "utf8"), "// changed\n");
  writeGenerated(root, generated);
  const stale = join(directory, "error_codes_999.generated.go");
  writeFileSync(stale, "// obsolete\n");
  assert.throws(() => writeGenerated(root, generated, { check: true }), /error_codes_999/);
  assert.equal(readFileSync(stale, "utf8"), "// obsolete\n");
  const handwritten = join(directory, "problem.go");
  writeFileSync(handwritten, "package common\n");
  writeGenerated(root, generated);
  assert.throws(() => readFileSync(stale), { code: "ENOENT" });
  assert.equal(readFileSync(handwritten, "utf8"), "package common\n");
  assert.doesNotThrow(() => writeGenerated(root, generated, { check: true }));
});

test("CLI generation and freshness checks work through a symlinked checkout", (t) => {
  const root = mkdtempSync(join(tmpdir(), "error-code-cli-"));
  const alias = `${root}-alias`;
  t.after(() => {
    rmSync(alias, { force: true });
    rmSync(root, { recursive: true, force: true });
  });
  for (const directory of ["scripts", "backend/api/common", "frontend/src/lib"]) {
    mkdirSync(join(root, directory), { recursive: true });
  }
  writeFileSync(join(root, "scripts/generate-error-codes.mjs"),
    readFileSync(new URL("./generate-error-codes.mjs", import.meta.url)));
  writeFileSync(join(root, "error-registry.json"), JSON.stringify(registry));
  symlinkSync(root, alias, "dir");
  const script = join(alias, "scripts/generate-error-codes.mjs");
  const generated = spawnSync(process.execPath, [script], { encoding: "utf8" });
  assert.equal(generated.status, 0, generated.stderr);
  const shard = join(root, "backend/api/common/error_codes_000.generated.go");
  assert.match(readFileSync(shard, "utf8"), /Code generated/);
  unlinkSync(shard);
  const stale = spawnSync(process.execPath, [script, "--check"], { encoding: "utf8" });
  assert.notEqual(stale.status, 0);
  assert.match(stale.stderr, /generated error codes are stale/);
});
