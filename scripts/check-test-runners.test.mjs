import assert from 'node:assert/strict';
import test from 'node:test';
import { findUnrunTests } from './check-test-runners.mjs';

function check(files) {
  return findUnrunTests(Object.keys(files), file => files[file]);
}

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
