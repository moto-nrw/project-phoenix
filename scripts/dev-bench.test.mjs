import assert from 'node:assert/strict';
import { spawn, spawnSync } from 'node:child_process';
import { existsSync, mkdirSync, mkdtempSync, readFileSync, readdirSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import test from 'node:test';

const script = new URL('./dev-bench.sh', import.meta.url).pathname;

// A throwaway Git checkout whose test runners are fixtures, so no test runs
// Go, pnpm or a real test suite. FIXTURE_CALLS records what the runners saw.
function fixture(t) {
  const root = mkdtempSync(join(tmpdir(), 'moto-dev-bench-'));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  for (const directory of ['backend/leaf', 'frontend/src/lib', 'scripts', 'bin']) {
    mkdirSync(join(root, directory), { recursive: true });
  }
  const write = (path, body, mode = 0o644) => writeFileSync(join(root, path), body, { mode });
  write('.gitignore', 'calls\ntmp/\npnpm-started\n');
  write('frontend/package.json', '{}');
  write('frontend/src/lib/probe-target.ts', 'export const value = 1;\n');
  write('backend/leaf/leaf.go', 'package leaf\n');
  write('backend/leaf/leaf_test.go', 'package leaf_test\n');
  write('scripts/total-memory-gb.sh', '#!/bin/sh\necho 16\n', 0o755);
  write('scripts/go-package-workers.sh', '#!/bin/sh\necho 2\n', 0o755);
  write('scripts/run-go-toolchain.sh', '#!/bin/sh\nexec "$@"\n', 0o755);
  write('scripts/test-changed.sh', `#!/bin/bash
probe=$(ls backend/leaf/zz_dev_bench_probe.go 2>/dev/null || echo none)
printf 'test-changed|%s|%s\\n' "$*" "$probe" >> "$FIXTURE_CALLS"
echo '==> go test (1 affected packages; -p 2, -parallel 8)'
echo 'ok  	example/leaf	0.1s'
exit "\${FIXTURE_TEST_STATUS:-0}"
`, 0o755);
  write('bin/pnpm', `#!/bin/bash
[ "$1" = --version ] && { echo 10.0.0; exit 0; }
printf 'pnpm|%s|%s\\n' "$*" "$(tail -1 src/lib/probe-target.ts)" >> "$FIXTURE_CALLS"
[ -n "\${FIXTURE_PNPM_SLEEP:-}" ] && { touch "$FIXTURE_ROOT/pnpm-started"; sleep "$FIXTURE_PNPM_SLEEP"; }
exit 0
`, 0o755);
  const git = (...args) => spawnSync('git', ['-C', root, ...args], { encoding: 'utf8' });
  git('init', '-q');
  git('add', '.');
  git('-c', 'user.email=fixture@example.invalid', '-c', 'user.name=fixture', 'commit', '-qm', 'fixture');
  const calls = join(root, 'calls');
  const env = {
    ...process.env,
    PATH: `${join(root, 'bin')}:${process.env.PATH}`,
    FIXTURE_ROOT: root,
    FIXTURE_CALLS: calls,
    DEV_BENCH_GO_LEAF: 'leaf',
    DEV_BENCH_FE_FILE: 'src/lib/probe-target.ts',
  };
  const run = (args, extra = {}) => spawnSync('/bin/bash', [script, '--root', root, ...args], {
    cwd: root, env: { ...env, ...extra }, encoding: 'utf8', timeout: 60_000,
  });
  const lines = () => (existsSync(calls) ? readFileSync(calls, 'utf8').trim().split('\n') : []);
  const status = () => git('status', '--porcelain').stdout.trim();
  const results = () => {
    const dir = join(root, 'tmp/dev-bench');
    const tsv = readdirSync(dir).filter(name => name.endsWith('.tsv')).sort().at(-1);
    return readFileSync(join(dir, tsv), 'utf8');
  };
  return { root, env, run, lines, status, results };
}

test('a failing measured command fails the run and marks its row', t => {
  const f = fixture(t);
  const result = f.run(['--', 'broken', 'false']);
  assert.equal(result.status, 1, result.stderr);
  assert.match(f.results(), /^broken\t.*\tFAILED\(1\)\t/m);
  assert.match(result.stderr, /1 measurement\(s\) failed/);
});

test('a passing measured command records an ok row and exits 0', t => {
  const f = fixture(t);
  const result = f.run(['--', 'fine', 'true']);
  assert.equal(result.status, 0, result.stderr);
  assert.match(f.results(), /^fine\t[0-9.]+\t0\.0\t[0-9.]+\t\d+\t-?\d+\t-\tok\t/m);
});

test('the Go probe exists for both runs and is removed afterwards', t => {
  const f = fixture(t);
  const result = f.run(['go-leaf']);
  assert.equal(result.status, 0, result.stderr);
  const probe = 'backend/leaf/zz_dev_bench_probe.go';
  assert.deepEqual(f.lines(), [`test-changed|--fast HEAD|${probe}`, `test-changed|--fast HEAD|${probe}`]);
  assert.match(f.results(), /^go-leaf-change\t.*go pkgs 1, ok 1 \(cached 0\)/m);
  assert.equal(f.status(), '');
});

test('a failing Go test run fails the benchmark and still removes the probe', t => {
  const f = fixture(t);
  const result = f.run(['go-leaf'], { FIXTURE_TEST_STATUS: '1' });
  assert.equal(result.status, 1);
  assert.equal(f.status(), '');
});

test('change stages refuse a dirty checkout instead of measuring a different selection', t => {
  const f = fixture(t);
  writeFileSync(join(f.root, 'unrelated.txt'), 'local work\n');
  const result = f.run(['go-leaf']);
  assert.equal(result.status, 1);
  assert.match(f.results(), /^go-leaf\t.*uncommitted changes would change the test selection/m);
  assert.deepEqual(f.lines(), []);
  assert.equal(existsSync(join(f.root, 'backend/leaf/zz_dev_bench_probe.go')), false);
});

test('the frontend probe is visible to the runners and restored byte for byte', t => {
  const f = fixture(t);
  const result = f.run(['fe-change']);
  assert.equal(result.status, 0, result.stderr);
  const checks = f.lines().filter(line => line.startsWith('pnpm|run check|'));
  assert.equal(checks.length, 2);
  for (const line of checks) assert.match(line, /dev-bench probe \d+$/);
  assert.equal(readFileSync(join(f.root, 'frontend/src/lib/probe-target.ts'), 'utf8'), 'export const value = 1;\n');
  assert.equal(f.status(), '');
});

test('a locally modified probe target is refused and left untouched', t => {
  const f = fixture(t);
  const target = join(f.root, 'frontend/src/lib/probe-target.ts');
  writeFileSync(target, 'export const value = 2;\n');
  const result = f.run(['fe-change']);
  assert.equal(result.status, 1);
  assert.match(f.results(), /has local changes; not probing it/);
  assert.equal(readFileSync(target, 'utf8'), 'export const value = 2;\n');
  assert.deepEqual(f.lines(), []);
});

test('an interrupted run restores the probed frontend file', async t => {
  const f = fixture(t);
  const child = spawn('/bin/bash', [script, '--root', f.root, 'fe-change'], {
    cwd: f.root, env: { ...f.env, FIXTURE_PNPM_SLEEP: '30' }, stdio: 'ignore',
  });
  const exited = new Promise(resolve => child.on('exit', (code, signal) => resolve({ code, signal })));
  const started = join(f.root, 'pnpm-started');
  for (let i = 0; i < 400 && !existsSync(started); i++) await new Promise(r => setTimeout(r, 50));
  assert.ok(existsSync(started), 'pnpm fixture never started');
  child.kill('SIGTERM');
  const { code } = await exited;
  assert.equal(code, 130);
  assert.equal(readFileSync(join(f.root, 'frontend/src/lib/probe-target.ts'), 'utf8'), 'export const value = 1;\n');
});
