import assert from 'node:assert/strict';
import { execFileSync, spawnSync } from 'node:child_process';
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import test from 'node:test';
import { checkLocalMigrationVersions, checkMigrationVersions, lateMigrations } from './check-migration-versions.mjs';
import { gitTestEnvironment, initGitFixture } from './test-helpers/git-fixture.mjs';

const script = resolve('scripts/check-migration-versions.mjs');
const migration = number => `backend/database/migrations/${number}_fixture.go`;
function fixture(t) {
  const cwd = mkdtempSync(join(tmpdir(), 'migration-versions-'));
  t.after(() => rmSync(cwd, { recursive: true, force: true }));
  const env = gitTestEnvironment();
  const git = (...args) => execFileSync('git', args, { cwd, env, encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'] }).trim();
  initGitFixture(git);
  const add = (file, content = '// fixture\n') => {
    mkdirSync(dirname(join(cwd, file)), { recursive: true });
    writeFileSync(join(cwd, file), content);
  };
  const commit = () => { git('add', '.'); git('commit', '-qm', 'fixture'); };
  add(migration('001015800'));
  commit();
  git('branch', '-M', 'development');
  git('remote', 'add', 'origin', cwd);
  git('fetch', '-q', 'origin');
  git('checkout', '-qb', 'feature');
  const run = (base = 'origin/development', head = 'HEAD', ...flags) => spawnSync(process.execPath, [script, ...flags, base, head], { cwd, env, encoding: 'utf8' });
  return { cwd, git, add, commit, run };
}

// The fixture's history is the nine-digit 001015800; new migrations are
// timestamps (ADR 0223).
const early = '20261101080000';
const later = '20261105120000';
const now = Date.UTC(2026, 10, 13);

test('timestamp additions pass in any order; historical edits and test files are ignored', t => {
  const f = fixture(t);
  f.add(migration(early));
  f.add('backend/database/migrations/001015100_fixture_test.go');
  writeFileSync(join(f.cwd, migration('001015800')), '// edited history\n');
  f.commit();
  f.git('checkout', '-q', 'development');
  f.add(migration(later));
  f.commit();
  f.git('checkout', '-q', 'feature');
  const result = f.run();
  assert.equal(result.status, 0, result.stderr);
  assert.match(result.stdout, /OK/);
});

test('a migration below the newest one of the advanced remote tip is late, not rejected', t => {
  const f = fixture(t);
  f.add(migration(early));
  f.add(migration('20261110090000'));
  f.commit();
  assert.deepEqual(lateMigrations({ cwd: f.cwd }), [], 'nothing is late while the base only has older migrations');
  f.git('checkout', '-q', 'development');
  f.add(migration(later));
  f.commit();
  f.git('checkout', '-q', 'feature');
  const result = f.run('origin/development', 'HEAD', '--late');
  assert.equal(result.status, 0, result.stderr);
  assert.equal(result.stdout, `${early}\n`);
});

test('the late list is the same when the script runs from backend/, as the CI test step does', t => {
  const f = fixture(t);
  f.add(migration(early));
  f.commit();
  f.git('checkout', '-q', 'development');
  f.add(migration(later));
  f.commit();
  f.git('checkout', '-q', 'feature');
  const result = spawnSync(process.execPath, [script, '--late', 'origin/development', 'HEAD'],
    { cwd: join(f.cwd, 'backend'), env: gitTestEnvironment(), encoding: 'utf8' });
  assert.equal(result.status, 0, result.stderr);
  assert.equal(result.stdout, `${early}\n`);
});

test('a timestamp another migration of the remote tip already has fails', t => {
  const f = fixture(t);
  f.add(migration(early));
  f.commit();
  f.git('checkout', '-q', 'development');
  f.add(`backend/database/migrations/${early}_other.go`);
  f.commit();
  f.git('checkout', '-q', 'feature');
  const result = f.run();
  assert.equal(result.status, 1);
  assert.ok(result.stderr.includes(`${migration(early)}: Migrationsnummer ${early} hat auf origin/development schon backend/database/migrations/${early}_other.go`));
});

test('two added migrations cannot share a timestamp', t => {
  const f = fixture(t);
  f.add(migration(early));
  f.add(`backend/database/migrations/${early}_other.go`);
  f.commit();
  const result = f.run();
  assert.equal(result.status, 1);
  assert.match(result.stderr, new RegExp(`Migrationsnummer ${early} hat auch `));
});

test('stacked PR checks its own additions against both current base and development', t => {
  const f = fixture(t);
  f.git('branch', '-m', 'parent');
  f.add(migration('20261101070000'));
  f.commit();
  f.git('checkout', '-qb', 'child');
  f.add(migration(early));
  f.add(migration('20261103080000'));
  f.commit();
  f.git('checkout', '-q', 'parent');
  f.add(`backend/database/migrations/${early}_parent.go`);
  f.commit();
  f.git('checkout', '-q', 'development');
  f.add('backend/database/migrations/20261103080000_development.go');
  f.add(migration(later));
  f.commit();
  f.git('checkout', '-q', 'child');
  const result = f.run('origin/parent');
  assert.equal(result.status, 1);
  assert.match(result.stderr, new RegExp(`${early}.*origin/parent`));
  assert.match(result.stderr, /20261103080000.*origin\/development/);
  assert.ok(!result.stderr.includes(`${migration('20261101070000')}:`), 'inherited migration is not an addition of the child');
  assert.deepEqual(lateMigrations({ cwd: f.cwd, base: 'origin/parent' }), [early, '20261103080000'],
    'late against development too; the inherited migration is not the child\'s');
});

test('local worktree includes staged and untracked migrations; renames count as additions', t => {
  const f = fixture(t);
  f.git('mv', migration('001015800'), migration('001015799'));
  f.add(migration('001015798'));
  f.add(migration(early));
  const errors = checkMigrationVersions({ cwd: f.cwd, worktree: true, now });
  assert.equal(errors.length, 2);
  assert.ok(errors.some(error => error.includes(migration('001015799'))));
  assert.ok(errors.some(error => error.includes(migration('001015798'))));
});

test('missing remote base fails closed', t => {
  const f = fixture(t);
  for (const flags of [[], ['--late']]) {
    const result = f.run('origin/missing', 'HEAD', ...flags);
    assert.equal(result.status, 2);
    assert.equal(result.stdout, '');
    assert.match(result.stderr, /Migration versions:/);
  }
});

test('local branch and commit comparisons remain usable without a published parent', t => {
  const f = fixture(t);
  f.add(migration(early));
  f.commit();
  f.git('branch', 'unpublished-parent');
  const parent = f.git('rev-parse', 'HEAD');
  f.add(migration(later));
  for (const base of ['unpublished-parent', parent]) {
    assert.deepEqual(checkLocalMigrationVersions({ cwd: f.cwd, base }), []);
  }
});

test('local stack refs check the current published parent as well as development', t => {
  const f = fixture(t);
  f.git('branch', 'parent');
  f.add(migration(early));
  f.commit();
  f.git('checkout', '-q', 'parent');
  f.add(`backend/database/migrations/${early}_parent.go`);
  f.commit();
  f.git('checkout', '-q', 'feature');
  f.git('fetch', '-q', 'origin');
  const errors = checkLocalMigrationVersions({ cwd: f.cwd, base: 'parent' });
  assert.equal(errors.length, 1);
  assert.match(errors[0], new RegExp(`${early}.*origin/parent`));
});

test('local development release accepts promoted migrations but checks local additions', t => {
  const f = fixture(t);
  f.git('branch', 'main', 'development');
  f.git('checkout', '-q', 'development');
  f.add(migration(early));
  f.commit();
  assert.deepEqual(checkLocalMigrationVersions({ cwd: f.cwd, base: 'origin/main' }), []);
  f.add(migration('001015840'));
  const errors = checkLocalMigrationVersions({ cwd: f.cwd, base: 'origin/main' });
  assert.equal(errors.length, 1);
  assert.match(errors[0], /001015840.*kein Zeitstempel/);
});

test('only a real UTC timestamp that is not in the future names a new migration', t => {
  const f = fixture(t);
  const rejected = { '000002': /kein Zeitstempel/, '001015801': /kein Zeitstempel/, '0010157991': /kein Zeitstempel/,
    '20261341250000': /kein gültiges Datum/, '20260230120000': /kein gültiges Datum/, '20261115000000': /in der Zukunft/ };
  for (const number of Object.keys(rejected)) f.add(migration(number));
  f.add(migration('20261113235959'));
  const errors = checkMigrationVersions({ cwd: f.cwd, worktree: true, now });
  assert.equal(errors.length, Object.keys(rejected).length, errors.join('\n'));
  for (const [number, reason] of Object.entries(rejected)) {
    const error = errors.find(candidate => candidate.startsWith(`${migration(number)}:`));
    assert.match(error, reason);
    assert.match(error, /date -u \+%Y%m%d%H%M%S/);
  }
});

test('a new migration may create a function but not redefine an existing one', t => {
  const f = fixture(t);
  f.git('checkout', '-q', 'development');
  f.add(migration('001015700'), 'const sql = `CREATE OR REPLACE FUNCTION platform.erase_visitors() RETURNS void`\n');
  f.add('backend/database/migrations/erase_helper.go', 'const sql = `create\tfunction  Auth.is_unused(BIGINT) RETURNS boolean`\n');
  f.commit();
  f.git('checkout', '-q', 'feature');
  f.git('merge', '-q', 'development');
  f.add(migration(early), 'const sql = `CREATE FUNCTION platform.brand_new() RETURNS void`\n');
  f.add('backend/database/migrations/repeatable_platform_erase_visitors.go',
    'const sql = `CREATE OR REPLACE FUNCTION platform.erase_visitors() RETURNS void`\n');
  f.add('backend/database/migrations/20261101090000_fixture_test.go', '// CREATE OR REPLACE FUNCTION platform.erase_visitors()\n');
  f.commit();
  assert.equal(f.run().status, 0, 'a new function, the repeatable definition and a test file are fine');

  f.add(migration(later), 'const sql = `CREATE  OR REPLACE\n\tFUNCTION platform.erase_visitors() RETURNS void`\n');
  f.commit();
  f.add('backend/database/migrations/erase_helper_two.go', 'const sql = `CREATE OR REPLACE FUNCTION auth.is_unused(BIGINT) RETURNS boolean`\n');
  const errors = checkMigrationVersions({ cwd: f.cwd, worktree: true, now });
  assert.equal(errors.length, 2, errors.join('\n'));
  assert.match(errors[0], new RegExp(`${later}_fixture\\.go: definiert die bestehende Funktion platform\\.erase_visitors neu.*repeatable_platform_erase_visitors\\.go`));
  assert.match(errors[1], /erase_helper_two\.go: definiert die bestehende Funktion auth\.is_unused neu/);
});
