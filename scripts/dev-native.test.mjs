import assert from 'node:assert/strict';
import { spawn, spawnSync } from 'node:child_process';
import { copyFileSync, existsSync, mkdirSync, mkdtempSync, readFileSync, rmSync, symlinkSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import test from 'node:test';

const script = new URL('./dev-native.sh', import.meta.url).pathname;

// Only these host utilities are visible. Docker, Git, Go, Air, Node and lsof
// below are fixtures, so no test can operate on an actual checkout or service.
function fixture(t, { tools = ['go', 'air', 'node'], env = {}, occupiedPort = '' } = {}) {
  const root = mkdtempSync(join(tmpdir(), 'moto-dev-native-'));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  const bin = join(root, 'bin');
  mkdirSync(bin);
  for (const command of ['bash', 'mkdir', 'rm', 'cat', 'tee', 'sed', 'pgrep', 'tail', 'awk', 'head', 'tr', 'sleep']) {
    const host = process.env.PATH.split(':').map(directory => join(directory, command)).find(existsSync);
    assert.ok(host, `Missing host utility ${command}`);
    symlinkSync(host, join(bin, command));
  }
  for (const directory of ['backend', 'frontend']) mkdirSync(join(root, directory));
  writeFileSync(join(root, 'backend/.air.toml'), '');
  writeFileSync(join(root, 'frontend/package.json'), '{}');
  mkdirSync(join(root, 'scripts'));
  copyFileSync(script, join(root, 'scripts/dev-native.sh'));
  writeFileSync(join(root, '.env'), [
    'SERVER_HOST_PORT=18080', 'FRONTEND_HOST_PORT=13000', 'POSTGRES_HOST_PORT=15432',
    'MAILPIT_HOST_PORT=18025', 'MAILPIT_SMTP_HOST_PORT=11025', 'DB_DSN=postgres://fixture@postgres:5432/fixture',
  ].join('\n'));
  const calls = join(root, 'calls');
  const environment = {
    PATH: bin, FIXTURE_ROOT: root, FIXTURE_CALLS: calls, FIXTURE_OCCUPIED_PORT: occupiedPort, ...env,
  };
  const tool = (name, body) => writeFileSync(join(bin, name), `#!/bin/bash\n${body}\n`, { mode: 0o755 });
  const log = name => `printf '${name}|%s|GOFLAGS=%s|GOMAXPROCS=%s|PORT=%s|API_URL=%s|DB_DSN=%s\\n' "$*" "\${GOFLAGS-<unset>}" "\${GOMAXPROCS-<unset>}" "\${PORT-}" "\${API_URL-}" "\${DB_DSN-}" >> "$FIXTURE_CALLS"`;
  tool('git', 'printf "%s\\n" "$FIXTURE_ROOT"');
  tool('docker', log('docker'));
  tool('lsof', `${log('lsof')}\n[[ "$*" == *"-iTCP:$FIXTURE_OCCUPIED_PORT "* ]] && [ -n "$FIXTURE_OCCUPIED_PORT" ]`);
  if (tools.includes('go')) tool('go', `${log('go')}
if [ "$1" = build ]; then
  mkdir -p tmp
  cat >tmp/main <<'MAIN'
#!/bin/bash
printf 'migrate|%s\\n' "$*" >> "$FIXTURE_CALLS"
MAIN
  /bin/chmod +x tmp/main
fi`);
  for (const name of ['air', 'node']) {
    if (tools.includes(name)) tool(name, `${log(name)}
echo fixture-ready:${name}
if [ "\${FIXTURE_EXIT_SERVICE-}" = ${name} ]; then exit 9; fi
exec sleep 30`);
  }
  const lines = () => existsSync(calls) ? readFileSync(calls, 'utf8').trim().split('\n') : [];
  const run = (...args) => spawnSync('/bin/bash', [join(root, 'scripts/dev-native.sh'), ...args], {
    cwd: root, env: environment, encoding: 'utf8', timeout: 5000,
  });
  const start = (args = ['up'], readyTools = ['air', 'node']) => {
    const child = spawn('/bin/bash', [join(root, 'scripts/dev-native.sh'), ...args], {
      cwd: root, env: environment, detached: true,
    });
    t.after(() => {
      // The process group is created by this fixture only, never by the user.
      try { process.kill(-child.pid, 'SIGKILL'); } catch (error) { if (error.code !== 'ESRCH') throw error; }
    });
    let output = '';
    let resolveReady;
    let rejectReady;
    const ready = new Promise((resolve, reject) => { resolveReady = resolve; rejectReady = reject; });
    const timer = setTimeout(() => rejectReady(new Error(`Fixture startup timed out: ${output}`)), 5000);
    const collect = data => {
      output += data;
      if (output.includes('dev-native: logs in') && readyTools.every(name => output.includes(`fixture-ready:${name}`))) {
        clearTimeout(timer);
        resolveReady();
      }
    };
    child.stdout.on('data', collect);
    child.stderr.on('data', collect);
    const closed = new Promise(resolve => child.on('close', (code, signal) => {
      clearTimeout(timer);
      rejectReady(new Error(`Fixture exited before startup: ${output}`));
      resolve({ code, signal, output });
    }));
    return { child, ready, closed, output: () => output };
  };
  return { root, run, start, lines };
}

async function stop(runner) {
  runner.child.kill('SIGTERM');
  const result = await runner.closed;
  assert.equal(result.code, 130, result.output);
  return result;
}

test('default up starts both services, bounds Go children and cleans up on termination', { timeout: 10000 }, async t => {
  const f = fixture(t);
  const runner = f.start();
  await runner.ready;
  const calls = f.lines();
  assert.ok(calls.some(line => line.startsWith('docker|compose up -d --wait postgres mailpit|')));
  assert.ok(calls.some(line => line.startsWith('go|build ') && line.includes('|GOFLAGS=-p=4|GOMAXPROCS=4|PORT=18080|API_URL=http://localhost:18080|DB_DSN=postgres://fixture@localhost:15432/fixture')));
  assert.ok(calls.some(line => line.startsWith('air|') && line.includes('|GOFLAGS=-p=4|GOMAXPROCS=4|')));
  assert.ok(calls.some(line => line.startsWith('migrate|migrate')));
  assert.ok(calls.some(line => line.startsWith('node|') && line.includes('|GOFLAGS=<unset>|GOMAXPROCS=<unset>|PORT=13000|')));
  const result = await stop(runner);
  assert.match(result.output, /stopped backend/);
  assert.match(result.output, /stopped frontend/);
  for (const name of ['backend', 'frontend']) assert.equal(existsSync(join(f.root, 'tmp/dev-native', `${name}.pid`)), false);
  assert.equal(f.lines().filter(line => line.startsWith('docker|compose stop ')).length, 1, 'termination leaves shared infrastructure running');
});

test('backend-only needs no Node or pnpm and ignores the occupied frontend port', { timeout: 10000 }, async t => {
  const f = fixture(t, { tools: ['go', 'air'], occupiedPort: '13000' });
  const runner = f.start(['up', 'backend'], ['air']);
  await runner.ready;
  assert.ok(f.lines().some(line => line.startsWith('migrate|')));
  assert.equal(f.lines().some(line => line.startsWith('node|')), false);
  assert.equal(f.lines().some(line => line.startsWith('lsof|') && line.includes('-iTCP:13000 ')), false);
  assert.equal(existsSync(join(f.root, 'tmp/dev-native/frontend.pid')), false);
  await stop(runner);
});

test('frontend-only needs no Go or Air, skips migrations and ignores the backend port', { timeout: 10000 }, async t => {
  const f = fixture(t, { tools: ['node'], occupiedPort: '18080' });
  const runner = f.start(['up', 'frontend'], ['node']);
  await runner.ready;
  assert.equal(f.lines().some(line => /^(go|air|migrate)\|/.test(line)), false);
  assert.equal(f.lines().some(line => line.startsWith('lsof|') && line.includes('-iTCP:18080 ')), false);
  assert.equal(existsSync(join(f.root, 'tmp/dev-native/backend.pid')), false);
  await stop(runner);
});

test('backend CLI and startup preserve explicit Go overrides, including empty GOFLAGS', { timeout: 10000 }, async t => {
  const f = fixture(t, { tools: ['go', 'air'], env: { GOFLAGS: '', GOMAXPROCS: '0' } });
  const cli = f.run('backend', 'go', 'env');
  assert.equal(cli.status, 0, cli.stderr);
  const runner = f.start(['up', 'backend'], ['air']);
  await runner.ready;
  const goCalls = f.lines().filter(line => /^(go|air)\|/.test(line));
  assert.equal(goCalls.length, 3);
  for (const line of goCalls) assert.match(line, /\|GOFLAGS=\|GOMAXPROCS=0\|/);
  await stop(runner);
});

test('invalid service and excess arguments fail before operating on services', t => {
  for (const args of [['up', 'other'], ['up', 'backend', 'frontend']]) {
    const f = fixture(t, { tools: [] });
    const result = f.run(...args);
    assert.equal(result.status, 1);
    assert.match(result.stderr, /unknown service 'other'|usage: scripts\/dev-native.sh up/);
    assert.deepEqual(f.lines(), []);
  }
});

test('help explains replacement semantics and the shared infrastructure', t => {
  const f = fixture(t, { tools: [] });
  const result = f.run('--help');
  assert.equal(result.status, 0, result.stderr);
  assert.match(result.stdout, /up \[backend\|frontend\]/);
  assert.match(result.stdout, /Every `up` replaces both managed native apps/);
  assert.match(result.stdout, /frontend-only skips Go and migrations/);
  assert.deepEqual(f.lines(), []);
});

test('a failed selected child stops its sibling instead of leaving a dev process behind', { timeout: 10000 }, async t => {
  const f = fixture(t, { env: { FIXTURE_EXIT_SERVICE: 'node' } });
  const runner = f.start();
  await runner.ready;
  const result = await runner.closed;
  assert.equal(result.code, 1, result.output);
  assert.match(result.output, /frontend exited, stopping the rest/);
  assert.match(result.output, /stopped backend/);
  for (const name of ['backend', 'frontend']) assert.equal(existsSync(join(f.root, 'tmp/dev-native', `${name}.pid`)), false);
});

test('switching from both apps to backend-only cannot let the old supervisor stop the replacement', { timeout: 15000 }, async t => {
  const f = fixture(t);
  const original = f.start();
  await original.ready;
  const replacement = f.start(['up', 'backend'], ['air']);
  await replacement.ready;
  await original.closed;
  assert.equal(replacement.child.exitCode, null, replacement.output());
  const pid = Number(readFileSync(join(f.root, 'tmp/dev-native/backend.pid'), 'utf8'));
  assert.doesNotThrow(() => process.kill(pid, 0));
  assert.equal(existsSync(join(f.root, 'tmp/dev-native/frontend.pid')), false);
  await stop(replacement);
});

test('.env references resolve like Compose before the host override', t => {
  const f = fixture(t, { tools: [], env: { POSTGRES_PASSWORD: 'from-shell' } });
  writeFileSync(join(f.root, '.env'), [
    'SERVER_HOST_PORT=18080', 'FRONTEND_HOST_PORT=13000', 'POSTGRES_HOST_PORT=15432',
    'MAILPIT_HOST_PORT=18025', 'MAILPIT_SMTP_HOST_PORT=11025', 'POSTGRES_PASSWORD=from-file',
    'DB_DSN=postgres://postgres:${POSTGRES_PASSWORD}@postgres:5432/postgres?sslmode=require',
    'TEST_DB_DSN="postgres://postgres:${POSTGRES_PASSWORD}@localhost:5433/phoenix_test"',
    "LITERAL='${POSTGRES_PASSWORD}'", 'MIXED=a$$b${POSTGRES_PASSWORD}}${SERVER_HOST_PORT}',
    'UNBRACED=$POSTGRES_PASSWORD:$SERVER_HOST_PORT',
    'DOUBLE_UNBRACED="$POSTGRES_PASSWORD"',
    'ESCAPED=$${POSTGRES_PASSWORD}$$',
  ].join('\n'));
  const result = f.run('backend', 'bash', '-c', 'printf "%s\\n" "$DB_DSN" "$TEST_DB_DSN" "$LITERAL" "$MIXED" "$UNBRACED" "$DOUBLE_UNBRACED" "$ESCAPED"');
  assert.equal(result.status, 0, result.stderr);
  assert.deepEqual(result.stdout.trim().split('\n'), [
    'postgres://postgres:from-shell@localhost:15432/postgres?sslmode=require',
    'postgres://postgres:from-shell@localhost:5433/phoenix_test',
    '${POSTGRES_PASSWORD}',
    'a$bfrom-shell}18080',
    'from-shell:18080',
    'from-shell',
    '${POSTGRES_PASSWORD}$',
  ]);
});

test('unresolvable .env references stop before any command runs', t => {
  for (const [value, message] of [
    ['postgres://postgres:${MISSING_PASSWORD}@postgres:5432/postgres', /DB_DSN in .env references \$\{MISSING_PASSWORD\}, which is not set/],
    ['postgres://postgres:$MISSING_PASSWORD@postgres:5432/postgres', /DB_DSN in .env references \$MISSING_PASSWORD, which is not set/],
    ['postgres://postgres:${POSTGRES_PASSWORD:-x}@postgres:5432/postgres', /DB_DSN in .env uses an unsupported \$\{\.\.\.\} form/],
  ]) {
    const f = fixture(t, { tools: [] });
    writeFileSync(join(f.root, '.env'), [
      'SERVER_HOST_PORT=18080', 'FRONTEND_HOST_PORT=13000', 'POSTGRES_HOST_PORT=15432',
      'MAILPIT_HOST_PORT=18025', 'MAILPIT_SMTP_HOST_PORT=11025', `DB_DSN=${value}`,
    ].join('\n'));
    for (const args of [['env'], ['backend', 'bash', '-c', 'echo ran']]) {
      const result = f.run(...args);
      assert.equal(result.status, 1, result.stdout);
      assert.match(result.stderr, message);
      assert.doesNotMatch(result.stdout, /ran/);
    }
    assert.deepEqual(f.lines(), []);
  }
});
