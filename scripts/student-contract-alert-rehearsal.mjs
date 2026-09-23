// Local-only disposable alert exercise. Never uses production data or contacts.
import assert from 'node:assert/strict';
import { execFileSync } from 'node:child_process';
import { mkdtempSync, mkdirSync, writeFileSync, copyFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { randomBytes } from 'node:crypto';

const endpoint = execFileSync('docker', ['context', 'inspect', '--format', '{{.Endpoints.docker.Host}}'], { encoding: 'utf8' }).trim();
assert.ok(endpoint.startsWith('unix://') && !process.env.DOCKER_HOST && !process.env.DOCKER_CONTEXT, 'local Docker context required');
const root = mkdtempSync(join(tmpdir(), 'student-contract-alerts-'));
const project = 'student-contract-alerts-' + randomBytes(4).toString('hex');
const password = randomBytes(24).toString('hex');
const source = 'monitoring/grafana/provisioning/alerting/student-contract.yml';
const rules = JSON.parse(execFileSync('yq', ['.', source], { encoding: 'utf8' }));
mkdirSync(join(root, 'provisioning/alerting'), { recursive: true });
mkdirSync(join(root, 'provisioning/datasources'), { recursive: true });
copyFileSync(source, join(root, 'provisioning/alerting/student-contract.yml'));
for (const name of ['loki.yml', 'prometheus.yml']) copyFileSync('monitoring/grafana/provisioning/datasources/' + name, join(root, 'provisioning/datasources', name));
writeFileSync(join(root, 'sink.mjs'), `
import http from 'node:http';
let ok = 1; const events = [];
http.createServer(async (req,res) => {
  if (req.url === '/metrics') { res.setHeader('content-type','text/plain'); res.end('phoenix_student_contract_observation_ok '+ok+'\\nphoenix_student_contract_observation_timestamp_seconds '+Math.floor(Date.now()/1000)+'\\n'); return; }
  if (req.url === '/fail') { ok=0; res.end('ok'); return; }
  if (req.url === '/clear') { events.length=0; res.end('ok'); return; }
  if (req.url === '/events') { res.setHeader('content-type','application/json'); res.end(JSON.stringify(events)); return; }
  let body=''; for await (const chunk of req) body+=chunk;
  events.push(JSON.parse(body)); res.end('ok');
}).listen(8080,'0.0.0.0');
`);
writeFileSync(join(root, 'prometheus.yml'), JSON.stringify({ global: { scrape_interval: '1s' }, scrape_configs: [{ job_name: 'fixture', static_configs: [{ targets: ['sink:8080'] }] }] }));
writeFileSync(join(root, 'loki.yml'), JSON.stringify({ auth_enabled: false, server: { http_listen_port: 3100 },
  common: { path_prefix: '/tmp/loki', replication_factor: 1, ring: { kvstore: { store: 'inmemory' } }, storage: { filesystem: { chunks_directory: '/tmp/loki/chunks', rules_directory: '/tmp/loki/rules' } } },
  schema_config: { configs: [{ from: '2024-01-01', store: 'tsdb', object_store: 'filesystem', schema: 'v13', index: { prefix: 'index_', period: '24h' } }] } }));
const composePath = join(root, 'compose.json');
writeFileSync(composePath, JSON.stringify({ services: {
  grafana: { image: 'grafana/grafana:11.5.2', ports: ['127.0.0.1::3000'], environment: {
    GF_SECURITY_ADMIN_USER: 'fixture', GF_SECURITY_ADMIN_PASSWORD: password,
    GF_ANALYTICS_REPORTING_ENABLED: 'false', GF_ANALYTICS_CHECK_FOR_UPDATES: 'false',
  }, volumes: ['./provisioning:/etc/grafana/provisioning:ro'] },
  loki: { image: 'grafana/loki:3.4.3', ports: ['127.0.0.1::3100'], command: ['-config.file=/etc/loki/fixture.yml'], volumes: ['./loki.yml:/etc/loki/fixture.yml:ro'] },
  prometheus: { image: 'prom/prometheus:v3.7.3', volumes: ['./prometheus.yml:/etc/prometheus/prometheus.yml:ro'] },
  sink: { image: 'node:24-alpine', command: ['node', '/fixture.mjs'], ports: ['127.0.0.1::8080'], volumes: ['./sink.mjs:/fixture.mjs:ro'] },
} }), { mode: 0o600 });
const compose = (...args) => execFileSync('docker', ['compose', '--project-name', project, '--env-file', '/dev/null', '-f', composePath, ...args], { encoding: 'utf8', timeout: 300000 });
const sleep = ms => new Promise(resolve => setTimeout(resolve, ms));
async function waitFor(label, predicate) {
  for (let attempt = 0; attempt < 180; attempt++) {
    try { if (await predicate()) { console.log('PASS', label); return; } } catch { /* startup and eventual evaluation */ }
    await sleep(1000);
  }
  throw new Error('Timed out: ' + label);
}
async function request(base, path, body, auth = false, method) {
  const headers = { 'Content-Type': 'application/json' };
  if (auth) headers.Authorization = 'Basic ' + Buffer.from('fixture:' + password).toString('base64');
  const response = await fetch(base + path, { method: method ?? (body === undefined ? 'GET' : 'POST'), headers, body: body === undefined ? undefined : JSON.stringify(body), signal: AbortSignal.timeout(10000) });
  assert.ok(response.ok, 'HTTP ' + response.status + ' for ' + path);
  const text = await response.text();
  try { return JSON.parse(text); } catch { return text; }
}
try {
  compose('config', '--quiet');
  compose('up', '-d');
  const url = (service, port) => 'http://' + compose('port', service, port).trim();
  const grafana = url('grafana', '3000'), loki = url('loki', '3100'), sink = url('sink', '8080');
  await waitFor('Grafana ready', async () => (await request(grafana, '/api/health')).database === 'ok');
  await waitFor('Loki ready', async () => (await request(loki, '/ready')).trim() === 'ready');
  const provisioned = await request(grafana, '/api/v1/provisioning/alert-rules', undefined, true);
  assert.deepEqual(provisioned.map(r => r.uid).sort(), rules.groups[0].rules.map(r => r.uid).sort());
  console.log('PASS exact checked-in alert rules provisioned');
  const logExpr = rules.groups[0].rules[1].data[0].model.expr;
  const logCount = async () => Number((await request(loki, '/loki/api/v1/query?query=' + encodeURIComponent(logExpr))).data.result[0].value[1]);
  assert.equal(await logCount(), 0);
  await request(grafana, '/api/v1/provisioning/contact-points', { uid: 'fixture-sink', name: 'Local fixture', type: 'webhook', settings: { url: 'http://sink:8080/notify' }, disableResolveMessage: true }, true);
  await request(grafana, '/api/v1/provisioning/policies', { receiver: 'Local fixture', group_by: ['alertname'], group_wait: '0s', group_interval: '1s', repeat_interval: '1h' }, true, 'PUT');
  const states = async () => (await request(grafana, '/api/prometheus/grafana/api/v1/rules', undefined, true)).data.groups.flatMap(g => g.rules);
  await waitFor('both alerts healthy before injection', async () => {
    const rs = await states(); return rs.length === 2 && rs.every(r => r.state === 'inactive' && r.health === 'ok');
  });
  await request(sink, '/clear');
  const lines = ['ERROR: relation "users.students" does not exist', 'ERROR: relation "users.students_legacy" does not exist', 'ERROR: relation "users.expired_privacy_consents" does not exist', 'ERROR: unrelated fixture failure', 'LOG: unrelated statement'];
  const timestamp = BigInt(Date.now()) * 1000000n;
  await request(loki, '/loki/api/v1/push', { streams: [{ stream: { env: 'prod', service: 'postgres' }, values: lines.map((line, i) => [String(timestamp + BigInt(i)), line]) }] });
  await waitFor('LogQL matches exactly three old-object errors', async () => await logCount() === 3);
  await request(sink, '/fail');
  await waitFor('both Grafana alerts firing', async () => {
    const rs = await states(); return rs.length === 2 && rs.every(r => r.state === 'firing' && r.health === 'ok');
  });
  await waitFor('both notifications delivered to local-only sink', async () => {
    const events = await request(sink, '/events');
    const titles = events.flatMap(e => e.alerts ?? []).filter(a => a.status === 'firing').map(a => a.labels.alertname);
    return rules.groups[0].rules.every(rule => titles.includes(rule.title));
  });
} finally {
  compose('down', '--volumes', '--remove-orphans');
  rmSync(root, { recursive: true, force: true });
}
