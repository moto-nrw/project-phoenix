// `perf:bundle-ratchet` compiles cold, writes a versioned measurement artifact,
// checks each route's growth against the merge-base measurement (feature PRs)
// and the few absolute ceilings in bundle-ceilings.json (every run).
// BUNDLE_GROWTH_APPROVED=true (set by CI from the PR label) turns delta
// failures into warnings.
import { spawnSync } from "node:child_process";
import {
  appendFileSync,
  existsSync,
  readFileSync,
  renameSync,
  rmSync,
} from "node:fs";
import { join } from "node:path";

import { readRouteBundles } from "./bundle-routes.mjs";
import { perfServerEnv } from "./env.mjs";
import {
  compareBundles,
  GROWTH_APPROVAL_LABEL,
  readCeilings,
} from "./bundle-policy.mjs";
import {
  identityForRevision,
  makeMeasurement,
  measurementKey,
  readBaseComparison,
  writeMeasurement,
} from "./bundle-measurement.mjs";

const CEILINGS_FILE = join(process.cwd(), "scripts/perf/bundle-ceilings.json");
const MEASURED_FILE = join(process.cwd(), "perf-results/bundle-measured.json");
const ANALYSIS_IDENTITY = join(
  process.cwd(),
  ".next/diagnostics/analyze/bundle-source-identity.json",
);
const args = new Set(process.argv.slice(2));
for (const argument of args) {
  if (argument !== "--skip-analyze")
    throw new Error(`Unknown bundle ratchet option: ${argument}`);
}
const skipAnalyze = args.has("--skip-analyze");

function kb(bytes) {
  return `${(bytes / 1024).toFixed(1)} kB`;
}

const NEXT_DIR = join(process.cwd(), ".next");
const NEXT_CACHE_DIR = join(NEXT_DIR, "cache");
const NEXT_CACHE_PARKED = join(process.cwd(), ".next-cache-parked");

/**
 * Immer kalt kompilieren: mit dem inkrementellen Turbopack-Cache hängt die
 * Chunk-Aufteilung vom Cache-Zustand ab (auf CI +48 kB auf einer Route
 * zwischen zwei Läufen derselben Quelle, #2939). Zwei kalte Läufe sind
 * byte-identisch. Der Build-Cache wird dafür beiseitegelegt und danach
 * wiederhergestellt, damit der nächste `next build` warm bleibt.
 */
function analyze() {
  rmSync(NEXT_CACHE_PARKED, { recursive: true, force: true });
  if (existsSync(NEXT_CACHE_DIR)) renameSync(NEXT_CACHE_DIR, NEXT_CACHE_PARKED);
  try {
    rmSync(NEXT_DIR, { recursive: true, force: true });
    const result = spawnSync(
      "pnpm",
      ["exec", "next", "experimental-analyze", "--output"],
      {
        stdio: "inherit",
        env: {
          ...process.env,
          ...perfServerEnv(),
          SKIP_ENV_VALIDATION: "true",
        },
      },
    );
    if (result.status !== 0) {
      throw new Error(
        `next experimental-analyze ist mit Status ${result.status} beendet.`,
      );
    }
  } finally {
    if (existsSync(NEXT_CACHE_PARKED)) {
      rmSync(NEXT_CACHE_DIR, { recursive: true, force: true });
      renameSync(NEXT_CACHE_PARKED, NEXT_CACHE_DIR);
    }
  }
}

function summary(rows, failures, warnings, baseStatus, growthApproved) {
  const lines = ["## Bundle-Größen", "", baseStatus, ""];
  if (growthApproved)
    lines.push(`Zuwachs freigegeben mit Label \`${GROWTH_APPROVAL_LABEL}\`.`, "");
  lines.push(
    failures.length === 0
      ? `Alle ${rows.length} gemessenen Routen ohne Verstöße.`
      : `${failures.length} Verstöße:`,
  );
  for (const failure of failures) lines.push(`- ${failure}`);
  if (warnings.length > 0) {
    lines.push("", "### Warnungen");
    for (const warning of warnings) lines.push(`- ${warning}`);
    for (const warning of warnings)
      process.stdout.write(`::warning::${warning}\n`);
  }
  const shown = rows.some((row) => row.delta !== null)
    ? rows
        .filter((row) => row.delta !== null && Math.abs(row.delta) >= 512)
        .sort((a, b) => Math.abs(b.delta) - Math.abs(a.delta))
        .slice(0, 15)
    : rows.filter((row) => row.maxBytes !== undefined);
  if (shown.length > 0) {
    lines.push(
      "",
      "| Route | Merge-Basis | Ist | Zuwachs | Obergrenze |",
      "|---|---|---|---|---|",
    );
    for (const row of shown)
      lines.push(
        `| \`${row.route}\` | ${row.baseBytes === undefined ? "–" : kb(row.baseBytes)} | ${kb(row.bytes)} | ${row.delta === null ? "–" : `${row.delta > 0 ? "+" : ""}${kb(row.delta)}`} | ${row.maxBytes === undefined ? "–" : kb(row.maxBytes)} |`,
      );
  }
  return `${lines.join("\n")}\n`;
}

const ceilings = readCeilings(CEILINGS_FILE);
const growthApproved = process.env.BUNDLE_GROWTH_APPROVED === "true";
const current = identityForRevision();
if (!skipAnalyze) {
  analyze();
  if (
    measurementKey(identityForRevision().identity) !==
    measurementKey(current.identity)
  ) {
    throw new Error(
      "Frontend source/configuration changed during bundle analysis; measurement was not published.",
    );
  }
  writeMeasurement(ANALYSIS_IDENTITY, {
    format: "moto-bundle-analysis/v1",
    ...current,
  });
} else {
  const previous = JSON.parse(readFileSync(ANALYSIS_IDENTITY, "utf8"));
  if (
    previous?.format !== "moto-bundle-analysis/v1" ||
    measurementKey(previous.identity) !== measurementKey(current.identity)
  ) {
    throw new Error(
      "Existing bundle analysis belongs to another source tree or build configuration; rerun without --skip-analyze.",
    );
  }
}
const measured = new Map(
  readRouteBundles().map((route) => [route.route, route.compressed]),
);
if (measured.size === 0)
  throw new Error("Keine Routen in .next/diagnostics/analyze/data gefunden.");
writeMeasurement(
  MEASURED_FILE,
  makeMeasurement(
    current.identity,
    current.sourceRevision,
    measured,
    process.env.GITHUB_RUN_ID ?? null,
  ),
);
const base = readBaseComparison(process.cwd(), current.identity);
const { failures, warnings, rows } = compareBundles(
  ceilings,
  measured,
  base.routes,
  { growthApproved },
);
const text = summary(rows, failures, warnings, base.status, growthApproved);
process.stdout.write(text);
if (process.env.GITHUB_STEP_SUMMARY)
  appendFileSync(process.env.GITHUB_STEP_SUMMARY, text);
process.exit(failures.length === 0 ? 0 : 1);
