import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { mkdtempSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import test from 'node:test';

import { concurrencyFor, inputsKey, runSteps, summarize, verifyRate } from './check-changed.mjs';
import { gitTestEnvironment, initGitFixture } from './test-helpers/git-fixture.mjs';

test('steps run side by side only on a machine with room', () => {
  assert.equal(concurrencyFor(16, 64 * 1024 ** 3, 3), 2);
  assert.equal(concurrencyFor(4, 64 * 1024 ** 3, 0), 1);
  assert.equal(concurrencyFor(16, 8 * 1024 ** 3, 0), 1);
  assert.equal(concurrencyFor(16, 64 * 1024 ** 3, 12), 1, 'other agents already load the machine');
});

test('the cache verification rate defaults to 5 % and rejects nonsense', () => {
  assert.equal(verifyRate({}), 0.05);
  assert.equal(verifyRate({ PHX_CACHE_VERIFY_RATE: '0' }), 0);
  assert.equal(verifyRate({ PHX_CACHE_VERIFY_RATE: '1' }), 1);
  assert.throws(() => verifyRate({ PHX_CACHE_VERIFY_RATE: 'often' }), /from 0 to 1/);
});

test('steps print whole blocks, report failures and stop a step past its timeout with its children', async () => {
  const dir = mkdtempSync(join(tmpdir(), 'phoenix-check-changed-'));
  try {
    let printed = '';
    const results = await runSteps([
      { name: 'fail', command: ['sh', '-c', 'echo first; sleep 0.2; echo second; exit 3'] },
      { name: 'slow', command: ['sh', '-c', `sh -c 'sleep 5; touch ${join(dir, 'survived')}' & wait`] },
      { name: 'ok', command: ['sh', '-c', 'echo fine'] },
    ], { concurrency: 2, timeoutMs: 1000, cwd: dir, write: text => { printed += text; } });
    assert.deepEqual(results.map(result => result.status), ['FAILED', 'TIMEOUT', 'ok']);
    assert.equal(results[0].code, 3);
    assert.match(printed, /===== fail: FAILED \([\d.]+s\) =====\nfirst\nsecond\n/);
    assert.match(summarize(results), /^FAILED  fail/m);
    await new Promise(done => setTimeout(done, 5500));
    assert.equal(spawnSync('test', ['-e', join(dir, 'survived')]).status, 1, 'the timed-out step left a child running');
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
});

test('the inputs key follows untracked files and the base, not the order of runs', t => {
  const root = mkdtempSync(join(tmpdir(), 'phoenix-check-changed-key-'));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  const env = gitTestEnvironment();
  const git = (...args) => {
    const result = spawnSync('git', args, { cwd: root, env, encoding: 'utf8' });
    assert.equal(result.status, 0, result.stderr);
    return result.stdout.trim();
  };
  initGitFixture(git);
  writeFileSync(join(root, 'a.txt'), 'a\n');
  git('add', '.');
  git('commit', '-qm', 'base');
  const base = git('rev-parse', 'HEAD');
  const first = inputsKey(root, base, env);
  assert.equal(inputsKey(root, base, env), first);
  writeFileSync(join(root, 'untracked.go'), 'package x\n');
  const withUntracked = inputsKey(root, base, env);
  assert.notEqual(withUntracked, first);
  assert.notEqual(inputsKey(root, 'other-base', env), withUntracked);
  assert.equal(git('status', '--porcelain'), '?? untracked.go', 'the key leaves the real index alone');
});
