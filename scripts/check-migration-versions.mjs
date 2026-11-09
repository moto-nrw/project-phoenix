#!/usr/bin/env node
// Compare PR additions with current remote tips, not the historical fork point.
import { execFileSync } from 'node:child_process';
import { pathToFileURL } from 'node:url';

const directory = 'backend/database/migrations/';
const migration = /^backend\/database\/migrations\/(\d{1,14})_.+\.go$/;
function version(path) {
  return path.endsWith('_test.go') ? null : migration.exec(path)?.[1] ?? null;
}
function git(cwd, args) {
  return execFileSync('git', args, { cwd, encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'] }).trim();
}
function bases(base) {
  return [...new Set([base, 'origin/development'])];
}

export function fetchMigrationBases(cwd, base) {
  if (!base.startsWith('origin/')) throw new Error('migration base must be an origin/<branch> ref');
  const refs = bases(base).map(ref => {
    const branch = ref.slice('origin/'.length);
    git(cwd, ['check-ref-format', `refs/heads/${branch}`]);
    return `+refs/heads/${branch}:refs/remotes/${ref}`;
  });
  git(cwd, ['fetch', '--quiet', '--no-tags', 'origin', ...refs]);
}

export function checkMigrationVersions({ cwd, base = 'origin/development', head = 'HEAD', worktree = false, comparisonBase = base }) {
  const mergeBase = git(cwd, ['merge-base', comparisonBase, head]);
  // Disable rename detection: a renumbered migration is a new migration too.
  const added = git(cwd, ['diff', '--no-renames', '--diff-filter=A', '--name-only', '-z', mergeBase,
    ...(worktree ? [] : [head]), '--', directory]).split('\0');
  if (worktree) added.push(...git(cwd, ['ls-files', '--others', '--exclude-standard', '-z', '--', directory]).split('\0'));
  const errors = [];
  for (const ref of bases(base)) {
    const highest = git(cwd, ['ls-tree', '-r', '--name-only', '-z', ref, '--', directory])
      .split('\0').map(version).filter(Boolean).sort().at(-1);
    for (const file of new Set(added)) {
      const number = version(file);
      if (number && number.length !== 9) {
        errors.push(`${file}: Migrationsnummer ${number} muss neun Ziffern haben (höchste auf ${ref}: ${highest ?? 'keine'}). Auf eine höhere freie neunstellige Nummer umnummerieren.`);
      } else if (number && highest && number <= highest) {
        errors.push(`${file}: Migrationsnummer ${number} muss größer als ${highest} sein (höchste auf ${ref}). Migration auf eine höhere freie Nummer umnummerieren.`);
      }
    }
  }
  return errors;
}

// Keep check-changed's local branch/SHA comparison contract independent of the
// remote tips used as version ceilings. Unpublished parents have no remote tip.
export function checkLocalMigrationVersions({ cwd, base = 'origin/development' }) {
  let remoteBase = base.startsWith('origin/') ? base : 'origin/development';
  if (!base.startsWith('origin/')) {
    try {
      git(cwd, ['show-ref', '--verify', `refs/remotes/origin/${base}`]);
      remoteBase = `origin/${base}`;
    } catch { /* A commit SHA or unpublished branch still compares with development. */ }
  }
  fetchMigrationBases(cwd, remoteBase);
  // Release additions already in development were checked there. Still inspect
  // local additions against development, so a dirty release cannot bypass it.
  const release = remoteBase === 'origin/main' && git(cwd, ['rev-parse', '--abbrev-ref', 'HEAD']) === 'development';
  return checkMigrationVersions({ cwd, base: remoteBase, comparisonBase: release ? 'origin/development' : base, worktree: true });
}

if (import.meta.url === pathToFileURL(process.argv[1]).href) {
  try {
    if (process.argv.length > 4) throw new Error('Usage: node scripts/check-migration-versions.mjs [origin/<base>] [head]');
    const base = process.argv[2] ?? `origin/${process.env.GITHUB_BASE_REF || 'development'}`;
    let errors;
    if (process.argv[3]) {
      fetchMigrationBases(process.cwd(), base);
      errors = checkMigrationVersions({ cwd: process.cwd(), base, head: process.argv[3] });
    } else errors = checkLocalMigrationVersions({ cwd: process.cwd(), base });
    if (errors.length) {
      console.error(errors.join('\n'));
      process.exitCode = 1;
    } else console.log('Migration versions: OK');
  } catch (error) {
    console.error(`Migration versions: ${error.message}`);
    process.exitCode = 2;
  }
}
