import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { createHash, randomBytes } from 'node:crypto';
import { mkdtempSync, mkdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import test from 'node:test';

const scripts = dirname(fileURLToPath(import.meta.url));

test('external erasure list blocks an older PostgreSQL restore and ID reuse without changing B', { timeout: 240000 }, t => {
  const root = mkdtempSync(join(tmpdir(), 'moto-erasure-restore-'));
  const project = `moto-erasure-${process.pid}-${Date.now()}`;
  const compose = join(root, 'compose.yml');
  const ledger = join(root, 'completed.ledger');
  const marker = join(root, '.daily-restore-review-required');
  const password = randomBytes(24).toString('hex');
  writeFileSync(join(root, '.env'), '');
  writeFileSync(compose, `services:
  postgres:
    image: postgres:17-alpine
    network_mode: none
    environment:
      POSTGRES_PASSWORD: ${password}
    healthcheck:
      test: [CMD, pg_isready, -U, postgres]
      interval: 1s
      retries: 60
`);
  const env = { ...process.env, BACKUP_COMPOSE_FILE: compose, BACKUP_PROJECT: project,
    BACKUP_UPLOADS_VOLUME: `${project}-uploads`, BACKUP_TRANSPORT: 'local', BACKUP_TARGET: root,
    BACKUP_WORK_DIR: join(root, 'work'), BACKUP_CIPHER_DIR: join(root, 'cipher'), BACKUP_MIN_FREE_BYTES: '1' };
  mkdirSync(env.BACKUP_WORK_DIR); mkdirSync(env.BACKUP_CIPHER_DIR);
  const invoke = (cmd, args, input) => spawnSync(cmd, args, { env, input, encoding: 'utf8', timeout: 180000 });
  const composeArgs = ['compose', '--env-file', join(root, '.env'), '-f', compose, '-p', project];
  const dc = (...args) => invoke('docker', [...composeArgs, ...args]);
  const ok = result => { assert.equal(result.status, 0, 'local tool failed; diagnostics withheld'); return result.stdout.trim(); };
  const sql = query => ok(dc('exec', '-T', 'postgres', 'psql', '-X', '-U', 'postgres', '-d', 'postgres', '-At', '-v', 'ON_ERROR_STOP=1', '-c', query));
  const verify = () => invoke('python3', [join(scripts, 'daily_backup.py'), 'verify-erasures', ledger]);
  t.after(() => { dc('down', '-v'); rmSync(root, { recursive: true, force: true }); });
  ok(dc('up', '-d', '--wait', 'postgres'));
  sql('CREATE SCHEMA platform; CREATE TABLE platform.schools(id bigserial PRIMARY KEY); INSERT INTO platform.schools DEFAULT VALUES; INSERT INTO platform.schools DEFAULT VALUES;');
  // A=1 and B=2 are synthetic rows in this otherwise empty isolated cluster.
  const dump = spawnSync('docker', ['compose', '--env-file', join(root, '.env'), '-f', compose, '-p', project,
    'exec', '-T', 'postgres', 'pg_dump', '-U', 'postgres', '-d', 'postgres', '-n', 'platform', '-Fc'], { env, timeout: 180000 });
  assert.equal(dump.status, 0);
  sql('DELETE FROM platform.schools WHERE id=1;');
  const header = 'moto-erasure-ledger-v1\n';
  const entry = JSON.stringify({ school_id: 1, order_reference: 'ORDER-RESTORE', completed_at: '2026-10-24T12:00:00Z', receipt_sha256: 'a'.repeat(64) });
  const first = createHash('sha256').update(header).digest('hex');
  const digest = createHash('sha256').update(`${first}\n${entry}`).digest('hex');
  writeFileSync(ledger, `${header}${digest} ${entry}\n`, { mode: 0o600 });
  assert.equal(sql('SELECT id FROM platform.schools;'), '2');
  sql('DROP SCHEMA platform CASCADE;');
  ok(invoke('docker', [...composeArgs, 'exec', '-T', 'postgres', 'pg_restore', '-U', 'postgres', '-d', 'postgres', '--exit-on-error'], dump.stdout));
  writeFileSync(marker, JSON.stringify({ app_start: 'blocked' }));
  let result = verify();
  assert.notEqual(result.status, 0);
  assert.equal(result.stdout, 'restore_result=failed step=erasure_replay_required app_start=blocked\n');
  assert.equal(sql("SELECT string_agg(id::text,',' ORDER BY id) FROM platform.schools;"), '1,2');
  sql('DELETE FROM platform.schools WHERE id=1;'); // CLI deletion covered by Go integration tests.
  assert.equal(verify().status, 0);
  assert.equal(sql('SELECT id FROM platform.schools;'), '2');
  assert.equal(JSON.parse(readFileSync(marker, 'utf8')).app_start, 'blocked');
  const later = JSON.stringify({ school_id: 9, order_reference: 'ORDER-LATER', completed_at: '2026-10-24T12:00:00Z', receipt_sha256: 'b'.repeat(64) });
  const next = createHash('sha256').update(`${digest}\n${later}`).digest('hex');
  const content = `${header}${digest} ${entry}\n${next} ${later}\n`;
  writeFileSync(ledger, content);
  result = verify();
  assert.notEqual(result.status, 0);
  assert.match(result.stdout, /failed step=erasure_sequence_review_required app_start=blocked/);
  sql("SELECT setval('platform.schools_id_seq',9);");
  assert.equal(verify().status, 0);
  assert.equal(sql('SELECT id FROM platform.schools;'), '2');
  writeFileSync(ledger, content.slice(0, -1));
  assert.match(verify().stdout, /failed step=erasure_ledger app_start=blocked/);
  rmSync(ledger);
  assert.match(verify().stdout, /failed step=erasure_ledger app_start=blocked/);
});
