import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { copyFileSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import test from 'node:test';

function fixture(t) {
  const root = mkdtempSync(join(tmpdir(), 'frontend-changed-'));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  for (const dir of ['scripts', 'frontend', 'bin']) mkdirSync(join(root, dir));
  copyFileSync(new URL('./test-frontend-changed.mjs', import.meta.url), join(root, 'scripts/test-frontend-changed.mjs'));
  const log = join(root, 'calls');
  writeFileSync(join(root, 'bin/pnpm'), `#!/usr/bin/env node
const fs = require('node:fs');
const args = process.argv.slice(2);
fs.appendFileSync(process.env.CALL_LOG, JSON.stringify(args) + '\\n');
if (args[2] === 'list') {
  if (process.env.FAIL_LIST) process.exit(7);
  process.stdout.write(JSON.stringify([
    { file: process.cwd() + '/src/lib/loading-boundaries.test.ts' },
    { file: process.cwd() + '/src/lib/changed.test.ts' },
  ]));
} else if (process.env.FAIL_RUN) process.exit(8);
`, { mode: 0o755 });
  const run = (env = {}) => spawnSync(process.execPath, [join(root, 'scripts/test-frontend-changed.mjs'), 'origin/development', '--project', 'logic-node'], {
    encoding: 'utf8', env: { ...process.env, PATH: `${root}/bin:${process.env.PATH}`, CALL_LOG: log, ...env },
  });
  const calls = () => readFileSync(log, 'utf8').trim().split('\n').map(line => JSON.parse(line));
  return { run, calls };
}

test('incremental selection includes filesystem guards once alongside changed tests', t => {
  const f = fixture(t);
  const result = f.run();
  assert.equal(result.status, 0, result.stderr);
  const [list, run] = f.calls();
  assert.deepEqual(list, ['exec', 'vitest', 'list', '--filesOnly', '--json', '--changed', 'origin/development', '--project', 'logic-node']);
  assert.deepEqual(run.slice(0, 5), ['exec', 'vitest', 'run', '--project', 'logic-node']);
  const files = run.slice(5);
  assert.equal(files.length, 15, '14 filesystem guards plus the changed test');
  assert.equal(new Set(files).size, files.length);
  for (const file of [
    'src/app/api/route-cache-policy.test.ts',
    'src/lib/loading-boundaries.test.ts',
    'src/components/settings/settings-filter.test.ts',
    'src/components/school-setup/setup-tour-targets.test.ts',
    'src/lib/changed.test.ts',
  ]) assert.ok(files.includes(file), `${file} must run without an import edge`);
});

for (const [env, status, count] of [[{ FAIL_LIST: '1' }, 7, 1], [{ FAIL_RUN: '1' }, 8, 2]]) {
  test(`incremental runner preserves exit ${status}`, t => {
    const f = fixture(t);
    assert.equal(f.run(env).status, status);
    assert.equal(f.calls().length, count);
  });
}
