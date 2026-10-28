import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import { parse } from "yaml";

function workflow(name) {
  return parse(
    readFileSync(
      new URL(`../../.github/workflows/${name}`, import.meta.url),
      "utf8",
    ),
  );
}

test("bundle artifact reads stay job-scoped and are permitted by every reusable-workflow caller", () => {
  const called = workflow("lint.yml");
  assert.equal(called.jobs["frontend-build"].permissions.actions, "read");
  for (const [name, job] of Object.entries(called.jobs)) {
    if (name !== "frontend-build") {
      assert.equal(
        job.permissions?.actions,
        undefined,
        `${name} does not need artifact access`,
      );
    }
  }
  for (const name of ["main.yml", "nightly.yml"]) {
    const caller = workflow(name);
    const calls = Object.values(caller.jobs).filter(
      (job) => job.uses === "./.github/workflows/lint.yml",
    );
    assert.ok(calls.length > 0, `${name} must exercise its reusable lint call`);
    for (const job of calls) {
      // GitHub rejects the entire caller before starting any job when the
      // called workflow requests permissions absent from the caller.
      assert.equal(
        job.permissions.actions,
        "read",
        `${name} must permit the artifact reader`,
      );
    }
    assert.equal(
      caller.permissions.actions,
      undefined,
      "no workflow-wide permission expansion",
    );
  }
});
