import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { copyFileSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import test from 'node:test';

const source = dirname(fileURLToPath(import.meta.url));
const scripts = JSON.parse(readFileSync(join(source, '../frontend/package.json'), 'utf8')).scripts;

function fixture(t) {
  const root = mkdtempSync(join(tmpdir(), 'check-exit-status-'));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  mkdirSync(join(root, 'bin'));
  const log = join(root, 'calls');
  const write = (file, text) => writeFileSync(join(root, file), text, { mode: 0o755 });
  const run = (command, env = {}, shell = 'bash') => spawnSync(shell, ['-c', command], {
    cwd: root, encoding: 'utf8', env: { ...process.env, PATH: `${root}/bin:${process.env.PATH}`, CALL_LOG: log, ...env },
  });
  return { root, log, write, run };
}

// Execute the actual package-script shell chains. Only the tools are stand-ins:
// each failure must stop the chain, including tools that fail without output.
for (const name of ['check', 'check:fresh']) {
  test(`${name} propagates each stage failure and stops before later tools`, async t => {
    const stages = ['scripts/verify-locales.mjs', 'scripts/verify-error-catalog.mjs', 'oxlint', './node_modules/typescript-native/bin/tsc'];
    for (const failed of [...stages, '']) {
      await t.test(failed || 'passing tools with expected error logs', t => {
        const f = fixture(t);
        const tool = `#!/bin/sh
stage=$1
[ "$stage" != . ] || stage=oxlint
printf '%s\\n' "$stage" >>"$CALL_LOG"
if [ "$stage" = "$FAILED_STAGE" ]; then exit 7; fi
echo 'expected error log from a passing tool' >&2
`;
        f.write('bin/node', tool);
        f.write('bin/oxlint', tool);
        const result = f.run(scripts[name], { FAILED_STAGE: failed }, 'sh');
        assert.equal(result.status, failed ? 7 : 0, result.stdout + result.stderr);
        const last = failed ? stages.indexOf(failed) + 1 : stages.length;
        assert.deepEqual(readFileSync(f.log, 'utf8').trim().split('\n'), stages.slice(0, last));
      });
    }
  });
}

test('the pinned Go runner preserves the child exit code', t => {
  const f = fixture(t);
  mkdirSync(join(f.root, 'scripts'));
  mkdirSync(join(f.root, 'backend'));
  mkdirSync(join(f.root, '.devbox/nix/profile/default/bin'), { recursive: true });
  copyFileSync(join(source, 'run-go-toolchain.sh'), join(f.root, 'scripts/run-go-toolchain.sh'));
  f.write('backend/go.mod', 'module probe\n\ngo 1.27.3\n');
  f.write('.devbox/nix/profile/default/bin/go', '#!/bin/sh\necho go1.27.3\n');
  f.write('scripts/probe', '#!/bin/sh\necho "child error output" >&2\nexit "$CHILD_STATUS"\n');
  for (const status of [0, 7]) {
    const result = f.run('bash scripts/run-go-toolchain.sh scripts/probe', { CHILD_STATUS: String(status) });
    assert.equal(result.status, status, result.stdout + result.stderr);
    assert.match(result.stderr, /child error output/);
  }
});

test('an outer pipe without pipefail masks a failed check', t => {
  const f = fixture(t);
  const result = f.run('set +o pipefail; (echo "type error" >&2; exit 7) 2>&1 | tail -4');
  assert.equal(result.status, 0);
  assert.match(result.stdout, /type error/);
});

test('the documented logging command preserves failures from either check', async t => {
  const operations = readFileSync(join(source, '../docs/agents/operations.md'), 'utf8');
  const command = operations.match(/```bash\n# check-status-log[^\n]*\n([\s\S]*?)\n```/)?.[1];
  assert.ok(command, 'operations must contain the tested logging command');
  for (const [check, changed] of [[7, 0], [0, 9], [0, 0]]) {
    await t.test(`check=${check}, changed=${changed}`, t => {
      const f = fixture(t);
      mkdirSync(join(f.root, 'scripts'));
      // Model Devbox's command handoff, without bootstrapping Nix in a unit test.
      f.write('bin/devbox', '#!/bin/sh\nshift 2\nexec "$@"\n');
      f.write('bin/pnpm', '#!/bin/sh\necho "check ran" >>"$CALL_LOG"\necho "check diagnostic" >&2\nexit "$CHECK_STATUS"\n');
      f.write('scripts/run-go-toolchain.sh', '#!/bin/sh\necho "changed ran" >>"$CALL_LOG"\necho "expected error log" >&2\nexit "$CHANGED_STATUS"\n');
      const result = f.run(command, { CHECK_STATUS: String(check), CHANGED_STATUS: String(changed) });
      assert.equal(result.status, check || changed, result.stdout + result.stderr);
      assert.equal(readFileSync(f.log, 'utf8'), check ? 'check ran\n' : 'check ran\nchanged ran\n');
      assert.match(readFileSync(join(f.root, 'check.log'), 'utf8'), /check diagnostic/);
      if (!check) assert.match(readFileSync(join(f.root, 'test-changed.log'), 'utf8'), /expected error log/);
    });
  }
});
