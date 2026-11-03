import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { existsSync, mkdtempSync, mkdirSync, readFileSync, readdirSync, realpathSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import test from 'node:test';
import { gitTestEnvironment, initGitFixture } from './test-helpers/git-fixture.mjs';

const profile = 'backend-full-worker3';
const policyFile = 'scripts/go-package-workers.sh';
const originalLine = 'if [ "$(uname -s)" = Darwin ]; then max_workers=2; fi';
const candidateLines = `if [ "$(uname -s)" = Darwin ]; then
  max_workers=2
  # One extra package only on large Macs; preserve CPU/RAM headroom below.
  if [ "$memory_gb" -ge 64 ]; then max_workers=3; fi
fi`;
const currentPolicy = readFileSync(new URL('./go-package-workers.sh', import.meta.url), 'utf8');
const policies = {
  B: currentPolicy.replace(candidateLines, originalLine),
  W: currentPolicy.replace(originalLine, candidateLines),
};
assert.notEqual(policies.B, policies.W);
const events = [
  { Action: 'start', Package: 'example/fixture' },
  { Action: 'run', Package: 'example/fixture', Test: 'TestRejectWrongTenant' },
  { Action: 'pass', Package: 'example/fixture', Test: 'TestRejectWrongTenant' },
  { Action: 'run', Package: 'example/fixture', Test: 'TestOther' },
  { Action: 'pass', Package: 'example/fixture', Test: 'TestOther' },
  { Action: 'pass', Package: 'example/fixture' },
].map(e => JSON.stringify(e)).join('\n') + '\n';

// Exercise the real CLI, dev-bench and real local Git history. Test commands are
// recorders only: no real Go tests, services, DB, pnpm or network in this fixture.
function fixture(t, alterManifest) {
  // ESM resolves /var to /private/var on macOS. Match the CLI main guard.
  const root = realpathSync(mkdtempSync(join(tmpdir(), 'moto-worker3-verifier-')));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  for (const dir of ['backend', 'frontend', 'scripts', 'bin', 'tmp/contract']) mkdirSync(join(root, dir), { recursive: true });
  const write = (file, text, mode = 0o644) => writeFileSync(join(root, file), text, { mode });
  const shell = (file, text) => write(file, `#!/bin/sh\nset -eu\n${text}\n`, 0o755);
  write('.gitignore', 'tmp/\nbackend/dev.env\n');
  write('backend/go.mod', 'module fixture\n\ngo 1.27.3\n');
  write('backend/go.sum', 'fixture\n');
  write('backend/dev.env', 'fixture=before\n');
  write('frontend/package.json', '{}\n');
  write('source.txt', 'before\n');
  write(policyFile, policies.B, 0o755);
  write('scripts/total-memory-gb.sh', readFileSync(new URL('./total-memory-gb.sh', import.meta.url)) , 0o755);
  shell('bin/uname', 'echo Darwin');
  shell('bin/getconf', 'echo 16');
  shell('bin/sysctl', '[ "$2" = hw.memsize ] && { echo 68719476736; exit 0; }; echo 16');
  shell('bin/go', 'exit 99');
  shell('bin/pnpm', 'exit 98');
  shell('scripts/run-go-toolchain.sh', `
if [ "$1" = go ] && [ "$2" = version ]; then echo 'go version fixture'; exit 0; fi
exec "$@"`);
  shell('scripts/test-backend.sh', `
printf '%s\\n' "$@" > "$PERF_FIXTURE/args"
cat "$PERF_FIXTURE/report" > "$GOTESTSUM_JSONFILE"
case "$PERF_MODE" in
 treatment-change) cp "$PERF_FIXTURE/W" scripts/go-package-workers.sh ;;
 context-change) printf after > backend/dev.env ;;
 source-change) printf after > source.txt ;;
esac
[ "$PERF_MODE" != fail ] || exit 7`);
  shell('bin/gate', `
[ "$1" != --status ] || { echo fixture; exit 0; }
printf '%s\\n' "$@" > "$PERF_FIXTURE/gate"
[ "$1" != --light ] || shift
[ "$1" = -- ]; shift; exec "$@"`);
  const env = gitTestEnvironment({ ...process.env, PATH: `${join(root, 'bin')}:${process.env.PATH}`,
    AGENT_HEAVY: join(root, 'bin/gate'), PERF_FIXTURE: join(root, 'tmp/contract'), PERF_MODE: 'success',
    TEST_DB_DSN: 'example-not-a-connection' });
  const git = (...args) => {
    const r = spawnSync('git', args, { cwd: root, env, encoding: 'utf8' });
    assert.equal(r.status, 0, r.stderr);
  };
  initGitFixture(git); git('add', '.'); git('commit', '-qm', 'fixture');
  git('update-ref', 'refs/remotes/origin/development', 'HEAD');
  write('tmp/contract/W', policies.W);
  write('tmp/contract/report', events);
  let evaluator = new URL('./test-perf.mjs', import.meta.url).pathname;
  if (alterManifest) {
    for (const file of ['test-perf.mjs', 'dev-bench.sh']) write(`scripts/${file}`, readFileSync(new URL(file, import.meta.url)));
    const manifest = JSON.parse(readFileSync(new URL('./test-perf-workloads.json', import.meta.url), 'utf8'));
    alterManifest(manifest);
    write('scripts/test-perf-workloads.json', JSON.stringify(manifest));
    evaluator = join(root, 'scripts/test-perf.mjs');
  }
  const cli = args => {
    const result = spawnSync(process.execPath, [evaluator, ...args], {
      cwd: root, env, encoding: 'utf8', timeout: 30_000,
    });
    if (result.status !== 0) t.diagnostic(JSON.stringify({ kind: 'raw-cli-rejection', args,
      exit: result.status, signal: result.signal, stdout: result.stdout, stderr: result.stderr }));
    return result;
  };
  let id = 0;
  const run = (name = profile) => {
    const r = cli(['run', name, `fixture-${++id}`, '--root', root, '--cache-state', 'warm']);
    assert.equal(r.signal, null, r.stderr + r.stdout);
    const directory = /test-perf: artifacts (.+)/.exec(r.stdout)?.[1];
    assert.ok(directory, r.stderr + r.stdout);
    const summary = JSON.parse(readFileSync(join(directory, 'summary.json'), 'utf8'));
    const meter = join(directory, 'measure');
    const rawMeter = existsSync(meter) ? Object.fromEntries(readdirSync(meter)
      .filter(name => /\.(tsv|log)$/.test(name)).map(name => [name, readFileSync(join(meter, name), 'utf8')])) : {};
    if (!summary.valid) t.diagnostic(JSON.stringify({ kind: 'raw-invalid-trial', summary,
      report: existsSync(summary.rawReport) ? readFileSync(summary.rawReport, 'utf8') : null, meter: rawMeter }));
    return { r, directory, summary, rawMeter };
  };
  const compare = (bs, ws) => cli(['compare', ...bs.map(x => x.directory), '--', ...ws.map(x => x.directory)]);
  return { root, env, write, git, run, compare, cli, side: side => write(policyFile, policies[side], 0o755) };
}

function valid(trial) {
  assert.equal(trial.r.status, 0, trial.r.stderr + trial.r.stdout);
  assert.equal(trial.summary.valid, true, trial.summary.error);
}
function rejected(result, error) {
  assert.equal(result.signal, null, result.stderr + result.stdout);
  assert.equal(result.status, 1, result.stderr + result.stdout);
  assert.match(result.stderr + result.stdout, error);
}

test('worker3 CLI admits only the declared two policies, records both endpoints and uses heavy admission', t => {
  const f = fixture(t), b = f.run(); valid(b);
  f.side('W'); const w = f.run(); valid(w);
  for (const [trial, side, workers] of [[b, 'B', 2], [w, 'W', 3]]) {
    assert.equal(trial.summary.treatmentBefore.side, side);
    assert.equal(trial.summary.treatmentBefore.defaultWorkers, workers);
    assert.match(trial.summary.treatmentBefore.sha256, /^[a-f0-9]{64}$/);
    assert.deepEqual(trial.summary.treatmentAfter, trial.summary.treatmentBefore);
    t.diagnostic(JSON.stringify({ kind: 'source-treatment-endpoints', state: trial.summary.state,
      before: trial.summary.treatmentBefore, after: trial.summary.treatmentAfter }));
    assert.ok(!trial.summary.context.wrappers.some(([file]) => file === policyFile));
    assert.equal(trial.summary.command.at(-2), '-count=1');
    assert.equal(trial.summary.command.at(-1), './...');
  }
  assert.equal(readFileSync(join(f.root, 'tmp/contract/args'), 'utf8'), '-count=1\n./...\n');
  assert.equal(readFileSync(join(f.root, 'tmp/contract/gate'), 'utf8').split('\n')[0], '--');
  const comparison = f.compare([b], [w]);
  assert.equal(comparison.status, 0, comparison.stderr);
  assert.equal(JSON.parse(comparison.stdout).treatment.candidate.defaultWorkers, 3);
});

for (const normal of ['backend-full', 'backend-smoke', 'backend-architecture', 'backend-export', 'backend-lifecycle']) {
  test(`normal ${normal} rejects a real workerpolicy change`, t => {
    const f = fixture(t), b = f.run(normal); valid(b);
    f.side('W'); const w = f.run(normal); valid(w);
    rejected(f.compare([b], [w]), /Non-equivalent context/);
  });
}

for (const file of ['scripts/test-backend.sh', 'scripts/run-go-toolchain.sh', 'scripts/total-memory-gb.sh', 'backend/go.sum', 'backend/dev.env']) {
  test(`worker3 still rejects different shared ${file}`, t => {
    const f = fixture(t), b = f.run(); valid(b);
    f.side('W');
    // Semantically harmless bytes are still different shared inputs.
    f.write(file, readFileSync(join(f.root, file), 'utf8') + '\n');
    const w = f.run(); valid(w);
    rejected(f.compare([b], [w]), /Non-equivalent context/);
  });
}

for (const mode of ['treatment-change', 'context-change', 'source-change', 'fail']) {
  test(`worker3 rejects a real ${mode} during CLI execution`, t => {
    const f = fixture(t); f.env.PERF_MODE = mode;
    const result = f.run();
    assert.equal(result.r.status, 1);
    assert.equal(result.summary.valid, false);
    assert.match(result.summary.error, mode === 'treatment-change' ? /Treatment changed during trial/ :
      mode === 'context-change' ? /Environment changed/ : mode === 'source-change' ? /Checkout changed/ : /Measurement failed/);
    assert.equal(result.summary.exit.code, mode === 'fail' ? 1 : 0);
    assert.equal(result.summary.exit.signal, null);
    if (mode === 'fail') assert.match(Object.values(result.rawMeter).join('\n'), /FAILED\(7\)/);
    if (mode === 'treatment-change') {
      assert.equal(result.summary.treatmentBefore.defaultWorkers, 2);
      assert.equal(result.summary.treatmentAfter.defaultWorkers, 3);
    }
  });
}

test('worker3 rejects unknown policy bytes even when they resolve the same default', t => {
  const f = fixture(t);
  f.write(policyFile, policies.B + '\nprintf executed > tmp/contract/unknown-policy-executed\n');
  const result = f.run();
  assert.equal(existsSync(join(f.root, 'tmp/contract/unknown-policy-executed')), false,
    'Unknown policy must be rejected before executing its bytes');
  assert.equal(result.r.status, 1);
  assert.match(result.summary.error, /Unknown worker3 policy/);
  assert.match(result.summary.treatmentBeforeRaw.sha256, /^[a-f0-9]{64}$/);
  assert.equal(Object.hasOwn(result.summary.treatmentBeforeRaw, 'defaultWorkers'), false);
  assert.equal(existsSync(join(f.root, 'tmp/contract/args')), false);
});

for (const group of ['baseline', 'candidate']) {
  test(`worker3 rejects a treatment switch within the ${group} group even for ignored policy inputs`, t => {
    const f = fixture(t);
    f.git('rm', '--cached', policyFile);
    f.write('.gitignore', readFileSync(join(f.root, '.gitignore'), 'utf8') + policyFile + '\n');
    f.git('add', '.gitignore'); f.git('commit', '-qm', 'ignored treatment fixture');
    f.git('update-ref', 'refs/remotes/origin/development', 'HEAD');
    const b1 = f.run(), b2 = f.run(); valid(b1); valid(b2);
    f.side('W'); const w1 = f.run(), w2 = f.run(); valid(w1); valid(w2);
    // Root source states are now identical: only the explicit treatment guard
    // can reject a side switch, not the generic Git dirty-state guard.
    assert.deepEqual(b1.summary.state, w1.summary.state);
    // Independent identities, no trial reuse masking the treatment guard.
    const b3 = (f.side('B'), f.run()); valid(b3);
    rejected(group === 'baseline' ? f.compare([b1, w1], [w2, b3]) : f.compare([b1, b2], [w1, b3]),
      /Treatment changed within sample group/);
  });
}

for (const change of ['skip', 'duplicate', 'rename']) {
  test(`worker3 keeps exact inventory: ${change}`, t => {
    const f = fixture(t), b = f.run(); valid(b);
    f.side('W');
    let report = events;
    if (change === 'skip') report = report.replace('"Action":"pass","Package":"example/fixture","Test":"TestRejectWrongTenant"',
      '"Action":"skip","Package":"example/fixture","Test":"TestRejectWrongTenant"');
    if (change === 'rename') report = report.replaceAll('TestRejectWrongTenant', 'TestDifferent');
    if (change === 'duplicate') {
      const duplicate = [
        { Action: 'run', Package: 'example/fixture', Test: 'TestOther' },
        { Action: 'pass', Package: 'example/fixture', Test: 'TestOther' },
      ].map(e => JSON.stringify(e)).join('\n') + '\n';
      report = report.replace('{"Action":"pass","Package":"example/fixture"}\n', duplicate + '{"Action":"pass","Package":"example/fixture"}\n');
    }
    f.write('tmp/contract/report', report);
    const w = f.run(); valid(w);
    rejected(f.compare([b], [w]), /Non-equivalent inventory/);
  });
}

for (const change of ['override', 'ignore-field', 'other-profile']) {
  test(`workload definition cannot grant arbitrary treatment permission: ${change}`, t => {
    const f = fixture(t, manifest => {
      if (change === 'override') manifest[profile].args.push('-p', '1');
      else if (change === 'ignore-field') manifest[profile].ignoreWrappers = [policyFile];
      else manifest['backend-smoke'].treatment = 'darwin-worker3-v1';
    });
    const r = f.cli(['run', change !== 'other-profile' ? profile : 'backend-smoke', 'bad', '--root', f.root, '--cache-state', 'warm']);
    rejected(r, /exact versioned full workload|Treatment is restricted/);
    assert.equal(existsSync(join(f.root, 'tmp/contract/args')), false);
  });
}

test('worker3 still rejects a changed inherited GOFLAGS hash', t => {
  const f = fixture(t); f.env.GOFLAGS = '-p=4';
  const b = f.run(); valid(b);
  f.side('W'); f.env.GOFLAGS = '-p=1';
  const w = f.run(); valid(w);
  rejected(f.compare([b], [w]), /Non-equivalent context/);
});

test('worker3 rejects a real Git source change within a side', t => {
  const f = fixture(t), b1 = f.run(); valid(b1);
  f.write('source.txt', 'other revision\n');
  const b2 = f.run(); valid(b2);
  f.side('W'); const w1 = f.run(), w2 = f.run(); valid(w1); valid(w2);
  rejected(f.compare([b1, b2], [w1, w2]), /Source state changed within sample group/);
});

test('worker3 rejects a real Git comparison basis change across sides', t => {
  const f = fixture(t), b = f.run(); valid(b);
  f.git('commit', '--allow-empty', '-qm', 'second commit');
  f.git('update-ref', 'refs/remotes/origin/development', 'HEAD');
  f.side('W'); const w = f.run(); valid(w);
  rejected(f.compare([b], [w]), /Non-equivalent context/);
});

test('worker3 rejects a changed resolved default under byte-identical policy', t => {
  const f = fixture(t);
  f.write('bin/getconf', '#!/bin/sh\necho 2\n');
  const result = f.run();
  assert.equal(result.r.status, 1);
  assert.match(result.summary.error, /host resolving B=2 and W=3/);
  assert.equal(result.summary.treatmentBeforeRaw.defaultWorkers, 1);
});


test('worker3 cannot qualify B/B, W/W or reversed W/B as the declared experiment', t => {
  const f = fixture(t), b1 = f.run(), b2 = f.run(); valid(b1); valid(b2);
  f.side('W'); const w1 = f.run(), w2 = f.run(); valid(w1); valid(w2);
  rejected(f.compare([b1], [b2]), /Worker3 candidate must be W/);
  rejected(f.compare([w1], [w2]), /Worker3 baseline must be B/);
  rejected(f.compare([w1], [b1]), /Worker3 baseline must be B/);
});
