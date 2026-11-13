import assert from "node:assert/strict";
import { mkdtempSync, readFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";
import { runInNewContext } from "node:vm";
import { parse } from "yaml";

import { readBaseComparison } from "./perf/bundle-measurement.mjs";
import {
  compareBundles,
  GROWTH_APPROVAL_LABEL,
} from "./perf/bundle-policy.mjs";

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
  const ceilings = new Map([["/traeger", 1_300_000]]);
  const releaseGrowth = new Map([["/traeger", 1_289_707]]);
  assert.deepEqual(
    compareBundles(ceilings, releaseGrowth, base.routes).failures,
    [],
    "a release cycle's growth is not a feature PR delta",
  );
  assert.equal(
    compareBundles(ceilings, new Map([["/traeger", 1_300_001]]), base.routes)
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

test("one regular build of the synthetic merge runs beside the head-only analysis, not before it", () => {
  const jobs = workflow("lint.yml").jobs;
  const isBuild = (step) => step.run === "pnpm run build";
  const mergeBuild = jobs["frontend-merge-build"];
  assert.equal(mergeBuild.steps.filter(isBuild).length, 1);
  assert.equal(mergeBuild.if, jobs["frontend-build"].if);
  const mergeCheckout = mergeBuild.steps.find((step) =>
    step.uses?.startsWith("actions/checkout@"),
  );
  assert.equal(
    mergeCheckout.with?.ref,
    undefined,
    "the regular build must retain GitHub's synthetic merge",
  );
  assert.ok(
    mergeBuild.steps.findIndex(isBuild) >
      mergeBuild.steps.findIndex((step) => step.run?.includes("pnpm install")),
  );
  // Both builds see the same public build environment.
  const buildEnv = mergeBuild.steps.find(isBuild).env;
  for (const [key, value] of Object.entries(jobs["frontend-build"].env))
    assert.equal(buildEnv[key], value, key);
  for (const [name, job] of Object.entries(jobs)) {
    if (name !== "frontend-merge-build")
      assert.equal(job.steps.filter(isBuild).length, 0, name);
  }

  const steps = jobs["frontend-build"].steps;
  const selectHead = steps.findIndex(
    (step) =>
      step.name === "Select feature PR head for bundle delta measurements",
  );
  const identify = steps.findIndex((step) => step.id === "bundle-key");
  const analyze = steps.findIndex(
    (step) => step.run === "pnpm run perf:bundle-ratchet",
  );
  assert.ok(selectHead >= 0 && selectHead < identify && identify < analyze);
  assert.match(
    steps[selectHead].run,
    /git checkout --detach "\$HEAD_REVISION"/,
  );
  assert.match(steps[selectHead].run, /pnpm install --frozen-lockfile/);
});

test("a merge-base measurement is restored by its identity before the build and saved only after a cold build", () => {
  const steps = workflow("lint.yml").jobs["frontend-build"].steps;
  const index = (predicate) => steps.findIndex(predicate);
  const identify = index((step) => step.id === "bundle-key");
  const restore = index((step) => step.id === "bundle-base-cache");
  const build = index(
    (step) =>
      step.run === "node scripts/perf/bundle-measurement.mjs --build-base",
  );
  const save = index((step) => step.uses?.startsWith("actions/cache/save@"));
  const ratchet = index((step) => step.run === "pnpm run perf:bundle-ratchet");
  assert.ok(
    identify >= 0 &&
      identify < restore &&
      restore < build &&
      build < save &&
      save < ratchet,
  );
  assert.match(steps[restore].uses, /^actions\/cache\/restore@/);
  const key = "${{ steps.bundle-key.outputs.base-measurement-key }}";
  for (const step of [steps[restore], steps[save]]) {
    assert.equal(step.with.key, key);
    assert.equal(step.with.path, "frontend/perf-results/bundle-base.json");
    assert.equal(step.with["restore-keys"], undefined, "exact identity only");
  }
  // The script decides whether a restored file counts, so the build step
  // never skips on a cache hit; only the save does.
  assert.equal(steps[build].if, steps[restore].if);
  assert.equal(
    steps[save].if,
    `${steps[restore].if} && steps.bundle-base-cache.outputs.cache-hit != 'true'`,
  );
  for (const baseRef of ["development", "feature/stack-base"])
    assert.equal(
      expression(steps[restore].if, bundleContext("pull_request", baseRef)),
      true,
    );
  assert.equal(
    expression(steps[restore].if, bundleContext("pull_request", "main")),
    false,
  );
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
    "the contract tests run on GitHub's synthetic merge",
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
  const tokenSteps = build.steps.filter(
    (step) => step.env?.GH_TOKEN || step.env?.GITHUB_TOKEN,
  );
  assert.deepEqual(
    tokenSteps.map((step) => step.id),
    ["bundle-approval"],
    "only the label read sees a token",
  );
  assert.equal(build.steps[0].id, "bundle-approval", "it runs before PR code");
  assert.equal(build.steps[0].uses, undefined);
  assert.deepEqual(build.permissions, {
    contents: "read",
    "pull-requests": "read",
  });
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
      assert.equal(
        job.permissions["pull-requests"],
        "read",
        `${name} must permit the approval label read`,
      );
    }
    assert.equal(
      caller.permissions.actions,
      undefined,
      "no workflow-wide permission expansion",
    );
  }
});

test("the approval label is read fresh on feature PRs only and reaches the ratchet", () => {
  const steps = workflow("lint.yml").jobs["frontend-build"].steps;
  const approval = steps.find((step) => step.id === "bundle-approval");
  const ratchet = steps.find(
    (step) => step.run === "pnpm run perf:bundle-ratchet",
  );
  assert.ok(approval.run.includes(`'${GROWTH_APPROVAL_LABEL}'`));
  assert.match(
    approval.run,
    /gh api "repos\/\$REPOSITORY\/issues\/\$PR_NUMBER\/labels"/,
  );
  for (const baseRef of ["development", "feature/stack-base"])
    assert.equal(
      expression(approval.if, bundleContext("pull_request", baseRef)),
      true,
    );
  assert.equal(
    expression(approval.if, bundleContext("pull_request", "main")),
    false,
  );
  for (const [approved, expected] of [
    ["true", true],
    ["false", false],
    [undefined, false],
  ]) {
    assert.equal(
      expression(ratchet.env.BUNDLE_GROWTH_APPROVED, {
        steps: { "bundle-approval": { outputs: { approved } } },
      }),
      expected,
    );
  }
});
