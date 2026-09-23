import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { runInNewContext } from 'node:vm';
import test from 'node:test';

const build = readFileSync(new URL('../.github/workflows/build.yml', import.meta.url), 'utf8');
const rollback = readFileSync(new URL('../.github/workflows/rollback.yml', import.meta.url), 'utf8');

function stepRun(workflow, name) {
  const start = workflow.indexOf(`      - name: ${name}\n`);
  assert.ok(start >= 0, name);
  const step = workflow.slice(start).split('\n      - name: ')[0];
  const run = step.match(/        run: \|\n([\s\S]*)/);
  assert.ok(run, name);
  return run[1].split('\n').filter(line => !line || line.startsWith('          '))
    .map(line => line.slice(10)).join('\n');
}

function interpolate(script, context) {
  return script.replace(/\$\{\{\s*(.*?)\s*\}\}/g, (_, expression) =>
    String(runInNewContext(expression, context, { timeout: 100 })));
}

function runStep(t, script, env = {}) {
  const directory = mkdtempSync(join(tmpdir(), 'moto-demo-workflow-'));
  t.after(() => rmSync(directory, { recursive: true, force: true }));
  const output = join(directory, 'output');
  writeFileSync(output, '');
  const result = spawnSync('bash', ['-eu', '-c', script], {
    env: { ...process.env, ...env, GITHUB_OUTPUT: output }, encoding: 'utf8', timeout: 5000,
  });
  return { ...result, output: readFileSync(output, 'utf8') };
}

for (const [event, ref, target, expected] of [
  ['push', 'main', '', 'production'],
  ['push', 'development', '', 'staging'],
  ['workflow_dispatch', 'main', 'demo', 'demo'],
  ['workflow_dispatch', 'main', 'production', 'production'],
  ['workflow_dispatch', 'development', 'staging', 'staging'],
  ['workflow_dispatch', 'development', 'demo', null],
  ['workflow_dispatch', 'feature', 'demo', null],
  ['workflow_dispatch', 'development', 'production', null],
  ['workflow_dispatch', 'main', 'unknown', null],
]) {
  test(`build routing: ${event} ${ref} ${target}`, t => {
    const script = interpolate(stepRun(build, 'Determine environment'), {
      github: { event_name: event, ref: `refs/heads/${ref}`, event: { inputs: { environment: target } } },
      inputs: { environment: target },
    });
    const result = runStep(t, script);
    if (expected) {
      assert.equal(result.status, 0, result.stderr);
      assert.match(result.output, new RegExp(`^environment=${expected}$`, 'm'));
    } else {
      assert.notEqual(result.status, 0);
    }
  });
}

test('demo job rejects push events and non-main refs independently of routing', t => {
  const script = stepRun(build, 'Guard demo source');
  for (const [event, ref, status] of [
    ['push', 'refs/heads/main', 1], ['workflow_dispatch', 'refs/heads/development', 1],
    ['workflow_dispatch', 'refs/heads/main', 0],
  ]) {
    assert.equal(runStep(t, script, { EVENT_NAME: event, REF_NAME: ref }).status, status);
  }
  const job = build.split('  deploy-demo:\n')[1].split('\n  # Summary job')[0];
  assert.match(job, /if: github.event_name == 'workflow_dispatch' && github.ref == 'refs\/heads\/main'/);
  assert.match(job, /group: deploy-demo/);
  assert.match(job, /ref: \$\{\{ github.sha \}\}/);
});

test('demo rollback accepts only main and selects its own directory', t => {
  const script = stepRun(rollback, 'Set environment config');
  assert.equal(runStep(t, script, { ENVIRONMENT: 'demo', REF_NAME: 'refs/heads/development' }).status, 1);
  const result = runStep(t, script, { ENVIRONMENT: 'demo', REF_NAME: 'refs/heads/main' });
  assert.equal(result.status, 0, result.stderr);
  assert.equal(result.output, 'dir=demo\n');
});

test('demo browser hosts are generated at build time', t => {
  const context = { needs: { 'check-environment': { outputs: { environment: 'demo' } } } };
  // GitHub permits hyphens in property names; JavaScript bracket syntax is equivalent here.
  for (const [name, expected] of [
    ['Determine API URL', 'url=https://api.demo.moto-app.de'],
    ['Determine operator hostname', 'hostname=operator.demo.moto-app.de'],
    ['Determine parents hostname', 'hostname=eltern.demo.moto-app.de'],
    ['Determine school hostname', 'hostname=schule.demo.moto-app.de'],
    ['Determine tenant domain', 'domain=demo.moto-app.de'],
  ]) {
    const script = interpolate(stepRun(build, name).replaceAll('needs.check-environment', 'needs["check-environment"]'), context);
    const result = runStep(t, script);
    assert.equal(result.status, 0, result.stderr);
    assert.equal(result.output.trim(), expected);
  }
  const metadata = build.split('      - name: Extract metadata for frontend\n')[1]
    .split('      - name: Determine API URL')[0];
  const tags = interpolate(metadata.replaceAll('needs.check-environment', 'needs["check-environment"]'),
    { ...context, env: { FRONTEND_IMAGE: 'frontend' } });
  assert.match(tags, /type=sha,prefix=demo-,format=short/);
  assert.match(tags, /type=ref,event=branch,enable=false/);
  assert.match(tags, /type=raw,value=latest,enable=false/);
  assert.match(build, /NEXT_PUBLIC_APP_ENV=\$\{\{ needs.check-environment.outputs.environment \}\}/);
});

// Staging and production share a host but not a concurrency group; demo has its
// own VM with the same layout. Bash reads a script while it runs, so one
// environment's copy step must never overwrite a script another environment's
// release is executing (#3482).
test('each environment copies and runs its release scripts in its own directory', () => {
  for (const [workflow, step, directory] of [
    [build, 'Deploy to staging', 'staging'], [build, 'Deploy to production', 'production'],
    [build, 'Deploy to demo', 'demo'], [rollback, 'Rollback', '$DEPLOY_DIR'],
  ]) {
    const paths = stepRun(workflow, step).match(/~\/scripts[^\s"]*/g);
    assert.ok(paths.length >= 3, step);
    for (const path of paths) assert.ok(`${path}/`.startsWith(`~/scripts/${directory}/`), `${step}: ${path}`);
  }
});

test('demo deploy ships one Compose file that carries the demo runtime', () => {
  const script = stepRun(build, 'Deploy to demo');
  assert.match(script, /environments\/demo\.compose\.yml root@\$SSH_HOST:~\/demo\/docker-compose\.yml\.new/);
  const stack = readFileSync(new URL('../environments/demo.compose.yml', import.meta.url), 'utf8');
  assert.match(stack, /^  demo-runtime:$/m);
  for (const target of ['staging', 'production']) {
    const other = readFileSync(new URL(`../environments/${target}.compose.yml`, import.meta.url), 'utf8');
    assert.doesNotMatch(other, /demo-runtime/);
  }
});
