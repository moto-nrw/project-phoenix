import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { chmodSync, existsSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import test from 'node:test';

const script = new URL('./report-sentry-deploy.sh', import.meta.url).pathname;
const build = readFileSync(new URL('../.github/workflows/build.yml', import.meta.url), 'utf8');
const release = '0123456789abcdef0123456789abcdef01234567';
const token = 'sntrys_test-token-value';

// Runs the script with a fake sentry-cli on PATH that logs its arguments and
// the token it saw, then exits with cliExit.
function report(t, { args = [release, 'staging'], env = {}, cliExit = 0 } = {}) {
  const directory = mkdtempSync(join(tmpdir(), 'moto-sentry-deploy-'));
  t.after(() => rmSync(directory, { recursive: true, force: true }));
  const calls = join(directory, 'calls');
  const cli = join(directory, 'sentry-cli');
  writeFileSync(cli, `#!/usr/bin/env bash
printf '%s|token=%s\\n' "$*" "\${SENTRY_AUTH_TOKEN:-}" >> "${calls}"
echo "error: API request failed" >&2
exit ${cliExit}
`);
  chmodSync(cli, 0o755);
  const result = spawnSync('bash', [script, ...args], {
    env: { PATH: `${directory}:/usr/bin:/bin`, SENTRY_AUTH_TOKEN: token, SENTRY_ORG: 'moto', ...env },
    encoding: 'utf8', timeout: 5000,
  });
  const lines = existsSync(calls) ? readFileSync(calls, 'utf8').trim().split('\n') : [];
  return { ...result, calls: lines };
}

test('reports release, finalize and deploy for backend and frontend', t => {
  const result = report(t);
  assert.equal(result.status, 0, result.stderr);
  const projects = '--project backend --project frontend';
  assert.deepEqual(result.calls, [
    `releases new ${release} ${projects}|token=${token}`,
    `releases finalize ${release}|token=${token}`,
    `deploys new --release ${release} --env staging ${projects}|token=${token}`,
  ]);
  assert.doesNotMatch(result.stdout + result.stderr, /warning/);
});

test('without sentry-cli on PATH the script runs sentry-cli 3.8.0 through npx', t => {
  const directory = mkdtempSync(join(tmpdir(), 'moto-sentry-deploy-'));
  t.after(() => rmSync(directory, { recursive: true, force: true }));
  const calls = join(directory, 'calls');
  const npx = join(directory, 'npx');
  writeFileSync(npx, `#!/usr/bin/env bash\nprintf '%s\\n' "$*" >> "${calls}"\n`);
  chmodSync(npx, 0o755);
  const result = spawnSync('bash', [script, release, 'demo'], {
    env: { PATH: `${directory}:/usr/bin:/bin`, SENTRY_AUTH_TOKEN: token, SENTRY_ORG: 'moto' },
    encoding: 'utf8', timeout: 5000,
  });
  assert.equal(result.status, 0, result.stderr);
  const lines = readFileSync(calls, 'utf8').trim().split('\n');
  assert.equal(lines.length, 3);
  for (const line of lines) {
    assert.match(line, /^--yes @sentry\/cli@3\.8\.0 (releases|deploys) /);
  }
});

test('an unreachable Sentry only warns and keeps the deploy green', t => {
  const result = report(t, { cliExit: 1 });
  assert.equal(result.status, 0, result.stderr);
  assert.equal(result.calls.length, 1);
  assert.match(result.stdout, /^::warning title=Sentry deploy not reported::could not create release/m);
});

test('a failed deploy record after finalize still warns instead of failing', t => {
  const directory = mkdtempSync(join(tmpdir(), 'moto-sentry-deploy-'));
  t.after(() => rmSync(directory, { recursive: true, force: true }));
  const cli = join(directory, 'sentry-cli');
  writeFileSync(cli, '#!/usr/bin/env bash\n[ "$1" != deploys ]\n');
  chmodSync(cli, 0o755);
  const result = spawnSync('bash', [script, release, 'production'], {
    env: { PATH: `${directory}:/usr/bin:/bin`, SENTRY_AUTH_TOKEN: token, SENTRY_ORG: 'moto' },
    encoding: 'utf8', timeout: 5000,
  });
  assert.equal(result.status, 0, result.stderr);
  assert.match(result.stdout, /::could not record deploy of .* to production/);
});

for (const [name, options, message] of [
  ['missing token', { env: { SENTRY_AUTH_TOKEN: '' } }, /SENTRY_AUTH_TOKEN is not set/],
  ['missing org', { env: { SENTRY_ORG: '' } }, /SENTRY_ORG is not set/],
  ['missing release', { args: ['', 'staging'] }, /release argument is missing/],
  ['unknown environment', { args: [release, 'development'] }, /unexpected environment 'development'/],
]) {
  test(`${name} skips the report with a warning`, t => {
    const result = report(t, options);
    assert.equal(result.status, 0, result.stderr);
    assert.deepEqual(result.calls, []);
    assert.match(result.stdout, message);
  });
}

test('the token never appears in the script output', t => {
  for (const cliExit of [0, 1]) {
    const result = report(t, { cliExit });
    assert.doesNotMatch(result.stdout + result.stderr, new RegExp(token));
  }
});

for (const environment of ['staging', 'production', 'demo']) {
  test(`deploy-${environment} reports the deploy to Sentry without blocking`, () => {
    const job = build.split(`  deploy-${environment}:\n`)[1].split('\n  # ')[0];
    const step = job.split('      - name: Report deploy to Sentry\n')[1]?.split('\n      - name: ')[0];
    assert.ok(step, `deploy-${environment} has no Sentry step`);
    assert.ok(job.indexOf('Report deploy to Sentry') > job.indexOf(`name: Deploy to ${environment}`));
    assert.doesNotMatch(step, /\n        if:/);
    assert.match(step, /continue-on-error: true/);
    assert.match(step, /timeout-minutes: \d+/);
    assert.match(step, /SENTRY_AUTH_TOKEN: \$\{\{ secrets\.SENTRY_AUTH_TOKEN \}\}/);
    assert.match(step, new RegExp(
      `run: bash scripts/report-sentry-deploy\\.sh "\\$\\{\\{ github\\.sha \\}\\}" ${environment}$`, 'm'));
  });
}
