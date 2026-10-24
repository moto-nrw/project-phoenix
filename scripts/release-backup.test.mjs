import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { mkdtempSync, mkdirSync, writeFileSync, readFileSync, rmSync, existsSync, utimesSync, symlinkSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import test from 'node:test';

const scripts = dirname(fileURLToPath(import.meta.url));
function fixture(t, target = 'staging') {
  const root = mkdtempSync(join(tmpdir(), 'moto-release-'));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  const cwd = join(root, target);
  mkdirSync(cwd); mkdirSync(join(root, 'bin'));
  writeFileSync(join(root, 'bin/docker'), `#!/bin/sh\nexec '${process.execPath}' '${join(scripts, 'test-support/release-docker.mjs')}' "$@"\n`, { mode: 0o755 });
  for (const [file, content] of Object.entries({ '.env': 'MODE=old\n', '.deploy-state': 'CURRENT_SHA=aaaaaaa\n',
    'docker-compose.yml': 'old config\n', '.env.new': 'MODE=new\n', 'docker-compose.yml.new': 'new config\n' })) {
    writeFileSync(join(cwd, file), content);
  }
  const log = join(root, 'docker.log');
  const env = { ...process.env, DEPLOY_DIR: target, DEPLOY_SHA: 'bbbbbbb', BACKUP_RETENTION: '2', RELEASE_SNAPSHOT_MAX_AGE_DAYS: '30',
    PATH: `${join(root, 'bin')}:${process.env.PATH}`, RELEASE_TEST_LOG: log };
  const bundle = join(root, 'backups', target, 'release-20260914T010000Z-bbbbbbb');
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

test('demo deploy pins its backend and frontend images and can restore its own snapshot', t => {
  const f = fixture(t, 'demo');
  writeFileSync(join(f.cwd, 'docker-compose.yml.new'),
    'services:\n  server:\n    image: ghcr.io/moto-nrw/phoenix-server:demo\n  frontend:\n    image: ghcr.io/moto-nrw/phoenix-frontend:demo\n');
  const result = f.run('deploy-remote.sh');
  assert.equal(result.status, 0, result.stdout + result.stderr);
  const compose = readFileSync(join(f.cwd, 'docker-compose.yml'), 'utf8');
  assert.match(compose, /phoenix-server:demo-bbbbbbb/);
  assert.match(compose, /phoenix-frontend:demo-bbbbbbb/);
  const state = readFileSync(join(f.cwd, '.deploy-state'), 'utf8');
  const id = state.match(/BACKUP_ID=(.+)/)[1];
  const bundle = join(f.root, 'backups/demo', id);
  assert.equal(readFileSync(join(bundle, 'environment'), 'utf8'), 'demo\n');
  const rollback = f.run('rollback-remote.sh');
  assert.equal(rollback.status, 0, rollback.stdout + rollback.stderr);
  assert.equal(readFileSync(join(f.cwd, '.env'), 'utf8'), 'MODE=old\n');
});

for (const target of ['staging', 'production']) {
  test(`${target} deploy keeps unprefixed SHA tags for backend and frontend`, t => {
    const f = fixture(t, target);
    writeFileSync(join(f.cwd, 'docker-compose.yml.new'),
      `services:\n  server:\n    image: ghcr.io/moto-nrw/phoenix-server:${target}\n  frontend:\n    image: ghcr.io/moto-nrw/phoenix-frontend:${target}\n`);
    const result = f.run('deploy-remote.sh');
    assert.equal(result.status, 0, result.stdout + result.stderr);
    const compose = readFileSync(join(f.cwd, 'docker-compose.yml'), 'utf8');
    assert.match(compose, /phoenix-server:bbbbbbb/);
    assert.match(compose, /phoenix-frontend:bbbbbbb/);
  });
}

const demoStack = 'services:\n  server:\n    image: ghcr.io/moto-nrw/phoenix-server:demo\n  frontend:\n    image: ghcr.io/moto-nrw/phoenix-frontend:demo\n' +
  '  demo-runtime:\n    image: ghcr.io/moto-nrw/phoenix-server:demo\n';
// server and frontend gate the release; the sidecar follows, recreated because
// it lives in the network namespace of the server container just replaced.
const sidecarStart = /^compose up -d --wait --remove-orphans server frontend\ncompose up -d --no-deps --force-recreate demo-runtime$/m;
test('first demo deploy with the sidecar starts it although the old stack never had it', t => {
  const f = fixture(t, 'demo');
  writeFileSync(join(f.cwd, 'docker-compose.yml.new'), demoStack);
  const result = f.run('deploy-remote.sh');
  assert.equal(result.status, 0, result.stdout + result.stderr);
  // The previous file has no such service; naming it would fail the stop.
  assert.match(f.calls(), /^compose stop server frontend$/m);
  assert.match(f.calls(), sidecarStart);
  // Same immutable backend revision as the server.
  assert.match(readFileSync(join(f.cwd, 'docker-compose.yml'), 'utf8'),
    /demo-runtime:\n    image: ghcr.io\/moto-nrw\/phoenix-server:demo-bbbbbbb/);
});
test('repeated demo deploy stops the sidecar before the snapshot and restarts it', t => {
  const f = fixture(t, 'demo');
  writeFileSync(join(f.cwd, 'docker-compose.yml'), demoStack);
  writeFileSync(join(f.cwd, 'docker-compose.yml.new'), demoStack);
  const result = f.run('deploy-remote.sh');
  assert.equal(result.status, 0, result.stdout + result.stderr);
  const calls = f.calls().trim().split('\n');
  const stop = calls.indexOf('compose stop server frontend demo-runtime');
  const backup = calls.findIndex(line => line.includes('pg_dump '));
  const start = calls.indexOf('compose up -d --no-deps --force-recreate demo-runtime');
  assert.ok(stop >= 0 && backup > stop && start > backup, f.calls());
  const id = readFileSync(join(f.cwd, '.deploy-state'), 'utf8').match(/BACKUP_ID=(.+)/)[1];
  assert.match(readFileSync(join(f.root, 'backups/demo', id, 'images.tsv'), 'utf8'), /^demo-runtime\t/m);
  // Rollback returns the sidecar to the snapshot's state along with the rest.
  writeFileSync(f.log, '');
  const rollback = f.run('rollback-remote.sh');
  assert.equal(rollback.status, 0, rollback.stdout + rollback.stderr);
  assert.match(f.calls(), /^compose stop server frontend demo-runtime$/m);
  assert.match(f.calls(), sidecarStart);
});
// Redeploying the same revision rebuilds the demo frontend under the same tag.
// The pull must not leave the running images nameless, or the snapshot cannot
// resolve their registry digests on Docker's containerd image store.
test('deploy keeps the running images under a local tag before pulling', t => {
  const f = fixture(t, 'demo');
  writeFileSync(join(f.cwd, 'docker-compose.yml'), demoStack);
  writeFileSync(join(f.cwd, 'docker-compose.yml.new'), demoStack);
  const result = f.run('deploy-remote.sh');
  assert.equal(result.status, 0, result.stdout + result.stderr);
  const calls = f.calls().trim().split('\n');
  const pull = calls.findIndex(line => line.endsWith(' pull'));
  const image = `sha256:${'1'.repeat(64)}`;
  for (const tag of ['postgres:moto-release-postgres', 'ghcr.io/moto-nrw/phoenix-server:moto-release-server',
    'ghcr.io/moto-nrw/phoenix-frontend:moto-release-frontend', 'ghcr.io/moto-nrw/phoenix-server:moto-release-demo-runtime']) {
    const index = calls.indexOf(`tag ${image} ${tag}`);
    assert.ok(index >= 0 && index < pull, f.calls());
  }
});
test('failing to keep the running images aborts before the pull', t => {
  const f = fixture(t, 'demo');
  writeFileSync(join(f.cwd, 'docker-compose.yml'), demoStack);
  writeFileSync(join(f.cwd, 'docker-compose.yml.new'), demoStack);
  const result = f.run('deploy-remote.sh', [], { RELEASE_TEST_FAIL: 'tag' });
  assert.equal(result.status, 1, result.stdout + result.stderr);
  assert.match(result.stderr, /running release was not touched/);
  assert.doesNotMatch(f.calls(), / pull$|stop|pg_dump/m);
  assert.equal(readFileSync(join(f.cwd, '.env'), 'utf8'), 'MODE=old\n');
});
test('first deploy without a running stack keeps no images', t => {
  const f = fixture(t, 'demo');
  rmSync(join(f.cwd, 'docker-compose.yml'));
  const result = f.run('release-backup.sh', ['keep-images']);
  assert.equal(result.status, 0, result.stdout + result.stderr);
  assert.equal(existsSync(f.log) ? f.calls() : '', '');
});
// The sidecar seeds for minutes and may fail on its own. Gating the release on
// it would hold the deploy and then restore the database of a healthy application.
test('a sidecar that cannot start neither fails nor rolls back the demo release', t => {
  const f = fixture(t, 'demo');
  writeFileSync(join(f.cwd, 'docker-compose.yml.new'), demoStack);
  const result = f.run('deploy-remote.sh', [], { RELEASE_TEST_FAIL: 'sidecar' });
  assert.equal(result.status, 0, result.stdout + result.stderr);
  assert.match(result.stderr, /WARNING: demo-runtime did not start/);
  assert.doesNotMatch(f.calls(), /pg_restore -U/);
  assert.match(readFileSync(join(f.cwd, '.deploy-state'), 'utf8'), /CURRENT_SHA=bbbbbbb/);
});
test('a running sidecar cannot produce a complete backup', t => {
  const f = fixture(t, 'demo');
  writeFileSync(join(f.cwd, 'docker-compose.yml'), demoStack);
  const result = f.run('release-backup.sh', ['create', f.bundle], { RELEASE_TEST_FAIL: 'running-sidecar' });
  assert.notEqual(result.status, 0);
  assert.match(result.stderr, /demo-runtime is still running/);
  assert.equal(existsSync(join(f.bundle, 'complete')), false);
});
test('failed demo deploy rolls back to a stack without the sidecar', t => {
  const f = fixture(t, 'demo');
  writeFileSync(join(f.cwd, 'docker-compose.yml.new'), demoStack);
  const result = f.run('deploy-remote.sh', [], { RELEASE_TEST_FAIL: 'migrate' });
  assert.equal(result.status, 10, result.stdout + result.stderr);
  assert.equal(f.calls().trim().split('\n').at(-1), 'compose up -d --wait --remove-orphans server frontend');
});
for (const target of ['staging', 'production']) {
  test(`${target} deploy and rollback never name the demo sidecar`, t => {
    const f = fixture(t, target);
    assert.equal(f.run('deploy-remote.sh').status, 0);
    assert.equal(f.run('rollback-remote.sh').status, 0);
    assert.doesNotMatch(f.calls(), /demo-runtime/);
  });
}

for (const source of ['demo', 'staging', 'production']) {
  for (const target of ['demo', 'staging', 'production'].filter(value => value !== source)) {
    test(`${target} rejects a ${source} snapshot before touching services`, t => {
      const f = fixture(t, source); f.backup();
      const result = f.run('restore-db.sh', [f.bundle], { DEPLOY_DIR: target });
      assert.notEqual(result.status, 0);
      assert.match(result.stderr, /Backup belongs to another environment/);
      assert.equal(f.calls(), '');
    });
  }
}
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
test('age retention expires complete and interrupted snapshots below the count limit', t => {
  const f = fixture(t);
  const base = join(f.root, 'backups/staging');
  for (const [id, complete] of [['release-20200101T000000Z-aaaaaaa', true],
    ['release-20200102T000000Z-aaaaaaa', false]]) {
    const path = join(base, id);
    mkdirSync(path, { recursive: true });
    if (complete) writeFileSync(join(path, 'complete'), 'moto-release-backup-v1\n');
    writeFileSync(join(path, 'roles.sql'), 'private');
    // Touching a directory must not extend the lifetime of its contents.
    utimesSync(path, new Date(), new Date());
  }
  const result = f.run('deploy-remote.sh', [], { BACKUP_RETENTION: '7' });
  assert.equal(result.status, 0, result.stdout + result.stderr);
  assert.equal(existsSync(join(base, 'release-20200101T000000Z-aaaaaaa')), false);
  assert.equal(existsSync(join(base, 'release-20200102T000000Z-aaaaaaa')), false);
});
test('count retention removes whole completed sets but preserves recent interrupted snapshots', t => {
  const f = fixture(t);
  const base = join(f.root, 'backups/staging');
  const stamp = new Date().toISOString().replace(/[-:]/g, '').replace(/\.\d{3}Z$/, 'Z');
  const ids = ['aaaaaaa', 'aaaaaab', 'aaaaaac'].map(sha => `release-${stamp}-${sha}`);
  for (const id of ids) {
    mkdirSync(join(base, id), { recursive: true });
    writeFileSync(join(base, id, 'complete'), 'moto-release-backup-v1\n');
    writeFileSync(join(base, id, 'roles.sql'), 'private');
  }
  const interrupted = `release-${stamp}-aaaaaad`;
  mkdirSync(join(base, interrupted));
  const result = f.run('deploy-remote.sh');
  assert.equal(result.status, 0, result.stdout + result.stderr);
  assert.equal(existsSync(join(base, ids[0])), false);
  assert.equal(existsSync(join(base, ids[1])), false);
  assert.equal(existsSync(join(base, ids[2], 'roles.sql')), true);
  assert.equal(existsSync(join(base, interrupted)), true);
});
test('count retention preserves the just-created rollback point after a backward clock adjustment', t => {
  const f = fixture(t);
  const future = join(f.root, 'backups/staging/release-20990101T000000Z-aaaaaaa');
  mkdirSync(future, { recursive: true });
  writeFileSync(join(future, 'complete'), 'moto-release-backup-v1\n');
  const result = f.run('deploy-remote.sh', [], { BACKUP_RETENTION: '1' });
  assert.equal(result.status, 0, result.stdout + result.stderr);
  const state = readFileSync(join(f.cwd, '.deploy-state'), 'utf8');
  const id = state.match(/BACKUP_ID=(.+)/)[1];
  assert.ok(existsSync(join(f.root, 'backups/staging', id, 'complete')));
  assert.equal(existsSync(future), false);
});
test('independent cleanup expires snapshots without Docker and respects environment locks', t => {
  const f = fixture(t);
  const base = join(f.root, 'backups/staging');
  const expired = join(base, 'release-20200101T000000Z-aaaaaaa');
  mkdirSync(expired, { recursive: true });
  mkdirSync(join(f.cwd, '.release-operation.lock'));
  let result = f.run('release-backup-cleanup.sh', [f.cwd]);
  assert.notEqual(result.status, 0);
  assert.ok(existsSync(expired));
  rmSync(join(f.cwd, '.release-operation.lock'), { recursive: true });
  result = f.run('release-backup-cleanup.sh', [f.cwd]);
  assert.equal(result.status, 0, result.stdout + result.stderr);
  assert.equal(existsSync(expired), false);
  assert.equal(existsSync(join(f.cwd, '.release-operation.lock')), false);
  assert.equal(existsSync(f.log), false, 'cleanup must not access Docker');
});
test('retention rejects invalid ages before deleting snapshots or changing a deployment', t => {
  const f = fixture(t);
  const expired = join(f.root, 'backups/staging/release-20200101T000000Z-aaaaaaa');
  mkdirSync(expired, { recursive: true });
  for (const days of ['', '0', '91', '-1', '01', '1.5', '999999999999999999999']) {
    const env = { RELEASE_SNAPSHOT_MAX_AGE_DAYS: days };
    assert.notEqual(f.run('release-backup-cleanup.sh', [f.cwd], env).status, 0, days);
    assert.notEqual(f.run('deploy-remote.sh', [], env).status, 0, days);
    assert.ok(existsSync(expired), days);
    assert.equal(existsSync(join(f.cwd, '.release-operation.lock')), false, days);
  }
  assert.equal(existsSync(f.log), false, 'invalid configuration must fail before Docker');
});
test('configured age changes expiry and cleanup never follows snapshot symlinks', t => {
  const f = fixture(t);
  const base = join(f.root, 'backups/staging');
  const stamp = new Date(Date.now() - 60 * 86400000).toISOString().replace(/[-:]/g, '').replace(/\.\d{3}Z$/, 'Z');
  const snapshot = join(base, `release-${stamp}-aaaaaaa`);
  mkdirSync(snapshot, { recursive: true });
  writeFileSync(join(snapshot, 'complete'), 'moto-release-backup-v1\n');
  const outside = join(f.root, 'outside');
  mkdirSync(outside);
  writeFileSync(join(outside, 'roles.sql'), 'private');
  symlinkSync(outside, join(base, 'release-20200101T000000Z-bbbbbbb'));
  let result = f.run('release-backup-cleanup.sh', [f.cwd], { RELEASE_SNAPSHOT_MAX_AGE_DAYS: '90' });
  assert.equal(result.status, 0, result.stdout + result.stderr);
  assert.ok(existsSync(snapshot));
  result = f.run('release-backup-cleanup.sh', [f.cwd]);
  assert.equal(result.status, 0, result.stdout + result.stderr);
  assert.equal(existsSync(snapshot), false);
  assert.ok(existsSync(join(outside, 'roles.sql')));
});
test('failed deletion fails cleanup and releases the operation lock without reporting success', t => {
  const f = fixture(t);
  const expired = join(f.root, 'backups/staging/release-20200101T000000Z-aaaaaaa');
  mkdirSync(expired, { recursive: true });
  writeFileSync(join(f.root, 'bin/rm'), '#!/bin/sh\nexit 1\n', { mode: 0o755 });
  const result = f.run('release-backup-cleanup.sh', [f.cwd]);
  assert.notEqual(result.status, 0);
  assert.doesNotMatch(result.stdout, /retention complete/);
  assert.ok(existsSync(expired));
  assert.equal(existsSync(join(f.cwd, '.release-operation.lock')), false);
});
test('expiry includes the exact UTC cutoff and retains a newer snapshot', t => {
  const f = fixture(t);
  const base = join(f.root, 'backups/staging');
  writeFileSync(join(f.root, 'bin/date'), '#!/bin/sh\nprintf "%s\\n" 20261001T120000Z\n', { mode: 0o755 });
  for (const stamp of ['20261001T115959Z', '20261001T120000Z', '20261001T120001Z']) {
    const path = join(base, `release-${stamp}-aaaaaaa`);
    mkdirSync(path, { recursive: true });
    writeFileSync(join(path, 'complete'), 'moto-release-backup-v1\n');
  }
  const result = f.run('release-backup-cleanup.sh', [f.cwd], { RELEASE_SNAPSHOT_MAX_AGE_DAYS: '1', BACKUP_RETENTION: '7' });
  assert.equal(result.status, 0, result.stdout + result.stderr);
  assert.equal(existsSync(join(base, 'release-20261001T115959Z-aaaaaaa')), false);
  assert.equal(existsSync(join(base, 'release-20261001T120000Z-aaaaaaa')), false);
  assert.ok(existsSync(join(base, 'release-20261001T120001Z-aaaaaaa')));
});
// Only a healthy, recorded release frees the disk: the prune must neither run
// before the snapshot nor on any rollback path, and its failure is not a failed
// release.
test('successful deployment prunes unused images last', t => {
  const f = fixture(t);
  const result = f.run('deploy-remote.sh');
  assert.equal(result.status, 0, result.stdout + result.stderr);
  assert.equal(f.calls().trim().split('\n').at(-1), 'image prune -af');
});
test('aborted and rolled back deployments keep every image', t => {
  for (const [step, status] of [['preflight', 1], ['stop', 1], ['roles', 1], ['migrate', 10], ['health', 11]]) {
    const f = fixture(t);
    assert.equal(f.run('deploy-remote.sh', [], { RELEASE_TEST_FAIL: step }).status, status, step);
    assert.doesNotMatch(f.calls(), /prune/, step);
  }
});
test('a failing prune does not fail the healthy release', t => {
  const f = fixture(t);
  const result = f.run('deploy-remote.sh', [], { RELEASE_TEST_FAIL: 'prune' });
  assert.equal(result.status, 0, result.stdout + result.stderr);
  assert.match(result.stderr, /WARNING: image prune failed/);
  assert.match(readFileSync(join(f.cwd, '.deploy-state'), 'utf8'), /CURRENT_SHA=bbbbbbb/);
});
