import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { copyFileSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join, matchesGlob } from 'node:path';
import { fileURLToPath } from 'node:url';
import test from 'node:test';

const source = dirname(fileURLToPath(import.meta.url));
const scripts = JSON.parse(readFileSync(join(source, '../frontend/package.json'), 'utf8')).scripts;

function frontendGateFixture(t) {
  const f = fixture(t);
  mkdirSync(join(f.root, 'frontend'));
  mkdirSync(join(f.root, 'scripts'));
  for (const file of ['check-quality.sh', 'check-frontend-quality.sh', 'quality-timing.sh']) {
    copyFileSync(join(source, file), join(f.root, 'scripts', file));
  }
  f.write('frontend/package.json', JSON.stringify({ scripts }));
  f.write('bin/node', `#!/bin/sh
printf '%s\\n' "$1" >>"$CALL_LOG"
if [ "$1" = "$FAILED_STAGE" ]; then exit 7; fi
echo 'expected error log from a passing tool' >&2
`);
  // A warning succeeds by default, just as oxlint does. Only a supplied
  // threshold can reject it; removing the gate must turn these tests red.
  f.write('bin/oxlint', `#!/bin/sh
printf 'oxlint\\n' >>"$CALL_LOG"
if [ "$LINT_ERRORS" = 1 ]; then exit 7; fi
limit=-1
while [ "$#" -gt 0 ]; do
  case "$1" in
    --max-warnings) shift; limit=$1 ;;
  esac
  shift
done
if [ "$LINT_WARNINGS" -gt 0 ]; then echo 'warning: controlled lint diagnostic' >&2; fi
if [ "$limit" -ge 0 ] && [ "$LINT_WARNINGS" -gt "$limit" ]; then exit 1; fi
`);
  // Dispatch package scripts instead of hard-coding the CI lint flags here.
  f.write('bin/pnpm', `#!${process.execPath}
const { spawnSync } = require('node:child_process');
const { scripts } = require(process.cwd() + '/package.json');
const [mode, name, ...args] = process.argv.slice(2);
const command = mode === 'exec' ? [name, ...args].join(' ')
  : ['lint', 'typecheck'].includes(name) ? scripts[name] + ' ' + args.join(' ')
  : 'node ' + (mode === 'run' ? name : mode);
const result = spawnSync('sh', ['-c', command], { stdio: 'inherit' });
process.exit(result.status ?? 1);
`);
  return f;
}

const frontendGates = {
  check: scripts.check,
  'check:fresh': scripts['check:fresh'],
  'CI frontend': 'bash ../scripts/check-quality.sh frontend',
};

for (const [name, command] of Object.entries(frontendGates)) {
  test(`${name} rejects warnings and errors but accepts clean tools with error logs`, t => {
    const f = frontendGateFixture(t);
    for (const [warnings, errors, expected] of [[0, 0, 0], [1, 0, 1], [0, 1, 7]]) {
      f.write('calls', '');
      const result = f.run(`cd frontend && ${command}`, {
        LINT_WARNINGS: String(warnings), LINT_ERRORS: String(errors),
      });
      assert.equal(result.status, expected, result.stdout + result.stderr);
      const calls = readFileSync(f.log, 'utf8').trim().split('\n');
      assert.ok(calls.includes('oxlint'));
      assert.equal(calls.includes('./node_modules/typescript-native/bin/tsc'), expected === 0);
      if (warnings) assert.match(result.stderr, /warning: controlled lint diagnostic/);
    }
  });

  for (const failed of ['scripts/verify-error-catalog.mjs', './node_modules/typescript-native/bin/tsc']) {
    test(`${name} propagates ${failed} failure`, t => {
      const f = frontendGateFixture(t);
      const result = f.run(`cd frontend && ${command}`, {
        LINT_WARNINGS: '0', FAILED_STAGE: failed,
      });
      assert.equal(result.status, 7, result.stdout + result.stderr);
      const calls = readFileSync(f.log, 'utf8').trim().split('\n');
      assert.equal(calls.at(-1), failed);
      if (failed === 'scripts/verify-error-catalog.mjs') assert.ok(!calls.includes('oxlint'));
    });
  }
}

test('CI selects the frontend gate for every external error catalog input', () => {
  const workflow = readFileSync(join(source, '../.github/workflows/main.yml'), 'utf8');
  const condition = workflow.match(/^      run-frontend-lint: (.*)$/m)?.[1];
  assert.ok(condition);
  const filters = [...condition.matchAll(/steps\.filter\.outputs\.([\w-]+) == 'true'/g)].map(match => match[1]);
  const selected = file => filters.some(filter => {
    const block = workflow.match(new RegExp(`^            ${filter}:\\n((?:              .*\\n)+)`, 'm'))?.[1] ?? '';
    const patterns = [...block.matchAll(/- '([^']+)'/g)].map(match => match[1]);
    return patterns.some(pattern => !pattern.startsWith('!') && matchesGlob(file, pattern))
      && !patterns.some(pattern => pattern.startsWith('!') && matchesGlob(file, pattern.slice(1)));
  });
  for (const file of [
    'error-registry/general.json',
    'scripts/error-registry.mjs',
    'backend/modules/delivery/internal/adapters/localization/locales.json',
    'frontend/scripts/verify-error-catalog.mjs',
    'frontend/scripts/message-sources.mjs',
    'frontend/src/i18n/messages/de/errorCatalog/codes/general.json',
    'frontend/src/lib/error-catalog.generated.ts',
  ]) assert.ok(selected(file), `${file} must select the frontend quality gate`);
  assert.ok(!selected('docs/unrelated.md'));
});

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
    const stages = ['scripts/verify-locales.mjs', 'scripts/generate-help-index.mjs', 'test:source-contracts', 'scripts/verify-error-catalog.mjs', 'oxlint', './node_modules/typescript-native/bin/tsc'];
    for (const failed of [...stages, '']) {
      await t.test(failed || 'passing tools with expected error logs', t => {
        const f = fixture(t);
        const tool = `#!/bin/sh
stage=$1
[ "$stage" != run ] || stage=$2
[ "$stage" != . ] || stage=oxlint
printf '%s\\n' "$stage" >>"$CALL_LOG"
if [ "$stage" = "$FAILED_STAGE" ]; then exit 7; fi
echo 'expected error log from a passing tool' >&2
`;
        f.write('bin/node', tool);
        f.write('bin/oxlint', tool);
        f.write('bin/pnpm', tool);
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
