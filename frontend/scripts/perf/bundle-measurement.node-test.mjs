import assert from "node:assert/strict";
import { execFileSync, spawnSync } from "node:child_process";
import {
  mkdirSync,
  mkdtempSync,
  readFileSync,
  rmSync,
  writeFileSync,
} from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

import {
  assertProducerRun,
  assertProducerSource,
  buildConfiguration,
  identityForRevision,
  makeIdentity,
  makeMeasurement,
  measurementKey,
  readBaseComparison,
  readMeasurement,
  validateMeasurement,
  writeMeasurement,
} from "./bundle-measurement.mjs";

const sourceTree = "a".repeat(40);
const revision = "b".repeat(40);
const identity = makeIdentity(sourceTree, {
  node: "24",
  platform: "linux",
  config: "fixture",
});
const observations = new Map([["/parents/children", 100_000]]);

function inDirectory(run) {
  const directory = mkdtempSync(join(tmpdir(), "moto-bundle-measurement-"));
  try {
    run(directory);
  } finally {
    rmSync(directory, { recursive: true, force: true });
  }
}

test("source/configuration identity, not commit identity, selects a reusable observation", () => {
  const measurement = makeMeasurement(identity, revision, observations);
  assert.deepEqual(validateMeasurement(measurement, identity), observations);
  const anotherCommit = makeMeasurement(identity, "c".repeat(40), observations);
  assert.equal(
    measurementKey(measurement.identity),
    measurementKey(anotherCommit.identity),
  );
});

test("different real commits with identical frontend trees reuse one key", () =>
  inDirectory((directory) => {
    const git = (...args) =>
      execFileSync(
        "git",
        [
          "-c",
          "core.hooksPath=/dev/null",
          "-c",
          "commit.gpgsign=false",
          ...args,
        ],
        {
          cwd: directory,
          encoding: "utf8",
          stdio: ["ignore", "pipe", "pipe"],
        },
      ).trim();
    git("init");
    git("config", "user.name", "Bundle fixture");
    git("config", "user.email", "bundle@example.org");
    mkdirSync(join(directory, "frontend"));
    writeFileSync(
      join(directory, "frontend", "source.js"),
      "export const value = 1;\n",
    );
    git("add", ".");
    git("commit", "-m", "initial frontend");
    const first = identityForRevision("HEAD", directory, { recipe: "same" });
    writeFileSync(join(directory, "backend-note.txt"), "unrelated\n");
    git("add", ".");
    git("commit", "-m", "unrelated backend change");
    const second = identityForRevision("HEAD", directory, { recipe: "same" });
    assert.notEqual(first.sourceRevision, second.sourceRevision);
    assert.equal(
      measurementKey(first.identity),
      measurementKey(second.identity),
    );
    writeFileSync(
      join(directory, "frontend", "source.js"),
      "changed before commit",
    );
    assert.throws(
      () => identityForRevision("HEAD", directory, { recipe: "same" }),
      /must be committed/,
    );
    const frontend = join(directory, "frontend");
    assert.throws(
      () => identityForRevision("HEAD", frontend, { recipe: "same" }),
      /must be committed/,
    );
    git("add", ".");
    assert.throws(
      () => identityForRevision("HEAD", frontend, { recipe: "same" }),
      /must be committed/,
    );
    git("commit", "-m", "committed source change");
    writeFileSync(join(frontend, "new-source.js"), "new untracked module");
    assert.throws(
      () => identityForRevision("HEAD", frontend, { recipe: "same" }),
      /must be committed/,
    );
  }));

test("build input/configuration changes invalidate an observation without exposing values", () =>
  inDirectory((directory) => {
    const before = makeIdentity(
      sourceTree,
      buildConfiguration(directory, { NEXT_PUBLIC_APP_ENV: "test" }),
    );
    const differentEnv = makeIdentity(
      sourceTree,
      buildConfiguration(directory, { NEXT_PUBLIC_APP_ENV: "demo" }),
    );
    assert.notEqual(measurementKey(before), measurementKey(differentEnv));
    writeFileSync(
      join(directory, ".env.local"),
      "PRIVATE_TEST_VALUE=fixture\n",
    );
    const differentFile = makeIdentity(
      sourceTree,
      buildConfiguration(directory, { NEXT_PUBLIC_APP_ENV: "test" }),
    );
    assert.notEqual(measurementKey(before), measurementKey(differentFile));
    assert.deepEqual(Object.keys(differentFile).sort(), [
      "configurationHash",
      "project",
      "sourceTree",
    ]);
  }));

test("foreign project, stale source/configuration and invalid bytes are rejected", () => {
  const measurement = makeMeasurement(identity, revision, observations);
  assert.throws(
    () =>
      validateMeasurement(
        { ...measurement, identity: { ...identity, project: "foreign" } },
        identity,
      ),
    /foreign/,
  );
  assert.throws(
    () => validateMeasurement(measurement, makeIdentity("d".repeat(40), {})),
    /Stale/,
  );
  assert.throws(
    () =>
      validateMeasurement(
        measurement,
        makeIdentity(sourceTree, { changed: true }),
      ),
    /Stale/,
  );
  assert.throws(
    () =>
      validateMeasurement(
        { ...measurement, routes: { "/parents/children": -1 } },
        identity,
      ),
    /Invalid bundle observation/,
  );
});

test("a requested but missing base comparison fails closed instead of skipping PR delta", () =>
  inDirectory((directory) => {
    writeMeasurement(
      join(directory, "perf-results/bundle-base-identity.json"),
      { identity, sourceRevision: revision },
    );
    assert.throws(
      () => readBaseComparison(directory, identity),
      /Required merge-base measurement missing/,
    );
  }));

test("an unreadable/corrupt cached observation is an error, not a cache miss", () =>
  inDirectory((directory) => {
    assert.throws(() =>
      readMeasurement(join(directory, "missing.json"), identity),
    );
    writeMeasurement(
      join(directory, "perf-results/bundle-base-identity.json"),
      { identity, sourceRevision: revision },
    );
    writeFileSync(
      join(directory, "perf-results/bundle-base.json"),
      "broken json",
    );
    assert.throws(() => readBaseComparison(directory, identity));
  }));

test("a prepared request from an old build configuration is rejected", () =>
  inDirectory((directory) => {
    writeMeasurement(
      join(directory, "perf-results/bundle-base-identity.json"),
      { identity, sourceRevision: revision },
    );
    assert.throws(
      () =>
        readBaseComparison(
          directory,
          makeIdentity(sourceTree, { other: true }),
        ),
      /stale build configuration/,
    );
  }));

test("valid cached observations preserve route bytes and deterministic route order", () =>
  inDirectory((directory) => {
    const measurement = makeMeasurement(
      identity,
      revision,
      new Map([
        ["/parents/z", 10],
        ["/parents/a", 20],
      ]),
    );
    const file = join(directory, "measured.json");
    writeMeasurement(file, measurement);
    assert.deepEqual(
      [...readMeasurement(file, identity)],
      [
        ["/parents/a", 20],
        ["/parents/z", 10],
      ],
    );
  }));

const repository = "moto-nrw/project-phoenix";
const run = {
  id: 42,
  repository: { full_name: repository },
  head_repository: { full_name: repository },
  path: ".github/workflows/main.yml",
  head_sha: revision,
};
const measurement = makeMeasurement(
  identity,
  "c".repeat(40),
  observations,
  "42",
);
const commit = {
  sha: measurement.sourceRevision,
  parents: [{ sha: revision }],
  tree: { sha: "d".repeat(40) },
};
const tree = {
  sha: commit.tree.sha,
  tree: [{ path: "frontend", type: "tree", sha: sourceTree }],
};

test("a synthetic merge producer is accepted by parent relation and actual frontend tree", () => {
  assert.doesNotThrow(() =>
    assertProducerSource(repository, run, measurement, commit, tree),
  );
});

test("foreign repositories/workflows and mismatched producer revisions/trees fail closed", () => {
  assert.throws(
    () =>
      assertProducerRun(repository, {
        ...run,
        head_repository: { full_name: "attacker/fork" },
      }),
    /Foreign/,
  );
  assert.throws(
    () =>
      assertProducerRun(repository, {
        ...run,
        path: ".github/workflows/unrelated.yml",
      }),
    /Foreign/,
  );
  assert.throws(
    () =>
      assertProducerSource(
        repository,
        run,
        { ...measurement, runId: "43" },
        commit,
        tree,
      ),
    /run identity/,
  );
  assert.throws(
    () =>
      assertProducerSource(
        repository,
        run,
        measurement,
        { ...commit, parents: [] },
        tree,
      ),
    /revision/,
  );
  assert.throws(
    () =>
      assertProducerSource(repository, run, measurement, commit, {
        ...tree,
        tree: [],
      }),
    /source tree/,
  );
});

function fetchFixture(directory, { onlyFork = false, corrupt = false } = {}) {
  const name = `bundle-measured-${measurementKey(identity)}`;
  const measured = makeMeasurement(
    identity,
    measurement.sourceRevision,
    observations,
    "43",
  );
  if (corrupt) measured.routes["/parents/children"] = -1;
  const metadata = {
    artifacts: [
      { name, expired: false, workflow_run: { id: 42 } },
      ...(!onlyFork
        ? [{ name, expired: false, workflow_run: { id: 43 } }]
        : []),
    ],
    runs: {
      42: {
        ...run,
        status: "completed",
        conclusion: "success",
        head_repository: { full_name: "untrusted/fork" },
      },
      43: { ...run, id: 43, status: "completed", conclusion: "success" },
    },
    measurement: measured,
    commit,
    tree,
  };
  const bin = join(directory, "bin");
  mkdirSync(bin);
  const data = join(directory, "api.json");
  const calls = join(directory, "calls.log");
  writeFileSync(data, JSON.stringify(metadata));
  writeMeasurement(
    join(directory, "perf-results", "bundle-base-identity.json"),
    { identity, sourceRevision: revision },
  );
  writeFileSync(
    join(bin, "gh"),
    `#!/usr/bin/env node
const fs = require("node:fs");
const path = require("node:path");
const data = JSON.parse(fs.readFileSync(process.env.FAKE_API_DATA, "utf8"));
const args = process.argv.slice(2);
fs.appendFileSync(process.env.FAKE_CALLS, JSON.stringify(args)+"\\n");
if (args[0] === "api") {
 const endpoint = args[1];
 let value;
 if (endpoint.includes("/actions/artifacts?")) value = { artifacts: data.artifacts };
 else if (endpoint.includes("/actions/runs/")) value = data.runs[endpoint.split("/").pop()];
 else if (endpoint.includes("/git/commits/")) value = data.commit;
 else if (endpoint.includes("/git/trees/")) value = data.tree;
 else throw new Error("unexpected endpoint "+endpoint);
 process.stdout.write(JSON.stringify(value));
} else {
 if (args[0] !== "run" || args[1] !== "download" || args[2] !== "43") throw new Error("ineligible download");
 const target = args[args.indexOf("--dir")+1];
 fs.writeFileSync(path.join(target, "bundle-measured.json"), JSON.stringify(data.measurement));
}
`,
    { mode: 0o755 },
  );
  const result = spawnSync(
    process.execPath,
    [
      fileURLToPath(new URL("./bundle-measurement.mjs", import.meta.url)),
      "--fetch-base",
    ],
    {
      cwd: directory,
      encoding: "utf8",
      env: {
        ...process.env,
        PATH: `${bin}:${process.env.PATH}`,
        GITHUB_REPOSITORY: repository,
        FAKE_API_DATA: data,
        FAKE_CALLS: calls,
      },
    },
  );
  return {
    result,
    calls: readFileSync(calls, "utf8")
      .trim()
      .split("\n")
      .map((line) => JSON.parse(line)),
  };
}

test("base lookup skips a matching fork observation and reuses the next eligible run", () =>
  inDirectory((directory) => {
    const { result, calls } = fetchFixture(directory);
    assert.equal(result.status, 0, result.stdout + result.stderr);
    assert.match(result.stdout, /Reused bundle observation from run 43/);
    assert.deepEqual(
      calls.filter((args) => args[0] === "run").map((args) => args[2]),
      ["43"],
    );
    assert.deepEqual(
      readMeasurement(
        join(directory, "perf-results", "bundle-base.json"),
        identity,
      ),
      observations,
    );
  }));

test("only ineligible observations are an explicit miss, while corrupt eligible data still fails", () => {
  inDirectory((directory) => {
    const { result, calls } = fetchFixture(directory, { onlyFork: true });
    assert.equal(result.status, 0, result.stdout + result.stderr);
    assert.match(result.stdout, /No compatible successful base observation/);
    assert.equal(
      calls.some((args) => args[0] === "run"),
      false,
    );
    assert.throws(
      () => readBaseComparison(directory, identity),
      /Required merge-base measurement missing/,
    );
  });
  inDirectory((directory) => {
    const { result } = fetchFixture(directory, { corrupt: true });
    assert.notEqual(result.status, 0);
    assert.match(result.stderr, /Invalid bundle observation/);
  });
});
