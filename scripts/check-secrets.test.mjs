import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { copyFileSync, existsSync, mkdirSync, mkdtempSync, readFileSync, realpathSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import test from 'node:test';

const source = dirname(fileURLToPath(import.meta.url));
function fixture(t) {
  const root = mkdtempSync(join(tmpdir(), 'phoenix-secret-hook-'));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  const env = { ...process.env, GIT_CONFIG_GLOBAL: '/dev/null', GIT_CONFIG_NOSYSTEM: '1' };
  for (const key of ['GIT_DIR', 'GIT_WORK_TREE', 'GIT_INDEX_FILE', 'GIT_COMMON_DIR', 'GIT_PREFIX']) delete env[key];
  function run(command, args, extra = {}) {
    return spawnSync(command, args, { cwd: root, env: { ...env, ...extra }, encoding: 'utf8' });
  }
  function git(...args) {
    const result = run('git', args);
    assert.equal(result.status, 0, result.stderr);
    return result.stdout.trim();
  }
  git('init', '-q');
  git('config', 'user.name', 'Scanner fixture');
  git('config', 'user.email', 'scanner@example.invalid');
  mkdirSync(join(root, 'scripts'));
  copyFileSync(join(source, 'check-secrets.sh'), join(root, 'scripts/check-secrets.sh'));
  writeFileSync(join(root, '.gitguardian.yaml'), 'version: 2\nsecret:\n  ignored_paths: []\n');
  const bin = join(root, '.devbox/nix/profile/default/bin');
  mkdirSync(bin, { recursive: true });
  writeFileSync(join(bin, 'ggshield'), `#!/bin/bash
printf '%s\\n' "$@" > scanner-args
printf '%s\\n' "$GITGUARDIAN_EXIT_ZERO" "$GITGUARDIAN_SHOW_SECRETS" > scanner-env
exit "\${SCANNER_STATUS:-0}"
`, { mode: 0o755 });
  function commit(text) {
    writeFileSync(join(root, 'fixture.txt'), text);
    git('add', 'fixture.txt');
    git('commit', '-qm', 'fixture');
  }
  commit('base');
  git('branch', 'base');
  return { root, run, git, commit, scan: (args, extra) => run('bash', ['scripts/check-secrets.sh', ...args], extra),
    args: () => readFileSync(join(root, 'scanner-args'), 'utf8').trim().split('\n') };
}

test('pre-commit scans the index with explicit repository config and redacted failures', t => {
  const f = fixture(t);
  const result = f.scan(['pre-commit'], { SCANNER_STATUS: '1', GITGUARDIAN_EXIT_ZERO: 'true', GITGUARDIAN_SHOW_SECRETS: 'true' });
  assert.equal(result.status, 1, result.stderr);
  assert.deepEqual(f.args(), ['--config-path', join(realpathSync(f.root), '.gitguardian.yaml'), '--no-check-for-updates', 'secret', 'scan', 'pre-commit']);
  assert.equal(readFileSync(join(f.root, 'scanner-env'), 'utf8'), 'false\nfalse\n');
});

test('pre-push scans every branch commit, including credentials removed at the tip', t => {
  const f = fixture(t);
  const base = f.git('rev-parse', 'base');
  f.commit('credential added');
  f.commit('credential removed');
  const result = f.scan(['pre-push', 'base']);
  assert.equal(result.status, 0, result.stderr);
  assert.deepEqual(f.args().slice(-2), ['commit-range', `${base}..HEAD`]);
});

test('missing pinned scanner cannot fall back to another installation', t => {
  const f = fixture(t);
  rmSync(join(f.root, '.devbox/nix/profile/default/bin/ggshield'));
  const result = f.scan(['pre-commit']);
  assert.notEqual(result.status, 0);
  assert.match(result.stderr, /Pinned ggshield is missing/);
});

for (const ahead of [false, true]) {
  test(`pre-push skips scanning when HEAD ${ahead ? 'is behind' : 'equals'} the base`, t => {
    const f = fixture(t);
    if (ahead) {
      f.git('checkout', '-q', 'base');
      f.commit('base advanced');
      f.git('checkout', '-q', '--detach', 'HEAD~1');
    }
    const result = f.scan(['pre-push', 'base'], { SCANNER_STATUS: '2' });
    assert.equal(result.status, 0, result.stderr);
    assert.equal(existsSync(join(f.root, 'scanner-args')), false);
  });
}

test('authentication and network failures fail closed', t => {
  const f = fixture(t);
  for (const status of [2, 3]) assert.equal(f.scan(['pre-commit'], { SCANNER_STATUS: String(status) }).status, status);
});

test('unknown modes and invalid comparison refs fail before scanning', t => {
  const f = fixture(t);
  assert.notEqual(f.scan(['--exit-zero']).status, 0);
  assert.notEqual(f.scan(['pre-push', 'missing-ref']).status, 0);
});
