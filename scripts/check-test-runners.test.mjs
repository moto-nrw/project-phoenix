import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import test from 'node:test';
import { findUnrunTests } from './check-test-runners.mjs';

function check(files) {
  return findUnrunTests(Object.keys(files), file => files[file]);
}

test('CLI accepts a tracked-path list over 1 MiB without hiding an unrun test', () => {
  const root = mkdtempSync(join(tmpdir(), 'moto-test-runners-'));
  try {
    const bin = join(root, 'bin');
    mkdirSync(bin);
    mkdirSync(join(root, 'scripts'));
    writeFileSync(join(root, 'scripts/unrun.test.mjs'), '');
    writeFileSync(join(bin, 'git'), `#!${process.execPath}
if (process.argv[2] === 'rev-parse') process.stdout.write(process.env.FIXTURE_ROOT);
else process.stdout.write(('absent/'+'x'.repeat(120)+'\\0').repeat(9000)+'scripts/unrun.test.mjs\\0');
`, { mode: 0o755 });
    const result = spawnSync(process.execPath, [fileURLToPath(new URL('./check-test-runners.mjs', import.meta.url))], {
      cwd: root,
      encoding: 'utf8',
      env: { ...process.env, PATH: `${bin}:${process.env.PATH}`, FIXTURE_ROOT: root },
    });
    assert.equal(result.status, 1, result.stderr);
    assert.match(result.stderr, /Test files without a CI runner/);
    assert.match(result.stderr, /scripts\/unrun\.test\.mjs/);
    assert.doesNotMatch(result.stderr, /ENOBUFS/);
  } finally {
    rmSync(root, { recursive: true, force: true });
  }
});

const packageJson = JSON.stringify({
  scripts: {
    'test:layout': 'playwright test -c playwright.layout.config.ts',
    'test:e2e': 'playwright test',
  },
});

test('script tests need a workflow that runs them, directly or through scripts', () => {
  assert.deepEqual(check({
    '.github/workflows/main.yml': 'run: node --test scripts/direct.test.mjs\nrun: bash scripts/check-quality.sh',
    'scripts/direct.test.mjs': '',
    'scripts/check-quality.sh': 'exec bash "$root/scripts/check-backend-quality.sh"',
    'scripts/check-backend-quality.sh': 'bash "$root/scripts/nested_test.sh"',
    'scripts/nested_test.sh': '',
    'scripts/orphan_test.py': '',
    'monitoring/orphan.test.mjs': '',
  }), ['monitoring/orphan.test.mjs', 'scripts/orphan_test.py']);
});

test('a path matches only as a whole, not inside a longer name', () => {
  assert.deepEqual(check({
    '.github/workflows/main.yml': 'run: python3 -m unittest scripts/foobar_test.py',
    'scripts/foobar_test.py': '',
    'scripts/bar_test.py': '',
  }), ['scripts/bar_test.py']);
});

test('Python dot-test suites need an executable CI runner too', () => {
  const files = { 'scripts/ledger.test.py': '' };
  assert.deepEqual(check(files), ['scripts/ledger.test.py']);
  assert.deepEqual(check({
    ...files,
    '.github/workflows/main.yml': 'run: python3 scripts/ledger.test.py',
  }), []);
});

test('hooks, comments and paths filters do not count as runners', () => {
  assert.deepEqual(check({
    '.github/workflows/main.yml': [
      '# runs scripts/commented.test.mjs',
      '            hooks:',
      "              - 'scripts/pre-push.sh'",
      "              - 'scripts/filtered_test.sh'",
      '              - scripts/unquoted_test.sh',
      'run: echo ok # scripts/trailing_test.sh',
    ].join('\n'),
    'lefthook.yml': 'run: node --test scripts/hooked.test.mjs',
    'scripts/pre-push.sh': 'node --test scripts/hooked.test.mjs',
    'scripts/commented.test.mjs': '',
    'scripts/filtered_test.sh': '',
    'scripts/hooked.test.mjs': '',
    'scripts/unquoted_test.sh': '',
    'scripts/trailing_test.sh': '',
  }), [
    'scripts/commented.test.mjs',
    'scripts/filtered_test.sh',
    'scripts/hooked.test.mjs',
    'scripts/trailing_test.sh',
    'scripts/unquoted_test.sh',
  ]);
});

test('helper comments do not count as direct or transitive runners', () => {
  assert.deepEqual(check({
    '.github/workflows/main.yml': 'run: bash scripts/check.sh',
    'scripts/check.sh': [
      '# removed: node --test scripts/orphan.test.mjs',
      'echo ok # bash scripts/removed.sh',
      'bash scripts/active.sh # node --test scripts/trailing.test.mjs',
    ].join('\n'),
    'scripts/removed.sh': 'node --test scripts/transitive.test.mjs',
    'scripts/active.sh': 'node --test scripts/active.test.mjs',
    'scripts/orphan.test.mjs': '',
    'scripts/trailing.test.mjs': '',
    'scripts/transitive.test.mjs': '',
    'scripts/active.test.mjs': '',
  }), [
    'scripts/orphan.test.mjs',
    'scripts/trailing.test.mjs',
    'scripts/transitive.test.mjs',
  ]);
});

test('Go and Vitest files are left to their glob-based runners', () => {
  assert.deepEqual(check({
    'backend/core/core_test.go': '',
    'frontend/src/lib/date.test.ts': '',
    'frontend/scripts/plugin.test.mjs': '',
    // Go never runs a shell suite, even under backend/.
    'backend/scripts/helper_test.sh': '',
  }), ['backend/scripts/helper_test.sh']);
});

test('Playwright specs need a config whose package script a workflow runs', () => {
  const stack = {
    'frontend/package.json': packageJson,
    'frontend/playwright.config.ts': 'testDir: "./e2e",\n  testIgnore: "layout/**",',
    'frontend/playwright.layout.config.ts': 'testDir: "./e2e/layout",',
    'frontend/e2e/flow.spec.ts': '',
    'frontend/e2e/layout/grid.spec.ts': '',
    'frontend/e2e/layout/harness.ts': '',
  };
  assert.deepEqual(check({
    ...stack,
    '.github/workflows/test.yml': 'run: pnpm run test:layout',
    '.github/workflows/e2e.yml': 'run: pnpm run test:e2e',
  }), []);
  // Without e2e.yml the default config runs nowhere; testIgnore keeps the
  // layout specs on the layout config.
  assert.deepEqual(check({
    ...stack,
    '.github/workflows/test.yml': 'run: pnpm run test:layout',
  }), ['frontend/e2e/flow.spec.ts']);
  // A script named like a prefix of another (test:e2e-smoke) is not test:e2e.
  assert.deepEqual(check({
    ...stack,
    '.github/workflows/e2e.yml': 'run: pnpm run test:e2e-smoke',
  }), ['frontend/e2e/flow.spec.ts', 'frontend/e2e/layout/grid.spec.ts']);
});

test('testMatch narrows a config to the specs it names', () => {
  assert.deepEqual(check({
    'frontend/package.json': packageJson,
    'frontend/playwright.config.ts': 'testDir: "./e2e",\n  testMatch: "smoke.spec.ts",',
    'frontend/e2e/smoke.spec.ts': '',
    'frontend/e2e/other.spec.ts': '',
    '.github/workflows/e2e.yml': 'run: pnpm run test:e2e',
  }), ['frontend/e2e/other.spec.ts']);
});

test('testIgnore accepts an array of globs', () => {
  assert.deepEqual(check({
    'frontend/package.json': packageJson,
    'frontend/playwright.config.ts': 'testDir: "./e2e",\n  testIgnore: ["layout/**", "flaky.spec.ts"],',
    'frontend/e2e/flow.spec.ts': '',
    'frontend/e2e/flaky.spec.ts': '',
    '.github/workflows/e2e.yml': 'run: pnpm run test:e2e',
  }), ['frontend/e2e/flaky.spec.ts']);
});

test('a Playwright option the guard cannot read fails instead of passing', () => {
  assert.throws(() => check({
    'frontend/package.json': packageJson,
    'frontend/playwright.config.ts': 'testDir: "./e2e",\n  testIgnore: /layout/,',
    'frontend/e2e/flow.spec.ts': '',
  }), /testIgnore must be a string literal/);
});
