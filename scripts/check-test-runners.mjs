#!/usr/bin/env node
// Lists test files that no CI runner picks up (#4436).
//
// Go (`go test ./...`) and Vitest find their tests by glob, so a new
// `_test.go` or `src/**/*.test.ts` cannot fall outside them. Two kinds of tests
// need an explicit runner and are checked here:
//   1. Repository script tests outside backend/ and frontend/
//      (`*.test.{js,mjs,cjs,ts}`, `*_test.{py,sh}`). A workflow must name the
//      file, or name a script that names it (scripts/check-backend-quality.sh
//      runs check-deadcode_test.sh, for example). Git hooks alone do not count.
//   2. Playwright specs under frontend/e2e (Vitest excludes them). The spec
//      must lie in the testDir of a Playwright config, outside its testIgnore,
//      and a workflow must run a package script that runs that config.
// Playwright tools under frontend/scripts (screenshots, guides, perf) are
// generators run by their own workflows, not tests, and stay out of scope.
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const scriptTest = /(?:\.test\.[cm]?[jt]s|_test\.(?:py|sh))$/;
const playwrightSpec = /^frontend\/e2e\/.+\.spec\.ts$/;

function stringOption(source, key) {
  return source.match(new RegExp(`\\b${key}:\\s*["']([^"']+)["']`))?.[1];
}

// Playwright prefixes relative globs with `**/`; enough for testIgnore values
// such as "layout/**".
function globToRegExp(glob) {
  const pattern = glob.startsWith('**/') ? glob : `**/${glob}`;
  let out = '';
  for (let i = 0; i < pattern.length; i++) {
    const char = pattern[i];
    if (pattern.startsWith('**/', i)) { out += '(?:.*/)?'; i += 2; }
    else if (pattern.startsWith('**', i)) { out += '.*'; i += 1; }
    else if (char === '*') out += '[^/]*';
    else if (char === '?') out += '[^/]';
    else out += char.replace(/[.+^${}()|[\]\\]/g, '\\$&');
  }
  return new RegExp(`^${out}$`);
}

/**
 * @param {string[]} files tracked paths, repository-relative with `/`
 * @param {(file: string) => string} read file contents
 * @returns {string[]} test files without a runner
 */
export function findUnrunTests(files, read) {
  // Comments and quoted list items (paths filters, path triggers) name files
  // without running them.
  const workflows = files
    .filter(file => /^\.github\/workflows\/[^/]+\.ya?ml$/.test(file))
    .map(file => read(file)
      .split('\n')
      .filter(line => !/^\s*#/.test(line) && !/^\s*-\s*(['"])[^'"]*\1\s*(?:#.*)?$/.test(line))
      .join('\n'));
  const inWorkflow = text => workflows.some(workflow => workflow.includes(text));
  // Scripts a workflow runs, directly or through other such scripts
  // (lint.yml -> check-quality.sh -> check-backend-quality.sh).
  const helpers = files.filter(file => file.startsWith('scripts/') && !scriptTest.test(file));
  const reached = new Set(helpers.filter(file => inWorkflow(file)));
  for (let grew = true; grew;) {
    grew = false;
    const texts = [...reached].map(read);
    for (const file of helpers) {
      if (!reached.has(file) && texts.some(text => text.includes(`scripts/${path.posix.basename(file)}`))) {
        reached.add(file);
        grew = true;
      }
    }
  }
  const ciScripts = [...reached].map(read);

  const unrun = files
    .filter(file => !file.startsWith('backend/') && !file.startsWith('frontend/') && scriptTest.test(file))
    .filter(file => {
      const name = path.posix.basename(file);
      return !inWorkflow(name) && !ciScripts.some(script => script.includes(name));
    });

  const scripts = files.includes('frontend/package.json')
    ? Object.entries(JSON.parse(read('frontend/package.json')).scripts ?? {})
    : [];
  const configs = files
    .filter(file => /^frontend\/playwright(?:\.[\w-]+)?\.config\.ts$/.test(file))
    .map(file => {
      const source = read(file);
      const name = path.posix.basename(file);
      const testDir = path.posix.join('frontend', stringOption(source, 'testDir') ?? '.');
      const ignore = stringOption(source, 'testIgnore');
      const run = scripts.some(([script, command]) => {
        if (!/\bplaywright test\b/.test(command)) return false;
        const config = command.match(/(?:-c|--config)[ =](\S+)/)?.[1] ?? 'playwright.config.ts';
        return path.posix.basename(config) === name &&
          workflows.some(workflow => new RegExp(`\\bpnpm (?:run )?${script.replace(/[.*+?^${}()|[\]\\:]/g, '\\$&')}(?![\\w:-])`).test(workflow));
      });
      return { testDir, ignore: ignore ? globToRegExp(ignore) : null, run };
    });

  for (const file of files.filter(file => playwrightSpec.test(file))) {
    const covered = configs.some(({ testDir, ignore, run }) => {
      if (!run || !file.startsWith(`${testDir}/`)) return false;
      return !ignore?.test(file.slice(testDir.length + 1));
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
