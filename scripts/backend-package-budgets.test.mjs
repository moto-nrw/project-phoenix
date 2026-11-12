import { test } from "node:test";
import assert from "node:assert/strict";
import { mkdtempSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { budgetOf, check, loadBudgets, packageTimes } from "./backend-package-budgets.mjs";

const M = "github.com/moto-nrw/project-phoenix/";
const budgets = {
  defaultSeconds: 90,
  packages: { "modules/dataimport/importtest": { seconds: 300, reason: "r" }, api: { seconds: 200, reason: "r" } },
};

test("the repository budget file loads", () => {
  const loaded = loadBudgets();
  assert.ok(loaded.defaultSeconds > 0);
});

test("budget entries need a reason and more than the default", () => {
  const dir = mkdtempSync(join(tmpdir(), "budgets-"));
  const file = join(dir, "b.json");
  writeFileSync(file, JSON.stringify({ defaultSeconds: 90, packages: { api: { seconds: 80, reason: "x" } } }));
  assert.throws(() => loadBudgets(file), /above defaultSeconds/);
  writeFileSync(file, JSON.stringify({ defaultSeconds: 90, packages: { api: { seconds: 120 } } }));
  assert.throws(() => loadBudgets(file), /needs a reason/);
});

test("packageTimes reads the package events, not the test events", () => {
  const lines = [
    { Action: "pass", Package: `${M}api`, Test: "TestX", Elapsed: 500 },
    { Action: "pass", Package: `${M}api`, Elapsed: 150.5 },
    { Action: "fail", Package: `${M}b`, Elapsed: 95 },
    { Action: "output", Package: `${M}b`, Output: "x" },
  ]
    .map((e) => JSON.stringify(e))
    .join("\n");
  const times = packageTimes(lines + "\nnot json\n");
  assert.equal(times.get(`${M}api`), 150.5);
  assert.equal(times.get(`${M}b`), 95);
  assert.equal(times.size, 2);
});

test("check reports packages over their own or the default budget", () => {
  const times = new Map([
    [`${M}api`, 150],
    [`${M}b`, 95],
    [`${M}modules/dataimport/importtest`, 301],
  ]);
  const { over } = check(budgets, times);
  assert.deepEqual(
    over.map((r) => r.pkg),
    ["modules/dataimport/importtest", "b"],
  );
  assert.equal(budgetOf(budgets, `${M}b`), 90);
});
