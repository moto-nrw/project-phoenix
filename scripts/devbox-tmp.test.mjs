import assert from 'node:assert/strict';
import { spawn, spawnSync } from 'node:child_process';
import { chmodSync, copyFileSync, existsSync, mkdirSync, mkdtempSync, readFileSync, readdirSync, rmSync, symlinkSync, utimesSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import test from 'node:test';
import { unchangedForHour } from './cleanup-devbox-tmp.mjs';

function fixture(t) {
  const base = new URL('../tmp/', import.meta.url).pathname;
  mkdirSync(base, { recursive: true });
  const root = mkdtempSync(join(base, 'moto-devbox-tmp-'));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  mkdirSync(join(root, 'scripts'));
  for (const file of ['devbox-tmpdir.sh', 'cleanup-devbox-tmp.mjs']) {
    copyFileSync(new URL(file, import.meta.url), join(root, 'scripts', file));
  }
  assert.equal(spawnSync('git', ['init', '-q', root]).status, 0);
  const temp = join(root, '.git/moto-nix-tmp');
  const env = { ...process.env };
  delete env.TMPDIR;
  const setup = (overrides = {}) => spawnSync('bash', ['-c', 'source scripts/devbox-tmpdir.sh || exit; printf "%s" "$TMPDIR"'], {
    cwd: root, env: { ...env, ...overrides }, encoding: 'utf8',
  });
  const cleanup = (elapsed = 0) => spawnSync(process.execPath, [
    '--import', `data:text/javascript,Date.now = () => ${Date.now() + elapsed}`,
    'scripts/cleanup-devbox-tmp.mjs',
  ], {
    cwd: root, env: { ...env, TMPDIR: temp }, encoding: 'utf8',
  });
  const tree = name => { const path = join(temp, name); mkdirSync(path, { recursive: true }); writeFileSync(join(path, 'manifest.json'), '{}'); return path; };
  return { root, temp, setup, cleanup, tree };
}

test('bootstrap selects a private repo path and respects explicit TMPDIR, including spaces', t => {
  const f = fixture(t);
  assert.equal(f.setup().stdout, f.temp);
  assert.equal(f.setup({ TMPDIR: '/caller/custom temp' }).stdout, '/caller/custom temp');
  assert.equal(f.setup({ TMPDIR: '/tmp' }).stdout, '/tmp');
  const result = spawnSync('stat', process.platform === 'darwin' ? ['-f', '%Lp', f.temp] : ['-c', '%a', f.temp], { encoding: 'utf8' });
  assert.equal(result.stdout.trim(), '700');
});

test('bootstrap rejects symlinked namespace without touching its target', t => {
  const f = fixture(t);
  const target = join(f.root, 'unrelated');
  mkdirSync(target);
  symlinkSync(target, f.temp);
  assert.notEqual(f.setup().status, 0);
  assert.ok(existsSync(target));
});

test('cleanup removes only dead, unreferenced Nix trees, preserving live PIDs, links and unknown names', t => {
  const f = fixture(t);
  assert.equal(f.setup().status, 0);
  const orphan = f.tree('nix-2147483647-123');
  const live = f.tree(`nix-${process.pid}-456`);
  const unknown = f.tree('nix-not-a-pid');
  const target = join(f.root, 'foreign');
  mkdirSync(target);
  symlinkSync(target, join(f.temp, 'nix-2147483646-1'));
  assert.equal(f.cleanup(61 * 60 * 1000).status, 0);
  assert.equal(existsSync(orphan), false);
  for (const path of [live, unknown, target, join(f.temp, 'nix-2147483646-1')]) assert.ok(existsSync(path), path);
});

test('an open descriptor or cwd retains an orphan until the referencing process exits', async t => {
  const f = fixture(t);
  assert.equal(f.setup().status, 0);
  const orphan = f.tree('nix-2147483647-123');
  const child = spawn(process.execPath, ['-e', 'require("fs").openSync("manifest.json", "r"); process.stdout.write("ready"); process.stdin.resume()'], { cwd: orphan });
  t.after(() => { if (child.exitCode === null) child.kill(); });
  await new Promise(resolve => child.stdout.once('data', resolve));
  assert.equal(f.cleanup(61 * 60 * 1000).status, 0);
  assert.ok(existsSync(orphan));
  child.stdin.end();
  await new Promise(resolve => child.once('exit', resolve));
  assert.equal(f.cleanup(61 * 60 * 1000).status, 0);
  assert.equal(existsSync(orphan), false);
});

test('cleanup refuses a custom or publicly writable namespace', t => {
  const f = fixture(t);
  assert.equal(f.setup().status, 0);
  const orphan = f.tree('nix-2147483647-123');
  chmodSync(f.temp, 0o755);
  assert.equal(f.cleanup().status, 0);
  assert.ok(existsSync(orphan));
  chmodSync(f.temp, 0o700);
  const result = spawnSync(process.execPath, ['scripts/cleanup-devbox-tmp.mjs'], {
    cwd: f.root, env: { ...process.env, TMPDIR: '/tmp' }, encoding: 'utf8',
  });
  assert.equal(result.status, 0);
  assert.ok(existsSync(orphan));
});

test('.envrc sets TMPDIR before the first Devbox call', t => {
  const f = fixture(t);
  copyFileSync(new URL('../.envrc', import.meta.url), join(f.root, '.envrc'));
  const bin = join(f.root, 'bin');
  mkdirSync(bin);
  writeFileSync(join(bin, 'devbox'), '#!/bin/bash\nprintf \'printf "%%s" "$TMPDIR" > "$FIXTURE_RESULT"\\n\'\n', { mode: 0o755 });
  const result = spawnSync('bash', ['-c', 'watch_file() { :; }; dotenv_if_exists() { :; }; source .envrc'], {
    cwd: f.root,
    env: { ...process.env, TMPDIR: '', PATH: `${bin}:${process.env.PATH}`, FIXTURE_RESULT: join(f.root, 'result') },
    encoding: 'utf8',
  });
  assert.equal(result.status, 0, result.stderr);
  assert.equal(readFileSync(join(f.root, 'result'), 'utf8'), f.temp);
});

test('100 orphan trees and parallel cleanup retain live trees and bound stale growth', async t => {
  const f = fixture(t);
  assert.equal(f.setup().status, 0);
  for (let i = 0; i < 100; i++) f.tree(`nix-2147483647-${i}`);
  const live = f.tree(`nix-${process.pid}-100`);
  const run = () => new Promise(resolve => {
    const child = spawn(process.execPath, [
      '--import', `data:text/javascript,Date.now = () => ${Date.now() + 61 * 60 * 1000}`,
      'scripts/cleanup-devbox-tmp.mjs',
    ], { cwd: f.root, env: { ...process.env, TMPDIR: f.temp } });
    child.on('close', code => resolve(code));
  });
  assert.deepEqual(await Promise.all([run(), run(), run()]), [0, 0, 0]);
  // A racing sweep may stop after another removes a candidate. The next pass
  // must finish, with no lock/cache state left behind by any process.
  assert.equal(f.cleanup(61 * 60 * 1000).status, 0);
  assert.deepEqual(readdirSync(f.temp), [live.slice(f.temp.length + 1)]);
});


test('the one-hour fallback checks nested entries and both timestamps', t => {
  const f = fixture(t);
  assert.equal(f.setup().status, 0);
  const tree = f.tree('nix-2147483647-123');
  const now = Date.now();
  assert.equal(unchangedForHour(tree, now + 30 * 60 * 1000), false);
  assert.equal(unchangedForHour(tree, now + 61 * 60 * 1000), true);
  const changed = new Date(now + 60 * 60 * 1000);
  utimesSync(join(tree, 'manifest.json'), changed, changed);
  assert.equal(unchangedForHour(tree, now + 61 * 60 * 1000), false);
});

test('success, error and controlled termination leave only eligible leftovers for the next sweep', async t => {
  for (const command of ['true', 'return 7', 'kill -TERM $$']) {
    await t.test(command, t => {
      const f = fixture(t);
      copyFileSync(new URL('../.envrc', import.meta.url), join(f.root, '.envrc'));
      const bin = join(f.root, 'bin');
      mkdirSync(bin);
      writeFileSync(join(bin, 'devbox'), `#!/bin/bash
mkdir -p "$TMPDIR/nix-$$-123"
printf '{}' > "$TMPDIR/nix-$$-123/manifest.json"
printf '%s\\n' "$FIXTURE_COMMAND"
`, { mode: 0o755 });
      const result = spawnSync('bash', ['-c', 'watch_file() { :; }; dotenv_if_exists() { :; }; source .envrc'], {
        cwd: f.root,
        env: { ...process.env, TMPDIR: '', PATH: `${bin}:${process.env.PATH}`, FIXTURE_COMMAND: command },
        encoding: 'utf8',
      });
      if (command === 'return 7') assert.equal(result.status, 7);
      else if (command.startsWith('kill')) assert.equal(result.signal, 'SIGTERM');
      else assert.equal(result.status, 0, result.stderr);
      assert.equal(f.cleanup(61 * 60 * 1000).status, 0);
      assert.deepEqual(readdirSync(f.temp), []);
    });
  }
});

test('worktrees share the same private namespace', t => {
  const f = fixture(t);
  assert.equal(f.setup().status, 0);
  assert.equal(spawnSync('git', ['-C', f.root, '-c', 'user.name=Fixture', '-c', 'user.email=fixture@example.invalid', 'commit', '--allow-empty', '-qm', 'fixture']).status, 0);
  const worktree = join(f.root, 'review');
  assert.equal(spawnSync('git', ['-C', f.root, 'worktree', 'add', '--detach', worktree], { encoding: 'utf8' }).status, 0);
  mkdirSync(join(worktree, 'scripts'));
  copyFileSync(new URL('./devbox-tmpdir.sh', import.meta.url), join(worktree, 'scripts/devbox-tmpdir.sh'));
  const result = spawnSync('bash', ['-c', 'source scripts/devbox-tmpdir.sh; printf "%s" "$TMPDIR"'], {
    cwd: worktree, env: { ...process.env, TMPDIR: '' }, encoding: 'utf8',
  });
  assert.equal(result.status, 0, result.stderr);
  assert.equal(result.stdout, f.temp);
});
