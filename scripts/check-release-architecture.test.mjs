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
// development has no push run; the release pull request's own run carries the
// full suites, so the gate must not wait for development CI.
function executor(options = {}) {
  return (command, args) => {
    if (command !== 'git') assert.fail(`unexpected ${command} call`);
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
  await assert.rejects(() => checkRelease(context, executor({ diverged: true })), /ancestor/);
  await assert.rejects(() => checkRelease(context, executor({ changed: true })), /tree differs/);
});
test('Git failures fail closed', async () => {
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
  const execute = (command, args) => command === 'git' ? git(...args) : assert.fail(`unexpected ${command} call`);
  assert.equal(await checkRelease({ ...context, baseSha, headSha }, execute), headSha);
  commit('release-only edit');
  await assert.rejects(() => checkRelease({ ...context, baseSha, headSha }, execute), /tree differs/);
});
