#!/usr/bin/env node
// Wall-clock budgets for backend test packages in CI (#6784).
//
//   node scripts/backend-package-budgets.mjs check go-test.json
//
// Reads `gotestsum --jsonfile` (go test -json events), compares each
// package's elapsed seconds with its budget and fails when one is over. The
// budget file is backend-package-budgets.json next to this script: a default
// for every package and an entry with a reason for each larger one. Raise an
// entry only with the reason in the pull request; lower it when a package got
// cheaper.
import { readFileSync, appendFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const MODULE = "github.com/moto-nrw/project-phoenix/";

export function loadBudgets(path = join(dirname(fileURLToPath(import.meta.url)), "backend-package-budgets.json")) {
  const raw = JSON.parse(readFileSync(path, "utf8"));
  if (!Number.isFinite(raw.defaultSeconds) || raw.defaultSeconds <= 0) {
    throw new Error(`${path}: defaultSeconds must be a positive number`);
  }
  for (const [pkg, entry] of Object.entries(raw.packages ?? {})) {
    if (!Number.isFinite(entry.seconds) || entry.seconds <= raw.defaultSeconds) {
      throw new Error(`${path}: ${pkg} needs seconds above defaultSeconds (${raw.defaultSeconds})`);
    }
    if (typeof entry.reason !== "string" || entry.reason.trim() === "") {
      throw new Error(`${path}: ${pkg} needs a reason`);
    }
  }
  return { defaultSeconds: raw.defaultSeconds, packages: raw.packages ?? {} };
}

const relative = (importPath) => (importPath.startsWith(MODULE) ? importPath.slice(MODULE.length) : importPath);

export function budgetOf(budgets, importPath) {
  return budgets.packages[relative(importPath)]?.seconds ?? budgets.defaultSeconds;
}

// Elapsed seconds per package from go test -json: the package-level
// pass/fail/skip event (no Test field) carries the binary's run time.
export function packageTimes(jsonLines) {
  const times = new Map();
  for (const line of jsonLines.split("\n")) {
    if (!line.trim()) continue;
    let event;
    try {
      event = JSON.parse(line);
    } catch {
      continue;
    }
    if (event.Test || !event.Package || !["pass", "fail", "skip"].includes(event.Action)) continue;
    if (typeof event.Elapsed === "number") times.set(event.Package, event.Elapsed);
  }
  return times;
}

export function check(budgets, times) {
  const rows = [...times].map(([pkg, seconds]) => ({ pkg: relative(pkg), seconds, budget: budgetOf(budgets, pkg) }));
  rows.sort((a, b) => b.seconds - a.seconds);
  return { rows, over: rows.filter((r) => r.seconds > r.budget) };
}

function main() {
  const [command, file] = process.argv.slice(2);
  const budgets = loadBudgets();
  if (command !== "check" || !file) {
    console.error("usage: backend-package-budgets.mjs check go-test.json");
    process.exit(2);
  }
  const { rows, over } = check(budgets, packageTimes(readFileSync(file, "utf8")));
  const table = [
    "### Backend package time budgets",
    "",
    "| Package | Seconds | Budget |",
    "|---|---:|---:|",
    ...rows.slice(0, 10).map((r) => `| \`${r.pkg}\` | ${r.seconds.toFixed(1)} | ${r.budget} |`),
    "",
  ].join("\n");
  console.log(table);
  if (process.env.GITHUB_STEP_SUMMARY) appendFileSync(process.env.GITHUB_STEP_SUMMARY, table + "\n");
  for (const r of over) {
    console.log(
      `::error title=Backend package over its time budget::${r.pkg} ran ${r.seconds.toFixed(1)} s, budget ${r.budget} s ` +
        "(scripts/backend-package-budgets.json). Make its tests cheaper, or raise the budget with the measured reason in the pull request.",
    );
  }
  if (over.length > 0) process.exit(1);
}

if (process.argv[1] === fileURLToPath(import.meta.url)) main();
