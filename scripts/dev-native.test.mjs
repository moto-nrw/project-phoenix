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
  for (const command of ['bash', 'mkdir', 'rm', 'mv', 'cat', 'tee', 'sed', 'pgrep', 'ps', 'tail', 'awk', 'head', 'tr', 'sleep']) {
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
    PATH: bin, FIXTURE_ROOT: root, FIXTURE_CALLS: calls, FIXTURE_OCCUPIED_PORT: occupiedPort, FIXTURE_NODE: process.execPath, ...env,
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
  const resetCache = (shell = '/bin/bash') => new Promise(resolve => {
    const child = spawn(shell, ['-c', '"$BASH" scripts/dev-native.sh down && rm -rf -- frontend/.next && printf restarted > restarted'], {
      cwd: root, env: environment,
    });
    let output = '';
    child.stdout.on('data', data => { output += data; });
    child.stderr.on('data', data => { output += data; });
    child.on('close', (code, signal) => resolve({ code, signal, output }));
  });
  return { root, bin, run, start, lines, tool, resetCache };
}

async function stop(runner) {
  runner.child.kill('SIGTERM');
  const result = await runner.closed;
  assert.equal(result.code, 130, result.output);
  return result;
}

// The launcher exits on TERM, but its child can keep writing during shutdown.
// Use real processes and a private cache; no application or network is involved.
function delayedFrontend(f, { ignoreTerm = false, launcherExits = false } = {}) {
  mkdirSync(join(f.root, 'frontend/.next'));
  writeFileSync(join(f.root, 'frontend/.next/original'), 'keep on timeout');
  writeFileSync(join(f.root, 'dummy.mjs'), `
import { spawn } from 'node:child_process';
spawn(process.execPath, ['-e', ${JSON.stringify(`
const { writeFileSync } = require('node:fs');
writeFileSync(process.env.FIXTURE_ROOT + '/child.pid', String(process.pid));
process.on('SIGUSR1', () => process.exit(0));
process.on('SIGTERM', () => {
  ${ignoreTerm ? '' : `setTimeout(() => {
    writeFileSync(process.env.FIXTURE_ROOT + '/frontend/.next/late-write', 'shutdown');
    writeFileSync(process.env.FIXTURE_ROOT + '/child-ended', 'done');
    process.exit(0);
  }, 400);`}
});
setInterval(() => {}, 1000);
console.log('fixture-ready:node');
`)}], { stdio: 'inherit' });
${ignoreTerm && !launcherExits ? "process.on('SIGTERM', () => {});" : ''}
`);
  f.tool('node', 'exec "$FIXTURE_NODE" "$FIXTURE_ROOT/dummy.mjs"');
}

for (const shell of ['/bin/bash', 'bash']) {
  test(`down waits for orphaned children before cache deletion and restart (${shell})`, { timeout: 10000 }, async t => {
    const f = fixture(t, { tools: ['node'] });
    delayedFrontend(f);
    const runner = f.start(['up', 'frontend'], ['node']);
    await runner.ready;
    const result = await f.resetCache(shell === 'bash' ? join(f.bin, 'bash') : shell);
    assert.equal(result.code, 0, result.output);
    assert.equal(existsSync(join(f.root, 'child-ended')), true, 'down must wait for the child, not just the launcher');
    assert.equal(existsSync(join(f.root, 'frontend/.next')), false);
    assert.equal(readFileSync(join(f.root, 'restarted'), 'utf8'), 'restarted');
    assert.equal(existsSync(join(f.root, 'tmp/dev-native/frontend.pid')), false);
    await runner.closed;
  });
}

test('down timeout preserves the cache and PID file and prevents restart', { timeout: 20000 }, async t => {
  const f = fixture(t, { tools: ['node'] });
  delayedFrontend(f, { ignoreTerm: true });
  const runner = f.start(['up', 'frontend'], ['node']);
  await runner.ready;
  const result = await f.resetCache();
  assert.equal(result.code, 1, result.output);
  assert.match(result.output, /timed out.*stop/i);
  assert.equal(readFileSync(join(f.root, 'frontend/.next/original'), 'utf8'), 'keep on timeout');
  assert.equal(existsSync(join(f.root, 'restarted')), false);
  assert.equal(existsSync(join(f.root, 'tmp/dev-native/frontend.pid')), true);
});

test('down retries keep tracking a surviving child after its launcher and supervisor exit', { timeout: 35000 }, async t => {
  const f = fixture(t, { tools: ['node'] });
  delayedFrontend(f, { ignoreTerm: true, launcherExits: true });
  const runner = f.start(['up', 'frontend'], ['node']);
  await runner.ready;
  for (let attempt = 0; attempt < 2; attempt++) {
    const result = await f.resetCache(attempt === 0 ? '/bin/bash' : join(f.bin, 'bash'));
    assert.equal(result.code, 1, `attempt ${attempt + 1}: ${result.output}`);
    assert.match(result.output, /timed out.*stop/i);
    assert.equal(readFileSync(join(f.root, 'frontend/.next/original'), 'utf8'), 'keep on timeout');
    assert.equal(existsSync(join(f.root, 'restarted')), false);
    assert.equal(existsSync(join(f.root, 'tmp/dev-native/stopping-pids')), true);
  }
  assert.equal(existsSync(join(f.root, 'tmp/dev-native/frontend.pid')), false, 'the supervisor removed the launcher PID');
  process.kill(Number(readFileSync(join(f.root, 'child.pid'), 'utf8')), 'SIGUSR1');
  await runner.closed;
  const result = await f.resetCache();
  assert.equal(result.code, 0, result.output);
  assert.equal(existsSync(join(f.root, 'frontend/.next')), false);
  assert.equal(readFileSync(join(f.root, 'restarted'), 'utf8'), 'restarted');
  assert.equal(existsSync(join(f.root, 'tmp/dev-native/stopping-pids')), false);
});

test('down ignores a persisted PID whose start time no longer matches', async t => {
  const f = fixture(t, { tools: [] });
  const unrelated = spawn(process.execPath, ['-e', 'setInterval(() => {}, 1000)'], { stdio: 'ignore' });
  const closed = new Promise(resolve => unrelated.on('close', resolve));
  t.after(async () => { unrelated.kill('SIGKILL'); await closed; });
  const state = join(f.root, 'tmp/dev-native');
  mkdirSync(state, { recursive: true });
  writeFileSync(join(state, 'stopping-pids'), `${unrelated.pid} Thu Jan 1 00:00:00 1970\n`);
  const result = f.run('down');
  assert.equal(result.status, 0, result.stderr);
  await new Promise(resolve => setTimeout(resolve, 100));
  assert.equal(unrelated.signalCode, null, 'an unrelated process must not be terminated');
  assert.doesNotThrow(() => process.kill(unrelated.pid, 0), 'an unrelated process must not be signaled');
  assert.equal(existsSync(join(state, 'stopping-pids')), false);
});

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

test('a shell-provided native DB_DSN bypasses the Compose host rewrite', t => {
  const dbDsn = 'postgres://shell@localhost:5432/custom';
  const f = fixture(t, { tools: [], env: { DB_DSN: dbDsn } });
  const result = f.run('backend', 'bash', '-c', 'printf "%s\\n" "$DB_DSN"');
  assert.equal(result.status, 0, result.stderr);
  assert.equal(result.stdout.trim(), dbDsn);
});

// Devbox puts a Bash without programmable completion first on PATH, so the
// script must not rely on the `compgen` builtin to see shell-provided keys.
test('shell-provided keys win over .env in a Bash without compgen', t => {
  const dir = mkdtempSync(join(tmpdir(), 'moto-dev-native-bashenv-'));
  t.after(() => rmSync(dir, { recursive: true, force: true }));
  const bashEnv = join(dir, 'no-compgen.sh');
  writeFileSync(bashEnv, 'enable -n compgen\n');
  const dbDsn = 'postgres://shell@localhost:5432/custom';
  const f = fixture(t, { tools: [], env: { DB_DSN: dbDsn, SERVER_HOST_PORT: '19090', BASH_ENV: bashEnv } });
  const result = f.run('backend', 'bash', '-c', 'printf "%s\\n" "$DB_DSN" "$SERVER_HOST_PORT"');
  assert.equal(result.status, 0, result.stderr);
  assert.deepEqual(result.stdout.trim().split('\n'), [dbDsn, '19090']);
  assert.doesNotMatch(result.stderr, /compgen/);
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
