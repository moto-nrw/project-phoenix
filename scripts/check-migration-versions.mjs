#!/usr/bin/env node
// Checks the migrations a pull request adds (ADR 0223):
//
//   - A new migration is named after a UTC timestamp, YYYYMMDDHHMMSS, that no
//     other migration has. Two branches then never compete for a number.
//   - A new migration does not define a function that already exists. Its
//     current definition belongs in a repeatable_<schema>_<name>.go file, where
//     two branches that change it conflict in git instead of overwriting each
//     other (#6391).
//
// With --late it prints the new migrations that sort below the newest one of
// the base, separated by commas. Those reach a deployed database after later
// ones, and TestLateMigrationsYieldTheFreshSchema replays exactly that order.
//
// Compare additions with current remote tips, not the historical fork point.
import { execFileSync } from 'node:child_process';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { pathToFileURL } from 'node:url';

const directory = 'backend/database/migrations/';
const migration = /^backend\/database\/migrations\/(\d{1,14})_.+\.go$/;
const repeatable = /^backend\/database\/migrations\/repeatable_.+\.go$/;
const definedFunction = /CREATE\s+(?:OR\s+REPLACE\s+)?FUNCTION\s+([a-z_][a-z0-9_]*\.[a-z_][a-z0-9_]*)/gi;
// One day of tolerance for a clock that runs ahead.
const futureToleranceMs = 24 * 60 * 60 * 1000;

function version(path) {
  return path.endsWith('_test.go') ? null : migration.exec(path)?.[1] ?? null;
}
function git(cwd, args) {
  return execFileSync('git', args, { cwd, encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'], maxBuffer: 1 << 28 }).trim();
}
function bases(base) {
  return [...new Set([base, 'origin/development'])];
}

// timestampError returns why number is no usable timestamp, or null.
function timestampError(number, now) {
  const parts = /^(\d{4})(\d{2})(\d{2})(\d{2})(\d{2})(\d{2})$/.exec(number)?.slice(1).map(Number);
  if (!parts) return 'ist kein Zeitstempel aus 14 Ziffern';
  const [year, month, day, hour, minute, second] = parts;
  const time = Date.UTC(year, month - 1, day, hour, minute, second);
  if (new Date(time).toISOString().replace(/\D/g, '').slice(0, 14) !== number) return 'ist kein gültiges Datum';
  if (time > now + futureToleranceMs) return 'liegt in der Zukunft';
  return null;
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

// The files the branch adds since its merge-base. Rename detection is off: a
// renamed migration is a new migration too.
function addedFiles(cwd, mergeBase, head, worktree) {
  const added = git(cwd, ['diff', '--no-renames', '--diff-filter=A', '--name-only', '-z', mergeBase,
    ...(worktree ? [] : [head]), '--', directory]).split('\0');
  if (worktree) added.push(...untrackedFiles(cwd));
  return [...new Set(added.filter(Boolean))];
}
function untrackedFiles(cwd) {
  return git(cwd, ['ls-files', '--others', '--exclude-standard', '-z', '--', directory]).split('\0').filter(Boolean);
}
// The migrations of ref as a map from number to file.
function migrationsOn(cwd, ref) {
  const numbers = new Map();
  for (const file of git(cwd, ['ls-tree', '-r', '--name-only', '-z', ref, '--', directory]).split('\0')) {
    const number = version(file);
    if (number) numbers.set(number, file);
  }
  return numbers;
}

function functionNames(text) {
  return [...text.matchAll(definedFunction)].map(match => match[1].toLowerCase());
}
// git grep exits 1 without a match.
function grep(cwd, args) {
  try {
    return git(cwd, ['grep', '-h', '-i', '-o', '-E', ...args]);
  } catch (error) {
    if (error.status === 1) return '';
    throw error;
  }
}
// The lines the branch adds to Go sources of the migrations, by file.
function addedLines(cwd, mergeBase, head, worktree) {
  const lines = new Map();
  let file = null;
  const diff = git(cwd, ['diff', '--no-renames', '--no-color', '--src-prefix=a/', '--dst-prefix=b/', '-U0', mergeBase, ...(worktree ? [] : [head]), '--', directory]);
  for (const line of diff.split('\n')) {
    if (line.startsWith('+++ ')) file = line.startsWith('+++ b/') ? line.slice('+++ b/'.length) : null;
    else if (file && line.startsWith('+')) lines.set(file, `${lines.get(file) ?? ''}${line.slice(1)}\n`);
  }
  if (worktree) for (const untracked of untrackedFiles(cwd)) lines.set(untracked, readFileSync(join(cwd, untracked), 'utf8'));
  return lines;
}
function redefinitionErrors({ cwd, mergeBase, head, worktree }) {
  const existing = new Set(functionNames(grep(cwd, ['CREATE +(OR +REPLACE +)?FUNCTION +[a-z_0-9]+\\.[a-z_0-9]+',
    mergeBase, '--', directory, `:(exclude)${directory}*_test.go`])));
  const errors = [];
  for (const [file, text] of addedLines(cwd, mergeBase, head, worktree)) {
    if (!file.endsWith('.go') || file.endsWith('_test.go') || repeatable.test(file)) continue;
    for (const name of new Set(functionNames(text))) {
      if (!existing.has(name)) continue;
      errors.push(`${file}: definiert die bestehende Funktion ${name} neu. Ihre aktuelle Definition gehört nach ${directory}repeatable_${name.replace('.', '_')}.go (RegisterRepeatable, ADR 0223), nicht in eine Migration: dort überschreiben sich zwei Branches gegenseitig.`);
    }
  }
  return errors;
}

export function checkMigrationVersions({ cwd, base = 'origin/development', head = 'HEAD', worktree = false, comparisonBase = base, now = Date.now() }) {
  const mergeBase = git(cwd, ['merge-base', comparisonBase, head]);
  const added = addedFiles(cwd, mergeBase, head, worktree);
  const errors = [];
  const own = new Map();
  for (const file of added) {
    const number = version(file);
    if (!number) continue;
    const reason = timestampError(number, now);
    if (reason) {
      errors.push(`${file}: Migrationsnummer ${number} ${reason}. Neue Migrationen heißen nach ihrem UTC-Zeitstempel: date -u +%Y%m%d%H%M%S.`);
    } else if (own.has(number)) {
      errors.push(`${file}: Migrationsnummer ${number} hat auch ${own.get(number)}. Eine der beiden auf einen anderen Zeitstempel umbenennen.`);
    }
    own.set(number, file);
  }
  for (const ref of bases(base)) {
    const taken = migrationsOn(cwd, ref);
    for (const [number, file] of own) {
      if (taken.has(number) && taken.get(number) !== file) {
        errors.push(`${file}: Migrationsnummer ${number} hat auf ${ref} schon ${taken.get(number)}. Auf einen anderen Zeitstempel umbenennen.`);
      }
    }
  }
  errors.push(...redefinitionErrors({ cwd, mergeBase, head, worktree }));
  return errors;
}

// The added migrations that sort below the newest migration of a base: bun
// compares names as strings, so does this.
export function lateMigrations({ cwd, base = 'origin/development', head = 'HEAD', worktree = false, comparisonBase = base }) {
  const mergeBase = git(cwd, ['merge-base', comparisonBase, head]);
  const added = new Map(addedFiles(cwd, mergeBase, head, worktree).map(file => [version(file), file]));
  added.delete(null);
  let newest = '';
  for (const ref of bases(base)) {
    for (const [number, file] of migrationsOn(cwd, ref)) {
      if (added.get(number) !== file && number > newest) newest = number;
    }
  }
  return [...added.keys()].filter(number => number < newest).sort();
}

// Keep check-changed's local branch/SHA comparison contract independent of the
// remote tips used as comparison. Unpublished parents have no remote tip.
function localComparison({ cwd, base }) {
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
  return { cwd, base: remoteBase, comparisonBase: release ? 'origin/development' : base, worktree: true };
}
export function checkLocalMigrationVersions({ cwd, base = 'origin/development' }) {
  return checkMigrationVersions(localComparison({ cwd, base }));
}

if (import.meta.url === pathToFileURL(process.argv[1]).href) {
  try {
    const args = process.argv.slice(2);
    const late = args[0] === '--late';
    if (late) args.shift();
    if (args.length > 2) throw new Error('Usage: node scripts/check-migration-versions.mjs [--late] [origin/<base>] [head]');
    // The pathspecs are relative to the repository root; CI runs this from
    // backend/, where git would resolve them below backend/.
    const cwd = git(process.cwd(), ['rev-parse', '--show-toplevel']);
    const base = args[0] ?? `origin/${process.env.GITHUB_BASE_REF || 'development'}`;
    let comparison;
    if (args[1]) {
      fetchMigrationBases(cwd, base);
      comparison = { cwd, base, head: args[1] };
    } else comparison = localComparison({ cwd, base });
    if (late) {
      console.log(lateMigrations(comparison).join(','));
    } else {
      const errors = checkMigrationVersions(comparison);
      if (errors.length) {
        console.error(errors.join('\n'));
        process.exitCode = 1;
      } else console.log('Migration versions: OK');
    }
  } catch (error) {
    console.error(`Migration versions: ${error.message}`);
    process.exitCode = 2;
  }
}
