import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { mkdtempSync, mkdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import test from 'node:test';
import { gitTestEnvironment, initGitFixture } from './test-helpers/git-fixture.mjs';

function fixture(t) {
  const root = mkdtempSync(join(tmpdir(), 'moto-worker-budget-'));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  for (const dir of ['scripts', 'backend', 'bin', 'tmp']) mkdirSync(join(root, dir), { recursive: true });
  for (const file of ['go-package-workers.sh', 'total-memory-gb.sh', 'test-backend.sh', 'go-test-workspace.sh', 'test-run-id.sh']) {
    writeFileSync(join(root, 'scripts', file), readFileSync(new URL(file, import.meta.url)), { mode: 0o755 });
  }
  const shell = (name, body) => writeFileSync(join(root, 'bin', name), `#!/bin/sh\nset -eu\n${body}\n`, { mode: 0o755 });
  shell('uname', 'echo "$BUDGET_OS"');
  shell('getconf', 'echo "$BUDGET_CPUS"');
  shell('sysctl', '[ "$2" = hw.memsize ]; echo "$BUDGET_BYTES"');
  // The genuine test wrapper executes against a command recorder, never Go/DB.
  shell('go', `
case "$*" in
 'run ./internal/testdb/cmd/bootstrap') echo fixture-template; exit 0 ;;
 'run ./internal/testdb/cmd/sweep') echo sweep >> "$BUDGET_CALLS"; exit 0 ;;
 'tool gotestsum --help') [ "$BUDGET_GOTESTSUM" = yes ]; exit ;;
esac
printf '%s\\n' "$@" > "$BUDGET_ARGS"`);
  const env = gitTestEnvironment({ ...process.env, PATH: `${join(root, 'bin')}:${process.env.PATH}`,
    BUDGET_OS: 'Darwin', BUDGET_CPUS: '16', BUDGET_BYTES: String(64 * 1024 ** 3),
    BUDGET_CALLS: join(root, 'tmp/calls'), BUDGET_ARGS: join(root, 'tmp/args'), BUDGET_GOTESTSUM: 'yes',
    GOTMPDIR: join(root, 'tmp') });
  const git = (...args) => {
    const result = spawnSync('git', args, { cwd: root, env, encoding: 'utf8' });
    assert.equal(result.status, 0, result.stderr);
  };
  initGitFixture(git);
  return { root, env, run: (file, args = [], extra = {}) => spawnSync('bash', [join(root, 'scripts', file), ...args], {
    cwd: root, env: { ...env, ...extra }, encoding: 'utf8', timeout: 10_000,
  }) };
}

// Explicit boundary outcomes, not a second implementation of the budget formula.
for (const [os, cpu, gib, expected] of [
  ['Darwin', 16, 64, 3], ['Darwin', 16, 63, 2], ['Darwin', 16, 32, 2],
  ['Darwin', 4, 64, 2], ['Darwin', 2, 64, 1], ['Darwin', 1, 64, 1],
  ['Darwin', 16, 8, 1], ['Darwin', 16, 0, 1],
  ['Linux', 16, 64, 4], ['Linux', 16, 24, 3], ['Linux', 4, 64, 2], ['Linux', 1, 64, 1],
]) {
  test(`budget ${os} ${cpu} CPUs ${gib} GiB => ${expected}`, t => {
    const f = fixture(t);
    const r = f.run('go-package-workers.sh', [], {
      BUDGET_OS: os, BUDGET_CPUS: String(cpu), BUDGET_BYTES: String(gib * 1024 ** 3),
    });
    assert.equal(r.status, 0, r.stderr);
    assert.equal(r.stdout, `${expected}\n`);
  });
}

test('invalid CPU detection fails instead of inventing a budget', t => {
  const f = fixture(t), r = f.run('go-package-workers.sh', [], { BUDGET_CPUS: 'unknown' });
  assert.equal(r.status, 1);
  assert.match(r.stderr, /invalid CPU count/);
});

for (const gotestsum of ['yes', 'no']) {
  test(`real test-backend wrapper preserves defaults and explicit -p override, gotestsum=${gotestsum}`, t => {
    const f = fixture(t);
    for (const extra of [[], ['-p', '1']]) {
      const r = f.run('test-backend.sh', ['-count=1', './...', ...extra], { BUDGET_GOTESTSUM: gotestsum });
      assert.equal(r.status, 0, r.stderr);
      const args = readFileSync(f.env.BUDGET_ARGS, 'utf8').trim().split('\n');
      assert.deepEqual(args, [...(gotestsum === 'yes' ? ['tool', 'gotestsum', '--format', 'pkgname-and-test-fails', '--'] : ['test']),
        '-p', '3', '-parallel', '8', '-tags=testhooks', '-count=1', './...', ...extra]);
      assert.equal(readFileSync(f.env.BUDGET_CALLS, 'utf8').trim().split('\n').at(-1), 'sweep');
      // The real argv proves the caller override is last; Go's documented flag
      // precedence is not reimplemented or claimed to have been executed here.
    }
  });
}
