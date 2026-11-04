import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { copyFileSync, mkdirSync, mkdtempSync, readFileSync, rmSync, statSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import test from 'node:test';

import { gitTestEnvironment, initGitFixture } from './test-helpers/git-fixture.mjs';

const script = new URL('./worktree-ports.sh', import.meta.url).pathname;
const defaults = { FRONTEND_HOST_PORT: 3000, SERVER_HOST_PORT: 8080, POSTGRES_HOST_PORT: 5432, MAILPIT_HOST_PORT: 8025, MAILPIT_SMTP_HOST_PORT: 1025 };
const env = [
  ...Object.entries(defaults).map(([key, port]) => `${key}=${port}`),
  'TEST_DB_PORT=5433',
  'DB_DSN=postgres://u:p@postgres:5432/db',
  'NEXTAUTH_URL=http://localhost:3000',
  'NEXT_PUBLIC_SCHOOL_HOSTNAME=schule.localhost:3000',
  'CORS_ALLOWED_ORIGINS=http://localhost:1420,http://localhost:3000,http://localhost:8080',
  'OTHER_URL=http://localhost:30001',
  '',
].join('\n');
const devEnv = 'DB_DSN=postgres://u:p@localhost:5432/db\nMAILPIT_URL=http://localhost:8025\nTEST_DB_DSN=postgres://u:p@localhost:5433/t\n';

function fixture(t) {
  const base = mkdtempSync(join(tmpdir(), 'phoenix-worktree-ports-'));
  t.after(() => rmSync(base, { recursive: true, force: true }));
  const main = join(base, 'main');
  const linked = join(base, 'linked');
  mkdirSync(main);
  const gitEnv = gitTestEnvironment();
  const git = (cwd, ...args) => {
    const result = spawnSync('git', args, { cwd, env: gitEnv, encoding: 'utf8' });
    assert.equal(result.status, 0, result.stderr);
  };
  initGitFixture((...args) => git(main, ...args));
  writeFileSync(join(main, 'README.md'), 'fixture\n');
  git(main, 'add', '.');
  git(main, 'commit', '-qm', 'base');
  git(main, 'worktree', 'add', '-q', linked);
  // A fake lsof reports the ports listed in BUSY_PORTS as listening.
  const bin = join(base, 'bin');
  mkdirSync(bin);
  writeFileSync(join(bin, 'lsof'), '#!/bin/sh\nport=${2#-iTCP:}\ncase " $BUSY_PORTS " in *" $port "*) exit 0 ;; esac\nexit 1\n', { mode: 0o755 });
  for (const root of [main, linked]) {
    mkdirSync(join(root, 'scripts'), { recursive: true });
    mkdirSync(join(root, 'backend'), { recursive: true });
    copyFileSync(script, join(root, 'scripts/worktree-ports.sh'));
    writeFileSync(join(root, '.env'), env);
    writeFileSync(join(root, 'backend/dev.env'), devEnv);
  }
  const run = (root, busy = []) => spawnSync('bash', ['scripts/worktree-ports.sh'], {
    cwd: root, encoding: 'utf8', env: { ...gitEnv, PATH: `${bin}:${process.env.PATH}`, BUSY_PORTS: busy.join(' ') },
  });
  const read = (root, file = '.env') => readFileSync(join(root, file), 'utf8');
  const port = (root, key) => Number(new RegExp(`^${key}=(\\d+)$`, 'm').exec(read(root))[1]);
  return { main, linked, run, read, port };
}

test('the main checkout keeps the default ports', t => {
  const f = fixture(t);
  const result = f.run(f.main);
  assert.equal(result.status, 0, result.stderr);
  assert.equal(result.stdout, '');
  assert.equal(f.read(f.main), env);
});

test('a linked worktree on default ports gets a stable slot and its URLs follow', t => {
  const f = fixture(t);
  const result = f.run(f.linked);
  assert.equal(result.status, 0, result.stderr);
  const frontend = f.port(f.linked, 'FRONTEND_HOST_PORT');
  const slot = (frontend - 3000) / 10;
  assert.ok(Number.isInteger(slot) && slot >= 50 && slot < 250, `slot ${slot}`);
  assert.equal(f.port(f.linked, 'SERVER_HOST_PORT'), 8080 + 10 * slot);
  assert.equal(f.port(f.linked, 'POSTGRES_HOST_PORT'), 5432 + 20 * slot);
  assert.equal(f.port(f.linked, 'MAILPIT_HOST_PORT'), 8025 + 10 * slot);
  assert.equal(f.port(f.linked, 'MAILPIT_SMTP_HOST_PORT'), 1025 + 10 * slot);
  const linkedEnv = f.read(f.linked);
  assert.match(linkedEnv, new RegExp(`^NEXTAUTH_URL=http://localhost:${frontend}$`, 'm'));
  assert.match(linkedEnv, new RegExp(`^NEXT_PUBLIC_SCHOOL_HOSTNAME=schule.localhost:${frontend}$`, 'm'));
  assert.match(linkedEnv, new RegExp(`^CORS_ALLOWED_ORIGINS=http://localhost:1420,http://localhost:${frontend},http://localhost:${8080 + 10 * slot}$`, 'm'));
  assert.match(linkedEnv, /^OTHER_URL=http:\/\/localhost:30001$/m);
  assert.match(linkedEnv, /^DB_DSN=postgres:\/\/u:p@postgres:5432\/db$/m, 'Compose-internal addresses stay');
  assert.match(linkedEnv, /^TEST_DB_PORT=5433$/m, 'the test database stays shared');
  const devEnvAfter = f.read(f.linked, 'backend/dev.env');
  assert.match(devEnvAfter, new RegExp(`localhost:${5432 + 20 * slot}/db`));
  assert.match(devEnvAfter, new RegExp(`MAILPIT_URL=http://localhost:${8025 + 10 * slot}$`, 'm'));
  assert.match(devEnvAfter, /localhost:5433\/t/);
  assert.equal(statSync(join(f.linked, '.env')).mode & 0o077, 0, 'env files stay private');
  assert.equal(statSync(join(f.linked, 'backend/dev.env')).mode & 0o077, 0);

  const again = f.run(f.linked);
  assert.equal(again.status, 0, again.stderr);
  assert.equal(again.stdout, '', 'a second run changes nothing');
  assert.equal(f.port(f.linked, 'FRONTEND_HOST_PORT'), frontend);
});

test('a slot with a port in use shifts to the next slot', t => {
  const f = fixture(t);
  assert.equal(f.run(f.linked).status, 0);
  const first = (f.port(f.linked, 'FRONTEND_HOST_PORT') - 3000) / 10;
  writeFileSync(join(f.linked, '.env'), env);
  const result = f.run(f.linked, [5432 + 20 * first]);
  assert.equal(result.status, 0, result.stderr);
  const next = (f.port(f.linked, 'FRONTEND_HOST_PORT') - 3000) / 10;
  assert.equal(next, first === 249 ? 50 : first + 1);
});

test('running dev servers keep their ports until they are stopped', t => {
  const f = fixture(t);
  mkdirSync(join(f.linked, 'tmp/dev-native'), { recursive: true });
  writeFileSync(join(f.linked, 'tmp/dev-native/backend.pid'), `${process.pid}\n`);
  const result = f.run(f.linked);
  assert.equal(result.status, 0, result.stderr);
  assert.match(result.stderr, /dev servers still run on the default ports; run scripts\/dev-native.sh down/);
  assert.equal(f.read(f.linked), env);
  rmSync(join(f.linked, 'tmp/dev-native/backend.pid'));
  assert.equal(f.run(f.linked).status, 0);
  assert.notEqual(f.port(f.linked, 'FRONTEND_HOST_PORT'), 3000);
});

test('a wt-managed worktree keeps its lease; only stale default URLs move', t => {
  const f = fixture(t);
  writeFileSync(join(f.linked, '.wt.env'), 'FRONTEND_HOST_PORT=3010\nSERVER_HOST_PORT=8090\nPOSTGRES_HOST_PORT=5452\nMAILPIT_HOST_PORT=8035\nMAILPIT_SMTP_HOST_PORT=1035\n');
  writeFileSync(join(f.linked, '.env'), env.replace(/^FRONTEND_HOST_PORT=3000$/m, 'FRONTEND_HOST_PORT=3010'));
  const result = f.run(f.linked);
  assert.equal(result.status, 0, result.stderr);
  assert.match(f.read(f.linked), /^FRONTEND_HOST_PORT=3010$/m);
  assert.match(f.read(f.linked), /^SERVER_HOST_PORT=8080$/m, '.env lease values are wt\'s business');
  assert.match(f.read(f.linked), /^NEXTAUTH_URL=http:\/\/localhost:3010$/m);
  assert.match(f.read(f.linked, 'backend/dev.env'), /localhost:5452\/db/);
});
