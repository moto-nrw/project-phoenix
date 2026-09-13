import assert from 'node:assert/strict';
import test from 'node:test';
import { execFileSync } from 'node:child_process';
import { mkdtempSync, writeFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { checkRelease } from './check-release-architecture.mjs';

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
test('accepts only the exact successful development promotion', () => {
  assert.equal(checkRelease(context, executor()), context.headSha);
});
for (const change of [{ headRepository: 'fork/repo' }, { headRef: 'feature' }, { baseRef: 'development' },
  { event: 'push' }, { headSha: 'bbbbbbb' }]) {
  test(`rejects wrong promotion context ${JSON.stringify(change)}`, () => {
    assert.throws(() => checkRelease({ ...context, ...change }, executor()));
  });
}
test('rejects diverged main and merge-only changes', () => {
  assert.throws(() => checkRelease(context, executor([], { diverged: true })), /ancestor/);
  assert.throws(() => checkRelease(context, executor([], { changed: true })), /tree differs/);
});
for (const change of [{ event: 'pull_request' }, { head_sha: 'c'.repeat(40) }, { conclusion: 'failure' },
  { status: 'in_progress' }, { head_branch: 'feature' }, { path: '.github/workflows/build.yml' },
  { head_repository: { full_name: 'fork/repo' } }]) {
  test(`rejects unrelated or incomplete CI ${JSON.stringify(change)}`, () => {
    assert.throws(() => checkRelease(context, executor([{ ...successful, ...change }])), /No successful/);
  });
}
test('missing CI and API failures fail closed', () => {
  assert.throws(() => checkRelease(context, executor([])), /No successful/);
  assert.throws(() => checkRelease(context, () => { throw new Error('unavailable'); }), /unavailable/);
});

test('real Git ancestry and tree checks accept promotion but reject release-only edits', t => {
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
  assert.equal(checkRelease({ ...context, baseSha, headSha }, execute), headSha);
  commit('release-only edit');
  assert.throws(() => checkRelease({ ...context, baseSha, headSha }, execute), /tree differs/);
});
