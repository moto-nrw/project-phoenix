import { mkdirSync, mkdtempSync, readFileSync, rmSync, symlinkSync, unlinkSync, writeFileSync } from "node:fs";
import { test } from "node:test";
import { tmpdir } from "node:os";
import { spawnSync } from "node:child_process";
import { join } from "node:path";
import assert from "node:assert/strict";
import { readRegistry } from "./error-registry.mjs";
import { generate, validateRegistry, writeGenerated } from "./generate-error-codes.mjs";

const registry = readRegistry();

test("adding a code leaves every other domain's Go and TypeScript output byte-identical", () => {
  const entry = (code) => ({ code, class: "server", parameters: [], legacy: code });
  const before = generate({ codes: [entry("alpha.last"), entry("beta.first")] });
  const after = generate({ codes: [entry("alpha.first"), entry("alpha.last"), entry("beta.first")] });
  assert.ok(before.goFiles["error_codes_beta.generated.go"], "domain-addressed Go output missing");
  assert.equal(before.goFiles["error_codes_beta.generated.go"], after.goFiles["error_codes_beta.generated.go"]);
  assert.ok(before.tsFiles?.["error-codes/beta.generated.ts"], "domain-addressed TypeScript output missing");
  assert.equal(before.tsFiles["error-codes/beta.generated.ts"], after.tsFiles["error-codes/beta.generated.ts"]);
  assert.equal(before.ts, after.ts, "the public facade changes only when domains change");
});

test("fragment discovery validates global legacy identities, domain ownership and missing sources", (t) => {
  const root = mkdtempSync(join(tmpdir(), "error-registry-"));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  mkdirSync(join(root, "error-registry"));
  const write = (domain, code, legacy) => writeFileSync(join(root, "error-registry", `${domain}.json`),
    JSON.stringify({ codes: [{ code, legacy, class: "server", parameters: [] }] }));
  assert.throws(() => readRegistry(root), /no error registry fragments/);
  write("alpha", "alpha.first", "legacy_first");
  write("beta", "beta.first", "legacy_first");
  assert.throws(() => readRegistry(root), /duplicate legacy code/);
  write("beta", "beta.first", "legacy_second");
  assert.deepEqual(readRegistry(root).codes.map(({ code }) => code), ["alpha.first", "beta.first"]);
  write("beta", "alpha.misplaced", "legacy_second");
  assert.throws(() => readRegistry(root), /wrong domain/);
});

test("a longer symbol does not reformat earlier declarations in its domain", () => {
  const entry = (code) => ({ code, class: "input", parameters: [], legacy: code });
  const before = generate({ codes: [entry("alpha.first")] });
  const after = generate({ codes: [entry("alpha.first"), entry("alpha.substantially_longer_name")] });
  for (const line of before.goFiles["error_codes_alpha.generated.go"].split("\n")) {
    assert.ok(after.goFiles["error_codes_alpha.generated.go"].split("\n").includes(line), line);
  }
});

test("every registered code generates one Go constant and one TypeScript union member", () => {
  const entries = validateRegistry(registry);
  const generated = generate(registry);
  assert.equal((Object.values(generated.goFiles).join("\n").match(/ = "/g) ?? []).length, entries.length);
  assert.equal((Object.values(generated.goFiles).join("\n").match(/^\t\{Code\w+, "/gm) ?? []).length, entries.length);
  const tsCodes = Object.values(generated.tsFiles).flatMap((source) => {
    const declaration = source.match(/export const codes = (\[[\s\S]*?\]) as const;/);
    assert.ok(declaration, "domain is missing its TypeScript code union");
    return JSON.parse(declaration[1].replace(/,\s*\]/g, "]"));
  });
  assert.deepEqual(tsCodes, entries.map((entry) => entry.code));
});

test("Go error code shards preserve every registered class mapping", () => {
  const entries = validateRegistry(registry);
  const generated = generate(registry);
  assert.match(generated.goFiles["error_codes.generated.go"], /var errorClassByCode = make\(map\[string\]string\)/);
  const source = Object.values(generated.goFiles).join("\n");
  const constants = Object.fromEntries([...source.matchAll(/const (Code\w+) = "([^"]+)"/g)].map(([, symbol, code]) => [symbol, code]));
  const mappings = [...source.matchAll(/^\s*\{(Code\w+), "([^"]+)"\},$/gm)]
    .map(([, symbol, errorClass]) => [constants[symbol], errorClass]);
  assert.deepEqual(mappings.sort(([a], [b]) => a.localeCompare(b, "en")), entries.map((entry) => [entry.code, entry.class]));
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
    const constants = [...source.matchAll(/const (Code\w+) += "([^"]+)"/g)];
    assert.equal(constants.length, count);
    assert.deepEqual(new Set(constants.map(([, , code]) => code)), new Set(codes.map(({ code }) => code)));
    const names = Object.fromEntries(constants.map(([, symbol, code]) => [symbol, code]));
    const classes = [...source.matchAll(/\t\{(Code\w+), "([^"]+)"\},/g)];
    assert.equal(classes.length, count);
    assert.deepEqual(Object.fromEntries(classes.map(([, symbol, errorClass]) => [names[symbol], errorClass])),
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
  const shard = join(directory, "error_codes_general.generated.go");
  unlinkSync(shard);
  assert.throws(() => writeGenerated(root, generated, { check: true }), /error_codes_general/);
  writeGenerated(root, generated);
  writeFileSync(shard, "// changed\n");
  assert.throws(() => writeGenerated(root, generated, { check: true }), /error_codes_general/);
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
  const staleTS = join(root, "frontend/src/lib/error-codes/obsolete.generated.ts");
  writeFileSync(staleTS, "// obsolete\n");
  assert.throws(() => writeGenerated(root, generated, { check: true }), /obsolete.generated.ts/);
  assert.equal(readFileSync(staleTS, "utf8"), "// obsolete\n");
  writeGenerated(root, generated);
  assert.throws(() => readFileSync(staleTS), { code: "ENOENT" });
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
  for (const script of ["generate-error-codes.mjs", "error-registry.mjs"]) {
    writeFileSync(join(root, "scripts", script), readFileSync(new URL(script, import.meta.url)));
  }
  mkdirSync(join(root, "error-registry"));
  const domains = new Set(registry.codes.map(({ code }) => code.split(".")[0]));
  for (const domain of domains) {
    writeFileSync(join(root, "error-registry", `${domain}.json`), JSON.stringify({ codes: registry.codes.filter(({ code }) => code.startsWith(`${domain}.`)) }));
  }
  symlinkSync(root, alias, "dir");
  const script = join(alias, "scripts/generate-error-codes.mjs");
  const generated = spawnSync(process.execPath, [script], { encoding: "utf8" });
  assert.equal(generated.status, 0, generated.stderr);
  const shard = join(root, "backend/api/common/error_codes_general.generated.go");
  assert.match(readFileSync(shard, "utf8"), /Code generated/);
  unlinkSync(shard);
  const stale = spawnSync(process.execPath, [script, "--check"], { encoding: "utf8" });
  assert.notEqual(stale.status, 0);
  assert.match(stale.stderr, /generated error codes are stale/);
});
