import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { readFileSync } from 'node:fs';
import { setTimeout } from 'node:timers/promises';
import test from 'node:test';

test('daily backup LogQL detects silence and failures without trusting legacy backup logs', { timeout: 90000 }, async t => {
  const rules = readFileSync(new URL('../monitoring/grafana/provisioning/alerting/daily-backup.yml', import.meta.url), 'utf8');
  // Expressions deliberately occupy single quoted YAML scalars. YAML syntax is
  // validated separately; Loki itself parses and evaluates the extracted LogQL.
  const expressions = [...rules.matchAll(/expr: '([^']+)'/g)].map(match => match[1]);
  assert.equal(expressions.length, 2);
  const name = `moto-backup-alert-test-${process.pid}-${Date.now()}`;
  const docker = (...args) => spawnSync('docker', args, { encoding: 'utf8', timeout: 60000 });
  t.after(() => docker('rm', '-f', name));
  const started = docker('run', '-d', '--name', name, '--network', 'none',
    'grafana/loki:3.4.3', '-config.file=/etc/loki/local-config.yaml',
    '-ingester.min-ready-duration=0s', '-querier.query-ingesters-within=0',
    '-querier.split-queries-by-interval=0', '-querier.cache-results=false');
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
  const stamp = ms => String(BigInt(ms) * 1000000n);
  const push = (service, ms, line, env = 'prod') => {
    const result = request('/loki/api/v1/push', { streams: [
      { stream: { env, service }, values: [[stamp(ms), line]] },
    ] });
    assert.equal(result.status, 0, 'synthetic log ingestion failed');
  };
  const query = index => {
    const result = request(`/loki/api/v1/query?query=${encodeURIComponent(expressions[index])}&time=${stamp(now)}`);
    assert.equal(result.status, 0, 'LogQL query failed');
    const body = JSON.parse(result.stdout);
    assert.equal(body.status, 'success');
    return body.data.result.map(series => Number(series.value[1]));
  };
  const hour = 3600000;
  assert.deepEqual(query(0), [1]); // Includes a stream that has never existed.
  assert.deepEqual(query(1), []);
  push('backup', now - 1000, 'Backup complete');
  push('daily-backup', now - 1000, 'backup_result=ok step=complete', 'staging');
  push('daily-backup', now - 27 * hour, 'backup_result=ok step=complete');
  assert.deepEqual(query(0), [1]); // Legacy, wrong env, expired success cannot mask silence.
  push('daily-backup', now - 60000, 'backup_result=ok step=complete');
  assert.deepEqual(query(0), []);
  push('daily-backup', now - 1000, 'backup_result=failed step=upload');
  const unit = readFileSync(new URL('./systemd/moto-backup-failure@.service', import.meta.url), 'utf8');
  const failure = unit.match(/backup_result=failed[^\n]+/)[0];
  push('daily-backup', now - 500, failure);
  assert.deepEqual(query(1), [2]); // Both the script and systemd fallback reach the rule.
  push('daily-backup', now - 100, 'backup_result=ok step=complete');
  assert.deepEqual(query(1), [2]); // A later success does not erase a recent failure.
});
