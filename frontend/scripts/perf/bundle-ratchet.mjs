// Bundle-Size-Ratchet (#2939): Client-JavaScript je Route (komprimiert, laut
// Turbopack) gegen die eingecheckte Baseline `scripts/perf/bundle-baseline.json`.
//
//   pnpm run perf:bundle-ratchet             analysieren + prüfen (CI, Job frontend-build)
//   pnpm run perf:bundle-ratchet --update    analysieren + Baseline auf den Ist-Stand setzen
//   ... --skip-analyze                       vorhandene .next/diagnostics/analyze wiederverwenden
//
// Shrink-only nach dem Muster der bestehenden Ratchets: eine Route über
// Baseline + Toleranz schlägt fehl, eine Route unter Baseline - Toleranz auch
// (dann Baseline mit --update nachziehen, damit der Gewinn gesichert bleibt).
// Toleranz je Route: 2 % oder 10 kB, was größer ist, damit ein Patch-Bump einer
// Dependency keine Baseline-Pflege erzwingt. Neue Routen ohne Eintrag und
// Einträge ohne Route sind Fehler. Die Analyse kompiliert immer kalt (siehe
// analyze(): `.next` wird geleert, der Build-Cache beiseitegelegt); zwei kalte
// Läufe derselben Quelle liefern byte-identische Zahlen, deshalb ist die
// Prüfung hart. Lokal ist der eigene `.next`-Build danach weg, der Cache nicht.
//
// Die Baseline ist mit dem CI-Env erzeugt (nur NEXT_PUBLIC_API_URL und
// SKIP_ENV_VALIDATION). Lokal liegen die Routen rund 1 kB darüber, weil Next
// die NEXT_PUBLIC_*-Werte aus .env.local (PostHog, Sentry) in die Chunks
// schreibt; das bleibt innerhalb der Toleranz. Für ein sauberes --update:
//   env -i PATH="$PATH" HOME="$HOME" NEXT_PUBLIC_API_URL=http://localhost:8080 \
//     SKIP_ENV_VALIDATION=true pnpm run perf:bundle-ratchet --update
import { spawnSync } from "node:child_process";
import {
  appendFileSync,
  existsSync,
  readFileSync,
  renameSync,
  rmSync,
  writeFileSync,
} from "node:fs";
import { join } from "node:path";

import { readRouteBundles } from "./bundle-routes.mjs";

const BASELINE_FILE = join(process.cwd(), "scripts/perf/bundle-baseline.json");
const TOLERANCE_RATIO = 0.02;
const TOLERANCE_BYTES = 10 * 1024;
const UPDATE_HINT =
  "Baseline nachziehen: `cd frontend && pnpm run perf:bundle-ratchet --update` und scripts/perf/bundle-baseline.json mit einchecken.";

const args = new Set(process.argv.slice(2));
const update = args.has("--update");
const skipAnalyze = args.has("--skip-analyze");

function kb(bytes) {
  return `${(bytes / 1024).toFixed(1)} kB`;
}

function tolerance(baseline) {
  return Math.max(TOLERANCE_BYTES, Math.round(baseline * TOLERANCE_RATIO));
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
      { stdio: "inherit" },
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

/** @returns {Record<string, number>} Route -> komprimierte Bytes */
function readBaseline() {
  const parsed = JSON.parse(readFileSync(BASELINE_FILE, "utf8"));
  if (!parsed || typeof parsed.routes !== "object") {
    throw new Error(`${BASELINE_FILE}: Feld "routes" fehlt.`);
  }
  return parsed.routes;
}

function writeBaseline(measured) {
  const routes = Object.fromEntries(
    [...measured.entries()].sort(([a], [b]) => a.localeCompare(b)),
  );
  const content = {
    _doc: "Client-JS je Route, komprimierte Bytes laut `next experimental-analyze`. Pflege: pnpm run perf:bundle-ratchet --update (frontend/scripts/perf/bundle-ratchet.mjs, #2939).",
    routes,
  };
  writeFileSync(BASELINE_FILE, `${JSON.stringify(content, null, 2)}\n`);
}

function compare(baseline, measured) {
  const failures = [];
  const rows = [];
  for (const [route, bytes] of [...measured.entries()].sort()) {
    const expected = baseline[route];
    if (expected === undefined) {
      failures.push(
        `Neue Route ohne Baseline-Eintrag: ${route} (${kb(bytes)}). ${UPDATE_HINT}`,
      );
      continue;
    }
    const delta = bytes - expected;
    const limit = tolerance(expected);
    rows.push({ route, expected, bytes, delta });
    if (delta > limit) {
      failures.push(
        `${route}: ${kb(bytes)} liegt ${kb(delta)} über der Baseline ${kb(expected)} (Toleranz ${kb(limit)}). Bundle verkleinern (next/dynamic, Import prüfen) oder die Erhöhung im PR begründen und die Baseline mit --update setzen.`,
      );
    } else if (delta < -limit) {
      failures.push(
        `${route}: ${kb(bytes)} liegt ${kb(-delta)} unter der Baseline ${kb(expected)}. ${UPDATE_HINT}`,
      );
    }
  }
  for (const route of Object.keys(baseline)) {
    if (!measured.has(route)) {
      failures.push(
        `Baseline-Eintrag ohne Route: ${route}. Eintrag entfernen (${UPDATE_HINT})`,
      );
    }
  }
  return { failures, rows };
}

function summary(rows, failures) {
  const lines = ["## Bundle-Ratchet", ""];
  lines.push(
    failures.length === 0
      ? `Alle ${rows.length} Routen innerhalb der Toleranz (2 % oder 10 kB).`
      : `${failures.length} Verstöße:`,
  );
  lines.push("");
  for (const failure of failures) lines.push(`- ${failure}`);
  const moved = rows
    .filter((r) => r.delta !== 0)
    .sort((a, b) => Math.abs(b.delta) - Math.abs(a.delta))
    .slice(0, 10);
  if (moved.length > 0) {
    lines.push("", "| Route | Baseline | Ist | Delta |", "|---|---|---|---|");
    for (const r of moved) {
      lines.push(
        `| \`${r.route}\` | ${kb(r.expected)} | ${kb(r.bytes)} | ${r.delta > 0 ? "+" : ""}${kb(r.delta)} |`,
      );
    }
  }
  return `${lines.join("\n")}\n`;
}

if (!skipAnalyze) analyze();
const measured = new Map(
  readRouteBundles().map((r) => [r.route, r.compressed]),
);
if (measured.size === 0) {
  throw new Error("Keine Routen in .next/diagnostics/analyze/data gefunden.");
}

if (update) {
  writeBaseline(measured);
  process.stdout.write(
    `Baseline geschrieben: ${measured.size} Routen -> ${BASELINE_FILE}\n`,
  );
  process.exit(0);
}

const { failures, rows } = compare(readBaseline(), measured);
const text = summary(rows, failures);
process.stdout.write(text);
if (process.env.GITHUB_STEP_SUMMARY) {
  appendFileSync(process.env.GITHUB_STEP_SUMMARY, text);
}
process.exit(failures.length === 0 ? 0 : 1);
