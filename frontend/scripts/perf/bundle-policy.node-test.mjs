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
  NEAR_CEILING_BYTES,
  readBudgets,
} from "./bundle-policy.mjs";

const source = dirname(fileURLToPath(import.meta.url));
const approved = new Map([
  [
    "/parents/children",
    { referenceBytes: 100_000, maxBytes: legacyCeiling(100_000) },
  ],
]);

/**
 * Explicit, reviewed increases over the migrated 2 %/10 KiB ceilings:
 * #5260 room detail (logout safeguards) and #5399: the parents'
 * Elternsprechtag page (#4801) was budgeted without room for the
 * session-end barrier every signed-in page carries.
 */
const increases = new Map([
  ["/[tenant]/rooms/[id]", 2_048],
  ["/parents/children/[id]/elternsprechtag", 1_024],
]);

test("bundle policy retains migration ceilings except the explicit increases", () => {
  const budgets = readBudgets(join(source, "bundle-budgets"));
  assert.ok(budgets.size > 0);
  assert.deepEqual(budgets.get("/[tenant]/rooms/[id]"), {
    referenceBytes: 1_026_710,
    maxBytes: 1_049_292,
  });
  for (const [route, budget] of budgets) {
    assert.equal(
      budget.maxBytes,
      legacyCeiling(budget.referenceBytes) + (increases.get(route) ?? 0),
      route,
    );
  }
});

test("each policy still rejects one byte over its ceiling", () => {
  const budgets = readBudgets(join(source, "bundle-budgets"));
  for (const [route, { maxBytes }] of budgets) {
    assert.deepEqual(
      compareBudgets(budgets, new Map([[route, maxBytes]])).failures,
      [],
      route,
    );
    assert.equal(
      compareBudgets(budgets, new Map([[route, maxBytes + 1]])).failures.length,
      1,
      route,
    );
  }
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
  // The base keeps more than NEAR_CEILING_BYTES of headroom (cap 110 240).
  const measured = new Map([["/parents/children", 110_300]]);
  const base = new Map([["/parents/children", 109_000]]);
  const result = compareBudgets(approved, measured, base);
  assert.equal(result.rows[0].delta, 1_300);
  assert.equal(result.failures.length, 1);
  assert.match(result.failures[0], /Obergrenze/);
});

test("a base within NEAR_CEILING_BYTES of its ceiling leaves PRs to their own delta (#5396)", () => {
  const route = "/parents/children";
  const limit = approved.get(route).maxBytes;
  const over = new Map([[route, limit + 13]]);
  const near = compareBudgets(
    approved,
    over,
    new Map([[route, limit - NEAR_CEILING_BYTES + 1]]),
  );
  assert.deepEqual(near.failures, []);
  assert.equal(near.warnings.length, 1);
  assert.match(near.warnings[0], /eigenes Delta/);
  const intact = compareBudgets(
    approved,
    over,
    new Map([[route, limit - NEAR_CEILING_BYTES]]),
  );
  assert.equal(intact.failures.length, 1);
  assert.match(intact.failures[0], /Obergrenze/);
  // Development pushes, the nightly and release PRs compare without a base.
  assert.equal(compareBudgets(approved, over).failures.length, 1);
  // The own delta still blocks next to the ceiling.
  assert.ok(
    compareBudgets(
      approved,
      new Map([[route, limit + 11_000]]),
      new Map([[route, limit - 100]]),
    ).failures.some((failure) => /eigenes PR-Delta/.test(failure)),
  );
});

test("own PR delta over tolerance fails even with plenty of absolute headroom", () => {
  const policies = new Map([
    ["/parents/children", { referenceBytes: 100_000, maxBytes: 200_000 }],
  ]);
  const result = compareBudgets(
    policies,
    new Map([["/parents/children", 110_241]]),
    new Map([["/parents/children", 100_000]]),
  );
  assert.equal(result.failures.length, 1);
  assert.match(result.failures[0], /eigenes PR-Delta/);
});

test("historical baseline drift plus 512 B own growth passes (#5181/#5213)", () => {
  const route = "/[tenant]/home";
  const policies = new Map([
    [route, { referenceBytes: 1_058_646, maxBytes: legacyCeiling(1_058_646) }],
  ]);
  const result = compareBudgets(
    policies,
    new Map([[route, 1_079_590 + 512]]),
    new Map([[route, 1_079_590]]),
  );
  assert.equal(result.rows[0].delta, 512);
  // The base sat 229 B under the ceiling: the crossing warns on the PR and
  // stays red on development until the ceiling is reset (#5396).
  assert.deepEqual(result.failures, []);
  assert.equal(result.warnings.length, 1);
  assert.match(result.warnings[0], /Obergrenze/);
  policies.get(route).maxBytes = 1_100_000;
  assert.deepEqual(
    compareBudgets(
      policies,
      new Map([[route, 1_079_590 + 512]]),
      new Map([[route, 1_079_590]]),
    ).failures,
    [],
  );
});

test("only inherited absolute excess warns on PRs but remains red on development", () => {
  const measured = new Map([["/parents/children", 120_512]]);
  const result = compareBudgets(
    approved,
    measured,
    new Map([["/parents/children", 120_000]]),
  );
  assert.deepEqual(result.failures, []);
  assert.equal(result.warnings.length, 1);
  assert.match(result.warnings[0], /Obergrenze/);
  assert.equal(compareBudgets(approved, measured).failures.length, 1);
  assert.ok(
    compareBudgets(
      approved,
      new Map([["/parents/children", 131_000]]),
      new Map([["/parents/children", 120_000]]),
    ).failures.some((failure) => /eigenes PR-Delta/.test(failure)),
  );
});

test("PR delta accepts equality, rejects one byte more and secures improvements automatically", () => {
  const policies = new Map([
    ["/parents/children", { referenceBytes: 100_000, maxBytes: 200_000 }],
  ]);
  const base = new Map([["/parents/children", 100_000]]);
  assert.deepEqual(
    compareBudgets(policies, new Map([["/parents/children", 110_240]]), base)
      .failures,
    [],
  );
  assert.equal(
    compareBudgets(policies, new Map([["/parents/children", 110_241]]), base)
      .failures.length,
    1,
  );
  assert.deepEqual(
    compareBudgets(policies, new Map([["/parents/children", 60_000]]), base)
      .failures,
    [],
  );
  assert.equal(
    compareBudgets(policies, base, new Map([["/parents/children", 60_000]]))
      .failures.length,
    1,
  );
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
