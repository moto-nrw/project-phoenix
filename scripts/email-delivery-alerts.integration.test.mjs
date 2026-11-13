import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { setTimeout } from 'node:timers/promises';
import test from 'node:test';

// The outbox alerts of #6737, evaluated with real Loki and promtool in isolated
// containers. No application or production service is contacted.
const rules = readFileSync(new URL('../monitoring/grafana/provisioning/alerting/email-delivery.yml', import.meta.url), 'utf8');
const ruleBlock = uid => {
  const start = rules.indexOf(`- uid: ${uid}\n`);
  assert.ok(start >= 0, `rule ${uid} is provisioned`);
  const next = rules.indexOf('\n      - uid: ', start + 1);
  return rules.slice(start, next < 0 ? undefined : next);
};
const expressionOf = block => block.match(/expr: '([^']+)'/)[1];
const docker = (...args) => spawnSync('docker', args, { encoding: 'utf8', timeout: 90000 });

test('outbox mails failed for good alert from the worker log line', { timeout: 90000 }, async t => {
  const block = ruleBlock('email-delivery-failed-for-good');
  const expression = expressionOf(block);
  // The worker's line when a run dead-lettered a delivery (ErrDeadLettered
  // in backend/modules/delivery/contract/worker.go, logged by the outbox job).

  const name = `moto-email-delivery-alert-test-${process.pid}-${Date.now()}`;
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
  const push = (ms, record, env = 'prod') => {
    const line = JSON.stringify({ time: new Date(ms).toISOString(), ...record });
    const result = request('/loki/api/v1/push', { streams: [
      { stream: { env, service: 'server' }, values: [[stamp(ms), line]] },
    ] });
    assert.equal(result.status, 0, 'synthetic log ingestion failed');
  };
  const value = (at = now) => {
    const result = request(`/loki/api/v1/query?query=${encodeURIComponent(expression)}&time=${stamp(at)}`);
    assert.equal(result.status, 0, result.stderr);
    const body = JSON.parse(result.stdout);
    assert.equal(body.status, 'success');
    assert.equal(body.data.result.length, 1);
    return Number(body.data.result[0].value[1]);
  };
  const tick = error => ({ level: 'ERROR', msg: 'outbox worker tick failed', job_id: 'email-outbox', error });
  assert.equal(value(), 0, 'no line, no alarm');
  push(now - 120000, tick('delivery worker: claim email: connection refused'));
  assert.equal(value(), 0, 'other worker failures belong to the server error alarm');
  push(now - 60000, tick('delivery worker: deliveries failed for good after all attempts: 1 (email holiday_registration_received delivery 7: 421 4.7.0 rate limit)'), 'staging');
  assert.equal(value(), 0, 'staging cannot trigger the production alarm');
  push(now - 30000, tick('delivery worker: deliveries failed for good after all attempts: 2 (1 email dispatches with unknown outcome: dispatch fence expired without finalization; email welcome delivery 9: provider rejected)'));
  assert.equal(value(), 1, 'a dead-lettered delivery alerts');
  assert.equal(value(now + 16 * 60000), 0, 'the alarm resolves once no delivery failed for 15 minutes');
});

test('oldest pending outbox mail alerts after an hour of waiting', { timeout: 120000 }, t => {
  const block = ruleBlock('email-delivery-oldest-pending');
  const expression = expressionOf(block);
  const threshold = Number(block.match(/params: \[(\d+)\]\n\s+type: gt/)[1]);
  const pending = block.match(/\n {8}for: (\w+)\n/)[1];
  assert.equal(threshold, 3600);
  assert.equal(pending, '5m');
  // backend/observability/prometheus.go exports the gauge.
  assert.match(expression, /phoenix_delivery_oldest_pending_age_seconds\{env="prod",service="server"\}/);

  const image = readFileSync(new URL('../monitoring/prometheus-compose.yml', import.meta.url), 'utf8')
    .match(/image: (prom\/prometheus:\S+)/)[1];
  const dir = mkdtempSync(join(tmpdir(), 'moto-email-delivery-alert-'));
  t.after(() => rmSync(dir, { recursive: true, force: true }));
  // Grafana reduces the instant query with "last" and compares it with the
  // threshold; as a Prometheus rule that is the same comparison.
  writeFileSync(join(dir, 'rules.json'), JSON.stringify({ groups: [{ name: 'outbox', rules: [{
    alert: 'OutboxMailWaiting', expr: `(${expression}) > ${threshold}`, for: pending,
  }] }] }));
  const series = env => `phoenix_delivery_oldest_pending_age_seconds{job="phoenix-backend-${env}", instance="${env}-server-1:8080", env="${env}", service="server"}`;
  const alert = (minute, firing) => ({ eval_time: `${minute}m`, alertname: 'OutboxMailWaiting', exp_alerts: firing ? [{ exp_labels: {} }] : [] });
  // prod: a short queue, then a throttled mail ages one minute per minute from
  // minute 10 and goes out at minute 80. staging waits all along.
  const aging = Array.from({ length: 70 }, (_, i) => String((i + 1) * 60)).join(' ');
  writeFileSync(join(dir, 'tests.json'), JSON.stringify({
    rule_files: ['rules.json'],
    evaluation_interval: '1m',
    tests: [{
      interval: '1m',
      input_series: [
        { series: series('prod'), values: `30x9 ${aging} 0x20` },
        { series: series('staging'), values: '7200x100' },
      ],
      alert_rule_test: [
        alert(9, false),
        // The mail is older than an hour from minute 70; the alert waits its pending period.
        alert(72, false),
        alert(76, true),
        alert(79, true),
        alert(85, false),
      ],
    }, {
      // No worker observation yet: nothing to alert on.
      interval: '1m',
      input_series: [],
      alert_rule_test: [alert(10, false)],
    }],
  }));

  const name = `moto-email-delivery-promtool-${process.pid}-${Date.now()}`;
  t.after(() => docker('rm', '-f', name));
  const created = docker('create', '--name', name, '--network', 'none', '--workdir', '/rules',
    '--entrypoint', 'promtool', image, 'test', 'rules', 'tests.json');
  assert.equal(created.status, 0, created.stderr);
  assert.equal(docker('cp', `${dir}/.`, `${name}:/rules`).status, 0, 'copying the rule files failed');
  const run = docker('start', '--attach', name);
  assert.equal(run.status, 0, `promtool test rules failed:\n${run.stdout}${run.stderr}`);
  assert.match(run.stdout + run.stderr, /SUCCESS/);
});
