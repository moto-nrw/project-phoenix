#!/usr/bin/env node
// Lists test files that no CI runner picks up (#4436).
//
// Go (`go test ./...`) and Vitest find their tests by glob, so a new
// `_test.go` or `frontend/src/**/*.test.ts` cannot fall outside them. Two kinds
// of tests need an explicit runner and are checked here:
//   1. Script tests outside frontend/ (`*.test.{js,mjs,cjs,ts}`,
//      `*_test.{py,sh}`, also under backend/, where Go never runs them). A
//      workflow must name the file by its repository path, or name a script
//      that does (lint.yml -> check-quality.sh -> check-backend-quality.sh ->
//      check-deadcode_test.sh). Hooks, comments and paths filters do not count.
//   2. Playwright specs under frontend/e2e (Vitest excludes them). The spec
//      must match a Playwright config (testDir, testMatch, testIgnore), and a
//      workflow must run a package script that runs that config.
// Playwright tools under frontend/scripts (screenshots, guides, perf) are
// generators run by their own workflows, not tests, and stay out of scope.
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const scriptTest = /(?:\.test\.[cm]?[jt]s|_test\.(?:py|sh))$/;
const playwrightSpec = /^frontend\/e2e\/.+\.spec\.ts$/;
const escape = text => text.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');

// A whole repository path, optionally behind `$root/`, `${ROOT}/` or `../`;
// `scripts/bar_test.py` does not match inside `scripts/foobar_test.py`.
function namesPath(text, file) {
  return new RegExp(`(?:^|[^\\w./-])(?:\\$\\{?\\w+\\}?/|(?:\\.\\./)+)?${escape(file)}(?![\\w.-])`, 'm').test(text);
}

// Playwright prefixes relative globs with `**/`.
function globToRegExp(glob) {
  const pattern = glob.startsWith('**/') ? glob : `**/${glob}`;
  let out = '';
  for (let i = 0; i < pattern.length; i++) {
    const char = pattern[i];
    if (pattern.startsWith('**/', i)) { out += '(?:.*/)?'; i += 2; }
    else if (pattern.startsWith('**', i)) { out += '.*'; i += 1; }
    else if (char === '*') out += '[^/]*';
    else if (char === '?') out += '[^/]';
    else out += escape(char);
  }
  return new RegExp(`^${out}$`);
}

// Only a string literal or an array of string literals is understood; anything
// else fails loudly instead of counting ignored specs as covered.
function globOption(source, key, file) {
  if (!new RegExp(`\\b${key}:`).test(source)) return null;
  const literal = `["'][^"']+["']`;
  const value = source.match(new RegExp(`\\b${key}:\\s*(${literal}|\\[\\s*${literal}(?:\\s*,\\s*${literal})*\\s*,?\\s*\\])`))?.[1];
  if (!value) throw new Error(`${file}: ${key} must be a string literal or an array of them for check-test-runners.mjs`);
  const globs = [...value.matchAll(/["']([^"']+)["']/g)].map(match => globToRegExp(match[1]));
  return { test: relative => globs.some(glob => glob.test(relative)) };
}

/**
 * @param {string[]} files tracked paths, repository-relative with `/`
 * @param {(file: string) => string} read file contents
 * @returns {string[]} test files without a runner
 */
export function findUnrunTests(files, read) {
  // Comments and list items that hold only a path (paths filters, path
  // triggers) name files without running them.
  const workflows = files
    .filter(file => /^\.github\/workflows\/[^/]+\.ya?ml$/.test(file))
    .map(file => read(file)
      .split('\n')
      .filter(line => !/^\s*#/.test(line) && !/^\s*-\s*(['"]?)[^\s'":]+\1\s*(?:#.*)?$/.test(line))
      .map(line => line.replace(/\s+#.*$/, ''))
      .join('\n'));
  const inWorkflow = file => workflows.some(workflow => namesPath(workflow, file));

  // Scripts a workflow runs, directly or through other such scripts.
  const helpers = files.filter(file => file.startsWith('scripts/') && !scriptTest.test(file));
  const reached = new Set(helpers.filter(inWorkflow));
  for (let grew = true; grew;) {
    grew = false;
    const texts = [...reached].map(read);
    for (const file of helpers) {
      if (!reached.has(file) && texts.some(text => namesPath(text, file))) {
        reached.add(file);
        grew = true;
      }
    }
  }
  const ciScripts = [...reached].map(read);

  const unrun = files
    .filter(file => !file.startsWith('frontend/') && scriptTest.test(file))
    .filter(file => !inWorkflow(file) && !ciScripts.some(script => namesPath(script, file)));

  const scripts = files.includes('frontend/package.json')
    ? Object.entries(JSON.parse(read('frontend/package.json')).scripts ?? {})
    : [];
  const configs = files
    .filter(file => /^frontend\/playwright(?:\.[\w-]+)?\.config\.ts$/.test(file))
    .map(file => {
      const source = read(file);
      const name = path.posix.basename(file);
      const testDir = path.posix.join('frontend', source.match(/\btestDir:\s*["']([^"']+)["']/)?.[1] ?? '.');
      const match = globOption(source, 'testMatch', file);
      const ignore = globOption(source, 'testIgnore', file);
      const run = scripts.some(([script, command]) => {
        if (!/\bplaywright test\b/.test(command)) return false;
        const config = command.match(/(?:-c|--config)[ =](\S+)/)?.[1] ?? 'playwright.config.ts';
        return path.posix.basename(config) === name &&
          workflows.some(workflow => new RegExp(`\\bpnpm (?:run )?${escape(script)}(?![\\w:-])`).test(workflow));
      });
      return { testDir, match, ignore, run };
    });

  for (const file of files.filter(file => playwrightSpec.test(file))) {
    const covered = configs.some(({ testDir, match, ignore, run }) => {
      if (!run || !file.startsWith(`${testDir}/`)) return false;
      const relative = file.slice(testDir.length + 1);
      return (!match || match.test(relative)) && !ignore?.test(relative);
    });
    if (!covered) unrun.push(file);
  }
  return unrun.sort();
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  const root = execFileSync('git', ['rev-parse', '--show-toplevel'], { encoding: 'utf8' }).trim();
  const files = execFileSync('git', ['ls-files', '-z'], { cwd: root, encoding: 'utf8' })
    .split('\0')
    .filter(file => file && fs.existsSync(path.join(root, file)));
  const unrun = findUnrunTests(files, file => fs.readFileSync(path.join(root, file), 'utf8'));
  if (unrun.length > 0) {
    console.error('Test files without a CI runner (hook them into a workflow or delete them):');
    for (const file of unrun) console.error(`  ${file}`);
    process.exit(1);
  }
  console.log('Every checked test file has a CI runner.');
}
