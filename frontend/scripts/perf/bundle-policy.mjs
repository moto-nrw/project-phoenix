import { readFileSync, readdirSync } from "node:fs";
import { join } from "node:path";

export const BUDGET_FORMAT = "moto-bundle-budget/v1";
const TOLERANCE_RATIO = 0.02;
const TOLERANCE_BYTES = 10 * 1024;

export function legacyCeiling(referenceBytes) {
  return (
    referenceBytes +
    Math.max(TOLERANCE_BYTES, Math.round(referenceBytes * TOLERANCE_RATIO))
  );
}

function portalForRoute(route) {
  if (/^\/\[tenant\](\/|$)/.test(route)) return "tenant";
  const portal = /^\/(operator|parents|school|caterer|traeger)(\/|$)/.exec(
    route,
  )?.[1];
  return portal ?? "shared";
}

function budgetFileForRoute(route) {
  const portal = portalForRoute(route);
  const segments = route.split("/").filter(Boolean);
  if (portal !== "shared") segments.shift();
  const resource = (segments[0] ?? "index").replace(/[^a-zA-Z0-9_-]/g, "_");
  return `${portal}/${resource}.json`;
}

function budgetFiles(directory, prefix = "") {
  return readdirSync(join(directory, prefix), { withFileTypes: true })
    .flatMap((entry) => {
      const relative = prefix ? `${prefix}/${entry.name}` : entry.name;
      return entry.isDirectory()
        ? budgetFiles(directory, relative)
        : entry.isFile() && entry.name.endsWith(".json")
          ? [relative]
          : [];
    })
    .sort();
}

export function validBytes(value) {
  return Number.isSafeInteger(value) && value >= 0;
}

/** Policy is reviewed source. Measurement artifacts can never replace it. */
export function readBudgets(directory) {
  const budgets = new Map();
  const files = budgetFiles(directory);
  if (files.length === 0) throw new Error(`No bundle budgets in ${directory}`);
  for (const file of files) {
    const policy = JSON.parse(readFileSync(join(directory, file), "utf8"));
    if (
      policy?.format !== BUDGET_FORMAT ||
      !policy.routes ||
      Array.isArray(policy.routes)
    ) {
      throw new Error(`Invalid bundle budget policy: ${file}`);
    }
    for (const [route, budget] of Object.entries(policy.routes)) {
      if (!route.startsWith("/") || file !== budgetFileForRoute(route)) {
        throw new Error(
          `Bundle budget ${route} belongs in ${budgetFileForRoute(route)}, not ${file}`,
        );
      }
      if (
        !budget ||
        !validBytes(budget.referenceBytes) ||
        !validBytes(budget.maxBytes) ||
        budget.maxBytes < budget.referenceBytes
      ) {
        throw new Error(`Invalid byte budget for ${route} in ${file}`);
      }
      if (budgets.has(route))
        throw new Error(`Duplicate bundle budget: ${route}`);
      budgets.set(route, budget);
    }
  }
  if (budgets.size === 0)
    throw new Error("Bundle budget policy contains no routes");
  return budgets;
}

/** Improvements and retired observations do not require policy maintenance. */
export function compareBudgets(budgets, measured, base = null) {
  if (!(measured instanceof Map) || measured.size === 0)
    throw new Error("No measured bundle routes");
  const failures = [];
  const rows = [];
  for (const [route, bytes] of [...measured.entries()].sort(([a], [b]) =>
    a < b ? -1 : a > b ? 1 : 0,
  )) {
    if (!validBytes(bytes))
      throw new Error(`Invalid measured bundle bytes for ${route}`);
    const budget = budgets.get(route);
    if (!budget) {
      failures.push(
        `Neue Route ohne genehmigtes Bundle-Budget: ${route}. Budget im passenden scripts/perf/bundle-budgets/<portal>/<resource>.json explizit prüfen und ergänzen.`,
      );
      continue;
    }
    const baseBytes = base?.get(route);
    rows.push({
      route,
      bytes,
      referenceBytes: budget.referenceBytes,
      maxBytes: budget.maxBytes,
      baseBytes,
      delta: baseBytes === undefined ? null : bytes - baseBytes,
    });
    if (bytes > budget.maxBytes) {
      failures.push(
        `${route}: ${bytes} Bytes überschreiten die genehmigte Obergrenze ${budget.maxBytes} Bytes. Bundle verkleinern oder eine begründete Policy-Änderung reviewen lassen; eine neue Messung erhöht kein Budget.`,
      );
    }
  }
  return { failures, rows };
}
