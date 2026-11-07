#!/usr/bin/env node
import { readFileSync } from 'node:fs';
import { pathToFileURL } from 'node:url';

export function readReport(path) {
  return readFileSync(path, 'utf8').split('\n').flatMap((line, index) => {
    if (!line.trim()) return [];
    let row;
    try { row = JSON.parse(line); } catch (error) {
      throw new Error(`${path}:${index + 1}: ${error.message}`);
    }
    const integer = (value) => Number.isSafeInteger(value) && value >= 0;
    if (!row || typeof row.scenario !== 'string' || !row.scenario ||
        (row.test !== undefined && typeof row.test !== 'string') ||
        !integer(row.actual) || !integer(row.max) || typeof row.exact !== 'boolean' ||
        !row.duplicate_statements || Array.isArray(row.duplicate_statements) ||
        typeof row.duplicate_statements !== 'object' ||
        Object.values(row.duplicate_statements).some((count) => !integer(count) || count < 2)) {
      throw new Error(`${path}:${index + 1}: invalid query-budget report row`);
    }
    return [row];
  });
}

export function aggregate(rows) {
  const groups = new Map();
  for (const row of rows) {
    const key = JSON.stringify([row.scenario, row.test ?? '']);
    if (!groups.has(key)) groups.set(key, {
      scenario: row.scenario, test: row.test ?? '', samples: 0, total: 0,
      actuals: new Map(), budgets: new Set(), duplicates: new Map(),
    });
    const group = groups.get(key);
    group.samples++;
    group.total += row.actual;
    group.actuals.set(row.actual, (group.actuals.get(row.actual) ?? 0) + 1);
    group.budgets.add(`${row.exact ? '=' : '≤'}${row.max}`);
    for (const [form, count] of Object.entries(row.duplicate_statements)) {
      group.duplicates.set(form, (group.duplicates.get(form) ?? 0) + count - 1);
    }
  }
  return groups;
}

const ordered = (values) => [...values].sort((a, b) => a < b ? -1 : a > b ? 1 : 0);
const distribution = (group) => group ? [...group.actuals].sort((a, b) => a[0] - b[0])
  .map(([actual, count]) => `${actual}×${count}`).join(', ') : 'missing';
const cell = (value) => String(value).replaceAll('|', '\\|').replaceAll('\n', ' ');
const tableRow = (values) => `| ${values.map(cell).join(' | ')} |`;

export function renderScoreboard(beforeRows, afterRows, { duplicates = false } = {}) {
  const before = aggregate(beforeRows);
  const after = afterRows === undefined ? undefined : aggregate(afterRows);
  const keys = ordered(new Set([...before.keys(), ...(after?.keys() ?? [])]));
  const lines = [after
    ? '| Scenario | Test / fixture | Baseline actual×samples | Current actual×samples | Δ total | Budget | Duplicate forms |'
    : '| Scenario | Test / fixture | Actual×samples | Budget | Duplicate forms | Duplicate repetitions |',
  after ? '| --- | --- | --- | --- | ---: | --- | ---: |' : '| --- | --- | --- | --- | ---: | ---: |'];
  for (const key of keys) {
    const base = before.get(key);
    const current = after ? after.get(key) : base;
    const identity = current ?? base;
    const budget = current ? ordered(current.budgets).join(', ') : 'missing';
    const prefix = [identity.scenario, identity.test || '(test unspecified)'];
    if (after) {
      const delta = current && base && current.samples === base.samples
        ? `${current.total - base.total >= 0 ? '+' : ''}${current.total - base.total}`
        : 'coverage changed';
      lines.push(tableRow([...prefix, distribution(base), distribution(current), delta, budget,
        `${base?.duplicates.size ?? '-'} → ${current?.duplicates.size ?? '-'}`]));
    } else lines.push(tableRow([...prefix, distribution(current), budget, current.duplicates.size,
      [...current.duplicates.values()].reduce((sum, count) => sum + count, 0)]));
    if (duplicates && current) for (const form of ordered(current.duplicates.keys())) {
      lines.push(`\n    ${JSON.stringify(form)}: ${current.duplicates.get(form)} repeated statement(s)`);
    }
  }
  const total = (groups) => [...groups.values()].reduce((sum, group) => sum + group.total, 0);
  lines.push(`\nTotal statements (all samples): ${total(before)}${after ? ` → ${total(after)}` : ''}.`);
  if (after) lines.push('Deltas require equal sample counts; inspect fixture names and actual distributions before interpreting totals.');
  return `${lines.join('\n')}\n`;
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  try {
    const args = process.argv.slice(2);
    const paths = args.filter((arg) => arg !== '--duplicates');
    if (paths.length < 1 || paths.length > 2 || paths.some((arg) => arg.startsWith('--'))) {
      throw new Error('Usage: node scripts/query-budget-scoreboard.mjs [--duplicates] REPORT.jsonl [CURRENT.jsonl]');
    }
    process.stdout.write(renderScoreboard(readReport(paths[0]), paths[1] ? readReport(paths[1]) : undefined,
      { duplicates: args.includes('--duplicates') }));
  } catch (error) {
    process.stderr.write(`${error.message}\n`);
    process.exitCode = 1;
  }
}
