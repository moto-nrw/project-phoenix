#!/usr/bin/env node
// Explicit one-time backfill. Ordinary test runs never write authored budgets.
import { readFileSync, readdirSync, writeFileSync } from 'node:fs';
import { join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

function canonicalRoute(route) {
  while (route.includes('/*/')) route = route.replaceAll('/*/', '/');
  return route;
}
function routeSet(route) {
  const routes = typeof route === 'string' ? (route ? [route] : []) : route;
  if (!Array.isArray(routes) || routes.some(value => typeof value !== 'string' || !value)) throw new Error('Invalid route observation');
  return [...new Set(routes)].sort();
}
const sameRoutes = (a, b) => JSON.stringify(routeSet(a ?? [])) === JSON.stringify(routeSet(b ?? []));

export function capturedRoutes(jsonl) {
  const routes = new Map();
  for (const line of jsonl.split('\n').filter(line => line.trim())) {
    const { scenario, route = [] } = JSON.parse(line);
    if (typeof scenario !== 'string' || !scenario) throw new Error('Invalid route observation');
    if (routes.has(scenario) && !sameRoutes(routes.get(scenario), route)) throw new Error(`Conflicting routes for ${scenario}`);
    routes.set(scenario, routeSet(route));
  }
  return routes;
}

export function backfillBudget(budget, observations, exceptions = {}) {
  if (!observations.has(budget.scenario)) {
    if (budget.scenario.startsWith('api.')) throw new Error(`No observation for ${budget.scenario}`);
    return budget;
  }
  const route = routeSet(observations.get(budget.scenario));
  if (!route.length) {
    if (routeSet(budget.route ?? []).length) throw new Error(`HTTP route disappeared for ${budget.scenario}`);
    if (budget.scenario.startsWith('api.') && !exceptions[budget.scenario]) throw new Error(`Non-HTTP reason required for ${budget.scenario}`);
    return budget;
  }
  if (route.some(pattern => !/^[A-Z]+ \/\S*$/.test(pattern))) throw new Error(`Invalid chi route for ${budget.scenario}: ${route}`);
  if (exceptions[budget.scenario]) throw new Error(`HTTP measurement cannot claim an exception: ${budget.scenario}`);
  if (budget.route && !sameRoutes(budget.route, route)) throw new Error(`Refusing to overwrite ${budget.scenario}: ${budget.route} -> ${route}`);
  return { ...budget, route: route.length === 1 ? route[0] : route };
}

function fragments(directory, extension = '.json') {
  return readdirSync(directory, { withFileTypes: true }).flatMap(entry => {
    const path = join(directory, entry.name);
    return entry.isDirectory() ? fragments(path, extension) : entry.isFile() && path.endsWith(extension) ? [path] : [];
  });
}

function main(args) {
  const [report, ...options] = args;
  if (!report || options.some(option => option !== '--write')) throw new Error('Usage: node scripts/capture-query-budget-routes.mjs REPORT.jsonl [--write]');
  const root = resolve(fileURLToPath(new URL('..', import.meta.url)));
  const observations = capturedRoutes(readFileSync(report, 'utf8'));
  const exceptions = JSON.parse(readFileSync(join(root, 'backend/architecture/query-budget-non-http.json'), 'utf8'));
  // Validate the entire capture before changing a single file.
  const golden = new Set(fragments(join(root, 'backend/api/testdata/routes'), '.golden').flatMap(path => readFileSync(path, 'utf8').split('\n').filter(Boolean).map(canonicalRoute)));
  const updates = fragments(join(root, 'backend/test/query-budgets')).map(path => {
    const before = JSON.parse(readFileSync(path, 'utf8'));
    const after = backfillBudget(before, observations, exceptions);
    if (routeSet(after.route ?? []).some(route => !golden.has(route))) throw new Error(`Route absent from router golden: ${before.scenario}: ${after.route}`);
    return { path, before, after };
  }).filter(({ before, after }) => !sameRoutes(before.route, after.route));
  if (options.includes('--write')) for (const { path, after } of updates) writeFileSync(path, JSON.stringify(after, null, 2) + '\n');
  console.log(`${updates.length} route fields ${options.includes('--write') ? 'written' : 'ready; pass --write to apply'}. No maxima, exactness or identities changed.`);
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  try { main(process.argv.slice(2)); } catch (error) { console.error(error.message); process.exitCode = 1; }
}
