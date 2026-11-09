import assert from 'node:assert/strict';
import { mkdtempSync, writeFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { spawnSync } from 'node:child_process';
import { test } from 'node:test';
import { aggregate, readReport, renderScoreboard } from './query-budget-scoreboard.mjs';

const row = (scenario, actual, fixture = 'TestList/N=3') => ({
  scenario, test: fixture, actual, max: 20, exact: false,
  duplicate_statements: { 'select ?': 3 },
});

test('sorts scenarios and preserves test fixtures and differing counts within one scenario', () => {
  const rows = [row('z', 5), row('a', 7, 'TestList/N=9'), row('a', 3), row('a', 4), row('a', 3)];
  const text = renderScoreboard(rows);
  assert.ok(text.indexOf('| a |') < text.indexOf('| z |'));
  assert.match(text, /TestList\/N=3 \| 3×2, 4×1/);
  assert.match(text, /TestList\/N=9 \| 7×1/);
  assert.match(text, /Total statements \(all samples\): 22/);
  const group = aggregate(rows).get(JSON.stringify(['a', 'TestList/N=3']));
  assert.equal(group.duplicates.get('select ?'), 6);
});

test('compares identical coverage; marks missing fixtures and changed sample count', () => {
  const before = [row('same', 8), row('gone', 5), row('repeat', 6)];
  const after = [row('same', 4), row('added', 7), row('repeat', 6), row('repeat', 9)];
  const text = renderScoreboard(before, after);
  assert.match(text, /same \| TestList\/N=3 \| 8×1 \| 4×1 \| -4/);
  assert.match(text, /gone .*missing \| coverage changed/);
  assert.match(text, /repeat .*6×1, 9×1 \| coverage changed/);
  assert.match(text, /added .*missing \| 7×1 \| coverage changed/);
});

test('CLI reads appended JSONL, prints duplicate forms and rejects corrupt data', (t) => {
  const dir = mkdtempSync(join(tmpdir(), 'query-scoreboard-'));
  t.after(() => rmSync(dir, { recursive: true, force: true }));
  const path = join(dir, 'report.jsonl');
  writeFileSync(path, `${JSON.stringify(row('scenario', 7))}\n\n${JSON.stringify(row('scenario', 4))}\n`);
  assert.equal(readReport(path).length, 2);
  const run = (...args) => spawnSync(process.execPath, [new URL('./query-budget-scoreboard.mjs', import.meta.url).pathname, ...args], { encoding: 'utf8' });
  const result = run('--duplicates', path);
  assert.equal(result.status, 0, result.stderr);
  assert.match(result.stdout, /4×1, 7×1/);
  assert.match(result.stdout, /"select \?": 4 repeated/);
  writeFileSync(path, '{"scenario":\n');
  const invalid = run(path);
  assert.equal(invalid.status, 1);
  assert.match(invalid.stderr, /report\.jsonl:1:/);
  writeFileSync(path, JSON.stringify({ ...row('bad', 5), actual: -1 }));
  assert.throws(() => readReport(path), /invalid query-budget report row/);
});
