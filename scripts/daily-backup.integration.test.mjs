import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { randomBytes, createHash } from 'node:crypto';
import { mkdtempSync, mkdirSync, writeFileSync, readFileSync, readdirSync, rmSync, utimesSync, chmodSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import test from 'node:test';

const scripts = dirname(fileURLToPath(import.meta.url));
test('encrypted daily backup restores an isolated cluster and files, prunes by age and fails closed', { timeout: 1200000 }, t => {
  const root = mkdtempSync(join(tmpdir(), 'moto-daily-test-'));
  const project = `moto-daily-${process.pid}-${Date.now()}`;
  const volume = `${project}-uploads`;
  const restoredVolume = `${project}-restored`;
  const remote = join(root, 'remote'); mkdirSync(remote);
  const work = join(root, 'work'); mkdirSync(work);
  const marker = randomBytes(24).toString('hex');
  const credential = randomBytes(24).toString('hex');
  const compose = join(root, 'compose.yml');
  const ledger = join(root, 'erasure-ledger');
  writeFileSync(ledger, 'moto-erasure-ledger-v1\n', { mode: 0o600 });
  writeFileSync(join(root, '.env'), `PRIVATE_MARKER=${credential}\nPOSTGRES_PASSWORD=${credential}\n`);
  writeFileSync(compose, `services:
  postgres:
    image: postgres:17-alpine
    network_mode: none
    environment:
      POSTGRES_PASSWORD: ${credential}
      POSTGRES_INITDB_ARGS: --auth-host=scram-sha-256
    volumes: [database:/var/lib/postgresql/data]
    healthcheck:
      test: [CMD, pg_isready, -h, 127.0.0.1, -U, postgres]
      interval: 1s
      retries: 60
  server:
    image: postgres:17-alpine
    network_mode: none
    entrypoint: [sleep, infinity]
volumes:
  database:
`);
  const env = { ...process.env, BACKUP_COMPOSE_FILE: compose, BACKUP_PROJECT: project,
    BACKUP_UPLOADS_VOLUME: volume, BACKUP_TRANSPORT: 'local', BACKUP_TARGET: remote,
    BACKUP_WORK_DIR: work, BACKUP_CIPHER_DIR: join(root, 'cipher'),
    BACKUP_MIN_FREE_BYTES: '1048576', BACKUP_RETENTION_DAYS: '30',
    POSTGRES_PASSWORD: credential, PHOENIX_AUTH_PASSWORD: randomBytes(24).toString('hex'),
    PHOENIX_DEMO_PASSWORD: randomBytes(24).toString('hex') };
  mkdirSync(env.BACKUP_CIPHER_DIR);
  for (const key of ['COMPOSE_FILE', 'COMPOSE_PROJECT_NAME', 'COMPOSE_PROFILES']) delete env[key];
  const invoke = (command, args, input, overrides = {}) => spawnSync(command, args,
    { cwd: root, env: { ...env, ...overrides }, input, encoding: 'utf8', timeout: 180000 });
  const run = (command, args, input, overrides) => {
    const result = invoke(command, args, input, overrides);
    const diagnostic = command === 'docker' && args.includes('up')
      ? result.stderr.replaceAll(credential, '[redacted]').replaceAll(marker, '[redacted]') : 'diagnostics withheld';
    assert.equal(result.status, 0, `${command} failed: ${diagnostic}`);
    return result.stdout.trim();
  };
  const docker = (...args) => run('docker', args);
  const dc = (...args) => docker('compose', '--env-file', join(root, '.env'), '-f', compose, '-p', project, ...args);
  const databaseSQL = (database, query) => run('docker', ['compose', '--env-file', join(root, '.env'), '-f', compose, '-p', project,
    'exec', '-T', 'postgres', 'psql', '-X', '-U', 'postgres', '-d', database, '-At', '-v', 'ON_ERROR_STOP=1'], query);
  const sql = query => databaseSQL('postgres', query);
  // Main recovery above still destroys the actual volume. For the expanded
  // role matrix, reset this private cluster's DB and non-system roles instead
  // of restarting initdb for every source and target. Restore's real fresh-target guards still run.
  const resetCluster = () => {
    databaseSQL('template1', 'DROP DATABASE postgres WITH (FORCE);');
    databaseSQL('template1', `DO $$ DECLARE r record; BEGIN
      FOR r IN SELECT rolname FROM pg_roles WHERE rolname !~ '^pg_' AND rolname <> 'postgres' LOOP
        EXECUTE format('DROP ROLE %I', r.rolname);
      END LOOP;
    END $$;`);
    databaseSQL('template1', 'CREATE DATABASE postgres TEMPLATE template0;');
  };
  const backup = (overrides = {}) => invoke('bash', [join(scripts, 'daily-backup.sh')], undefined, overrides);
  const restore = (name, overrides = {}) => invoke('bash', [join(scripts, 'restore-daily-backup.sh'), name, ledger], undefined,
    { BACKUP_UPLOADS_VOLUME: restoredVolume, BACKUP_AGE_IDENTITY_FILE: join(root, 'identity'), ...overrides });
  t.after(() => {
    invoke('docker', ['compose', '-f', compose, '-p', project, 'down', '-v', '--remove-orphans']);
    invoke('docker', ['volume', 'rm', volume, restoredVolume, `${project}-rejected`]);
    rmSync(root, { recursive: true, force: true });
  });
  const restoreConfig = JSON.parse(run('docker', ['compose', '--env-file', join(root, '.env'),
    '-f', join(scripts, '../docs/operations/daily-restore.compose.yml'), 'config', '--format', 'json'], undefined,
    { TZ: 'UTC', RESTORE_POSTGRES_PORT: '55432', RESTORE_DATABASE_VOLUME: `${project}-drill` }));
  assert.deepEqual(Object.keys(restoreConfig.services), ['postgres']);
  assert.equal(restoreConfig.services.postgres.restart, 'no');
  assert.equal(restoreConfig.services.postgres.ports[0].host_ip, '127.0.0.1');
  assert.equal(restoreConfig.services.postgres.ports[0].published, '55432');
  assert.equal(restoreConfig.volumes.database.name, `${project}-drill`);
  // Observe the real transport boundary: ciphertext never occupies the
  // plaintext workspace, and backup must not materialize an outer tar there.
  const guardedBin = join(root, 'guarded-bin'); mkdirSync(guardedBin);
  const transport = run('which', ['rclone']);
  writeFileSync(join(guardedBin, 'rclone'), `#!/bin/sh
for arg in "$@"; do
  case "$arg" in '${work}'/*.age) exit 29;; esac
done
[ -z "$(find '${work}' -name bundle.tar -print -quit)" ] || exit 30
exec '${transport}' "$@"
`);
  chmodSync(join(guardedBin, 'rclone'), 0o700);
  env.PATH = `${guardedBin}:${env.PATH}`;
  run('age-keygen', ['-o', join(root, 'identity')]);
  env.BACKUP_AGE_RECIPIENT = run('age-keygen', ['-y', join(root, 'identity')]);
  dc('up', '-d', '--wait', 'postgres', 'server');
  docker('volume', 'create', volume);
  sql(`CREATE ROLE phoenix_auth LOGIN PASSWORD '${credential}';
CREATE ROLE phoenix_demo LOGIN;
CREATE ROLE phoenix_diagnostics NOLOGIN NOINHERIT;
CREATE ROLE "diagnose synthetic" LOGIN NOINHERIT;
GRANT phoenix_diagnostics TO "diagnose synthetic";
CREATE ROLE phoenix_school_erasure NOLOGIN NOINHERIT;
CREATE ROLE "erase synthetic" LOGIN NOINHERIT;
GRANT phoenix_school_erasure TO "erase synthetic";
CREATE ROLE phoenix_billing_retention NOLOGIN NOINHERIT;
CREATE ROLE "billing synthetic" LOGIN NOINHERIT;
GRANT phoenix_billing_retention TO "billing synthetic";
CREATE ROLE tenant_scope;
CREATE SCHEMA platform;
CREATE TABLE platform.schools(id bigserial PRIMARY KEY);
INSERT INTO platform.schools VALUES (41), (42);
SELECT setval('platform.schools_id_seq',42);
GRANT tenant_scope TO phoenix_auth;
CREATE TABLE records(tenant_id bigint, marker text);
ALTER TABLE records ENABLE ROW LEVEL SECURITY;
CREATE POLICY school_scope ON records USING (tenant_id = current_setting('test.school_id')::bigint);
GRANT SELECT ON records TO tenant_scope;
INSERT INTO records VALUES (41, '${marker}'), (42, '${marker}');`);
  docker('run', '--rm', '-v', `${volume}:/uploads`, 'postgres:17-alpine', 'sh', '-c',
    'mkdir -p /uploads/documents/41 /uploads/documents/42; printf first > /uploads/documents/41/file; printf second > /uploads/documents/42/file; printf hidden > /uploads/.hidden');
  const old = 'daily-20000101T000000Z-0000000000000000.tar.age';
  writeFileSync(join(remote, old), 'expired synthetic bundle');
  utimesSync(join(remote, old), new Date('2000-01-01'), new Date('2000-01-01'));
  writeFileSync(join(remote, 'unrelated'), 'keep');
  let result = backup({ BACKUP_MIN_FREE_BYTES: '999999999999999999' });
  assert.notEqual(result.status, 0); assert.match(result.stdout, /backup_result=failed step=workspace_capacity/);
  assert.deepEqual(readdirSync(remote).sort(), [old, 'unrelated'].sort());
  assert.deepEqual(readdirSync(work), []);
  result = backup();
  assert.equal(result.status, 0, result.stdout);
  assert.match(result.stdout, /backup_result=ok step=complete/);
  const names = readdirSync(remote);
  assert(!names.includes(old)); assert(names.includes('unrelated'));
  const name = names.find(n => n.endsWith('.age'));
  const ciphertext = readFileSync(join(remote, name));
  for (const forbidden of [marker, credential, env.PHOENIX_AUTH_PASSWORD, env.PHOENIX_DEMO_PASSWORD, 'SCRAM-SHA-256$', 'PRIVATE_MARKER']) assert(!ciphertext.includes(Buffer.from(forbidden)));
  run('age', ['-d', '-i', join(root, 'identity'), '-o', join(root, 'bundle.tar'), join(remote, name)]);
  const members = run('tar', ['-tf', join(root, 'bundle.tar')]).split('\n').sort();
  assert.deepEqual(members, ['database.dump', 'manifest.json', 'roles.sql', 'uploads.tar.gz']);
  const roles = run('tar', ['-xOf', join(root, 'bundle.tar'), 'roles.sql']);
  assert(!roles.includes('SCRAM-SHA-256$')); assert(!roles.includes(credential));
  const corrupted = 'daily-20000101T000000Z-2222222222222222.tar.age';
  run('python3', ['-c', `import io,tarfile,sys
with tarfile.open(sys.argv[1]) as source, tarfile.open(sys.argv[2],'w') as target:
 for member in source:
  data=source.extractfile(member).read()
  if member.name=='database.dump': data=b'corrupt'+data[7:]
  target.addfile(member,io.BytesIO(data))
`, join(root, 'bundle.tar'), join(root, 'corrupt.tar')]);
  run('age', ['-r', env.BACKUP_AGE_RECIPIENT, '-o', join(remote, corrupted), join(root, 'corrupt.tar')]);
  for (const key of ['BACKUP_COMPOSE_FILE', 'BACKUP_PROJECT', 'BACKUP_UPLOADS_VOLUME',
    'BACKUP_TRANSPORT', 'BACKUP_TARGET', 'BACKUP_WORK_DIR', 'BACKUP_CIPHER_DIR',
    'BACKUP_MIN_FREE_BYTES', 'POSTGRES_PASSWORD', 'PHOENIX_AUTH_PASSWORD',
    'PHOENIX_DEMO_PASSWORD', 'BACKUP_AGE_IDENTITY_FILE']) {
    result = restore(name, { [key]: '' });
    assert.notEqual(result.status, 0);
    assert.equal(result.stdout, `restore_result=failed step=configuration missing=${key} app_start=blocked\n`);
    assert.equal(result.stderr, '');
  }
  result = restore(corrupted);
  assert.notEqual(result.status, 0); assert.match(result.stdout, /step=checksums/);
  result = restore(name);
  assert.notEqual(result.status, 0); assert.match(result.stdout, /step=fresh_target/); // Running application is never an eligible restore target.
  assert.equal(sql('SELECT count(*) FROM records;'), '2');
  const rowsBefore = createHash('sha256').update(sql('SELECT tenant_id, marker FROM records ORDER BY tenant_id, marker;')).digest('hex');
  // Simulate a completed A erasure after this older backup was made. Its
  // independent record survives restoring the DB state which knew no erasure.
  const header = 'moto-erasure-ledger-v1\n';
  const entry = JSON.stringify({ school_id: 41, order_reference: 'ORDER-RESTORE',
    completed_at: '2026-10-24T12:00:00Z', receipt_sha256: 'a'.repeat(64) });
  const head = createHash('sha256').update(header).digest('hex');
  const digest = createHash('sha256').update(`${head}\n${entry}`).digest('hex');
  writeFileSync(ledger, `${header}${digest} ${entry}\n`);
  dc('down', '-v'); // Destroy database storage, not just its rows.
  docker('volume', 'rm', volume);
  dc('up', '-d', '--wait', 'postgres');
  // Fail at the external Docker boundary after the durable review marker is
  // written. Partial restores must retain the barrier, even on a fresh retry.
  const failingBin = join(root, 'failing-bin'); mkdirSync(failingBin);
  const realDocker = run('which', ['docker']);
  writeFileSync(join(failingBin, 'docker'), `#!/bin/sh
case "$*" in *psql*ON_ERROR_STOP=1*) case "$*" in *-At*) ;; *) exit 27;; esac;; esac
exec '${realDocker}' "$@"
`, { mode: 0o700 });
  result = restore(name, { PATH: `${failingBin}:${env.PATH}` });
  assert.notEqual(result.status, 0);
  assert.equal(result.stdout, 'restore_result=failed step=roles_restore app_start=blocked\n');
  const reviewMarker = join(root, '.daily-restore-review-required');
  const pendingReview = readFileSync(reviewMarker, 'utf8');
  assert.equal(JSON.parse(pendingReview).app_start, 'blocked');
  assert.deepEqual(readdirSync(work), []);
  assert.deepEqual(readdirSync(env.BACKUP_CIPHER_DIR), []);
  dc('down', '-v'); // Only this synthetic, isolated test project.
  dc('up', '-d', '--wait', 'postgres');
  result = restore(name);
  assert.notEqual(result.status, 0);
  assert.equal(readFileSync(reviewMarker, 'utf8'), pendingReview);
  // Operator removes it only after discarding the failed isolated target.
  rmSync(reviewMarker);
  result = restore(name);
  assert.notEqual(result.status, 0);
  assert.match(result.stdout, /restore_result=failed step=erasure_replay_required app_start=blocked/);
  assert.match(result.stdout, /restore_diagnostics_disabled=1/);
  assert.match(result.stdout, /restore_school_erasure_disabled=1/);
  assert.match(result.stdout, /restore_billing_retention_disabled=1/);
  assert.equal(sql(`SELECT rolcanlogin FROM pg_roles WHERE rolname = 'diagnose synthetic';`), 'f');
  assert.equal(sql(`SELECT pg_has_role('diagnose synthetic', 'phoenix_diagnostics', 'SET');`), 't');
  for (const [login, group] of [['erase synthetic', 'phoenix_school_erasure'], ['billing synthetic', 'phoenix_billing_retention']]) {
    assert.equal(sql(`SELECT rolcanlogin FROM pg_roles WHERE rolname = '${login}';`), 'f');
    assert.equal(sql(`SELECT pg_has_role('${login}', '${group}', 'SET');`), 't');
  }
  assert.equal(sql('SELECT count(*) FROM records;'), '2');
  assert.equal(createHash('sha256').update(sql('SELECT tenant_id, marker FROM records ORDER BY tenant_id, marker;')).digest('hex'), rowsBefore);
  const verifyErasures = () => invoke('python3', [join(scripts, 'daily_backup.py'), 'verify-erasures', ledger]);
  result = verifyErasures();
  assert.notEqual(result.status, 0);
  assert.equal(result.stdout, 'restore_result=failed step=erasure_replay_required app_start=blocked\n');
  assert.equal(JSON.parse(readFileSync(reviewMarker, 'utf8')).app_start, 'blocked');
  assert.equal(sql('SELECT string_agg(id::text,\',\' ORDER BY id) FROM platform.schools;'), '41,42');
  sql('DELETE FROM platform.schools WHERE id=41;'); // Repeat A, B is untouched.
  assert.equal(verifyErasures().status, 0);
  assert.equal(sql('SELECT id FROM platform.schools;'), '42');
  // A was created and erased AFTER the old backup: no root to find, but the
  // restored sequence must be fenced before creating new schools.
  const laterEntry = JSON.stringify({ school_id: 77, order_reference: 'ORDER-LATER',
    completed_at: '2026-10-24T12:00:00Z', receipt_sha256: 'b'.repeat(64) });
  const laterDigest = createHash('sha256').update(`${digest}\n${laterEntry}`).digest('hex');
  writeFileSync(ledger, `${header}${digest} ${entry}\n${laterDigest} ${laterEntry}\n`);
  assert.match(verifyErasures().stdout, /failed step=erasure_sequence_review_required app_start=blocked/);
  sql("SELECT setval('platform.schools_id_seq',77);");
  assert.equal(verifyErasures().status, 0);
  assert.equal(sql('SELECT id FROM platform.schools;'), '42');
  // Neither missing nor damaged independent state can grant release.
  rmSync(ledger);
  assert.match(verifyErasures().stdout, /failed step=erasure_ledger app_start=blocked/);
  writeFileSync(ledger, `${header}${digest} ${entry}`, { mode: 0o600 });
  assert.match(verifyErasures().stdout, /failed step=erasure_ledger app_start=blocked/);
  writeFileSync(ledger, header);
  // Passwordless role dumps are not usable app logins until explicitly re-keyed.
  for (const [role, key] of [['postgres', 'POSTGRES_PASSWORD'], ['phoenix_auth', 'PHOENIX_AUTH_PASSWORD'], ['phoenix_demo', 'PHOENIX_DEMO_PASSWORD']]) {
    const login = invoke('docker', ['compose', '--env-file', join(root, '.env'), '-f', compose, '-p', project,
      'exec', '-T', '-e', 'PGPASSWORD', 'postgres', 'psql', '-X', '-h', '127.0.0.1', '-U', role,
      '-d', 'postgres', '-At', '-c', 'SELECT current_user;'], undefined, { PGPASSWORD: env[key] });
    assert.equal(login.status, 0, `restored login ${role} must authenticate (diagnostics withheld)`);
    assert.equal(login.stdout.trim(), role);
    const wrongPassword = invoke('docker', ['compose', '--env-file', join(root, '.env'), '-f', compose, '-p', project,
      'exec', '-T', '-e', 'PGPASSWORD', 'postgres', 'psql', '-X', '-h', '127.0.0.1', '-U', role,
      '-d', 'postgres', '-At', '-c', 'SELECT current_user;'], undefined, { PGPASSWORD: randomBytes(24).toString('hex') });
    assert.notEqual(wrongPassword.status, 0, `restored login ${role} must reject a wrong password`);
  }
  assert.equal(sql("SET ROLE tenant_scope; SET test.school_id='41'; SELECT count(*) FROM records;" ).split('\n').at(-1), '1');
  assert.equal(sql("SET ROLE tenant_scope; SET test.school_id='42'; SELECT count(*) FROM records;" ).split('\n').at(-1), '1');
  for (const [file, value] of [['documents/41/file', 'first'], ['documents/42/file', 'second'], ['.hidden', 'hidden']]) {
    assert.equal(docker('run', '--rm', '-v', `${restoredVolume}:/uploads:ro`, 'postgres:17-alpine', 'cat', `/uploads/${file}`), value);
    assert.equal(docker('run', '--rm', '-v', `${restoredVolume}:/uploads:ro`, 'postgres:17-alpine', 'sha256sum', `/uploads/${file}`).split(' ')[0], createHash('sha256').update(value).digest('hex'));
  }
  assert.equal(dc('ps', '--status', 'running', '--services'), 'postgres');
  assert.notEqual(restore(name).status, 0); // Refuse populated database / existing volume.
  const bad = 'daily-20000101T000000Z-1111111111111111.tar.age';
  writeFileSync(join(remote, bad), ciphertext.subarray(0, 90));
  result = restore(bad, { BACKUP_UPLOADS_VOLUME: `${project}-unused` });
  assert.notEqual(result.status, 0);
  assert(!result.stdout.includes(marker));
  // Local transport still uses rclone; inject a real non-directory upload target.
  const blocked = join(root, 'blocked'); writeFileSync(blocked, 'not a directory');
  result = backup({ BACKUP_UPLOADS_VOLUME: restoredVolume, BACKUP_TARGET: join(blocked, 'child') });
  assert.notEqual(result.status, 0); assert.match(result.stdout, /backup_result=failed step=upload/);
  result = backup({ BACKUP_RETENTION_DAYS: '91' });
  assert.notEqual(result.status, 0); assert.match(result.stdout, /step=configuration/);
  result = backup({ BACKUP_AGE_RECIPIENT: '' });
  assert.notEqual(result.status, 0); assert.match(result.stdout, /step=configuration missing=BACKUP_AGE_RECIPIENT/);
  // Refusing an operation lock must not delete a lock owned by another process.
  mkdirSync(join(root, '.release-operation.lock'));
  result = backup();
  assert.notEqual(result.status, 0); assert.match(result.stdout, /step=operation_lock/);
  rmSync(join(root, '.release-operation.lock'), { recursive: true });
  // Real dumps/encryption/transfers with only the final cleanup operation failing.
  const shims = join(root, 'bin'); mkdirSync(shims);
  const realRclone = run('which', ['rclone']);
  writeFileSync(join(shims, 'rclone'), `#!/bin/sh\nfor arg in "$@"; do [ "$arg" != deletefile ] || exit 27; done\nexec '${realRclone}' "$@"\n`);
  chmodSync(join(shims, 'rclone'), 0o700);
  writeFileSync(join(remote, old), 'expired');
  result = backup({ BACKUP_UPLOADS_VOLUME: restoredVolume, PATH: `${shims}:${env.PATH}` });
  assert.notEqual(result.status, 0); assert.match(result.stdout, /step=retention/);
  assert(!result.stdout.includes('backup_result=ok'));
  assert(!result.stdout.includes(credential)); assert(!result.stderr.includes(marker));
  assert.deepEqual(readdirSync(work), []);
  assert.deepEqual(readdirSync(env.BACKUP_CIPHER_DIR), []);
  // An unrelated or more privileged personal login still blocks recovery.
  // Exercise real globals AND database dumps: direct ACLs/ownership are absent
  // from globals and must be rejected after database_restore as well.
  const unsafeRoles = [
    'CREATE ROLE unrelated LOGIN;',
    'GRANT tenant_scope TO "diagnose synthetic";',
    'ALTER ROLE "diagnose synthetic" SUPERUSER;',
    'ALTER ROLE "diagnose synthetic" BYPASSRLS;',
    'GRANT phoenix_diagnostics TO "diagnose synthetic" WITH ADMIN TRUE;',
    'GRANT phoenix_diagnostics TO "diagnose synthetic" WITH INHERIT TRUE;',
    'GRANT phoenix_diagnostics TO "diagnose synthetic" WITH SET FALSE;',
    'ALTER ROLE "diagnose synthetic" SET search_path = public;',
    'GRANT SELECT ON records TO "diagnose synthetic";',
    'GRANT SELECT (tenant_id) ON records TO "diagnose synthetic";',
    'ALTER TABLE records OWNER TO "diagnose synthetic";',
    'ALTER DEFAULT PRIVILEGES GRANT SELECT ON TABLES TO "diagnose synthetic";',
    'GRANT pg_monitor TO phoenix_diagnostics;',
  ];
  const profiles = [
    { group: 'phoenix_diagnostics', login: 'diagnose synthetic', label: 'diagnostics' },
    { group: 'phoenix_school_erasure', login: 'erase synthetic', label: 'school_erasure' },
    { group: 'phoenix_billing_retention', login: 'billing synthetic', label: 'billing_retention' },
  ];
  for (const profile of profiles) {
    const otherGroup = profile.group === 'phoenix_school_erasure' ? 'phoenix_billing_retention' : 'phoenix_school_erasure';
    const dualMembership = `CREATE ROLE ${otherGroup} NOLOGIN NOINHERIT; GRANT ${otherGroup} TO "diagnose synthetic";`;
    // A reserved group must not masquerade as an otherwise eligible login.
    const groupLogin = `CREATE ROLE ${otherGroup} LOGIN NOINHERIT; GRANT ${profile.group} TO ${otherGroup};`;
    for (const [index, unsafe] of ['', ...unsafeRoles, dualMembership, groupLogin].entries()) {
      resetCluster();
      rmSync(reviewMarker, { force: true });
      const statements = `CREATE ROLE ${profile.group} NOLOGIN NOINHERIT;
CREATE ROLE "diagnose synthetic" LOGIN NOINHERIT;
GRANT ${profile.group} TO "diagnose synthetic";
CREATE ROLE tenant_scope;
CREATE TABLE records(tenant_id bigint);
${unsafe.replaceAll('phoenix_diagnostics', profile.group)}`;
      sql(statements.replaceAll('diagnose synthetic', profile.login));
      const destination = join(root, `${profile.label}-${index}`); mkdirSync(destination);
      result = backup({ BACKUP_UPLOADS_VOLUME: restoredVolume, BACKUP_TARGET: destination });
      assert.equal(result.status, 0, result.stdout);
      const unsafeName = readdirSync(destination).find(n => n.endsWith('.age'));
      resetCluster();
      result = restore(unsafeName, { BACKUP_TARGET: destination, BACKUP_UPLOADS_VOLUME: `${project}-rejected` });
      if (!unsafe) {
        assert.equal(result.status, 0, `safe ${profile.label} login alone must be quarantined`);
        assert.match(result.stdout, new RegExp(`restore_${profile.label}_disabled=1`));
        assert.equal(sql(`SELECT rolcanlogin FROM pg_roles WHERE rolname='${profile.login}';`), 'f');
        assert.equal(sql(`SELECT pg_has_role('${profile.login}', '${profile.group}', 'SET');`), 't');
        docker('volume', 'rm', `${project}-rejected`);
      } else {
        assert.notEqual(result.status, 0, `unsafe ${profile.label} role case ${index} must fail`);
        assert.equal(result.stdout, 'restore_result=failed step=role_passwords app_start=blocked\n');
        assert.equal(JSON.parse(readFileSync(reviewMarker, 'utf8')).app_start, 'blocked');
        assert.notEqual(invoke('docker', ['volume', 'inspect', `${project}-rejected`]).status, 0);
      }
      assert(!result.stdout.includes(profile.login));
      assert.equal(result.stderr, '');
    }
  }
  t.diagnostic('role restore matrix: 3 accepted NOLOGIN profiles and 45 rejected profiles');
});
