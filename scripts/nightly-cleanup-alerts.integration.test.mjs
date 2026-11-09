import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { readFileSync } from 'node:fs';
import { setTimeout } from 'node:timers/promises';
import test from 'node:test';

// Runs the LogQL of the nightly cleanup alert (#6327) against an isolated Loki
// with log lines in the shape the Worker writes them.
test('nightly cleanup LogQL alerts per school and job and ends with the recovery', { timeout: 90000 }, async t => {
  const rules = readFileSync(new URL('../monitoring/grafana/provisioning/alerting/nightly-cleanup.yml', import.meta.url), 'utf8');
  const expressions = [...rules.matchAll(/expr: '([^']+)'/g)].map(match => match[1]);
  assert.equal(expressions.length, 1);
  const threshold = Number(rules.match(/params: \[(\d+)\]/)[1]);
  const name = `moto-nightly-cleanup-alert-test-${process.pid}-${Date.now()}`;
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
  const failing = (tenant, job, failures) => ({
    level: 'ERROR', msg: 'nightly cleanup failing repeatedly', job_id: job, tenant_id: tenant,
    consecutive_failures: failures, failed_steps: 'feedback', error: 'data cleanup incomplete',
  });
  const recovered = (tenant, job, previous) => ({
    level: 'INFO', msg: 'nightly cleanup recovered', job_id: job, tenant_id: tenant,
    consecutive_failures: 0, previous_failures: previous,
  });
  // firing returns "tenant/job" of every series above the rule's threshold.
  const firing = () => {
    const result = request(`/loki/api/v1/query?query=${encodeURIComponent(expressions[0])}&time=${stamp(now)}`);
    assert.equal(result.status, 0, 'LogQL query failed');
    const body = JSON.parse(result.stdout);
    assert.equal(body.status, 'success');
    return body.data.result
      .filter(series => Number(series.value[1]) > threshold)
      .map(series => `${series.metric.tenant_id}/${series.metric.job_id}`)
      .sort();
  };

  // Lines of one stream go in in time order; Loki refuses older ones.
  assert.deepEqual(firing(), [], 'no lines, no alert');
  push(now - 31 * hour, failing(7, 'visit-cleanup', 9));
  assert.deepEqual(firing(), [], 'a line older than 30 hours does not alert');

  push(now - 27 * hour, failing(7, 'visit-cleanup', 6));
  push(now - 5 * hour, failing(8, 'visit-cleanup', 3));
  push(now - 4 * hour, failing(7, 'timetable-cleanup', 4));
  assert.deepEqual(firing(), ['7/timetable-cleanup', '7/visit-cleanup', '8/visit-cleanup'],
    'one alert per school and job, also 27 hours after the last failed run');

  push(now - hour, recovered(7, 'visit-cleanup', 6));
  assert.deepEqual(firing(), ['7/timetable-cleanup', '8/visit-cleanup'], 'the recovery ends only its own alert');

  push(now - 60000, { level: 'ERROR', msg: 'cleanup step failed', tenant_id: 7, step: 'feedback', error: 'x' });
  push(now - 60000, failing(9, 'visit-cleanup', 3), 'staging');
  assert.deepEqual(firing(), ['7/timetable-cleanup', '8/visit-cleanup'], 'step errors and staging lines do not alert');
});
