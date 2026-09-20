import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { mkdtempSync, mkdirSync, writeFileSync, readFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import test from 'node:test';

function fixture(t) {
  const root = mkdtempSync(join(tmpdir(), 'student-contract-monitor-'));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  mkdirSync(join(root, 'bin'));
  writeFileSync(join(root, 'bin/docker'), '#!/bin/sh\ncat "$MONITOR_TEST_STATE"\nexit "${MONITOR_TEST_FAILURE:-0}"\n', { mode: 0o755 });
  const baseline = join(root, 'baseline.json');
  const output = join(root, 'status.prom');
  const stateFile = join(root, 'state.json');
  const state = { database: 'fixture', database_oid: 12, system_identifier: '34',
    compatibility_objects: 2, reads: 0, writes: 0, statistics_reset: '2026-09-20T00:00:00Z',
    statistics_dealloc: 0, tracking_all: true, old_query_fingerprint: 'a'.repeat(64) };
  const run = (mode, value = state, failure = '0') => {
    writeFileSync(stateFile, JSON.stringify(value));
    return spawnSync('bash', ['monitoring/student-contract/observe.sh', mode, 'fixture', 'fixture', baseline, output], {
      encoding: 'utf8', timeout: 10000,
      env: { ...process.env, PATH: `${join(root, 'bin')}:${process.env.PATH}`,
        MONITOR_TEST_STATE: stateFile, MONITOR_TEST_FAILURE: failure },
    });
  };
  return { run, state, baseline: () => readFileSync(baseline, 'utf8'), metrics: () => readFileSync(output, 'utf8') };
}

test('baseline is explicit and immutable; healthy polls and removal of both counters pass', t => {
  const f = fixture(t);
  assert.equal(f.run('snapshot').status, 0);
  const original = f.baseline();
  assert.equal(f.run('snapshot').status, 1);
  assert.equal(f.baseline(), original);
  assert.equal(f.run('check').status, 0);
  assert.match(f.metrics(), /observation_ok 1\n/);
  assert.match(f.metrics(), /observation_timestamp_seconds \d+\n/);
  assert.equal(f.run('check', { ...f.state, compatibility_objects: 0 }).status, 0);
});

for (const change of [{ reads: 1 }, { writes: 1 }, { old_query_fingerprint: 'b'.repeat(64) },
  { statistics_dealloc: 1 }, { statistics_reset: '2026-09-21T00:00:00Z' },
  { database_oid: 99 }, { system_identifier: 'other' }, { database: 'other' }, { compatibility_objects: 1 }, { tracking_all: false }]) {
  test(`changed observation alerts and preserves baseline: ${JSON.stringify(change)}`, t => {
    const f = fixture(t);
    assert.equal(f.run('snapshot').status, 0);
    const original = f.baseline();
    assert.equal(f.run('check', { ...f.state, ...change }).status, 1);
    assert.match(f.metrics(), /observation_ok 0\n/);
    assert.equal(f.baseline(), original);
  });
}

test('failed or malformed reads and missing baseline publish failure, not previous success', t => {
  const f = fixture(t);
  assert.equal(f.run('check').status, 1);
  assert.equal(f.run('snapshot').status, 0);
  for (const [value, exit] of [[f.state, '1'], [{}, '0'], [null, '0']]) {
    assert.equal(f.run('check', value, exit).status, 1);
    assert.match(f.metrics(), /observation_ok 0\n/);
    assert.match(f.metrics(), /observation_read_ok 0\n/);
  }
});

test('a snapshot cannot accept prior hits or already removed compatibility objects', t => {
  const f = fixture(t);
  for (const change of [{ reads: 1 }, { writes: 1 }, { compatibility_objects: 0 }]) {
    assert.equal(f.run('snapshot', { ...f.state, ...change }).status, 1);
  }
});
