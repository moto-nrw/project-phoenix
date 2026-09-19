import assert from 'node:assert/strict';
import test from 'node:test';
import { execFileSync } from 'node:child_process';
import { mkdtempSync, writeFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { checkRelease } from './check-release-architecture.mjs';

function polling(sequence) {
  let calls = 0;
  let elapsed = 0;
  const delays = [];
  return {
    execute(command, args) {
      if (command !== 'gh') return executor()(command, args);
      return executor(sequence[Math.min(calls++, sequence.length - 1)])(command, args);
    },
    options: {
      timeoutMs: 60, pollIntervalMs: 20, now: () => elapsed,
      sleep: async ms => { delays.push(ms); elapsed += ms; }, log: () => {},
    },
    delays,
    calls: () => calls,
  };
}

test('waits for discovery, queued and running CI before accepting success', async () => {
  const probe = polling([[], [{ ...successful, status: 'queued', conclusion: null }],
    [{ ...successful, status: 'in_progress', conclusion: null }], [successful]]);
  assert.equal(await checkRelease(context, probe.execute, probe.options), context.headSha);
  assert.deepEqual(probe.delays, [20, 20, 20]);
  assert.equal(probe.calls(), 4);
});

for (const conclusion of ['failure', 'cancelled', 'timed_out']) {
  test(`stops when development CI completes with ${conclusion}`, async () => {
    const probe = polling([[{ ...successful, status: 'in_progress', conclusion: null }],
      [{ ...successful, conclusion }]]);
    await assert.rejects(() => checkRelease(context, probe.execute, probe.options), /No successful/);
    assert.equal(probe.calls(), 2);
    assert.deepEqual(probe.delays, [20]);
  });
}

test('times out for missing, pending or unrelated CI', async () => {
  for (const runs of [[], [{ ...successful, status: 'in_progress', conclusion: null }],
    [{ ...successful, head_sha: 'c'.repeat(40) }]]) {
    const probe = polling([runs]);
    await assert.rejects(() => checkRelease(context, probe.execute, probe.options), /Timed out/);
    assert.equal(probe.calls(), 4);
    assert.deepEqual(probe.delays, [20, 20, 20]);
  }
});

const context = {
  event: 'pull_request', baseRef: 'main', headRef: 'development', repository: 'moto-nrw/project-phoenix',
  headRepository: 'moto-nrw/project-phoenix', baseSha: 'a'.repeat(40), headSha: 'b'.repeat(40),
};
const successful = {
  event: 'push', head_branch: 'development', head_sha: context.headSha,
  head_repository: { full_name: context.repository }, path: '.github/workflows/main.yml',
  status: 'completed', conclusion: 'success',
};
function executor(runs = [successful], options = {}) {
  return (command, args) => {
    if (command === 'gh') {
      assert.ok(args.includes(`head_sha=${context.headSha}`));
      assert.ok(args.includes('event=push'));
      return JSON.stringify({ workflow_runs: runs });
    }
    if (args[0] === 'merge-base') {
      if (options.diverged) throw new Error('not an ancestor');
      return '';
    }
    return options.changed && args[1] === 'HEAD^{tree}' ? 'changed' : 'tree';
  };
}
test('accepts only the exact successful development promotion', async () => {
  assert.equal(await checkRelease(context, executor()), context.headSha);
});
for (const change of [{ headRepository: 'fork/repo' }, { headRef: 'feature' }, { baseRef: 'development' },
  { event: 'push' }, { headSha: 'bbbbbbb' }]) {
  test(`rejects wrong promotion context ${JSON.stringify(change)}`, async () => {
    await assert.rejects(() => checkRelease({ ...context, ...change }, executor()));
  });
}
test('rejects diverged main and merge-only changes', async () => {
  await assert.rejects(() => checkRelease(context, executor([], { diverged: true })), /ancestor/);
  await assert.rejects(() => checkRelease(context, executor([], { changed: true })), /tree differs/);
});
for (const change of [{ event: 'pull_request' }, { head_sha: 'c'.repeat(40) }, { conclusion: 'failure' },
  { status: 'in_progress' }, { head_branch: 'feature' }, { path: '.github/workflows/build.yml' },
  { head_repository: { full_name: 'fork/repo' } }]) {
  test(`rejects unrelated or incomplete CI ${JSON.stringify(change)}`, async () => {
    await assert.rejects(async () => checkRelease(context, executor([{ ...successful, ...change }]), { timeoutMs: 0 }), /No successful/);
  });
}
test('missing CI and API failures fail closed', async () => {
  await assert.rejects(() => checkRelease(context, executor([]), { timeoutMs: 0 }), /No successful/);
  await assert.rejects(() => checkRelease(context, () => { throw new Error('unavailable'); }), /unavailable/);
});

test('real Git ancestry and tree checks accept promotion but reject release-only edits', async t => {
  const root = mkdtempSync(join(tmpdir(), 'moto-release-git-'));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  const env = { ...process.env, GIT_CONFIG_GLOBAL: '/dev/null', GIT_CONFIG_NOSYSTEM: '1' };
  for (const key of ['GIT_DIR', 'GIT_WORK_TREE', 'GIT_INDEX_FILE', 'GIT_COMMON_DIR', 'GIT_PREFIX']) delete env[key];
  const git = (...args) => execFileSync('git', args, { cwd: root, env, encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'] }).trim();
  git('init', '-q'); git('config', 'user.name', 'Release test'); git('config', 'user.email', 'test@example.invalid');
  git('config', 'gc.auto', '0'); git('config', 'maintenance.auto', 'false');
  const commit = content => {
    writeFileSync(join(root, 'file'), content); git('add', '.'); git('commit', '-qm', 'fixture'); return git('rev-parse', 'HEAD');
  };
  const baseSha = commit('main'); const headSha = commit('development');
  const execute = (command, args) => command === 'git' ? git(...args) :
    JSON.stringify({ workflow_runs: [{ ...successful, head_sha: headSha }] });
  assert.equal(await checkRelease({ ...context, baseSha, headSha }, execute), headSha);
  commit('release-only edit');
  await assert.rejects(() => checkRelease({ ...context, baseSha, headSha }, execute), /tree differs/);
});
