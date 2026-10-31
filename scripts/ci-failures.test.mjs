import assert from 'node:assert/strict';
import test from 'node:test';

import { cleanLog, digest, failedStep, jobKey, localCommand, repeatedFailures } from './ci-failures.mjs';

const log = [
  '﻿2026-10-31T19:49:15.6579162Z Current runner version: \'2.338.1\'',
  '2026-10-31T19:49:16.0000000Z ##[group]Run pnpm install --frozen-lockfile',
  '2026-10-31T19:49:16.0000000Z pnpm install --frozen-lockfile',
  '2026-10-31T19:49:16.0000000Z ##[endgroup]',
  '2026-10-31T19:49:20.0000000Z Done in 4s',
  '2026-10-31T19:49:21.0000000Z ##[group]Run node scripts/test-frontend-changed.mjs --check-guards',
  '2026-10-31T19:49:21.0000000Z node scripts/test-frontend-changed.mjs --check-guards',
  '2026-10-31T19:49:21.0000000Z shell: /usr/bin/bash -e {0}',
  '2026-10-31T19:49:21.0000000Z ##[endgroup]',
  '2026-10-31T19:49:22.0000000Z \x1b[31mAssertionError [ERR_ASSERTION]: Filesystem/process guard inventory differs\x1b[39m',
  '2026-10-31T19:49:22.0000000Z PASS: tests findings still fail',
  '2026-10-31T19:49:22.0000000Z 103. V1.12.6 - Remove unused is_internal column',
  '2026-10-31T19:49:22.0000000Z -   \'src/lib/moto-concepts.test.ts\',',
  '2026-10-31T19:49:23.0000000Z ##[error]Process completed with exit code 1.',
  '2026-10-31T19:49:24.0000000Z ##[group]Run actions/upload-artifact@v6',
];

test('job keys drop the caller workflow and matrix suffix', () => {
  assert.equal(jobKey('lint / frontend-quality'), 'frontend-quality');
  assert.equal(jobKey('test / test-frontend (dom-2)'), 'test-frontend');
  assert.equal(jobKey('ci-gate'), 'ci-gate');
  assert.match(localCommand('lint / backend-quality'), /check-deadcode\.sh/);
  assert.equal(localCommand('lint / not-a-mapped-job'), undefined);
});

test('the failed step is the last run group before the non-zero exit', () => {
  const step = failedStep(cleanLog(log.join('\n')));
  assert.equal(step.command, 'node scripts/test-frontend-changed.mjs --check-guards');
  assert.deepEqual(step.output, [
    'AssertionError [ERR_ASSERTION]: Filesystem/process guard inventory differs',
    'PASS: tests findings still fail',
    '103. V1.12.6 - Remove unused is_internal column',
    '-   \'src/lib/moto-concepts.test.ts\',',
  ]);
});

test('an action step that only annotates an error still yields its section', () => {
  const step = failedStep(cleanLog([
    '##[group]Run golangci/golangci-lint-action@v9',
    '##[endgroup]',
    'running golangci-lint',
    '##[error]api/x.go:1:1: unused variable (unused)',
  ].join('\n')));
  assert.equal(step.command, 'golangci/golangci-lint-action@v9');
  assert.deepEqual(step.output, ['running golangci-lint', '##[error]api/x.go:1:1: unused variable (unused)']);
  assert.equal(failedStep(['##[group]Run true', '##[endgroup]', 'ok']), undefined);
});

test('the digest leads with findings, skips passing noise and keeps the tail', () => {
  const output = [
    'ok  	github.com/x/a	1.2s',
    '--- FAIL: TestSomething (0.10s)',
    ...Array.from({ length: 40 }, (_, index) => `noise ${index}`),
    '##[error]src/a.tsx:1:1: Build resource keys in ~/lib/swr factories',
    'PASS: findings still fail',
    'Found 0 warnings and 1 error.',
  ];
  const lines = digest(output);
  assert.equal(lines[0], '##[error]src/a.tsx:1:1: Build resource keys in ~/lib/swr factories');
  assert.equal(lines[1], '--- FAIL: TestSomething (0.10s)');
  assert.ok(!lines.includes('ok  	github.com/x/a	1.2s'));
  assert.match(lines[2], /^… \d+ more lines …$/);
  assert.equal(lines.at(-1), 'Found 0 warnings and 1 error.');
  assert.equal(lines.filter(line => line.startsWith('##[error]')).length, 1);
  assert.ok(lines.length <= 30 + 1 + 12);
});

test('repeated failures compare jobs, not matrix shards', () => {
  assert.deepEqual(
    repeatedFailures(['test / test-frontend (node)', 'lint / frontend-quality'], ['test / test-frontend (dom-1)', 'lint / backend-lint']),
    ['test-frontend'],
  );
  assert.deepEqual(repeatedFailures(['lint / frontend-build'], []), []);
});
