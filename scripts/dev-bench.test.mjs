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
# The value must reach the test binary: Go caches test results by its content.
[ "$probe" = none ] || grep -q '^func init() { devBenchProbe = [0-9]* }$' "$probe" || probe=probe-not-linked
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

test('an explicit output directory isolates logs and measurements', t => {
  const f = fixture(t);
  const output = join(f.root, 'tmp', 'paired trial');
  const result = f.run(['--out', output, '--', 'fine', 'true']);
  assert.equal(result.status, 0, result.stderr);
  assert.equal(readdirSync(output).filter(name => name.endsWith('.tsv')).length, 1);
  assert.equal(readdirSync(output).filter(name => name.endsWith('.meta')).length, 1);
  assert.equal(readdirSync(output).filter(name => name.endsWith('.log')).length, 1);
  assert.equal(existsSync(join(f.root, 'tmp/dev-bench')), false);
});

test('light admission wraps only the measured command and preserves its status', t => {
  const f = fixture(t);
  const gate = join(f.root, 'bin/gate');
  writeFileSync(gate, '#!/bin/sh\nprintf "%s\\n" "$1" > "$FIXTURE_ROOT/admission"\n[ "$1" = --light ] && shift\n[ "$1" = -- ] && shift\nexec "$@"\n', { mode: 0o755 });
  const result = f.run(['--light', '--', 'broken', 'false'], { AGENT_HEAVY: gate });
  assert.equal(result.status, 1, result.stderr);
  assert.equal(readFileSync(join(f.root, 'admission'), 'utf8'), '--light\n');
  assert.match(f.results(), /FAILED\(1\)/);
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

for (const admission of ['ungated', 'admitted', 'queued', 'remove-error', 'copy-error']) {
  test(`SIGTERM removes owned measurement temporaries (${admission})`, { timeout: 20_000 }, async t => {
    const f = fixture(t);
    const temp = join(f.root, 'tmp', 'owned-temp');
    const output = join(f.root, 'tmp', 'abort-output');
    mkdirSync(temp, { recursive: true });
    const allocations = join(f.root, 'allocations');
    t.after(() => {
      if (existsSync(allocations)) {
        for (const path of readFileSync(allocations, 'utf8').trim().split('\n')) rmSync(path, { force: true });
      }
    });
    const started = join(f.root, 'dummy-pid');
    // Record actual mktemp return values, never infer ownership from names/age.
    writeFileSync(join(f.root, 'bin/mktemp'), `#!/bin/bash
path=$(/usr/bin/mktemp "$TMPDIR/measure.XXXXXXXX") || exit $?
printf '%s\\n' "$path" >> "$FIXTURE_ROOT/allocations"
printf '%s\\n' "$path"
`, { mode: 0o755 });
    const gate = join(f.root, 'bin/abort-gate');
    writeFileSync(gate, `#!/bin/bash
[ "$1" = --status ] && { echo fixture; exit; }
shift
${admission === 'queued' ? 'echo $$ > "$FIXTURE_ROOT/dummy-pid"; exec sleep 30' : 'exec "$@"'}
`, { mode: 0o755 });
    if (admission === 'remove-error' || admission === 'copy-error') {
      const tool = admission === 'remove-error' ? 'rm' : 'cp';
      writeFileSync(join(f.root, 'bin', tool), `#!/bin/bash
while IFS= read -r owned; do
  for arg in "$@"; do [ "$arg" != "$owned" ] || exit 74; done
done < "$FIXTURE_ROOT/allocations"
exec /bin/${tool} "$@"
`, { mode: 0o755 });
    }
    const dummy = join(f.root, 'dummy.mjs');
    writeFileSync(dummy, `import {writeFileSync} from 'node:fs';
writeFileSync(process.env.FIXTURE_ROOT + '/dummy-pid', String(process.pid));
setTimeout(() => {}, 30_000);
`);
    const child = spawn('/bin/bash', [script, '--root', f.root, '--out', output,
      ...(admission === 'ungated' ? [] : ['--heavy']), '--', 'abort', process.execPath, dummy], {
      cwd: f.root, env: { ...f.env, TMPDIR: temp, AGENT_HEAVY: gate }, stdio: ['ignore', 'pipe', 'pipe'],
    });
    let stderr = '';
    child.stderr.on('data', chunk => { stderr += chunk; });
    child.stdout.resume();
    const exited = new Promise(resolve => child.on('exit', (code, signal) => resolve({ code, signal })));
    t.after(async () => {
      if (child.exitCode === null && child.signalCode === null) child.kill('SIGTERM');
      await exited;
    });
    for (let i = 0; i < 200 && !existsSync(started); i++) await new Promise(r => setTimeout(r, 25));
    assert.ok(existsSync(started), `bounded dummy never started: ${stderr}`);
    const pid = Number(readFileSync(started, 'utf8'));
    const paths = readFileSync(allocations, 'utf8').trim().split('\n');
    assert.equal(paths.length, 2);
    assert.notEqual(paths[0], paths[1]);
    for (const path of paths) assert.ok(existsSync(path));
    const before = paths.map(path => readFileSync(path, 'utf8'));
    const metaPath = join(output, readdirSync(output).find(name => name.endsWith('.meta')));
    const beforeMeta = readFileSync(metaPath, 'utf8');
    child.kill('SIGTERM');
    const result = await exited;
    const remaining = paths.filter(existsSync);
    const rawFiles = readdirSync(output).filter(name => name.endsWith('.raw'));
    const evidence = { admission, meterPid: child.pid, dummyPid: pid, paths, before, result,
      remaining, beforeMeta, afterMeta: readFileSync(metaPath, 'utf8'), stderr,
      raw: Object.fromEntries(rawFiles.map(name => [name, readFileSync(join(output, name), 'utf8')])) };
    t.diagnostic(JSON.stringify(evidence));
    assert.deepEqual(result, { code: 130, signal: null }, stderr);
    // The real process tree must be gone, not merely the meter parent.
    for (let i = 0; i < 100; i++) {
      const ps = spawnSync('ps', ['-p', String(pid), '-o', 'stat='], { encoding: 'utf8' });
      if (ps.status !== 0 || ps.stdout.trim().startsWith('Z')) break;
      await new Promise(r => setTimeout(r, 20));
    }
    const ps = spawnSync('ps', ['-p', String(pid), '-o', 'stat='], { encoding: 'utf8' });
    assert.ok(ps.status !== 0 || ps.stdout.trim().startsWith('Z'), 'owned dummy still running');
    assert.deepEqual(remaining, admission === 'remove-error' ? paths : [],
      'meter must remove owned temporaries unless removal itself fails');
    if (admission === 'remove-error') assert.match(stderr, /could not remove owned (time|admission) file/);
    if (admission === 'copy-error') assert.match(stderr, /could not preserve (time|admission) raw file/);
    for (const [index, kind] of ['time', 'admission'].entries()) {
      assert.ok(beforeMeta.includes(`measure_${kind}_path=${paths[index]}\n`), 'ownership must precede dispatch');
      assert.ok(beforeMeta.includes(`measure_owner_pid=${child.pid}\n`));
      const raw = rawFiles.find(name => name.endsWith(`.${kind}.raw`));
      if (admission === 'copy-error') {
        assert.equal(raw, undefined, 'failed preservation must not invent raw contents');
      } else {
        assert.ok(raw, `missing ${kind} raw abort artifact`);
      }
      // Admission is either its exact timestamp or empty while queued, never fabricated.
      if (kind === 'admission' && admission !== 'copy-error') assert.equal(evidence.raw[raw], before[index]);
      assert.equal(evidence.afterMeta.includes(`measure_cleanup_removed=${paths[index]}\n`), admission !== 'remove-error');
    }
    const table = readFileSync(join(output, readdirSync(output).find(name => name.endsWith('.tsv'))), 'utf8');
    assert.equal(table.trim().split('\n').length, 1, 'abort must not synthesize a measurement row');
  });
}

test('allocation failure cleans the first owned temporary and preserves its exit', t => {
  const f = fixture(t);
  const temp = join(f.root, 'tmp', 'allocation-temp');
  mkdirSync(temp, { recursive: true });
  const owned = join(temp, 'first-path');
  writeFileSync(join(f.root, 'bin/mktemp'), `#!/bin/bash
[ ! -f "$FIXTURE_ROOT/allocated" ] || exit 73
path=$(/usr/bin/mktemp "$TMPDIR/measure.XXXXXXXX") || exit $?
printf '%s' "$path" > "$FIXTURE_ROOT/allocated"
printf '%s' "$path"
`, { mode: 0o755 });
  const result = f.run(['--', 'not-dispatched', 'touch', owned], { TMPDIR: temp });
  const path = readFileSync(join(f.root, 'allocated'), 'utf8');
  t.diagnostic(JSON.stringify({ result: { code: result.status, signal: result.signal }, path,
    remaining: existsSync(path), stderr: result.stderr }));
  assert.equal(result.status, 73, result.stderr);
  assert.equal(result.signal, null);
  assert.equal(existsSync(path), false);
  assert.equal(existsSync(owned), false, 'must not dispatch after an allocation failure');
  const output = join(f.root, 'tmp/dev-bench');
  const raw = readdirSync(output).filter(name => name.endsWith('.raw'));
  assert.equal(raw.length, 1, 'only the allocated time file can be preserved');
  assert.ok(raw[0].endsWith('.time.raw'));
  assert.equal(readFileSync(join(output, raw[0]), 'utf8'), '');
  assert.equal(f.results().trim().split('\n').length, 1);
});
