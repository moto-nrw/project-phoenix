import { readFileSync } from "node:fs";
import { test } from "node:test";
import assert from "node:assert/strict";
import { generate, validateRegistry } from "./generate-error-codes.mjs";

const registry = JSON.parse(readFileSync(new URL("../error-registry.json", import.meta.url), "utf8"));

test("every registered code generates one Go constant and one TypeScript union member", () => {
  const entries = validateRegistry(registry);
  const generated = generate(registry);
  assert.equal((generated.go.match(/ = "/g) ?? []).length, entries.length);
  assert.equal((generated.ts.match(/^  "[a-z][a-z0-9_]*\.[a-z][a-z0-9_]*",$/gm) ?? []).length, entries.length);
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
