import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { mkdtempSync, mkdirSync, writeFileSync, readFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import test from 'node:test';

const scripts = dirname(fileURLToPath(import.meta.url));
test('real PostgreSQL snapshot restores data, outbox schema, roles, uploads and image configuration', { timeout: 180000 }, t => {
  const root = mkdtempSync(join(tmpdir(), 'moto-restore-integration-'));
  const cwd = join(root, 'staging'); mkdirSync(cwd);
  const project = `moto-restore-${process.pid}-${Date.now()}`;
  const env = { ...process.env, DEPLOY_DIR: 'staging' };
  for (const key of ['COMPOSE_FILE', 'COMPOSE_PROJECT_NAME', 'COMPOSE_PROFILES']) delete env[key];
  const run = (command, args, input) => {
    const result = spawnSync(command, args, { cwd, env, input, encoding: 'utf8', timeout: 120000 });
    assert.equal(result.status, 0, `${command} failed: ${result.stdout}\n${result.stderr}`);
    return result.stdout.trim();
  };
  const docker = (...args) => run('docker', args);
  const sql = query => run('docker', ['compose', 'exec', '-T', 'postgres', 'psql', '-X', '-U', 'postgres', '-d', 'postgres', '-v', 'ON_ERROR_STOP=1', '-At'], query);
  docker('pull', 'postgres:17-alpine');
  const image = docker('image', 'inspect', 'postgres:17-alpine', '--format', '{{index .RepoDigests 0}}');
  writeFileSync(join(cwd, '.env'), 'RELEASE_MARKER=before\n');
  writeFileSync(join(cwd, '.deploy-state'), 'CURRENT_SHA=9c95677\n');
  writeFileSync(join(cwd, 'docker-compose.yml'), `name: ${project}
services:
  postgres:
    image: ${image}
    environment:
      POSTGRES_HOST_AUTH_METHOD: trust
    volumes:
      - database:/var/lib/postgresql/data
    healthcheck:
      test: [CMD, pg_isready, -U, postgres]
      interval: 1s
      retries: 30
  server:
    image: ${image}
    entrypoint: [sleep, infinity]
    volumes:
      - uploads:/app/public/uploads
  frontend:
    image: ${image}
    entrypoint: [sleep, infinity]
  migrate:
    image: ${image}
    entrypoint: ["true"]
    profiles: [maintenance]
volumes:
  database:
  uploads:
`);
  t.after(() => {
    spawnSync('docker', ['compose', 'down', '-v', '--remove-orphans'], { cwd, env, encoding: 'utf8' });
    rmSync(root, { recursive: true, force: true });
  });
  docker('compose', 'up', '-d', '--wait', 'postgres', 'server', 'frontend');
  sql(`CREATE ROLE phoenix_auth LOGIN NOINHERIT;
CREATE ROLE phoenix_tenant; CREATE ROLE phoenix_admin;
GRANT phoenix_tenant, phoenix_admin TO phoenix_auth;
CREATE SCHEMA auth; CREATE SCHEMA platform; CREATE SCHEMA active;
CREATE TABLE auth.accounts(id bigint PRIMARY KEY);
CREATE TABLE auth.account_tenants(id bigint);
CREATE TABLE platform.operators(id bigint);
CREATE TABLE active.visits(id bigint);
GRANT USAGE ON SCHEMA auth, platform, active TO phoenix_auth, phoenix_tenant, phoenix_admin;
GRANT SELECT ON auth.accounts, platform.operators TO phoenix_auth;
GRANT SELECT ON auth.account_tenants TO phoenix_tenant;
GRANT INSERT ON active.visits TO phoenix_tenant;
GRANT SELECT ON platform.operators TO phoenix_admin;
INSERT INTO auth.accounts VALUES (41);
CREATE TABLE platform.email_outbox(id bigint PRIMARY KEY, status text NOT NULL CHECK (status IN ('pending','sending','sent','failed')));
INSERT INTO platform.email_outbox VALUES (71, 'pending');`);
  docker('compose', 'exec', '-T', 'server', 'sh', '-c', 'printf original > /app/public/uploads/kept.txt; printf hidden > /app/public/uploads/.hidden');
  docker('compose', 'stop', 'server', 'frontend');
  const bundle = join(root, 'backups/staging/release-20260914T010000Z-bbbbbbb');
  run('bash', [join(scripts, 'release-backup.sh'), 'create', bundle]);
  assert.match(readFileSync(join(bundle, 'images.tsv'), 'utf8'), /migrate\tpostgres@sha256:/);
  const uploads = readFileSync(join(bundle, 'uploads-volume'), 'utf8').trim();
  sql(`UPDATE auth.accounts SET id=99;
ALTER TABLE platform.email_outbox ADD COLUMN recipient jsonb;
REVOKE phoenix_admin FROM phoenix_auth;
CREATE ROLE post_release_role;
GRANT post_release_role TO phoenix_auth;
ALTER ROLE phoenix_auth NOLOGIN;`);
  docker('run', '--rm', '--mount', `type=volume,src=${uploads},dst=/uploads`, '--entrypoint', 'sh', image,
    '-c', 'rm /uploads/.hidden; printf changed > /uploads/kept.txt; printf new > /uploads/added.txt');
  writeFileSync(join(cwd, '.env'), 'RELEASE_MARKER=after\n');
  writeFileSync(join(cwd, '.deploy-state'), 'CURRENT_SHA=bbbbbbb\nBACKUP_ID=release-20260914T010000Z-bbbbbbb\n');
  run('bash', [join(scripts, 'rollback-remote.sh'), cwd]);
  assert.equal(sql('SELECT id FROM auth.accounts;'), '41');
  assert.equal(sql("SELECT count(*) FROM information_schema.columns WHERE table_schema='platform' AND table_name='email_outbox' AND column_name='recipient';"), '0');
  assert.equal(sql("SELECT rolcanlogin FROM pg_roles WHERE rolname='phoenix_auth';"), 't');
  assert.equal(sql("SELECT pg_has_role('phoenix_auth','phoenix_admin','MEMBER');"), 't');
  assert.equal(sql("SELECT count(*) FROM pg_roles WHERE rolname='post_release_role';"), '0');
  // The previous main worker's status transition works again after full restore.
  sql("UPDATE platform.email_outbox SET status='sending' WHERE id=71;");
  assert.equal(docker('compose', 'exec', '-T', 'server', 'cat', '/app/public/uploads/kept.txt'), 'original');
  assert.equal(docker('compose', 'exec', '-T', 'server', 'cat', '/app/public/uploads/.hidden'), 'hidden');
  docker('compose', 'exec', '-T', 'server', 'test', '!', '-e', '/app/public/uploads/added.txt');
  assert.equal(readFileSync(join(cwd, '.env'), 'utf8'), 'RELEASE_MARKER=before\n');
  assert.equal(readFileSync(join(cwd, '.deploy-state'), 'utf8'), 'CURRENT_SHA=9c95677\n');
  assert.match(readFileSync(join(cwd, 'docker-compose.yml'), 'utf8'), /image: postgres@sha256:/);
  // Reproduce an interrupted restore after DROP DATABASE. The same complete
  // snapshot must remain usable even when the application database is absent.
  docker('compose', 'stop', 'server', 'frontend');
  docker('compose', 'exec', '-T', 'postgres', 'psql', '-X', '-U', 'postgres', '-d', 'template1',
    '-v', 'ON_ERROR_STOP=1', '-c', 'DROP DATABASE postgres WITH (FORCE);');
  run('bash', [join(scripts, 'restore-db.sh'), bundle]);
  assert.equal(sql('SELECT id FROM auth.accounts;'), '41');
});
