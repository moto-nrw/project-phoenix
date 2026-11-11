import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { readFileSync } from 'node:fs';
import { setTimeout } from 'node:timers/promises';
import test from 'node:test';

// Evaluate the provisioned LogQL with real Loki, including recovery and a
// stopped worker. No application or production service is contacted.
test('holiday closure alerts cover overdue work, recovery and a missing watchdog', { timeout: 90000 }, async t => {
  const rules = readFileSync(new URL('../monitoring/grafana/provisioning/alerting/holiday-closure-notifications.yml', import.meta.url), 'utf8');
  const expressions = [...rules.matchAll(/expr: '([^']+)'/g)].map(match => match[1]);
  assert.equal(expressions.length, 2);
  const name = `moto-holiday-closure-alert-test-${process.pid}-${Date.now()}`;
  const docker = (...args) => spawnSync('docker', args, { encoding: 'utf8', timeout: 60000 });
  t.after(() => docker('rm', '-f', name));
  const started = docker('run', '-d', '--name', name, '--network', 'none',
    'grafana/loki:3.4.3', '-config.file=/etc/loki/local-config.yaml',
    '-ingester.min-ready-duration=0s', '-querier.query-ingesters-within=0',
    '-querier.split-queries-by-interval=0', '-querier.cache-results=false',
    // The default 2 h chunk age drops older samples silently; the rule reads 30 h.
    '-ingester.max-chunk-age=60h');
  assert.equal(started.status, 0, started.stderr);
  const request = (path, payload) => docker('exec', name, '/busybox/wget', '-qO-',
    ...(payload ? ['--header=Content-Type: application/json', `--post-data=${JSON.stringify(payload)}`] : []),
    `http://127.0.0.1:3100${path}`);
  let ready = false;
  for (let attempt = 0; attempt < 60; attempt++) {
    if (request('/ready').status === 0) { ready = true; break; }
    await setTimeout(500);
  }
  assert.ok(ready, 'isolated Loki did not become ready');
  const now = Date.now();
  const hour = 3600000;
  const stamp = ms => String(BigInt(ms) * 1000000n);
  const push = (ms, record, env = 'prod') => {
    const line = JSON.stringify({ time: new Date(ms).toISOString(), ...record });
    const result = request('/loki/api/v1/push', { streams: [
      { stream: { env, service: 'server' }, values: [[stamp(ms), line]] },
    ] });
    assert.equal(result.status, 0, 'synthetic log ingestion failed');
  };
  const value = (index, at = now) => {
    const result = request(`/loki/api/v1/query?query=${encodeURIComponent(expressions[index])}&time=${stamp(at)}`);
    assert.equal(result.status, 0, result.stderr);
    const body = JSON.parse(result.stdout);
    assert.equal(body.status, 'success');
    assert.equal(body.data.result.length, 1);
    return Number(body.data.result[0].value[1]);
  };
  const checked = overdue => ({ level: 'INFO', msg: 'holiday closure watchdog checked', overdue });
  assert.equal(value(1), 0, 'no heartbeat alerts even with the worker stopped');
  push(now - 120000, checked(5));
  assert.equal(value(0), 5, 'old commands and blocked work alert');
  assert.ok(value(1) >= 1, 'the watchdog heartbeat is present');
  push(now - 60000, checked(0));
  assert.equal(value(0), 0, 'the latest healthy observation resolves the alarm');
  push(now - 30000, checked(8), 'staging');
  assert.equal(value(0), 0, 'staging cannot trigger the production alarm');
  assert.equal(value(1, now + 11 * 60000), 0, 'a stopped watchdog alerts after ten minutes');
});
