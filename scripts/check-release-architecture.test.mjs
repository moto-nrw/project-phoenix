import { gitTestEnvironment, initGitFixture } from './test-helpers/git-fixture.mjs';
import assert from 'node:assert/strict';
import test from 'node:test';
import { execFileSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import { mkdtempSync, mkdirSync, existsSync, readFileSync, writeFileSync, rmSync } from 'node:fs';
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
  const env = gitTestEnvironment();
  const git = (...args) => execFileSync('git', args, { cwd: root, env, encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'] }).trim();
  initGitFixture(git); git('config', 'user.name', 'Release test'); git('config', 'user.email', 'test@example.invalid');
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

// Exercise both script-language helpers against real Git, including inherited
// command config and hook-exported repository/identity variables.
test('Node and shell Git fixtures ignore inherited configuration and hooks', async t => {
  for (const language of ['node', 'shell']) {
    await t.test(language, t => {
      const root = mkdtempSync(join(tmpdir(), 'moto-hostile-git-'));
      t.after(() => rmSync(root, { recursive: true, force: true }));
      const hooks = join(root, 'hooks');
      mkdirSync(hooks);
      writeFileSync(join(hooks, 'pre-commit'), '#!/bin/sh\nprintf ran > "$PHX_GIT_FIXTURE_HOOK_MARKER"\nexit 1\n', { mode: 0o755 });
      const config = `[core]\n hooksPath = ${hooks}\n[commit]\n gpgsign = true\n[gpg]\n program = missing-fixture-gpg\n[fixture]\n inherited = true\n`;
      const global = join(root, 'global.gitconfig');
      writeFileSync(global, config);
      const marker = join(root, 'hook-ran');
      const env = {
        ...process.env, GIT_CONFIG_GLOBAL: global, GIT_CONFIG_SYSTEM: global, GIT_CONFIG_NOSYSTEM: '0',
        GIT_CONFIG: global, GIT_CONFIG_COUNT: '1', GIT_CONFIG_KEY_0: 'core.hooksPath', GIT_CONFIG_VALUE_0: hooks,
        GIT_CONFIG_PARAMETERS: "'fixture.inherited=parameters'", GIT_DIR: join(root, 'foreign.git'), GIT_WORK_TREE: root,
        GIT_AUTHOR_NAME: 'Inherited author', GIT_AUTHOR_EMAIL: 'inherited@example.invalid', GIT_TEMPLATE_DIR: root,
        PHX_GIT_FIXTURE_HOOK_MARKER: marker,
      };
      let git;
      if (language === 'node') {
        const isolated = gitTestEnvironment(env);
        git = (...args) => execFileSync('git', args, { cwd: root, env: isolated, encoding: 'utf8' }).trim();
        initGitFixture(git);
      } else {
        const helper = fileURLToPath(new URL('./test-helpers/git-fixture.sh', import.meta.url));
        execFileSync('bash', ['-euc', 'source "$1"; init_git_fixture "$2"', 'fixture', helper, root], { env });
        git = (...args) => execFileSync('bash', ['-euc', 'source "$1"; shift; git "$@"', 'fixture', helper, ...args], { cwd: root, env, encoding: 'utf8' }).trim();
      }
      git('commit', '--allow-empty', '-qm', 'isolated fixture');
      assert.equal(git('log', '-1', '--format=%an <%ae>'), 'Git fixture <fixture@example.invalid>');
      assert.equal(git('config', '--local', '--get', 'core.hooksPath'), '/dev/null');
      assert.doesNotMatch(git('config', '--list'), /fixture\.inherited/);
      assert.equal(existsSync(marker), false, 'caller hook must not run');
      assert.equal(readFileSync(global, 'utf8'), config, 'caller configuration must stay unchanged');
      assert.equal(env.GIT_CONFIG_GLOBAL, global, 'isolation must not mutate the caller environment');
    });
  }
});
