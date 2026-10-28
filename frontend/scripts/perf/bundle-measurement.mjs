// Content-addressed observations, never approved budget policy.
import { execFileSync } from "node:child_process";
import { createHash } from "node:crypto";
import {
  appendFileSync,
  existsSync,
  mkdirSync,
  mkdtempSync,
  readFileSync,
  readdirSync,
  rmSync,
  writeFileSync,
} from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

import { perfServerEnv } from "./env.mjs";
import { validBytes } from "./bundle-policy.mjs";

const MEASUREMENT_FORMAT = "moto-bundle-measurement/v1";
const PROJECT = "moto-nrw/project-phoenix/frontend";
const SHA = /^(?:[a-f0-9]{40}|[a-f0-9]{64})$/;
const CONFIG_SHA = /^[a-f0-9]{64}$/;
const RESULTS = "perf-results";
const BASE_IDENTITY = `${RESULTS}/bundle-base-identity.json`;
const BASE_MEASUREMENT = `${RESULTS}/bundle-base.json`;

function hash(value) {
  return createHash("sha256").update(value).digest("hex");
}

export function measurementKey(identity) {
  assertIdentity(identity);
  return `bundle-v1-${identity.sourceTree}-${identity.configurationHash}`;
}

function assertIdentity(identity) {
  if (
    identity?.project !== PROJECT ||
    !SHA.test(identity.sourceTree) ||
    !CONFIG_SHA.test(identity.configurationHash)
  ) {
    throw new Error("Invalid or foreign bundle measurement identity");
  }
}

export function makeIdentity(sourceTree, configuration) {
  const identity = {
    project: PROJECT,
    sourceTree,
    configurationHash: hash(JSON.stringify(configuration)),
  };
  assertIdentity(identity);
  return identity;
}

function git(args, cwd) {
  return execFileSync("git", args, { cwd, encoding: "utf8" }).trim();
}

/** Build values are hashed only; none are persisted in observation metadata. */
export function buildConfiguration(directory = process.cwd(), environment) {
  const env = environment ?? {
    ...process.env,
    ...perfServerEnv(),
    SKIP_ENV_VALIDATION: "true",
  };
  const buildEnv = Object.fromEntries(
    Object.keys(env)
      .filter(
        (key) =>
          key.startsWith("NEXT_PUBLIC_") ||
          key.startsWith("SENTRY_") ||
          [
            "NODE_ENV",
            "NODE_OPTIONS",
            "CI",
            "TZ",
            "API_URL",
            "TENANT_DOMAIN",
            "NEXTAUTH_URL",
            "NEXTAUTH_SECRET",
            "AUTH_JWT_EXPIRY",
            "AUTH_JWT_REFRESH_EXPIRY",
            "METRICS_BEARER_TOKEN",
            "SKIP_ENV_VALIDATION",
          ].includes(key),
      )
      .sort()
      .map((key) => [key, env[key]]),
  );
  const files = [
    ".env",
    ".env.local",
    ".env.production",
    ".env.production.local",
    "scripts/perf/bundle-routes.mjs",
    "scripts/perf/bundle-measurement.mjs",
    "scripts/perf/env.mjs",
  ];
  const configuration = {
    recipe: ["next", "experimental-analyze", "--output", "cold"],
    node: process.versions.node,
    platform: process.platform,
    architecture: process.arch,
    buildEnvironment: buildEnv,
    files: Object.fromEntries(
      files.map((file) => [
        file,
        existsSync(join(directory, file))
          ? hash(readFileSync(join(directory, file)))
          : null,
      ]),
    ),
  };
  return configuration;
}

/** Exact source tree survives a PR synthetic merge becoming a real merge. */
export function identityForRevision(
  revision = "HEAD",
  directory = process.cwd(),
  configuration = buildConfiguration(directory),
) {
  const sourceRevision = git(["rev-parse", "--verify", revision], directory);
  const sourceTree = git(
    ["rev-parse", `${sourceRevision}:frontend`],
    directory,
  );
  if (
    sourceRevision === git(["rev-parse", "HEAD"], directory) &&
    git(
      [
        "status",
        "--porcelain",
        "--untracked-files=normal",
        "--",
        ":(top)frontend",
      ],
      directory,
    )
  ) {
    throw new Error(
      "Frontend sources must be committed before producing source-addressed bundle observations; uncommitted files do not match the HEAD tree.",
    );
  }
  return { identity: makeIdentity(sourceTree, configuration), sourceRevision };
}

export function makeMeasurement(
  identity,
  sourceRevision,
  routes,
  runId = null,
) {
  assertIdentity(identity);
  if (!SHA.test(sourceRevision))
    throw new Error("Invalid bundle producer revision");
  const measurement = {
    format: MEASUREMENT_FORMAT,
    identity,
    sourceRevision,
    runId,
    measuredAt: new Date().toISOString(),
    routes: Object.fromEntries(
      [...routes.entries()].sort(([a], [b]) => (a < b ? -1 : a > b ? 1 : 0)),
    ),
  };
  validateMeasurement(measurement, identity);
  return measurement;
}

export function validateMeasurement(measurement, expectedIdentity) {
  assertIdentity(expectedIdentity);
  assertIdentity(measurement?.identity);
  if (
    measurement?.format !== MEASUREMENT_FORMAT ||
    !SHA.test(measurement.sourceRevision) ||
    measurementKey(measurement.identity) !== measurementKey(expectedIdentity) ||
    !measurement.routes ||
    Array.isArray(measurement.routes) ||
    typeof measurement.routes !== "object" ||
    Object.keys(measurement.routes).length === 0
  ) {
    throw new Error("Stale, foreign or invalid bundle measurement");
  }
  const routes = new Map();
  for (const [route, bytes] of Object.entries(measurement.routes)) {
    if (!route.startsWith("/") || !validBytes(bytes))
      throw new Error(`Invalid bundle observation for ${route}`);
    routes.set(route, bytes);
  }
  return routes;
}

export function readMeasurement(file, expectedIdentity) {
  // A corrupt/unreadable file is not a cache miss and must never look green.
  return validateMeasurement(
    JSON.parse(readFileSync(file, "utf8")),
    expectedIdentity,
  );
}

export function writeMeasurement(file, measurement) {
  mkdirSync(dirname(file), { recursive: true });
  writeFileSync(file, `${JSON.stringify(measurement, null, 2)}\n`);
}

export function readBaseComparison(directory = process.cwd(), currentIdentity) {
  const identityFile = join(directory, BASE_IDENTITY);
  if (!existsSync(identityFile))
    return {
      routes: null,
      status:
        "Nicht angefordert; genehmigte absolute Budgets werden vollständig geprüft.",
    };
  const { identity } = JSON.parse(readFileSync(identityFile, "utf8"));
  assertIdentity(identity);
  if (
    currentIdentity &&
    identity.configurationHash !== currentIdentity.configurationHash
  ) {
    throw new Error(
      "Base measurement request uses a stale build configuration",
    );
  }
  const file = join(directory, BASE_MEASUREMENT);
  if (!existsSync(file))
    return {
      routes: null,
      status:
        "Keine passende Basismessung vorhanden; Basis-Regressionsvergleich nicht durchgeführt. Genehmigte absolute Budgets werden vollständig geprüft.",
    };
  return {
    routes: readMeasurement(file, identity),
    status: `Basismessung für Quellbaum ${identity.sourceTree} wiederverwendet; Delta ist informativ, genehmigte absolute Budgets sind verbindlich.`,
  };
}

function prepareCI(baseRevision) {
  mkdirSync(RESULTS, { recursive: true });
  rmSync(BASE_IDENTITY, { force: true });
  rmSync(BASE_MEASUREMENT, { force: true });
  const current = identityForRevision();
  const key = measurementKey(current.identity);
  if (process.env.GITHUB_OUTPUT)
    appendFileSync(process.env.GITHUB_OUTPUT, `measurement-key=${key}\n`);
  process.stdout.write(`Bundle measurement key: ${key}\n`);
  if (baseRevision)
    writeMeasurement(BASE_IDENTITY, identityForRevision(baseRevision));
}

function github(endpoint) {
  return JSON.parse(
    execFileSync("gh", ["api", endpoint], { encoding: "utf8" }),
  );
}

function eligibleProducerRun(repository, run) {
  return (
    run?.repository?.full_name === repository &&
    run?.head_repository?.full_name === repository &&
    [".github/workflows/main.yml", ".github/workflows/nightly.yml"].includes(
      run.path?.split("@")[0],
    )
  );
}

export function assertProducerRun(repository, run) {
  if (!eligibleProducerRun(repository, run)) {
    throw new Error("Foreign bundle measurement producer repository/workflow");
  }
}

export function assertProducerSource(
  repository,
  run,
  measurement,
  commit,
  tree,
) {
  assertProducerRun(repository, run);
  if (measurement.runId !== String(run.id))
    throw new Error("Bundle measurement run identity mismatch");
  if (
    commit?.sha !== measurement.sourceRevision ||
    (measurement.sourceRevision !== run.head_sha &&
      !commit.parents?.some((parent) => parent.sha === run.head_sha))
  ) {
    throw new Error(
      "Bundle producer revision does not belong to its workflow run",
    );
  }
  if (
    tree?.sha !== commit.tree?.sha ||
    !tree.tree?.some(
      (entry) =>
        entry.path === "frontend" &&
        entry.type === "tree" &&
        entry.sha === measurement.identity.sourceTree,
    )
  ) {
    throw new Error(
      "Bundle producer source tree does not match the requested base",
    );
  }
}

/** Cross-run artifacts are accessible across PR refs, unlike PR-scoped caches. */
function fetchBase() {
  if (!existsSync(BASE_IDENTITY)) {
    process.stdout.write(
      "No base revision; absolute bundle budgets remain mandatory.\n",
    );
    return;
  }
  const expected = JSON.parse(readFileSync(BASE_IDENTITY, "utf8")).identity;
  const name = `bundle-measured-${measurementKey(expected)}`;
  const repository = process.env.GITHUB_REPOSITORY;
  if (!/^[\w.-]+\/[\w.-]+$/.test(repository ?? ""))
    throw new Error("GITHUB_REPOSITORY is required for bundle artifact reuse");
  const listed = github(
    `repos/${repository}/actions/artifacts?name=${encodeURIComponent(name)}&per_page=100`,
  );
  if (!Array.isArray(listed.artifacts))
    throw new Error("Invalid GitHub bundle artifact listing");
  for (const artifact of listed.artifacts.filter(
    (entry) => entry.name === name && !entry.expired,
  )) {
    const run = github(
      `repos/${repository}/actions/runs/${artifact.workflow_run.id}`,
    );
    if (!eligibleProducerRun(repository, run)) continue;
    if (run.status !== "completed" || run.conclusion !== "success") continue;
    const temporary = mkdtempSync(join(tmpdir(), "moto-bundle-measurement-"));
    try {
      execFileSync(
        "gh",
        [
          "run",
          "download",
          String(run.id),
          "--repo",
          repository,
          "--name",
          name,
          "--dir",
          temporary,
        ],
        { stdio: "pipe" },
      );
      if (readdirSync(temporary).join(",") !== "bundle-measured.json")
        throw new Error("Unexpected files in bundle measurement artifact");
      const measurement = JSON.parse(
        readFileSync(join(temporary, "bundle-measured.json"), "utf8"),
      );
      validateMeasurement(measurement, expected);
      const commit = github(
        `repos/${repository}/git/commits/${measurement.sourceRevision}`,
      );
      const tree = github(`repos/${repository}/git/trees/${commit.tree.sha}`);
      assertProducerSource(repository, run, measurement, commit, tree);
      writeMeasurement(BASE_MEASUREMENT, measurement);
      process.stdout.write(
        `Reused bundle observation from run ${run.id}; no base build.\n`,
      );
      return;
    } finally {
      rmSync(temporary, { recursive: true, force: true });
    }
  }
  process.stdout.write(
    "No compatible successful base observation found; no base build was started. Absolute budgets remain mandatory.\n",
  );
}

if (
  process.argv[1] &&
  resolve(process.argv[1]) === fileURLToPath(import.meta.url)
) {
  if (process.argv[2] === "--prepare-ci") prepareCI(process.argv[3]);
  else if (process.argv[2] === "--fetch-base") fetchBase();
  else throw new Error("Expected --prepare-ci [base-revision] or --fetch-base");
}
