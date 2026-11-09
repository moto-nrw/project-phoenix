import assert from 'node:assert/strict';
import { execFileSync, spawnSync } from 'node:child_process';
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import test from 'node:test';
import { checkMigrationVersions } from './check-migration-versions.mjs';
import { gitTestEnvironment, initGitFixture } from './test-helpers/git-fixture.mjs';

const script = resolve('scripts/check-migration-versions.mjs');
const migration = number => `backend/database/migrations/${number}_fixture.go`;
function fixture(t) {
  const cwd = mkdtempSync(join(tmpdir(), 'migration-versions-'));
  t.after(() => rmSync(cwd, { recursive: true, force: true }));
  const env = gitTestEnvironment();
  const git = (...args) => execFileSync('git', args, { cwd, env, encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'] }).trim();
  initGitFixture(git);
  const add = file => {
    mkdirSync(dirname(join(cwd, file)), { recursive: true });
    writeFileSync(join(cwd, file), '// fixture\n');
  };
  const commit = () => { git('add', '.'); git('commit', '-qm', 'fixture'); };
  add(migration('001015800'));
  commit();
  git('branch', '-M', 'development');
  git('remote', 'add', 'origin', cwd);
  git('fetch', '-q', 'origin');
  git('checkout', '-qb', 'feature');
  const run = (base = 'origin/development', head = 'HEAD') => spawnSync(process.execPath, [script, base, head], { cwd, env, encoding: 'utf8' });
  return { cwd, git, add, commit, run };
}

test('higher added versions pass, historical edits and test files are ignored', t => {
  const f = fixture(t);
  f.add(migration('001015844'));
  f.add('backend/database/migrations/001015100_fixture_test.go');
  writeFileSync(join(f.cwd, migration('001015800')), '// edited history\n');
  f.commit();
  const result = f.run();
  assert.equal(result.status, 0, result.stderr);
  assert.match(result.stdout, /OK/);
});

test('lower or equal versions fail against a newly advanced remote tip, not the merge-base', t => {
  const f = fixture(t);
  f.add(migration('001015840'));
  f.add(migration('001015843'));
  f.commit();
  f.git('checkout', '-q', 'development');
  f.add('backend/database/migrations/001015843_other.go');
  f.commit();
  f.git('checkout', '-q', 'feature');
  const result = f.run();
  assert.equal(result.status, 1);
  for (const number of ['001015840', '001015843']) {
    assert.ok(result.stderr.includes(`${migration(number)}: Migrationsnummer ${number}`));
  }
  assert.match(result.stderr, /001015843.*origin\/development/);
  assert.match(result.stderr, /umnummerieren/);
});

test('stacked PR checks its own additions against both current base and development', t => {
  const f = fixture(t);
  f.git('branch', '-m', 'parent');
  f.add(migration('001015820'));
  f.commit();
  f.git('checkout', '-qb', 'child');
  f.add(migration('001015840'));
  f.commit();
  f.git('checkout', '-q', 'parent');
  f.add(migration('001015850'));
  f.commit();
  f.git('checkout', '-q', 'development');
  f.add(migration('001015843'));
  f.commit();
  f.git('checkout', '-q', 'child');
  const result = f.run('origin/parent');
  assert.equal(result.status, 1);
  assert.match(result.stderr, /001015850.*origin\/parent/);
  assert.match(result.stderr, /001015843.*origin\/development/);
  assert.ok(!result.stderr.includes(`${migration('001015820')}:`), 'inherited migration is not an addition of the child');
});

test('local worktree includes staged and untracked migrations; renames count as additions', t => {
  const f = fixture(t);
  f.git('mv', migration('001015800'), migration('001015799'));
  f.add(migration('001015798'));
  const errors = checkMigrationVersions({ cwd: f.cwd, worktree: true });
  assert.equal(errors.length, 2);
  assert.ok(errors.some(error => error.includes(migration('001015799'))));
  assert.ok(errors.some(error => error.includes(migration('001015798'))));
});

test('missing remote base fails closed', t => {
  const f = fixture(t);
  const result = f.run('origin/missing');
  assert.equal(result.status, 2);
  assert.match(result.stderr, /Migration versions:/);
});
