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

test("bundle builds need no artifact-read token and do not persist checkout credentials", () => {
  const called = workflow("lint.yml");
  const build = called.jobs["frontend-build"];
  assert.equal(build.permissions.actions, undefined);
  const checkout = build.steps.find((step) =>
    step.uses?.startsWith("actions/checkout@"),
  );
  assert.equal(checkout.with["persist-credentials"], false);
  assert.equal(
    checkout.with.ref,
    "${{ github.event.pull_request.head.sha || github.sha }}",
  );
  assert.equal(checkout.with["fetch-depth"], 0);
  assert.ok(
    build.steps.some(
      (step) =>
        step.run === "node scripts/perf/bundle-base.mjs" &&
        step.if === "github.event_name == 'pull_request'",
    ),
  );
  assert.equal(
    build.steps.some((step) => step.env?.GH_TOKEN || step.env?.GITHUB_TOKEN),
    false,
  );
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
      // The reusable build now needs only source access, including on forks.
      assert.equal(
        job.permissions.contents,
        "read",
        `${name} must permit source access`,
      );
    }
    assert.equal(
      caller.permissions.actions,
      undefined,
      "no workflow-wide permission expansion",
    );
  }
});
