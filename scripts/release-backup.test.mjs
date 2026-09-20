import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { mkdtempSync, mkdirSync, writeFileSync, readFileSync, rmSync, existsSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import test from 'node:test';

const scripts = dirname(fileURLToPath(import.meta.url));
function fixture(t) {
  const root = mkdtempSync(join(tmpdir(), 'moto-release-'));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  const cwd = join(root, 'staging');
  mkdirSync(cwd); mkdirSync(join(root, 'bin'));
  writeFileSync(join(root, 'bin/docker'), `#!/bin/sh\nexec '${process.execPath}' '${join(scripts, 'test-support/release-docker.mjs')}' "$@"\n`, { mode: 0o755 });
  for (const [file, content] of Object.entries({ '.env': 'MODE=old\n', '.deploy-state': 'CURRENT_SHA=aaaaaaa\n',
    'docker-compose.yml': 'old config\n', '.env.new': 'MODE=new\n', 'docker-compose.yml.new': 'new config\n' })) {
    writeFileSync(join(cwd, file), content);
  }
  const log = join(root, 'docker.log');
  const env = { ...process.env, DEPLOY_DIR: 'staging', DEPLOY_SHA: 'bbbbbbb', BACKUP_RETENTION: '2',
    PATH: `${join(root, 'bin')}:${process.env.PATH}`, RELEASE_TEST_LOG: log };
  const bundle = join(root, 'backups/staging/release-20260914T010000Z-bbbbbbb');
  const run = (file, args = [], extra = {}) => spawnSync('bash', [join(scripts, file),
    ...(['deploy-remote.sh', 'rollback-remote.sh'].includes(file) ? [cwd] : []), ...args], {
    cwd, env: { ...env, ...extra }, encoding: 'utf8', timeout: 30000,
  });
  const backup = () => {
    const result = run('release-backup.sh', ['create', bundle]);
    assert.equal(result.status, 0, result.stdout + result.stderr);
    writeFileSync(log, '');
  };
  return { root, cwd, bundle, log, run, backup, calls: () => readFileSync(log, 'utf8') };
}

test('complete snapshot restores all components and the saved deployment state', t => {
  const f = fixture(t); f.backup();
  const result = f.run('restore-db.sh', [f.bundle]);
  assert.equal(result.status, 0, result.stdout + result.stderr);
  assert.match(f.calls(), /pg_restore.*--exit-on-error/);
  assert.match(f.calls(), /find \/volume -mindepth/);
  assert.equal(readFileSync(join(f.cwd, '.env'), 'utf8'), 'MODE=old\n');
});
for (const file of ['roles.sql', 'uploads.tar.gz', '.env', 'compose.yml', 'images.tsv', 'database.dump']) {
  test(`missing ${file} blocks restore before stop or deletion`, t => {
    const f = fixture(t); f.backup(); rmSync(join(f.bundle, file));
    const result = f.run('restore-db.sh', [f.bundle]);
    assert.notEqual(result.status, 0);
    assert.doesNotMatch(f.calls(), /stop|DROP DATABASE|find \/volume/);
  });
}
test('corruption and wrong environment fail preflight', t => {
  const f = fixture(t); f.backup();
  assert.notEqual(f.run('restore-db.sh', [f.bundle], { DEPLOY_DIR: 'production' }).status, 0);
  writeFileSync(join(f.bundle, 'database.dump'), 'corrupt');
  assert.notEqual(f.run('restore-db.sh', [f.bundle]).status, 0);
  assert.doesNotMatch(f.calls(), /stop|DROP DATABASE/);
});
test('unavailable old images block restore before stopping the application', t => {
  const f = fixture(t); f.backup();
  assert.notEqual(f.run('restore-db.sh', [f.bundle], { RELEASE_TEST_FAIL: 'unavailable-image' }).status, 0);
  assert.doesNotMatch(f.calls(), /stop|DROP DATABASE/);
});
for (const step of ['restore-roles', 'restore-db', 'restore-uploads']) {
  test(`${step} failure is not reported as successful restore`, t => {
    const f = fixture(t); f.backup();
    const result = f.run('restore-db.sh', [f.bundle], { RELEASE_TEST_FAIL: step });
    assert.notEqual(result.status, 0);
    assert.doesNotMatch(result.stdout, /configuration restored/);
    assert.doesNotMatch(f.calls(), /up.*frontend/);
  });
}
test('running writers and invalid archives cannot produce a complete backup', t => {
  for (const step of ['running', 'archive']) {
    const f = fixture(t);
    assert.notEqual(f.run('release-backup.sh', ['create', f.bundle], { RELEASE_TEST_FAIL: step }).status, 0);
    assert.equal(existsSync(join(f.bundle, 'complete')), false);
  }
});
test('manual rollback uses the recorded backup, not an arbitrary target image', t => {
  const f = fixture(t); f.backup();
  writeFileSync(join(f.cwd, '.deploy-state'), `CURRENT_SHA=bbbbbbb\nBACKUP_ID=${f.bundle.split('/').at(-1)}\n`);
  const result = f.run('rollback-remote.sh');
  assert.equal(result.status, 0, result.stdout + result.stderr);
  assert.equal(readFileSync(join(f.cwd, '.deploy-state'), 'utf8'), 'CURRENT_SHA=aaaaaaa\n');
  assert.match(f.calls(), /up -d --wait --remove-orphans server frontend/);
});
test('deployment stop or backup failure never reaches migrations', t => {
  for (const step of ['stop', 'roles']) {
    const f = fixture(t);
    assert.equal(f.run('deploy-remote.sh', [], { RELEASE_TEST_FAIL: step }).status, 1);
    assert.doesNotMatch(f.calls(), /run --rm migrate/);
    assert.equal(readFileSync(join(f.cwd, '.env'), 'utf8'), 'MODE=old\n');
  }
  const f = fixture(t);
  writeFileSync(join(f.root, 'backups'), 'not a directory');
  assert.equal(f.run('deploy-remote.sh').status, 1);
  assert.doesNotMatch(f.calls(), /stop|run --rm migrate/);
});
// The whole point of the preflight: a migration that would refuse on data
// somebody has to correct must abort the release while the previous one is
// still serving, rather than after the stop and the backup, where the only way
// back is a restore.
test('preflight failure aborts before the application is stopped', t => {
  const f = fixture(t);
  const result = f.run('deploy-remote.sh', [], { RELEASE_TEST_FAIL: 'preflight' });
  assert.equal(result.status, 1, result.stdout + result.stderr);
  // The flags are the assertion: without them the preflight would run the
  // PREVIOUS image against the old config and pass while checking the wrong
  // binary's preconditions, which is the exact failure this step exists to stop.
  assert.match(f.calls(),
    /--env-file \.env\.new -f docker-compose\.yml\.new run --rm --no-deps migrate \.\/main migrate preflight/);
  assert.doesNotMatch(f.calls(), /stop|pg_dump|pg_restore/);
  assert.equal(readFileSync(join(f.cwd, '.env'), 'utf8'), 'MODE=old\n');
  assert.equal(readFileSync(join(f.cwd, 'docker-compose.yml'), 'utf8'), 'old config\n');
});
test('a passing preflight runs before the stop and does not migrate', t => {
  const f = fixture(t);
  const result = f.run('deploy-remote.sh', [], { RELEASE_TEST_FAIL: 'stop' });
  assert.equal(result.status, 1, result.stdout + result.stderr);
  const calls = f.calls().trim().split('\n');
  const preflight = calls.findIndex(line => line.includes('migrate preflight'));
  const stop = calls.findIndex(line => /stop server frontend/.test(line));
  assert.ok(preflight >= 0 && stop > preflight, f.calls());
  assert.doesNotMatch(f.calls(), /run --rm migrate$/m);
});

test('retired evidence argument is rejected before deployment', t => {
  const f = fixture(t);
  const result = f.run('deploy-remote.sh', [join(f.root, 'evidence.json')]);
  assert.equal(result.status, 1);
  assert.match(result.stderr, /evidence files are no longer supported/);
  assert.equal(existsSync(f.log), false);
});

test('migration failure invokes complete automatic rollback', t => {
  const f = fixture(t);
  const result = f.run('deploy-remote.sh', [], { RELEASE_TEST_FAIL: 'migrate' });
  assert.equal(result.status, 10, result.stdout + result.stderr);
  assert.match(f.calls(), /pg_restore.*--exit-on-error/);
  assert.match(f.calls(), /find \/volume/);
  assert.equal(readFileSync(join(f.cwd, '.env'), 'utf8'), 'MODE=old\n');
});
test('successful deployment records a complete snapshot', t => {
  const f = fixture(t); const result = f.run('deploy-remote.sh');
  assert.equal(result.status, 0, result.stdout + result.stderr);
  const calls = f.calls().trim().split('\n');
  const preflight = calls.findIndex(line => line.includes('migrate preflight'));
  const stop = calls.findIndex(line => line.includes('stop server frontend'));
  const backup = calls.findIndex(line => line.includes('pg_dump '));
  const verifyBackup = calls.findIndex(line => line.includes('pg_restore --list'));
  const migrate = calls.findIndex(line => line === 'compose run --rm migrate');
  const start = calls.findIndex(line => line.includes('up -d --wait --remove-orphans server frontend'));
  assert.ok(preflight >= 0 && stop > preflight && backup > stop &&
    verifyBackup > backup && migrate > verifyBackup && start > migrate, f.calls());
  assert.doesNotMatch(f.calls(), /student-contract-evidence/);
  const state = readFileSync(join(f.cwd, '.deploy-state'), 'utf8');
  assert.match(state, /CURRENT_SHA=bbbbbbb\nPREVIOUS_SHA=aaaaaaa/);
  const id = state.match(/BACKUP_ID=(.+)/)[1];
  assert.ok(existsSync(join(f.root, 'backups/staging', id, 'complete')));
});
test('failed new and restored healthchecks leave the application stopped', t => {
  const f = fixture(t);
  const result = f.run('deploy-remote.sh', [], { RELEASE_TEST_FAIL: 'health' });
  assert.equal(result.status, 11, result.stdout + result.stderr);
  assert.match(f.calls().trim().split('\n').at(-1), /stop server frontend/);
});
test('retention removes whole completed sets and preserves incomplete backups', t => {
  const f = fixture(t);
  const base = join(f.root, 'backups/staging');
  for (const name of ['release-20200101', 'release-20200102', 'release-20200103']) {
    mkdirSync(join(base, name), { recursive: true });
    writeFileSync(join(base, name, 'complete'), 'old');
    writeFileSync(join(base, name, 'roles.sql'), 'private');
  }
  mkdirSync(join(base, 'release-incomplete'));
  const result = f.run('deploy-remote.sh');
  assert.equal(result.status, 0, result.stdout + result.stderr);
  assert.equal(existsSync(join(base, 'release-20200101')), false);
  assert.equal(existsSync(join(base, 'release-20200102')), false);
  assert.equal(existsSync(join(base, 'release-20200103/roles.sql')), true);
  assert.equal(existsSync(join(base, 'release-incomplete')), true);
});
