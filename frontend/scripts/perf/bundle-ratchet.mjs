// Approved bundle budgets are reviewed policy, not the latest observation.
// `perf:bundle-ratchet` compiles cold, writes a versioned measurement artifact,
// and enforces fixed per-route ceilings from bundle-budgets/<portal>/<resource>.json.
// Improvements/retired observations pass; new routes need an explicit budget.
// --update is deliberately unsupported: observations cannot approve budgets.
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
import { compareBudgets, readBudgets } from "./bundle-policy.mjs";
import {
  identityForRevision,
  makeMeasurement,
  measurementKey,
  readBaseComparison,
  writeMeasurement,
} from "./bundle-measurement.mjs";

const BUDGET_DIR = join(process.cwd(), "scripts/perf/bundle-budgets");
const MEASURED_FILE = join(process.cwd(), "perf-results/bundle-measured.json");
const ANALYSIS_IDENTITY = join(
  process.cwd(),
  ".next/diagnostics/analyze/bundle-source-identity.json",
);
const args = new Set(process.argv.slice(2));
if (args.has("--update")) {
  throw new Error(
    "--update cannot approve bundle budgets. Review an explicit bundle-budgets/<portal>/<resource>.json policy change instead.",
  );
}
for (const argument of args) {
  if (argument !== "--skip-analyze")
    throw new Error(`Unknown bundle budget option: ${argument}`);
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

function summary(rows, failures, warnings, baseStatus) {
  const lines = ["## Bundle-Budgets", "", baseStatus, ""];
  lines.push(
    failures.length === 0
      ? `Alle ${rows.length} gemessenen Routen ohne neue Delta- oder Budget-Verstöße.`
      : `${failures.length} Delta-/Budget-Verstöße:`,
  );
  for (const failure of failures) lines.push(`- ${failure}`);
  if (warnings.length > 0) {
    lines.push(
      "",
      "### Bereits auf der Merge-Basis überschrittene absolute Budgets (Warnung)",
    );
    for (const warning of warnings) lines.push(`- ${warning}`);
    for (const warning of warnings)
      process.stdout.write(`::warning::${warning}\n`);
  }
  const moved = rows
    .map((row) => ({
      ...row,
      change: row.bytes - (row.baseBytes ?? row.referenceBytes),
    }))
    .filter((row) => Math.abs(row.change) >= 512)
    .sort((a, b) => Math.abs(b.change) - Math.abs(a.change))
    .slice(0, 10);
  if (moved.length > 0) {
    lines.push(
      "",
      "| Route | Genehmigte Referenz | Obergrenze | Ist | Delta zur Basis oder Referenz |",
      "|---|---|---|---|---|",
    );
    for (const row of moved)
      lines.push(
        `| \`${row.route}\` | ${kb(row.referenceBytes)} | ${kb(row.maxBytes)} | ${kb(row.bytes)} | ${row.change > 0 ? "+" : ""}${kb(row.change)} |`,
      );
  }
  return `${lines.join("\n")}\n`;
}

const budgets = readBudgets(BUDGET_DIR);
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
const { failures, warnings, rows } = compareBudgets(
  budgets,
  measured,
  base.routes,
);
const text = summary(rows, failures, warnings, base.status);
process.stdout.write(text);
if (process.env.GITHUB_STEP_SUMMARY)
  appendFileSync(process.env.GITHUB_STEP_SUMMARY, text);
process.exit(failures.length === 0 ? 0 : 1);
