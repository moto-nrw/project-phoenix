import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { mkdtempSync, mkdirSync, writeFileSync, readFileSync, copyFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import test from 'node:test';

const source = dirname(fileURLToPath(import.meta.url));

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
  t.after(() => rmSync(root, { recursive: true, force: true }));
  const env = { ...process.env, GIT_CONFIG_GLOBAL: '/dev/null', GIT_CONFIG_NOSYSTEM: '1' };
  // Git exports these to hooks; fixture repositories must not inherit them.
  for (const key of ['GIT_DIR', 'GIT_WORK_TREE', 'GIT_INDEX_FILE', 'GIT_COMMON_DIR', 'GIT_PREFIX']) delete env[key];
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
  git('init', '-q');
  git('config', 'user.name', 'Hook fixture');
  git('config', 'user.email', 'hook@example.invalid');
  write('.gitignore', '.devbox/\n');
  for (const name of ['pre-push.sh', 'check-secrets.sh']) {
    write(`scripts/${name}`, readFileSync(join(source, name), 'utf8'));
  }
  write('scripts/run-go-toolchain.sh', `#!/bin/bash
printf '%s\\n' "$*"
if [[ "$*" == *check-quality.sh* && "\${FAIL_QUALITY:-}" == 1 ]]; then exit 9; fi
`);
  write('.devbox/nix/profile/default/bin/git-secrets', '#!/bin/bash\nexit "${FAIL_SECRETS:-0}"\n');
  write('.devbox/nix/profile/default/bin/node', '#!/bin/bash\nexit 0\n');
  write('.devbox/nix/profile/default/bin/pnpm', '#!/bin/bash\necho "pnpm $*"\n');
  write('.devbox/nix/profile/default/bin/npx', '#!/bin/bash\nexit 0\n');
  write('scripts/check-quality.sh', '#!/bin/bash\necho "quality $*"\n');
  write('backend/.keep', '');
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
  assert.match(result.stdout, /check-quality.sh backend/);
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

test('shared quality configuration changes select both stacks', t => {
  const f = fixture(t);
  f.commit('lefthook.yml', 'changed hook configuration\n');
  const result = f.check();
  assert.equal(result.status, 0, result.stdout + result.stderr);
  assert.match(result.stdout, /backend=true frontend=true/);
  assert.match(result.stdout, /quality frontend/);
  assert.match(result.stdout, /quality react-doctor base/);
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

test('deleted backend files still trigger quality and affected tests', t => {
  const f = fixture(t);
  rmSync(join(f.root, 'backend/.keep'));
  f.commit('README.md', 'deleted backend file\n');
  const result = f.check();
  assert.equal(result.status, 0, result.stderr);
  assert.match(result.stdout, /check-quality.sh backend/);
  assert.match(result.stdout, /test-changed.sh base/);
});

test('missing pinned secret scanner fails closed', t => {
  const f = fixture(t);
  f.commit('README.md', 'docs\n');
  rmSync(join(f.root, '.devbox/nix/profile/default/bin/git-secrets'));
  const result = f.check();
  assert.notEqual(result.status, 0);
  assert.match(result.stderr, /Pinned git-secrets is missing/);
});

test('secret findings fail the push', t => {
  const f = fixture(t);
  const result = f.check({ FAIL_SECRETS: '7' });
  assert.equal(result.status, 7);
});

test('missing pinned Go installation fails before analysis', t => {
  const f = fixture(t);
  copyFileSync(join(source, 'run-go-toolchain.sh'), join(f.root, 'scripts/run-go-toolchain.sh'));
  f.commit('backend/example.go', 'package example\n');
  const result = f.check();
  assert.notEqual(result.status, 0);
  assert.match(result.stderr, /Pinned Go toolchain is not installed/);
});

test('missing pinned linter cannot fall back to a global installation', t => {
  const f = fixture(t);
  copyFileSync(join(source, 'run-go-toolchain.sh'), join(f.root, 'scripts/run-go-toolchain.sh'));
  f.write('.devbox/nix/profile/default/bin/go', '#!/bin/bash\necho go1.27.0\n');
  f.commit('backend/go.mod', 'module example\n\ngo 1.27.0\n');
  const result = f.check();
  assert.notEqual(result.status, 0);
  assert.match(result.stderr, /Pinned command is not installed: golangci-lint/);
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
  f.write('limited/dirname', '#!/bin/bash\nprintf "%s\\n" "${1%/*}"\n');
  f.write('limited/gofmt', '#!/bin/bash\nexit 0\n');
  const result = f.run('/bin/bash', ['scripts/check-quality.sh', 'backend'], {
    env: { ...f.env, PATH: join(f.root, 'limited') },
  });
  assert.equal(result.status, 127);
  assert.match(result.stderr, /go: command not found/);
});
