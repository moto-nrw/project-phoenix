import assert from "node:assert/strict";
import { mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import test from "node:test";

import {
  CEILINGS_FORMAT,
  compareBundles,
  deltaTolerance,
  GROWTH_APPROVAL_LABEL,
  NEAR_CEILING_BYTES,
  readCeilings,
} from "./bundle-policy.mjs";

const source = dirname(fileURLToPath(import.meta.url));
const route = "/parents/children";
const none = new Map();
const ceilings = new Map([[route, 110_240]]);

test("the checked-in ceilings stay a short list with each entry enforced to the byte", () => {
  const checkedIn = readCeilings(join(source, "bundle-ceilings.json"));
  assert.ok(checkedIn.size > 0 && checkedIn.size <= 12, "few hot routes only");
  for (const [hot, maxBytes] of checkedIn) {
    const measured = new Map(checkedIn);
    assert.deepEqual(compareBundles(checkedIn, measured).failures, [], hot);
    measured.set(hot, maxBytes + 1);
    assert.equal(compareBundles(checkedIn, measured).failures.length, 1, hot);
  }
});

test("PR delta accepts the tolerance, rejects one byte more and passes improvements", () => {
  const base = new Map([[route, 100_000]]);
  assert.equal(deltaTolerance(100_000), 10_240);
  assert.equal(deltaTolerance(1_000_000), 20_000);
  assert.deepEqual(
    compareBundles(none, new Map([[route, 110_240]]), base).failures,
    [],
  );
  const over = compareBundles(none, new Map([[route, 110_241]]), base);
  assert.equal(over.failures.length, 1);
  assert.match(over.failures[0], /PR-Zuwachs 10241 Bytes/);
  assert.ok(over.failures[0].includes(GROWTH_APPROVAL_LABEL));
  assert.deepEqual(
    compareBundles(none, new Map([[route, 60_000]]), base).failures,
    [],
  );
});

test("the approval label turns delta failures into warnings", () => {
  const result = compareBundles(
    none,
    new Map([[route, 200_000]]),
    new Map([[route, 100_000]]),
    { growthApproved: true },
  );
  assert.deepEqual(result.failures, []);
  assert.equal(result.warnings.length, 1);
  assert.match(result.warnings[0], /Freigegeben/);
});

test("routes without a ceiling need no policy entry, also when new", () => {
  const measured = new Map([
    [route, 100_000],
    ["/parents/new", 5_000_000],
  ]);
  assert.deepEqual(
    compareBundles(ceilings, measured, new Map([[route, 100_000]])).failures,
    [],
  );
});

test("a ceiling fails one byte over, also with a tiny PR delta and with the label", () => {
  assert.deepEqual(
    compareBundles(ceilings, new Map([[route, 110_240]])).failures,
    [],
  );
  // The base keeps more than NEAR_CEILING_BYTES of headroom.
  const result = compareBundles(
    ceilings,
    new Map([[route, 110_241]]),
    new Map([[route, 109_000]]),
    { growthApproved: true },
  );
  assert.equal(result.failures.length, 1);
  assert.match(result.failures[0], /Obergrenze 110240/);
  assert.match(result.failures[0], /Label gilt hier nicht/);
});

test("a base within NEAR_CEILING_BYTES of its ceiling leaves PRs to their own delta (#5396)", () => {
  const over = new Map([[route, 110_240 + 13]]);
  const near = compareBundles(
    ceilings,
    over,
    new Map([[route, 110_240 - NEAR_CEILING_BYTES + 1]]),
  );
  assert.deepEqual(near.failures, []);
  assert.equal(near.warnings.length, 1);
  assert.match(near.warnings[0], /eigener Zuwachs/);
  assert.equal(
    compareBundles(
      ceilings,
      over,
      new Map([[route, 110_240 - NEAR_CEILING_BYTES]]),
    ).failures.length,
    1,
  );
  // Development pushes, the nightly and release PRs compare without a base.
  assert.equal(compareBundles(ceilings, over).failures.length, 1);
  // The own delta still blocks next to the ceiling.
  assert.ok(
    compareBundles(
      ceilings,
      new Map([[route, 110_240 + 11_000]]),
      new Map([[route, 110_240 - 100]]),
    ).failures.some((failure) => /PR-Zuwachs/.test(failure)),
  );
});

test("a ceiling whose route is no longer measured fails instead of silently guarding nothing", () => {
  const result = compareBundles(ceilings, new Map([["/parents", 1]]));
  assert.equal(result.failures.length, 1);
  assert.match(result.failures[0], /ohne gemessene Route/);
});

test("missing, unreadable and malformed ceiling policy fail closed", () => {
  const directory = mkdtempSync(join(tmpdir(), "moto-bundle-ceilings-"));
  const file = join(directory, "bundle-ceilings.json");
  try {
    assert.throws(() => readCeilings(file));
    writeFileSync(file, "invalid-json");
    assert.throws(() => readCeilings(file));
    for (const routes of [{}, [], { [route]: -1 }, { relative: 1 }]) {
      writeFileSync(file, JSON.stringify({ format: CEILINGS_FORMAT, routes }));
      assert.throws(() => readCeilings(file), /bundle ceiling/i);
    }
    writeFileSync(file, JSON.stringify({ format: "other", routes: { [route]: 1 } }));
    assert.throws(() => readCeilings(file), /Invalid bundle ceiling policy/);
  } finally {
    rmSync(directory, { recursive: true, force: true });
  }
});

test("invalid observations and empty measurements fail closed", () => {
  assert.throws(() => compareBundles(ceilings, new Map()), /No measured/);
  for (const bytes of [-1, 1.5, Number.NaN])
    assert.throws(
      () => compareBundles(ceilings, new Map([[route, bytes]])),
      /Invalid measured/,
    );
  assert.throws(
    () => compareBundles(none, new Map([[route, 1]]), new Map([[route, -1]])),
    /Invalid base/,
  );
});
