import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { mkdtempSync, mkdirSync, readFileSync, readdirSync, rmSync, statSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import test from 'node:test';
import { inventory, compare, goInventory, goInventoryFile } from './test-perf.mjs';
import { gitTestEnvironment, initGitFixture } from './test-helpers/git-fixture.mjs';

const frontend = '/repo/frontend';
function report() {
  return {
    success: true, numFailedTests: 0, numFailedTestSuites: 0,
    numPassedTests: 2, numTotalTests: 2,
    testResults: [{ name: `${frontend}/src/probe.test.ts`, status: 'passed', assertionResults: [
      { fullName: 'rejects wrong tenant', status: 'passed' },
      { fullName: 'rejects stale response', status: 'passed' },
    ] }],
  };
}

let trial = 0;
function sample(wall = 10) {
  return {
    schema: 1, id: `trial-${++trial}`, valid: true, profile: 'quick', evaluator: 'fixed', workload: 'fixed',
    context: { cacheState: 'warm', node: '24' }, state: { revision: 'a', diff: 'none' },
    inventory: inventory(report(), frontend),
    metrics: { wallSeconds: wall, cpuSeconds: 20, peakTreeMB: 500, queueSeconds: 0 },
  };
}

test('paired comparisons report medians, ranges and individual speedups', () => {
  const result = compare([sample(10), sample(14)], [sample(5), sample(7)]);
  assert.equal(result.speedup, 2);
  assert.deepEqual(result.pairedSpeedups, [2, 2]);
  assert.deepEqual(result.metrics.wallSeconds.baseline, { median: 12, min: 10, max: 14 });
});

test('renamed worktrees and collection order do not change test identities', () => {
  const baseline = inventory(report(), frontend);
  const candidate = report();
  candidate.testResults[0].name = '/other/frontend/src/probe.test.ts';
  candidate.testResults[0].assertionResults.reverse();
  assert.deepEqual(inventory(candidate, '/other/frontend'), baseline);
});

test('a green report with zero executed tests is not a baseline', () => {
  assert.throws(() => inventory({ ...report(), numPassedTests: 0 }, frontend), /No tests passed/);
});

test('failed collection and incomplete inventories are rejected', () => {
  const failed = report();
  failed.testResults[0].status = 'failed';
  assert.throws(() => inventory(failed, frontend), /Failed file/);
  assert.throws(() => inventory({ ...report(), numTotalTests: 3 }, frontend), /Incomplete/);
});

for (const change of ['skip', 'delete', 'rename', 'duplicate-project']) {
  test(`faster execution cannot win through ${change}`, () => {
    const baseline = sample();
    const candidate = sample(1);
    if (change === 'skip') candidate.inventory.entries[0][2] = 'skipped';
    if (change === 'delete') candidate.inventory.entries.pop();
    if (change === 'rename') candidate.inventory.entries[0][1] = 'unrelated assertion';
    if (change === 'duplicate-project') candidate.inventory.files.push(candidate.inventory.files[0]);
    assert.throws(() => compare([baseline], [candidate]), /Non-equivalent inventory/);
  });
}

for (const key of ['evaluator', 'workload', 'context', 'profile']) {
  test(`incompatible ${key} cannot produce a speedup claim`, () => {
    const candidate = sample(1);
    candidate[key] = key === 'context' ? { cacheState: 'warm', node: 'different' } : 'different';
    assert.throws(() => compare([sample()], [candidate]), /Non-equivalent/);
  });
}

test('failed, uncontrolled, unpaired and mixed-revision trials cannot be compared', () => {
  assert.throws(() => compare([sample()], [{ ...sample(), valid: false }]), /Invalid trial/);
  const uncontrolled = sample();
  uncontrolled.context.cacheState = 'uncontrolled';
  assert.throws(() => compare([uncontrolled], [sample()]), /Declare comparable/);
  assert.throws(() => compare([sample()], []), /paired sample/);
  assert.throws(() => compare([sample(), { ...sample(), state: { revision: 'b' } }], [sample(), sample()]), /Source state/);
});

test('reusing one trial cannot inflate paired sample counts', () => {
  const reused = sample();
  assert.throws(() => compare([reused], [reused]), /Reused trial/);
});

test('invalid resource data cannot produce a speedup claim', () => {
  const candidate = sample(1);
  delete candidate.metrics.cpuSeconds;
  assert.throws(() => compare([sample()], [candidate]), /Invalid metrics/);
});

test('a real measured command failure retains evidence and a failing exit', t => {
  const root = mkdtempSync(join(tmpdir(), 'moto-test-perf-'));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  for (const directory of ['frontend/node_modules/vitest', 'backend', 'bin']) mkdirSync(join(root, directory), { recursive: true });
  writeFileSync(join(root, '.gitignore'), 'tmp/\n');
  writeFileSync(join(root, 'frontend/package.json'), '{}');
  writeFileSync(join(root, 'frontend/node_modules/vitest/package.json'), '{"version":"fixture"}');
  writeFileSync(join(root, 'bin/pnpm'), '#!/bin/sh\n[ "$1" = --version ] && { echo fixture; exit 0; }\nexit 7\n', { mode: 0o755 });
  // The measured command is a shell fixture, not a real heavy job. Isolate
  // its nested gate so this test never queues behind its own outer admission.
  const env = gitTestEnvironment({ ...process.env, PATH: `${join(root, 'bin')}:${process.env.PATH}`,
    AGENT_HEAVY_STATE_DIR: join(root, 'tmp/gate') });
  const git = (...args) => {
    const result = spawnSync('git', args, { cwd: root, env, encoding: 'utf8' });
    assert.equal(result.status, 0, result.stderr);
  };
  initGitFixture(git);
  git('add', '.');
  git('commit', '-qm', 'fixture');
  const result = spawnSync(process.execPath, [new URL('./test-perf.mjs', import.meta.url).pathname, 'run', 'quick', 'failed', '--root', root], {
    env, encoding: 'utf8', timeout: 30_000,
  });
  assert.equal(result.status, 1, result.stderr);
  const out = join(root, 'tmp/test-perf', readdirSync(join(root, 'tmp/test-perf'))[0]);
  const summary = JSON.parse(readFileSync(join(out, 'summary.json'), 'utf8'));
  assert.equal(summary.valid, false);
  assert.match(summary.error, /Measurement failed/);
  const meter = join(out, 'measure');
  const table = readFileSync(join(meter, readdirSync(meter).find(name => name.endsWith('.tsv'))), 'utf8');
  assert.match(table, /FAILED\(7\)/);
});

// Backend controls are additive. The frontend fixtures/assertions above stay intact.
const goEvent = (Action, Test, more = {}) => ({ Package: 'moto/probe', Action, ...(Test ? { Test } : {}), ...more });
const goRows = () => [goEvent('start'), goEvent('run', 'TestTenantBoundary'), goEvent('pass', 'TestTenantBoundary'), goEvent('pass')];
const goJSONL = rows => rows.map(row => JSON.stringify(row)).join('\n');

function goReportFile(t, text) {
  const root = mkdtempSync(join(tmpdir(), 'moto-test-perf-jsonl-'));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  const file = join(root, 'go.jsonl');
  writeFileSync(file, text);
  return file;
}

function backendSample(rows = goRows(), wall = 10) {
  return { ...sample(wall), profile: 'backend-smoke', inventory: goInventory(goJSONL(rows)) };
}

test('Go inventory retains package outcomes, nested tests, skips and collection-order independence', () => {
  const rows = goRows();
  rows.splice(3, 0, goEvent('run', 'TestTenantBoundary/other'), goEvent('skip', 'TestTenantBoundary/other'));
  rows.push(goEvent('start', null, { Package: 'moto/empty' }), goEvent('skip', null, { Package: 'moto/empty' }));
  const result = goInventory(goJSONL(rows));
  assert.deepEqual(result.packages, [['moto/empty', 'skip'], ['moto/probe', 'pass']]);
  assert.deepEqual(result.entries, [
    ['moto/probe', 'TestTenantBoundary', 'pass'],
    ['moto/probe', 'TestTenantBoundary/other', 'skip'],
  ].sort((a, b) => JSON.stringify(a).localeCompare(JSON.stringify(b))));
  const reordered = [...rows.slice(-2), ...rows.slice(0, -2)];
  assert.deepEqual(goInventory(goJSONL(reordered)), result);
});

test('Go inventory retains legitimate internal/external duplicate-name multiplicity', async t => {
  const rows = goRows();
  rows.splice(2, 0, goEvent('run', 'TestTenantBoundary'));
  rows.splice(4, 0, goEvent('pass', 'TestTenantBoundary'));
  const result = goInventory(goJSONL(rows));
  assert.equal(result.entries.length, 2);
  assert.deepEqual(result.entries[0], result.entries[1]);
  assert.deepEqual(await goInventoryFile(goReportFile(t, goJSONL(rows))), result);
  // Equal names may end differently; each pass/skip occurrence still matters.
  rows[4] = goEvent('skip', 'TestTenantBoundary');
  assert.deepEqual(goInventory(goJSONL(rows)).entries.map(entry => entry[2]).sort(), ['pass', 'skip']);
  assert.deepEqual(await goInventoryFile(goReportFile(t, goJSONL(rows))), goInventory(goJSONL(rows)));
});

test('Go file inventory handles split UTF-8, JSON tokens and lines without a final newline', async t => {
  const rows = goRows().map(row => row.Test ? { ...row, Test: 'TestGrüße/🦊' } : row);
  rows.splice(2, 0, goEvent('output', null, { Output: 'Grüße 🦊\\nquoted "JSON"\n' }));
  rows.push(goEvent('start', null, { Package: 'moto/empty' }), goEvent('skip', null, { Package: 'moto/empty' }));
  const text = ` \r\n\t\n${goJSONL(rows).replaceAll('\n', '\r\n')}`;
  const file = goReportFile(t, text);
  for (const highWaterMark of [1, 2, 7, 64 * 1024]) {
    assert.deepEqual(await goInventoryFile(file, { highWaterMark }), goInventory(text));
  }
  assert.equal(readFileSync(file, 'utf8'), text, 'The raw report must not change');
});

test('Go file inventory handles an event spanning multiple default-sized chunks', async t => {
  const rows = goRows();
  rows.splice(2, 0, goEvent('output', null, { Output: 'ä🦊'.repeat(32 * 1024) }));
  const text = goJSONL(rows) + '\n';
  assert.deepEqual(await goInventoryFile(goReportFile(t, text)), goInventory(text));
});

test('Go file inventory rejects a missing report', async t => {
  await assert.rejects(goInventoryFile(goReportFile(t, '') + '.missing'), { code: 'ENOENT' });
});

for (const [name, change] of [
  ['empty report', () => []],
  ['no tests', () => [goEvent('start'), goEvent('pass')]],
  ['all skipped', rows => rows.map(row => row.Action === 'pass' && row.Test ? { ...row, Action: 'skip' } : row)],
  ['unfinished test', rows => rows.filter((_, index) => index !== 2)],
  ['unfinished package', rows => rows.slice(0, -1)],
  ['unfinished duplicate run', rows => [...rows.slice(0, 2), goEvent('run', 'TestTenantBoundary'), ...rows.slice(2)]],
  ['terminal without run', rows => rows.filter((_, index) => index !== 1)],
  ['extra duplicate test terminal', rows => [...rows, goEvent('pass', 'TestTenantBoundary')]],
  ['duplicate package start', rows => [rows[0], rows[0], ...rows.slice(1)]],
  ['duplicate package terminal', rows => [...rows, goEvent('pass')]],
  ['package terminal without start', rows => rows.slice(1)],
  ['test failure', rows => [...rows, goEvent('fail', 'TestTenantBoundary')]],
  ['leftover-gate package failure after successful tests', rows => [...rows.slice(0, -1), goEvent('fail')]],
  ['late package failure after a pass', rows => [...rows, goEvent('fail')]],
  ['cached result replay', rows => [...rows, goEvent('output', null, { Output: 'ok moto/probe (cached)\n' })]],
  ['truncated extra package', rows => [...rows, goEvent('start', null, { Package: 'moto/incomplete' })]],
  ['missing package identity', rows => [...rows, { Action: 'output' }]],
  ['missing action', rows => [...rows, { Package: 'moto/probe' }]],
]) {
  test(`Go inventory rejects ${name}`, async t => {
    const text = goJSONL(change(goRows()));
    assert.throws(() => goInventory(text));
    await assert.rejects(goInventoryFile(goReportFile(t, text), { highWaterMark: 7 }));
  });
}

test('Go inventory rejects whitespace-only, malformed and truncated JSONL', async t => {
  for (const input of [' \n\t', '{not-json}', `${goJSONL(goRows())}\n{"Action":`]) {
    assert.throws(() => goInventory(input));
    await assert.rejects(goInventoryFile(goReportFile(t, input), { highWaterMark: 2 }));
  }
});

for (const change of ['skip', 'missing', 'duplicate', 'package-outcome']) {
  test(`Go comparison rejects a faster multiset changed by ${change}`, () => {
    const rows = goRows();
    rows.splice(3, 0, goEvent('run', 'TestOther'), goEvent('pass', 'TestOther'));
    const baseline = backendSample(rows);
    const candidate = backendSample(rows, 1);
    if (change === 'skip') candidate.inventory.entries[0][2] = 'skip';
    if (change === 'missing') candidate.inventory.entries.pop();
    if (change === 'duplicate') candidate.inventory.entries.push(candidate.inventory.entries[0]);
    if (change === 'package-outcome') candidate.inventory.packages[0][1] = 'skip';
    assert.throws(() => compare([baseline], [candidate]), /Non-equivalent inventory/);
  });
}

test('backend profiles pin explicit uncached package selections without changing frontend arrays', () => {
  const manifest = JSON.parse(readFileSync(new URL('./test-perf-workloads.json', import.meta.url), 'utf8'));
  assert.deepEqual(Object.fromEntries(Object.entries(manifest).filter(([name]) => name.startsWith('backend-'))), {
    'backend-full-worker3': { runner: 'go', args: ['-count=1', './...'], treatment: 'darwin-worker3-v1' },
    'backend-api-full': { runner: 'go', args: ['-count=1', './api'] },
    'backend-api-auth': { runner: 'go', args: ['-count=1', './api', '-run', '^(TestPasswordFailuresLockAccountAcrossPortals|TestPasswordLoginLockExpiresAndSuccessfulCredentialsResetAllPortals|TestConcurrentPasswordFailuresCannotLoseAttemptsOrLockOtherIdentities|TestPasswordLoginRefusalsDoNotEnumerateAccountsAcrossPortals|TestOperatorMFAAuditSharesTriggeringTransaction|TestOperatorMFAAuditFailureRollsBackEnrollment|TestSchoolLoginHandler_PortalRoleGate|TestSchoolPortalTokenMatrix|TestSchoolMiddleware_RejectsOtherScopes|TestSchoolMiddleware_ZeroTenantID_Unauthorized|TestTenantMiddleware_RejectsSchoolScope)$'] },
    'backend-full': { runner: 'go', args: ['-count=1', './...'] },
    'backend-architecture': { runner: 'go', args: ['-count=1', './internal/architecture'] },
    'backend-export': { runner: 'go', args: ['-count=1', './modules/organizationtenancy/compose'] },
    'backend-lifecycle': { runner: 'go', args: ['-count=1', './internal/testdb'] },
    'backend-smoke': { runner: 'go', args: ['-count=1', './modules/identityaccess/internal/domain'] },
  });
  assert.ok(['quick', 'full', 'ci-node', 'ci-dom-1', 'ci-dom-2'].every(name => Array.isArray(manifest[name])));
});

function backendFixture(t, profile = 'backend-smoke', mode = 'success', overrides = {}) {
  const root = mkdtempSync(join(tmpdir(), 'moto-test-perf-go-'));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  for (const directory of ['frontend', 'backend', 'scripts', 'bin', 'tmp/contract']) mkdirSync(join(root, directory), { recursive: true });
  writeFileSync(join(root, '.gitignore'), 'tmp/\nbackend/dev.env\n');
  writeFileSync(join(root, 'frontend/package.json'), '{}');
  writeFileSync(join(root, 'backend/go.mod'), 'module fixture\n\ngo 1.27.3\n');
  writeFileSync(join(root, 'backend/go.sum'), 'fixture\n');
  writeFileSync(join(root, 'backend/dev.env'), 'fixture=before\n');
  writeFileSync(join(root, 'source.txt'), 'before\n');
  const shell = (file, text) => writeFileSync(join(root, file), `#!/bin/sh\nset -eu\n${text}\n`, { mode: 0o755 });
  shell('scripts/run-go-toolchain.sh', `
if [ "$1" = go ] && [ "$2" = version ]; then echo 'go version go-fixture fixture/fixture'; exit 0; fi
printf '%s\\n' "$@" > "$PERF_FIXTURE/dispatch"
exec "$@"`);
  shell('scripts/test-backend.sh', `
[ -n "\${TEST_DB_DSN:-}" ] || exit 91
printf '%s\\n' "$@" > "$PERF_FIXTURE/args"
printf '%s' "$GOTESTSUM_JSONFILE" > "$PERF_FIXTURE/report-path"
[ "$PERF_MODE" != missing ] || exit 0
cat > "$GOTESTSUM_JSONFILE" <<'REPORT'
${goJSONL(goRows())}
REPORT
[ "$PERF_MODE" != output-heavy ] || "$PERF_NODE" "$PERF_FIXTURE/output-heavy.cjs"
case "$PERF_MODE" in
  fail) exit 7 ;;
  source-change) printf 'after\\n' > source.txt ;;
  context-change) printf 'fixture=after\\n' > backend/dev.env ;;
  git-ref-change) git update-ref refs/remotes/origin/development HEAD^ ;;
  git-ref-advance)
    next=$(printf 'advance target\\n' | git commit-tree 'HEAD^{tree}' -p HEAD)
    git update-ref refs/remotes/origin/development "$next" ;;
esac`);
  // More raw output than the CLI child's entire heap, generated one event at
  // a time. No large string/file allocation is needed in the test process.
  writeFileSync(join(root, 'tmp/contract/output-heavy.cjs'), `
const { appendFileSync } = require('node:fs');
const line = JSON.stringify(${JSON.stringify(goEvent('output', null, { Output: 'x'.repeat(8192) }))}) + '\\n';
for (let i = 0; i < 8192; i++) appendFileSync(process.env.GOTESTSUM_JSONFILE, line);
`);
  shell('bin/gate', `
[ "$1" != --status ] || { echo fixture; exit 0; }
printf '%s\\n' "$@" > "$PERF_FIXTURE/gate-args"
[ "$1" != --light ] || shift
[ "$1" = -- ] || exit 92
shift
exec "$@"`);
  // dev-bench probes optional tools in its metadata; neither is needed by
  // backendContext. A frontend-context regression would fail on pnpm/Vitest.
  shell('bin/pnpm', 'exit 98');
  shell('bin/go', 'exit 99');
  const env = gitTestEnvironment({ ...process.env, PATH: `${join(root, 'bin')}:${process.env.PATH}`,
    AGENT_HEAVY: join(root, 'bin/gate'), PERF_FIXTURE: join(root, 'tmp/contract'), PERF_MODE: mode, PERF_NODE: process.execPath,
    TEST_DB_DSN: 'example-test-dsn-not-a-connection', GOTESTSUM_JSONFILE: join(root, 'tmp/decoy.jsonl'), ...overrides });
  const git = (...args) => {
    const result = spawnSync('git', args, { cwd: root, env, encoding: 'utf8' });
    assert.equal(result.status, 0, result.stderr);
  };
  initGitFixture(git);
  git('add', '.');
  git('commit', '-qm', 'fixture');
  if (mode === 'git-ref-change') git('commit', '--allow-empty', '-qm', 'second fixture');
  if (mode !== 'missing-ref') git('update-ref', 'refs/remotes/origin/development', 'HEAD');
  const result = spawnSync(process.execPath, [new URL('./test-perf.mjs', import.meta.url).pathname,
    'run', profile, 'fixture', '--root', root, '--cache-state', 'warm'], { env, encoding: 'utf8', timeout: 30_000 });
  assert.equal(result.signal, null, result.stderr + result.stdout);
  const out = join(root, 'tmp/test-perf', readdirSync(join(root, 'tmp/test-perf'))[0]);
  const summaryText = readFileSync(join(out, 'summary.json'), 'utf8');
  const summary = JSON.parse(summaryText);
  const meter = join(out, 'measure');
  const table = () => readFileSync(join(meter, readdirSync(meter).find(name => name.endsWith('.tsv'))), 'utf8');
  return { root, result, out, summary, summaryText, table,
    record: name => readFileSync(join(root, 'tmp/contract', name), 'utf8') };
}

test('backend CLI inventories output larger than its heap without loading the whole report', t => {
  const fixture = backendFixture(t, 'backend-smoke', 'output-heavy', { NODE_OPTIONS: '--max-old-space-size=32' });
  assert.equal(fixture.result.status, 0, fixture.result.stderr + fixture.result.stdout);
  assert.equal(fixture.summary.valid, true, fixture.summary.error);
  assert.deepEqual(fixture.summary.inventory, goInventory(goJSONL(goRows())));
  assert.deepEqual(fixture.summary.counts, { packages: 1, tests: 1, passed: 1, skipped: 0 });
  assert.ok(statSync(fixture.summary.rawReport).size > 64 * 1024 * 1024);
});

for (const [profile, selection, ...options] of [
  ['backend-full', './...'], ['backend-architecture', './internal/architecture'],
  ['backend-api-full', './api'],
  ['backend-api-auth', './api', '-run', '^(TestPasswordFailuresLockAccountAcrossPortals|TestPasswordLoginLockExpiresAndSuccessfulCredentialsResetAllPortals|TestConcurrentPasswordFailuresCannotLoseAttemptsOrLockOtherIdentities|TestPasswordLoginRefusalsDoNotEnumerateAccountsAcrossPortals|TestOperatorMFAAuditSharesTriggeringTransaction|TestOperatorMFAAuditFailureRollsBackEnrollment|TestSchoolLoginHandler_PortalRoleGate|TestSchoolPortalTokenMatrix|TestSchoolMiddleware_RejectsOtherScopes|TestSchoolMiddleware_ZeroTenantID_Unauthorized|TestTenantMiddleware_RejectsSchoolScope)$'],
  ['backend-export', './modules/organizationtenancy/compose'], ['backend-lifecycle', './internal/testdb'],
  ['backend-smoke', './modules/identityaccess/internal/domain'],
]) {
  test(`${profile} dispatches the existing wrapper and only gates its measured command`, t => {
    const fixture = backendFixture(t, profile);
    assert.equal(fixture.result.status, 0, fixture.result.stderr + fixture.result.stdout);
    assert.equal(fixture.summary.valid, true, fixture.summary.error);
    const args = ['-count=1', selection, ...options];
    assert.equal(fixture.record('args'), args.join('\n') + '\n');
    assert.equal(fixture.record('dispatch'), [join(fixture.root, 'scripts/test-backend.sh'), ...args].join('\n') + '\n');
    assert.deepEqual(fixture.summary.command, [join(fixture.root, 'scripts/run-go-toolchain.sh'),
      join(fixture.root, 'scripts/test-backend.sh'), ...args]);
    const gateArgs = fixture.record('gate-args').trim().split('\n');
    assert.equal(gateArgs[0], profile === 'backend-full' ? '--' : '--light');
    assert.ok(gateArgs.includes(join(fixture.root, 'scripts/run-go-toolchain.sh')));
    assert.ok(!gateArgs.some(arg => arg.endsWith('test-perf.mjs') || arg.endsWith('dev-bench.sh')));
    assert.equal(fixture.summary.rawReport, join(fixture.out, 'go.jsonl'));
    assert.equal(fixture.record('report-path'), fixture.summary.rawReport);
    assert.deepEqual(fixture.summary.inventory, goInventory(goJSONL(goRows())));
    assert.deepEqual(fixture.summary.counts, { packages: 1, tests: 1, passed: 1, skipped: 0 });
    assert.equal(fixture.summary.context.go, 'go version go-fixture fixture/fixture');
    assert.equal(fixture.summary.context.vitest, undefined);
    assert.equal(fixture.summary.context.pnpm, undefined);
    assert.deepEqual(fixture.summary.context.gitReferenceInputs, {
      targetResolutionExit: 0,
      mergeBases: { exit: 0, value: fixture.summary.state.revision },
      selectedBase: { exit: 0, value: fixture.summary.state.revision },
    });
    assert.deepEqual(fixture.summary.gitReferenceObservation.target, { exit: 0, value: fixture.summary.state.revision });
    assert.match(fixture.summary.context.environment, /^[a-f0-9]{64}$/);
    assert.ok(fixture.summary.context.locks.some(([file, hash]) => file === 'backend/go.sum' && /^[a-f0-9]{64}$/.test(hash)));
    assert.ok(fixture.summary.context.wrappers.some(([file, hash]) => file === 'scripts/test-backend.sh' && /^[a-f0-9]{64}$/.test(hash)));
    assert.ok(!fixture.summaryText.includes('example-test-dsn-not-a-connection'));
    assert.ok(!fixture.summary.command.join(' ').includes('TEST_DB_DSN'));
  });
}

test('wrapper fallback exit 0 without a gotestsum report remains invalid', t => {
  const fixture = backendFixture(t, 'backend-smoke', 'missing');
  assert.equal(fixture.result.status, 1);
  assert.equal(fixture.summary.exit.code, 0);
  assert.equal(fixture.summary.valid, false);
  assert.match(fixture.summary.error, /Missing gotestsum JSONL report/);
  assert.equal(fixture.summary.rawReport, join(fixture.out, 'go.jsonl'));
  assert.match(fixture.table(), /\tok\t/);
});

test('failed backend wrapper retains raw exit, report path and report despite successful test events', t => {
  const fixture = backendFixture(t, 'backend-smoke', 'fail');
  assert.equal(fixture.result.status, 1);
  assert.equal(fixture.summary.valid, false);
  assert.equal(fixture.summary.exit.code, 1); // dev-bench aggregates command failures.
  assert.match(fixture.summary.error, /Measurement failed/);
  assert.match(fixture.table(), /FAILED\(7\)/); // raw command exit is not lost.
  assert.equal(fixture.record('report-path'), fixture.summary.rawReport);
  assert.deepEqual(goInventory(readFileSync(fixture.summary.rawReport, 'utf8')), goInventory(goJSONL(goRows())));
});

for (const [mode, error] of [['source-change', /Checkout changed during trial/], ['context-change', /Environment changed during trial/], ['git-ref-change', /Environment changed during trial/]]) {
  test(`backend pre/post invariance rejects a real ${mode}`, t => {
    const fixture = backendFixture(t, 'backend-smoke', mode);
    assert.equal(fixture.result.status, 1);
    assert.equal(fixture.summary.valid, false);
    assert.equal(fixture.summary.exit.code, 0);
    assert.match(fixture.summary.error, error);
  });
}

test('backend metadata records unavailable Git bases without inventing a fallback', t => {
  const fixture = backendFixture(t, 'backend-smoke', 'missing-ref');
  assert.equal(fixture.result.status, 0, fixture.result.stderr + fixture.result.stdout);
  assert.deepEqual(fixture.summary.context.gitReferenceInputs, {
    targetResolutionExit: 128, mergeBases: { exit: 128, value: null }, selectedBase: { exit: 128, value: null },
  });
});

test('backend accepts a real target advance only when its immutable comparison basis is unchanged', t => {
  const fixture = backendFixture(t, 'backend-smoke', 'git-ref-advance');
  assert.equal(fixture.result.status, 0, fixture.result.stderr + fixture.result.stdout);
  assert.equal(fixture.summary.valid, true, fixture.summary.error);
  const before = fixture.summary.gitReferenceObservation;
  const after = fixture.summary.gitReferenceObservationAfter;
  assert.notDeepEqual(before.target, after.target);
  assert.deepEqual(before.basis, after.basis);
  assert.deepEqual(before.basis, fixture.summary.context.gitReferenceInputs);
  assert.equal(before.basis.mergeBases.value, fixture.summary.state.revision);
  assert.equal(before.basis.selectedBase.value, fixture.summary.state.revision);
});

test('paired backend comparisons reject a different Git basis, not merely a different recorded tip', () => {
  const baseline = sample(), candidate = sample();
  baseline.id = 'git-base-baseline'; candidate.id = 'git-base-candidate';
  baseline.context.gitReferenceInputs = { targetResolutionExit: 0, mergeBases: { exit: 0, value: 'base-a' }, selectedBase: { exit: 0, value: 'base-a' } };
  candidate.context.gitReferenceInputs = structuredClone(baseline.context.gitReferenceInputs);
  baseline.gitReferenceObservation = { target: 'old-tip' };
  candidate.gitReferenceObservation = { target: 'new-tip' };
  assert.doesNotThrow(() => compare([baseline], [candidate]));
  candidate.context.gitReferenceInputs.mergeBases.value = 'base-b';
  assert.throws(() => compare([baseline], [candidate]), /Non-equivalent context/);
  candidate.context.gitReferenceInputs.mergeBases.value = 'base-a';
  candidate.context.gitReferenceInputs.selectedBase.value = 'base-b';
  assert.throws(() => compare([baseline], [candidate]), /Non-equivalent context/);
});

test('backend trial without inherited TEST_DB_DSN fails before dispatch', t => {
  const fixture = backendFixture(t, 'backend-smoke', 'success', { TEST_DB_DSN: '' });
  assert.equal(fixture.result.status, 1);
  assert.equal(fixture.summary.valid, false);
  assert.match(fixture.summary.error, /require inherited TEST_DB_DSN/);
  assert.equal(fixture.summary.exit, undefined);
});
