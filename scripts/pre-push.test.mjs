import { gitTestEnvironment, initGitFixture } from './test-helpers/git-fixture.mjs';
import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { mkdtempSync, mkdirSync, writeFileSync, readFileSync, copyFileSync, rmSync, symlinkSync, readdirSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import test from 'node:test';

const source = dirname(fileURLToPath(import.meta.url));

test('Go imports hook preserves tool failures and rejects formatting diffs', async t => {
  const hooks = readFileSync(join(source, '../lefthook.yml'), 'utf8');
  const block = hooks.match(/    go-imports:\n[\s\S]*?      run: \|\n((?:        .*\n|\n)*)/);
  assert.ok(block, 'Go imports run block must exist');
  const command = block[1].replace(/^        /gm, '').replaceAll('{staged_files}', 'backend/example.go');
  for (const [name, extra, status] of [
    ['stderr-only tool failure', { TOOL_STATUS: '2', TOOL_ERROR: 'parse error\n' }, 2],
    ['tool failure with output', { TOOL_STATUS: '7', TOOL_OUTPUT: 'partial diff\n', TOOL_ERROR: 'tool failed\n' }, 7],
    ['clean run', {}, 0],
    ['formatting diff', { TOOL_OUTPUT: 'formatting diff\n' }, 1],
  ]) {
    await t.test(name, t => {
      const f = fixture(t);
      f.write('backend/example.go', 'package example\n');
      f.write('scripts/run-go-toolchain.sh', `#!/bin/sh
printf '%s\\n' "$@" > ../tool-args
printf '%s' "\${TOOL_OUTPUT:-}"
printf '%s' "\${TOOL_ERROR:-}" >&2
exit "\${TOOL_STATUS:-0}"
`);
      const result = f.run('sh', ['-c', command], { env: { ...f.env, ...extra } });
      assert.equal(result.status, status, result.stdout + result.stderr);
      assert.deepEqual(readFileSync(join(f.root, 'tool-args'), 'utf8').trim().split('\n'), ['go', 'tool', 'goimports', '-d', 'example.go']);
      assert.equal(result.stderr, extra.TOOL_ERROR ?? '');
      if (extra.TOOL_STATUS) assert.doesNotMatch(result.stdout, /Go imports not organized/);
      else if (extra.TOOL_OUTPUT) assert.match(result.stdout, /Go imports not organized/);
      else assert.equal(result.stdout, '');
    });
  }
});

test('Lefthook forwards every push to the runner and CI shares its quality commands', () => {
  const hooks = readFileSync(join(source, '../lefthook.yml'), 'utf8');
  const push = hooks.split('\npre-push:\n')[1].split('\npost-merge:')[0];
  assert.match(push, /use_stdin: true/);
  assert.match(push, /run: bash scripts\/pre-push.sh --hook/);
  assert.doesNotMatch(push, /\bglob:|\bfiles:|\bskip:/);
  const workflow = readFileSync(join(source, '../.github/workflows/lint.yml'), 'utf8');
  assert.match(workflow, /bash scripts\/check-quality.sh backend/);
  assert.match(workflow, /bash scripts\/check-quality.sh frontend/);
  assert.match(workflow, /check-quality.sh react-doctor/);
  const main = readFileSync(join(source, '../.github/workflows/main.yml'), 'utf8');
  assert.match(main, /node --test scripts\/pre-push.test.mjs/);
});

function fixture(t) {
  const root = mkdtempSync(join(tmpdir(), 'phoenix-pre-push-'));
  t.after(() => rmSync(root, { recursive: true, force: true, maxRetries: 3, retryDelay: 50 }));
  // Cache-hit verification is sampled; tests choose it explicitly.
  const env = { ...gitTestEnvironment(), PHX_CACHE_VERIFY_RATE: '0' };
  function run(command, args, options = {}) {
    return spawnSync(command, args, { cwd: root, env, encoding: 'utf8', ...options });
  }
  function git(...args) {
    const result = run('git', args);
    assert.equal(result.status, 0, result.stderr);
    return result.stdout.trim();
  }
  function write(path, content) {
    mkdirSync(dirname(join(root, path)), { recursive: true });
    writeFileSync(join(root, path), content, { mode: 0o755 });
  }
  function commit(path, content) {
    write(path, content);
    git('add', '.');
    git('commit', '-qm', 'fixture change');
  }
  initGitFixture(git);
  // Short-lived fixtures must not race background Git maintenance at teardown.
  git('config', 'maintenance.auto', 'false');
  git('config', 'gc.auto', '0');
  git('config', 'user.name', 'Hook fixture');
  git('config', 'user.email', 'hook@example.invalid');
  write('.gitignore', '.devbox/\n');
  for (const name of ['pre-push.sh', 'check-secrets.sh', 'pre-push-cache.mjs', 'quality-timing.sh']) {
    write(`scripts/${name}`, readFileSync(join(source, name), 'utf8'));
  }
  write('scripts/run-go-toolchain.sh', `#!/bin/bash
printf '%s\\n' "$*"
if [[ "$*" == *quality.sh* && "\${FAIL_QUALITY:-}" == 1 ]]; then exit 9; fi
if [[ "$*" == *quality.sh* && "\${MUTATE_SOURCE:-}" == 1 ]]; then echo changed >> backend/example.go; fi
`);
  write('.devbox/nix/profile/default/bin/gitleaks', '#!/bin/bash\nexit "${FAIL_SECRETS:-0}"\n');
  symlinkSync(process.execPath, join(root, '.devbox/nix/profile/default/bin/node'));
  write('.devbox/nix/profile/default/bin/go', '#!/bin/bash\nif [[ "$*" == *-json* ]]; then echo \'{"GOVERSION":"go1.27.0","GOWORK":"off"}\'; else echo go1.27.0; fi\n');
  for (const tool of ['gofmt', 'golangci-lint', 'govulncheck']) write(`.devbox/nix/profile/default/bin/${tool}`, '#!/bin/bash\nexit 0\n');
  for (const file of ['check-agent-context.mjs', 'check-agent-context.test.mjs', 'pre-push.test.mjs', 'check-secrets.test.mjs']) write(`scripts/${file}`, '// fixture\n');
  write('.devbox/nix/profile/default/bin/pnpm', '#!/bin/bash\necho "pnpm $*"\n');
  write('.devbox/nix/profile/default/bin/npx', '#!/bin/bash\nexit 0\n');
  write('scripts/check-quality.sh', '#!/bin/bash\necho "quality $*"\n');
  write('backend/.keep', '');
  write('backend/go.mod', 'module example\n\ngo 1.27.0\n');
  write('frontend/.keep', '');
  commit('README.md', 'base\n');
  git('branch', 'base');
  return { root, env, run, git, write, commit,
    check: (extra = {}) => run('bash', ['scripts/pre-push.sh', 'base'], { env: { ...env, ...extra } }) };
}

test('docs-only follow-up still runs backend gate and propagates its failure', t => {
  const f = fixture(t);
  f.commit('backend/example.go', 'package example\n');
  f.commit('README.md', 'docs follow-up\n');
  const result = f.check({ FAIL_QUALITY: '1' });
  assert.equal(result.status, 9, result.stdout + result.stderr);
  assert.match(result.stdout, /backend=true frontend=false/);
  assert.match(result.stdout, /check-backend-quality.sh lint/);
  assert.doesNotMatch(result.stdout, /quality checks passed/);
});

test('docs-only branch does not buy backend or frontend compute', t => {
  const f = fixture(t);
  f.commit('README.md', 'documentation only\n');
  const result = f.check();
  assert.equal(result.status, 0, result.stderr);
  assert.match(result.stdout, /backend=false frontend=false/);
  assert.doesNotMatch(result.stdout, /run-go-toolchain|check-quality.sh|pnpm/);
});

test('hook stdin validates the whole branch even if only the docs tip is pushed', t => {
  const f = fixture(t);
  f.commit('backend/example.go', 'package example\n');
  const remote = f.git('rev-parse', 'HEAD');
  f.commit('README.md', 'docs follow-up\n');
  const result = f.run('bash', ['scripts/pre-push.sh', '--hook', 'base'], {
    env: { ...f.env, FAIL_QUALITY: '1' },
    input: `HEAD ${f.git('rev-parse', 'HEAD')} refs/heads/topic ${remote}\n`,
  });
  assert.equal(result.status, 9, result.stdout + result.stderr);
});

test('runner-only changes run regression checks without selecting either stack', async t => {
  for (const path of ['lefthook.yml', 'scripts/dev-native.sh', 'scripts/go-package-workers.sh', 'scripts/total-memory-gb.sh']) {
    await t.test(path, t => {
      const f = fixture(t);
      f.commit(path, 'changed runner configuration\n');
      const result = f.check();
      assert.equal(result.status, 0, result.stdout + result.stderr);
      assert.match(result.stdout, /backend=false frontend=false/);
      assert.match(result.stdout, /\[hook-tests\]/);
      assert.doesNotMatch(result.stdout, /\[backend-quality\]|\[frontend-quality\]/);
    });
  }
});

test('missing pinned frontend package manager fails closed', t => {
  const f = fixture(t);
  f.commit('frontend/example.ts', 'export {};\n');
  rmSync(join(f.root, '.devbox/nix/profile/default/bin/pnpm'));
  const result = f.check();
  assert.notEqual(result.status, 0);
  assert.match(result.stderr, /Pinned pnpm is missing/);
});

test('quoted and newline-containing backend paths cannot evade selection', t => {
  const f = fixture(t);
  f.commit('backend/ü\nexample.go', 'package example\n');
  const result = f.check({ FAIL_QUALITY: '1' });
  assert.equal(result.status, 9, result.stdout + result.stderr);
});

test('deleted backend files still trigger quality but not integration tests', t => {
  const f = fixture(t);
  rmSync(join(f.root, 'backend/.keep'));
  f.commit('README.md', 'deleted backend file\n');
  const result = f.check();
  assert.equal(result.status, 0, result.stderr);
  assert.match(result.stdout, /check-backend-quality.sh lint/);
  assert.doesNotMatch(result.stdout, /test-changed.sh/);
});

test('missing pinned secret scanner fails closed', t => {
  const f = fixture(t);
  f.commit('README.md', 'docs\n');
  rmSync(join(f.root, '.devbox/nix/profile/default/bin/gitleaks'));
  const result = f.check();
  assert.notEqual(result.status, 0);
  assert.match(result.stderr, /Pinned gitleaks is missing/);
});

test('secret findings fail the push', t => {
  const f = fixture(t);
  f.commit('README.md', 'branch change\n');
  const result = f.check({ FAIL_SECRETS: '7' });
  assert.equal(result.status, 7);
});

test('missing pinned Go installation fails before analysis', t => {
  const f = fixture(t);
  copyFileSync(join(source, 'run-go-toolchain.sh'), join(f.root, 'scripts/run-go-toolchain.sh'));
  rmSync(join(f.root, '.devbox/nix/profile/default/bin/go'));
  f.commit('backend/example.go', 'package example\n');
  const result = f.check();
  assert.notEqual(result.status, 0);
  assert.match(result.stderr, /Pinned Go toolchain is not installed/);
});

test('missing pinned linter cannot fall back to a global installation', t => {
  const f = fixture(t);
  copyFileSync(join(source, 'run-go-toolchain.sh'), join(f.root, 'scripts/run-go-toolchain.sh'));
  f.write('.devbox/nix/profile/default/bin/go', '#!/bin/bash\necho go1.27.0\n');
  rmSync(join(f.root, '.devbox/nix/profile/default/bin/golangci-lint'));
  f.commit('backend/example.go', 'package example\n');
  const result = f.check();
  assert.notEqual(result.status, 0);
  assert.match(result.stderr, /Pinned golangci-lint is missing/);
  assert.doesNotMatch(result.stdout, /quality checks passed/);
});

test('native-shell Go calls inherit the Devbox CGO default but permit an explicit override', t => {
  const f = fixture(t);
  copyFileSync(join(source, 'run-go-toolchain.sh'), join(f.root, 'scripts/run-go-toolchain.sh'));
  f.write('.devbox/nix/profile/default/bin/go', '#!/bin/bash\necho go1.27.0\n');
  f.write('.devbox/nix/profile/default/bin/inspect-cgo', '#!/bin/bash\necho "$CGO_ENABLED"\n');
  f.write('backend/go.mod', 'module example\n\ngo 1.27.0\n');
  const env = { ...f.env };
  delete env.CGO_ENABLED;
  for (const [config, expected] of [[env, '0'], [{ ...env, CGO_ENABLED: '1' }, '1']]) {
    const result = f.run('bash', ['scripts/run-go-toolchain.sh', 'inspect-cgo'], { env: config });
    assert.equal(result.status, 0, result.stderr);
    assert.equal(result.stdout.trim(), expected);
  }
});

function goBudgetFixture(t) {
  const f = fixture(t);
  copyFileSync(join(source, 'run-go-toolchain.sh'), join(f.root, 'scripts/run-go-toolchain.sh'));
  const inspect = 'printf "GOFLAGS=%s\\nGOMAXPROCS=%s\\n" "${GOFLAGS-}" "${GOMAXPROCS-}"\n';
  f.write('.devbox/nix/profile/default/bin/go', `#!/bin/bash
if [[ "$*" == 'env GOVERSION' ]]; then echo go1.27.0; exit 0; fi
${inspect}`);
  f.write('.devbox/nix/profile/default/bin/inspect-budget', `#!/bin/bash\n${inspect}go inspect-budget\n`);
  f.write('scripts/delegate.sh', '#!/bin/bash\nexec go inspect-budget\n');
  delete f.env.GOFLAGS;
  delete f.env.GOMAXPROCS;
  return {
    ...f,
    inspect: (command, overrides = {}) => f.run('bash', ['scripts/run-go-toolchain.sh', command], {
      env: { ...f.env, ...overrides },
    }),
  };
}

test('the pinned runner bounds Go processes outside a Devbox shell', t => {
  const f = goBudgetFixture(t);
  const result = f.inspect('go');
  assert.equal(result.status, 0, result.stderr);
  assert.equal(result.stdout, 'GOFLAGS=-p=4\nGOMAXPROCS=4\n');
});

test('pinned tools and delegated scripts inherit the same budget in nested Go calls', t => {
  const f = goBudgetFixture(t);
  for (const [command, copies] of [['inspect-budget', 2], ['scripts/delegate.sh', 1]]) {
    const result = f.inspect(command);
    assert.equal(result.status, 0, result.stderr);
    assert.equal(result.stdout, 'GOFLAGS=-p=4\nGOMAXPROCS=4\n'.repeat(copies));
  }
});

test('caller resource settings remain explicit, including an empty GOFLAGS', t => {
  const f = goBudgetFixture(t);
  for (const GOFLAGS of ['-p=1 -tags=fixture', '']) {
    const result = f.inspect('inspect-budget', { GOFLAGS, GOMAXPROCS: '1' });
    assert.equal(result.status, 0, result.stderr);
    assert.equal(result.stdout, `GOFLAGS=${GOFLAGS}\nGOMAXPROCS=1\n`.repeat(2));
  }
});

test('pre-push exports Go budgets before computing cache inputs and preserves overrides', t => {
  const f = fixture(t);
  delete f.env.GOFLAGS;
  delete f.env.GOMAXPROCS;
  f.write('.devbox/nix/profile/default/bin/go', `#!/bin/bash
if [[ "$*" == *-json* ]]; then
  printf '%s|%s\\n' "\${GOFLAGS-}" "\${GOMAXPROCS-}" >> .devbox-budgets
  echo '{"GOVERSION":"go1.27.0","GOWORK":"off"}'
else
  echo go1.27.0
fi
`);
  f.write('.gitignore', '.devbox/\nbackend/.devbox-budgets\n');
  f.commit('backend/example.go', 'package example\n');
  for (const [overrides, expected] of [[{}, '-p=4|4'], [{ GOFLAGS: '-p=1', GOMAXPROCS: '1' }, '-p=1|1']]) {
    const budgetFile = join(f.root, 'backend/.devbox-budgets');
    rmSync(budgetFile, { force: true });
    const result = f.check(overrides);
    assert.equal(result.status, 0, result.stdout + result.stderr);
    const budgets = readFileSync(budgetFile, 'utf8').trim().split('\n');
    assert.ok(budgets.length > 0);
    assert.ok(budgets.every(budget => budget === expected), JSON.stringify(budgets));
  }
});

test('dirty working tree cannot mask the committed failure', t => {
  const f = fixture(t);
  f.commit('backend/example.go', 'package example\n');
  f.write('backend/example.go', 'package fixed\n');
  const result = f.check();
  assert.notEqual(result.status, 0);
  assert.match(result.stderr, /requires a clean working tree/);
  assert.doesNotMatch(result.stdout, /check-quality.sh/);
});

test('invalid base fails rather than selecting an empty check set', t => {
  const f = fixture(t);
  const result = f.run('bash', ['scripts/pre-push.sh', 'missing-base']);
  assert.notEqual(result.status, 0);
  assert.doesNotMatch(result.stdout, /quality checks passed/);
});

test('push checks use a private cache workspace when system temporary storage is unavailable', t => {
  const f = fixture(t);
  f.commit('README.md', 'documentation change\n');
  f.write('.devbox/nix/profile/default/bin/mktemp', `#!/bin/bash
if [[ $# -eq 0 ]]; then echo 'system temporary storage quota exceeded' >&2; exit 73; fi
exec /usr/bin/mktemp "$@"
`);
  f.write('.devbox/nix/profile/default/bin/gitleaks', `#!/bin/bash
[[ -d "$TMPDIR" ]] || exit 74
printf '%s' "$TMPDIR" > "$(git rev-parse --git-path observed-temp)"
exit "\${FAIL_SECRETS:-0}"
`);
  const cache = f.git('rev-parse', '--path-format=absolute', '--git-path', 'pre-push-cache');
  const path = `${join(f.root, '.devbox/nix/profile/default/bin')}:${f.env.PATH}`;
  for (const status of [0, 9]) {
    const result = f.check({ TMPDIR: '', PATH: path, FAIL_SECRETS: String(status) });
    assert.equal(result.status, status, result.stdout + result.stderr);
    const observed = readFileSync(join(f.root, '.git/observed-temp'), 'utf8');
    assert.ok(observed.startsWith(`${cache}/work.`), observed);
    assert.deepEqual(readdirSync(cache).filter(name => name.startsWith('work.')), []);
  }
  const callerTemp = join(f.root, '.devbox/caller-temp');
  mkdirSync(callerTemp);
  const result = f.check({ TMPDIR: callerTemp, PATH: path });
  assert.equal(result.status, 0, result.stdout + result.stderr);
  assert.equal(readFileSync(join(f.root, '.git/observed-temp'), 'utf8'), callerTemp);
  assert.deepEqual(readdirSync(callerTemp), []);
  assert.deepEqual(readdirSync(cache).filter(name => name.startsWith('work.')), []);
});

test('hook rejects pushing a revision other than the checkout', t => {
  const f = fixture(t);
  const old = f.git('rev-parse', 'HEAD');
  f.commit('README.md', 'new head\n');
  const result = f.run('bash', ['scripts/pre-push.sh', '--hook', 'base'], {
    input: `refs/heads/other ${old} refs/heads/other ${old}\n`,
  });
  assert.notEqual(result.status, 0);
  assert.match(result.stderr, /checked-out branch/);
});

test('branch deletion needs no code checks', t => {
  const f = fixture(t);
  const result = f.run('bash', ['scripts/pre-push.sh', '--hook', 'base'], {
    input: `(delete) ${'0'.repeat(40)} refs/heads/old ${f.git('rev-parse', 'HEAD')}\n`,
  });
  assert.equal(result.status, 0, result.stderr);
  assert.equal(result.stdout, '');
});

test('shared backend quality fails when an analyzer executable is missing', t => {
  const f = fixture(t);
  copyFileSync(join(source, 'check-quality.sh'), join(f.root, 'scripts/check-quality.sh'));
  copyFileSync(join(source, 'check-backend-quality.sh'), join(f.root, 'scripts/check-backend-quality.sh'));
  f.write('limited/dirname', '#!/bin/bash\nprintf "%s\\n" "${1%/*}"\n');
  f.write('limited/gofmt', '#!/bin/bash\nexit 0\n');
  symlinkSync('/bin/bash', join(f.root, 'limited/bash'));
  const result = f.run('/bin/bash', ['scripts/check-quality.sh', 'backend'], {
    // Keep the shell diagnostic below independent of the host locale.
    env: { ...f.env, PATH: join(f.root, 'limited'), LC_ALL: 'C' },
  });
  assert.equal(result.status, 127);
  assert.match(result.stderr, /go: command not found/);
});

test('successful backend checks survive a docs-only follow-up and CI-only analysis stays out of pre-push', t => {
  const f = fixture(t);
  f.commit('backend/example.go', 'package example\n');
  assert.equal(f.check().status, 0);
  f.commit('README.md', 'docs-only follow-up\n');
  const result = f.check();
  assert.equal(result.status, 0, result.stdout + result.stderr);
  assert.match(result.stdout, /\[backend-lint\] cache hit/);
  assert.doesNotMatch(result.stdout, /check-quality.sh backend|backend-architecture|vulnerabilities|react-doctor/);
});

test('failures never create successful cache records', t => {
  const f = fixture(t);
  f.commit('backend/example.go', 'package example\n');
  for (let attempt = 0; attempt < 2; attempt++) {
    const result = f.check({ FAIL_QUALITY: '1' });
    assert.equal(result.status, 9);
    assert.match(result.stdout, /\[backend-lint\] cache miss/);
  }
  assert.equal(f.check().status, 0);
});

test('source edits invalidate a successful static result', t => {
  const f = fixture(t);
  f.commit('backend/example.go', 'package example\n');
  assert.equal(f.check().status, 0);
  f.commit('backend/example.go', 'package changed\n');
  const result = f.check({ FAIL_QUALITY: '1' });
  assert.equal(result.status, 9);
  assert.match(result.stdout, /\[backend-lint\] cache miss/);
});

test('changing a checker invalidates its success', t => {
  const f = fixture(t);
  f.commit('backend/example.go', 'package example\n');
  assert.equal(f.check().status, 0);
  f.commit('scripts/check-quality.sh', '#!/bin/bash\n# changed checker\nexit 0\n');
  const result = f.check({ FAIL_QUALITY: '1' });
  assert.equal(result.status, 9);
});

test('advancing the base keeps cached results while the merge-base is unchanged', t => {
  const f = fixture(t);
  f.commit('backend/example.go', 'package example\n');
  assert.equal(f.check().status, 0);
  const next = f.git('commit-tree', f.git('rev-parse', 'base^{tree}'), '-p', f.git('rev-parse', 'base'), '-m', 'advance base');
  f.git('branch', '-f', 'base', next);
  const result = f.check({ FAIL_QUALITY: '1' });
  assert.equal(result.status, 0, result.stdout + result.stderr);
  assert.match(result.stdout, /\[backend-lint\] cache hit/);
});

test('changed installed tools invalidate results; missing tools also fail on a warm cache', t => {
  const f = fixture(t);
  f.commit('backend/example.go', 'package example\n');
  assert.equal(f.check().status, 0);
  f.write('.devbox/nix/profile/default/bin/golangci-lint', '#!/bin/bash\n# updated tool\nexit 0\n');
  assert.equal(f.check({ FAIL_QUALITY: '1' }).status, 9);
  rmSync(join(f.root, '.devbox/nix/profile/default/bin/golangci-lint'));
  assert.match(f.check().stderr, /Pinned golangci-lint is missing/);
});

test('a sampled cache hit runs the check again and forgets a pass that no longer holds', t => {
  const f = fixture(t);
  f.commit('backend/example.go', 'package example\n');
  assert.equal(f.check().status, 0);
  assert.match(f.check().stdout, /\[backend-lint\] cache hit \(/);
  const verified = f.check({ PHX_CACHE_VERIFY_RATE: '1' });
  assert.equal(verified.status, 0, verified.stderr);
  assert.match(verified.stdout, /\[backend-lint\] cache hit; running anyway to verify it/);
  assert.match(verified.stdout, /\[backend-lint\] cache hit verified/);
  const wrong = f.check({ PHX_CACHE_VERIFY_RATE: '1', FAIL_QUALITY: '1' });
  assert.equal(wrong.status, 9);
  assert.match(wrong.stderr, /\[backend-lint\] the cached pass for these inputs was wrong; entry removed/);
  assert.match(f.check().stdout, /\[backend-lint\] cache miss/);
});

test('worktree mode checks uncommitted and untracked changes without caching', t => {
  const f = fixture(t);
  f.write('backend/untracked.go', 'package example\n');
  const result = f.run('bash', ['scripts/pre-push.sh', '--worktree', 'base'], { env: f.env });
  assert.equal(result.status, 0, result.stderr);
  assert.match(result.stdout, /backend=true frontend=false/);
  assert.match(result.stdout, /\[backend-lint\] starting/);
  assert.doesNotMatch(result.stdout, /cache (hit|miss)/);
  assert.doesNotMatch(result.stderr, /clean working tree/);
});

test('a warm cache cannot hide dirty source or source changes during validation', t => {
  const f = fixture(t);
  f.commit('backend/example.go', 'package example\n');
  const mutated = f.check({ MUTATE_SOURCE: '1' });
  assert.notEqual(mutated.status, 0);
  assert.match(mutated.stderr, /Source changed during pre-push/);
  f.git('restore', 'backend/example.go');
  assert.match(f.check().stdout, /\[backend-lint\] cache miss/);
  f.write('backend/example.go', 'package dirty\n');
  assert.match(f.check().stderr, /requires a clean working tree/);
});

test('frontend-only quality changes do not run backend analysis', t => {
  const f = fixture(t);
  f.commit('scripts/check-frontend-quality.sh', '#!/bin/bash\nexit 0\n');
  const result = f.check();
  assert.equal(result.status, 0, result.stderr);
  assert.match(result.stdout, /backend=false frontend=true/);
  assert.doesNotMatch(result.stdout, /\[backend-lint\]/);
});

test('frontend cached results survive docs but invalidate on source and generated-type changes', t => {
  const f = fixture(t);
  f.commit('.gitignore', '.devbox/\nfrontend/.next/\n');
  f.commit('frontend/example.ts', 'export {};\n');
  assert.equal(f.check().status, 0);
  f.commit('README.md', 'docs\n');
  assert.match(f.check().stdout, /\[frontend-quality\] cache hit/);
  f.write('frontend/.next/types/routes.d.ts', 'export {};\n');
  assert.match(f.check().stdout, /\[frontend-quality\] cache miss/);
  f.commit('frontend/example.ts', 'export const changed = true;\n');
  assert.match(f.check().stdout, /\[frontend-quality\] cache miss/);
});

test('PHX_PREPUSH=skip bypasses every check', t => {
  const f = fixture(t);
  f.commit('backend/example.go', 'package example\n');
  const result = f.check({ PHX_PREPUSH: 'skip', FAIL_QUALITY: '1' });
  assert.equal(result.status, 0, result.stderr);
  assert.match(result.stdout, /Pre-push skipped/);
});

test('conflict-free base merge onto the pushed branch skips checks, own changes do not', t => {
  const f = fixture(t);
  f.git('switch', '-qc', 'topic');
  f.commit('backend/example.go', 'package example\n');
  const pushed = f.git('rev-parse', 'HEAD');
  f.git('switch', '-q', 'base');
  f.commit('backend/other.go', 'package other\n');
  f.git('switch', '-q', 'topic');
  f.git('merge', '-q', '--no-edit', 'base');
  const push = () => f.run('bash', ['scripts/pre-push.sh', '--hook', 'base'], {
    env: { ...f.env, FAIL_QUALITY: '1' },
    input: `HEAD ${f.git('rev-parse', 'HEAD')} refs/heads/topic ${pushed}\n`,
  });
  let result = push();
  assert.equal(result.status, 0, result.stdout + result.stderr);
  assert.match(result.stdout, /conflict-free merge/);
  f.git('reset', '-q', '--hard', 'HEAD^1');
  f.git('merge', '-q', '--no-edit', '--no-commit', 'base');
  f.write('backend/example.go', 'package edited\n');
  f.git('add', '.');
  f.git('commit', '-qm', 'merge with edit');
  result = push();
  assert.equal(result.status, 9, result.stdout + result.stderr);
});
