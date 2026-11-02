import assert from "node:assert/strict";
import { mkdtempSync, readFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";
import { runInNewContext } from "node:vm";
import { parse } from "yaml";

import { readBaseComparison } from "./perf/bundle-measurement.mjs";
import { compareBudgets } from "./perf/bundle-policy.mjs";

function workflow(name) {
  return parse(
    readFileSync(
      new URL(`../../.github/workflows/${name}`, import.meta.url),
      "utf8",
    ),
  );
}

// Evaluate the actual workflow's simple GitHub boolean expressions, not a
// separately reimplemented release policy. Hyphenated output keys need [] in JS.
function expression(value, context) {
  const source = String(value)
    .replace(/^\$\{\{\s*|\s*\}\}$/g, "")
    .replace(/\.([\w]+(?:-[\w]+)+)/g, '["$1"]');
  return runInNewContext(source, context);
}

function bundleContext(eventName, baseRef) {
  return {
    github: {
      event_name: eventName,
      base_ref: baseRef,
      event: {
        pull_request: {
          base: { sha: "599ab1b" },
          head: { sha: "feature-head" },
        },
      },
    },
  };
}

test("release development -> main prepares no delta reference and never builds the old release base", (t) => {
  const steps = workflow("lint.yml").jobs["frontend-build"].steps;
  const context = bundleContext("pull_request", "main");
  const identify = steps.find(
    (step) =>
      step.name === "Identify bundle observation source and configuration",
  );
  const baseBuild = steps.find(
    (step) =>
      step.run === "node scripts/perf/bundle-measurement.mjs --build-base",
  );
  assert.equal(expression(identify.env.BASE_REVISION, context), "");
  assert.equal(expression(baseBuild.if, context), false);
  const selectHead = steps.find(
    (step) =>
      step.name === "Select feature PR head for bundle delta measurements",
  );
  assert.equal(
    expression(selectHead.if, context),
    false,
    "release budgets cover the tested merge tree",
  );
  const directory = mkdtempSync(join(tmpdir(), "moto-bundle-release-"));
  t.after(() => rmSync(directory, { recursive: true, force: true }));
  const base = readBaseComparison(directory);
  assert.equal(base.routes, null);
  const approved = new Map([
    ["/traeger", { referenceBytes: 1_000_000, maxBytes: 1_300_000 }],
  ]);
  const releaseGrowth = new Map([["/traeger", 1_289_707]]);
  assert.deepEqual(
    compareBudgets(approved, releaseGrowth, base.routes).failures,
    [],
    "a release cycle's 289707 B growth is not a feature PR delta",
  );
  assert.equal(
    compareBudgets(approved, new Map([["/traeger", 1_300_001]]), base.routes)
      .failures.length,
    1,
    "absolute ceilings still block releases",
  );
});

test("feature and stacked PRs retain delta checks; nightly has only absolute budgets", () => {
  const steps = workflow("lint.yml").jobs["frontend-build"].steps;
  const identify = steps.find((step) => step.id === "bundle-key");
  const baseBuild = steps.find(
    (step) =>
      step.run === "node scripts/perf/bundle-measurement.mjs --build-base",
  );
  const selectHead = steps.find(
    (step) =>
      step.name === "Select feature PR head for bundle delta measurements",
  );
  for (const baseRef of ["development", "feature/stack-base"]) {
    const context = bundleContext("pull_request", baseRef);
    assert.equal(expression(identify.env.BASE_REVISION, context), "599ab1b");
    assert.equal(expression(baseBuild.if, context), true);
    assert.equal(expression(selectHead.if, context), true);
    assert.equal(
      expression(selectHead.env.HEAD_REVISION, context),
      "feature-head",
    );
  }
  for (const eventName of ["schedule", "workflow_dispatch", "push"]) {
    const context = {
      github: { event_name: eventName, base_ref: "", event: {} },
    };
    assert.equal(expression(identify.env.BASE_REVISION, context), "");
    assert.equal(expression(baseBuild.if, context), false);
    assert.equal(expression(selectHead.if, context), false);
  }
});

test("regular merge build precedes the head-only analysis without buying another Next build", () => {
  const steps = workflow("lint.yml").jobs["frontend-build"].steps;
  const build = steps.findIndex((step) => step.run === "pnpm run build");
  const selectHead = steps.findIndex(
    (step) =>
      step.name === "Select feature PR head for bundle delta measurements",
  );
  const identify = steps.findIndex((step) => step.id === "bundle-key");
  const analyze = steps.findIndex(
    (step) => step.run === "pnpm run perf:bundle-ratchet",
  );
  assert.ok(
    build >= 0 &&
      build < selectHead &&
      selectHead < identify &&
      identify < analyze,
  );
  assert.equal(steps.filter((step) => step.run === "pnpm run build").length, 1);
  assert.match(
    steps[selectHead].run,
    /git checkout --detach "\$HEAD_REVISION"/,
  );
  assert.match(steps[selectHead].run, /pnpm install --frozen-lockfile/);
});

test("a lint.yml-only PR selects frontend checks and the reusable frontend build", () => {
  const main = workflow("main.yml");
  const filter = main.jobs["detect-changes"].steps.find(
    (step) => step.id === "filter",
  );
  const filters = parse(filter.with.filters);
  assert.ok(filters["workflow-lint"].includes(".github/workflows/lint.yml"));
  const context = {
    ...bundleContext("pull_request", "development"),
    steps: {
      scope: { outputs: { release: "false" } },
      filter: {
        outputs: new Proxy(
          { "workflow-lint": "true" },
          { get: (values, key) => values[key] ?? "false" },
        ),
      },
    },
  };
  const outputs = main.jobs["detect-changes"].outputs;
  assert.equal(expression(outputs["run-frontend"], context), true);
  const lintContext = {
    needs: {
      "detect-changes": {
        outputs: {
          "run-frontend-lint": String(
            expression(outputs["run-frontend-lint"], context),
          ),
        },
      },
    },
  };
  const runFrontend = expression(
    main.jobs.lint.with["run-frontend"],
    lintContext,
  );
  assert.equal(runFrontend, true);
  assert.equal(
    expression(workflow("lint.yml").jobs["frontend-build"].if, {
      inputs: { "run-frontend": runFrontend },
    }),
    true,
  );
  context.steps.filter.outputs["workflow-lint"] = "false";
  assert.equal(
    expression(outputs["run-frontend"], context),
    false,
    "unrelated docs still skip frontend tests",
  );
  assert.equal(
    expression(outputs["run-frontend-lint"], context),
    false,
    "unrelated docs still skip frontend builds",
  );
});

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
    undefined,
    "the regular build must retain GitHub's synthetic merge",
  );
  assert.equal(checkout.with["fetch-depth"], 0);
  assert.ok(
    build.steps.some(
      (step) =>
        step.run === "node scripts/perf/bundle-measurement.mjs --build-base" &&
        step.if ===
          "github.event_name == 'pull_request' && github.base_ref != 'main'",
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
