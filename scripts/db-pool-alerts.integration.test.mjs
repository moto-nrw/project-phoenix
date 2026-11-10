import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import test from 'node:test';

// Evaluates the PromQL of the DB pool utilization alert (#6465) with promtool
// in an isolated Prometheus container: synthetic backend gauges must fire the
// alert after its pending period and resolve it again, and a scraped backend
// without the gauges must not pass as OK.
test('DB pool utilization alert fires, resolves and flags missing pool gauges', { timeout: 120000 }, t => {
  const rules = readFileSync(new URL('../monitoring/grafana/provisioning/alerting/infrastructure.yml', import.meta.url), 'utf8');
  const block = rules.slice(rules.indexOf('- uid: dfslb2pyr356ob'));
  assert.match(block, /^- uid: dfslb2pyr356ob\n {8}#/, 'the rule keeps its UID');
  const expression = block.match(/expr: '([^']+)'/)[1];
  const threshold = Number(block.match(/params:\n\s+- (\d+)\n\s+type: gt/)[1]);
  const pending = block.match(/\n {8}for: (\w+)\n/)[1];
  // Missing data and evaluation errors must not read as a healthy pool.
  assert.match(block, /\n {8}noDataState: Alerting\n {8}execErrState: Alerting\n/);
  assert.equal(threshold, 80);
  assert.equal(pending, '10m');

  const image = readFileSync(new URL('../monitoring/prometheus-compose.yml', import.meta.url), 'utf8')
    .match(/image: (prom\/prometheus:\S+)/)[1];
  const dir = mkdtempSync(join(tmpdir(), 'moto-db-pool-alert-'));
  t.after(() => rmSync(dir, { recursive: true, force: true }));
  // Grafana reduces the instant query with "last" and compares it with the
  // threshold; as a Prometheus rule that is the same comparison.
  writeFileSync(join(dir, 'rules.json'), JSON.stringify({ groups: [{ name: 'pool', rules: [{
    alert: 'DBPoolUtilizationHigh', expr: `(${expression}) > ${threshold}`, for: pending,
  }] }] }));
  const backend = env => `{job="phoenix-backend-${env}", instance="${env}-server-1:8080", env="${env}", service="server"}`;
  const frontend = env => `{job="phoenix-frontend-${env}", instance="${env}-frontend-1:3000", env="${env}", service="frontend"}`;
  const alert = (minute, ...labels) => ({ eval_time: `${minute}m`, alertname: 'DBPoolUtilizationHigh', exp_alerts: labels.map(exp_labels => ({ exp_labels })) });
  writeFileSync(join(dir, 'tests.json'), JSON.stringify({
    rule_files: ['rules.json'],
    evaluation_interval: '1m',
    tests: [
      {
        // prod: 10 of 40 connections, 36 (90%) from minute 10 to 40, then 10 again.
        interval: '1m',
        input_series: [
          { series: `up${backend('prod')}`, values: '1x70' },
          { series: `up${frontend('prod')}`, values: '1x70' },
          { series: `phoenix_db_max_open_connections${backend('prod')}`, values: '40x70' },
          { series: `phoenix_db_in_use_connections${backend('prod')}`, values: '10x9 36x30 10x30' },
        ],
        alert_rule_test: [
          alert(9),
          // The 5-minute mean crosses 80% at minute 14; the alert waits out its pending period.
          alert(20),
          alert(25, { env: 'prod' }),
          alert(40, { env: 'prod' }),
          alert(45),
        ],
      },
      {
        // staging runs a backend without the pool gauges until minute 20.
        interval: '1m',
        input_series: [
          { series: `up${backend('prod')}`, values: '1x40' },
          { series: `phoenix_db_max_open_connections${backend('prod')}`, values: '40x40' },
          { series: `phoenix_db_in_use_connections${backend('prod')}`, values: '4x40' },
          { series: `up${backend('staging')}`, values: '1x40' },
          { series: `phoenix_db_max_open_connections${backend('staging')}`, values: '_x20 40x20' },
          { series: `phoenix_db_in_use_connections${backend('staging')}`, values: '_x20 5x20' },
        ],
        alert_rule_test: [
          alert(5),
          alert(11, { env: 'staging', reason: 'pool_metric_missing' }),
          alert(25),
        ],
      },
      {
        // A backend that is down is Scrape Target Down's alert, not this one.
        interval: '1m',
        input_series: [
          { series: `up${backend('staging')}`, values: '0x30' },
        ],
        alert_rule_test: [alert(15)],
      },
    ],
  }));

  const name = `moto-db-pool-alert-test-${process.pid}-${Date.now()}`;
  const docker = (...args) => spawnSync('docker', args, { encoding: 'utf8', timeout: 90000 });
  t.after(() => docker('rm', '-f', name));
  const created = docker('create', '--name', name, '--network', 'none', '--workdir', '/rules',
    '--entrypoint', 'promtool', image, 'test', 'rules', 'tests.json');
  assert.equal(created.status, 0, created.stderr);
  assert.equal(docker('cp', `${dir}/.`, `${name}:/rules`).status, 0, 'copying the rule files failed');
  const run = docker('start', '--attach', name);
  assert.equal(run.status, 0, `promtool test rules failed:\n${run.stdout}${run.stderr}`);
  assert.match(run.stdout + run.stderr, /SUCCESS/);
});
