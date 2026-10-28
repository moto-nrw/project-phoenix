import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import test from "node:test";

import {
  BUDGET_FORMAT,
  compareBudgets,
  legacyCeiling,
  readBudgets,
} from "./bundle-policy.mjs";

const source = dirname(fileURLToPath(import.meta.url));
const approved = new Map([
  [
    "/parents/children",
    { referenceBytes: 100_000, maxBytes: legacyCeiling(100_000) },
  ],
]);

test("mechanically migrated budgets retain the original 2%/10KiB ceilings", () => {
  const budgets = readBudgets(join(source, "bundle-budgets"));
  assert.ok(budgets.size > 0);
  for (const [route, budget] of budgets)
    assert.equal(budget.maxBytes, legacyCeiling(budget.referenceBytes), route);
});

test("a significant improvement passes without lowering an observation baseline", () => {
  assert.deepEqual(
    compareBudgets(approved, new Map([["/parents/children", 60_000]])).failures,
    [],
  );
});

test("retired routes do not require cleanup of old observations", () => {
  const policies = new Map([
    ...approved,
    ["/parents/retired", { referenceBytes: 100_000, maxBytes: 110_240 }],
  ]);
  assert.deepEqual(
    compareBudgets(policies, new Map([["/parents/children", 100_000]]))
      .failures,
    [],
  );
});

test("the absolute ceiling still fails when a rolling PR delta is tiny", () => {
  const measured = new Map([["/parents/children", 111_000]]);
  const base = new Map([["/parents/children", 110_000]]);
  const result = compareBudgets(approved, measured, base);
  assert.equal(result.rows[0].delta, 1_000);
  assert.equal(result.failures.length, 1);
  assert.match(result.failures[0], /Obergrenze/);
});

test("a cached observation cannot grant a budget to a new route", () => {
  const route = "/parents/new";
  const result = compareBudgets(
    approved,
    new Map([[route, 1]]),
    new Map([[route, 1]]),
  );
  assert.equal(result.failures.length, 1);
  assert.match(result.failures[0], /ohne genehmigtes Bundle-Budget/);
});

test("equality at the approved ceiling passes but one more byte fails", () => {
  const limit = approved.get("/parents/children").maxBytes;
  assert.equal(
    compareBudgets(approved, new Map([["/parents/children", limit]])).failures
      .length,
    0,
  );
  assert.equal(
    compareBudgets(approved, new Map([["/parents/children", limit + 1]]))
      .failures.length,
    1,
  );
});

test("missing, unreadable and malformed budget policy fail closed", () => {
  const directory = mkdtempSync(join(tmpdir(), "moto-bundle-policy-"));
  try {
    assert.throws(() => readBudgets(join(directory, "absent")));
    mkdirSync(join(directory, "parents"));
    writeFileSync(join(directory, "parents", "children.json"), "invalid-json");
    assert.throws(() => readBudgets(directory));
    writeFileSync(
      join(directory, "parents", "children.json"),
      JSON.stringify({
        format: BUDGET_FORMAT,
        routes: { "/parents/children": { referenceBytes: 100, maxBytes: -1 } },
      }),
    );
    assert.throws(() => readBudgets(directory), /Invalid byte budget/);
  } finally {
    rmSync(directory, { recursive: true, force: true });
  }
});

test("portal ownership is enforced instead of silently accepting another fragment", () => {
  const directory = mkdtempSync(join(tmpdir(), "moto-bundle-policy-"));
  try {
    mkdirSync(join(directory, "tenant"));
    writeFileSync(
      join(directory, "tenant", "children.json"),
      JSON.stringify({
        format: BUDGET_FORMAT,
        routes: { "/parents/children": { referenceBytes: 100, maxBytes: 200 } },
      }),
    );
    assert.throws(
      () => readBudgets(directory),
      /belongs in parents\/children.json/,
    );
  } finally {
    rmSync(directory, { recursive: true, force: true });
  }
});

test("invalid observations and empty measurements fail closed", () => {
  assert.throws(() => compareBudgets(approved, new Map()), /No measured/);
  for (const bytes of [-1, 1.5, Number.NaN])
    assert.throws(
      () => compareBudgets(approved, new Map([["/parents/children", bytes]])),
      /Invalid measured/,
    );
});

test("--update is rejected before analysis rather than turning measurements into policy", () => {
  const result = spawnSync(
    process.execPath,
    [join(source, "bundle-ratchet.mjs"), "--update"],
    { encoding: "utf8" },
  );
  assert.notEqual(result.status, 0);
  assert.match(result.stderr, /cannot approve bundle budgets/);
});
