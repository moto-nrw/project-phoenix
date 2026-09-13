import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { mkdtempSync, mkdirSync, writeFileSync, readFileSync, copyFileSync, rmSync, symlinkSync } from 'node:fs';
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
  t.after(() => rmSync(root, { recursive: true, force: true, maxRetries: 3, retryDelay: 50 }));
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
if [[ "$*" == *check-quality.sh* && "\${FAIL_QUALITY:-}" == 1 ]]; then exit 9; fi
if [[ "$*" == *check-quality.sh* && "\${MUTATE_SOURCE:-}" == 1 ]]; then echo changed >> backend/example.go; fi
`);
  write('.devbox/nix/profile/default/bin/git-secrets', '#!/bin/bash\nexit "${FAIL_SECRETS:-0}"\n');
  symlinkSync(process.execPath, join(root, '.devbox/nix/profile/default/bin/node'));
  write('.devbox/nix/profile/default/bin/go', '#!/bin/bash\nif [[ "$*" == *-json* ]]; then echo \'{"GOVERSION":"go1.27.0","GOWORK":"off"}\'; else echo go1.27.0; fi\n');
  for (const tool of ['gofmt', 'golangci-lint', 'govulncheck']) write(`.devbox/nix/profile/default/bin/${tool}`, '#!/bin/bash\nexit 0\n');
  for (const file of ['check-agent-context.mjs', 'check-agent-context.test.mjs', 'pre-push.test.mjs']) write(`scripts/${file}`, '// fixture\n');
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

test('runner-only changes run regression checks without selecting either stack', t => {
  const f = fixture(t);
  f.commit('lefthook.yml', 'changed hook configuration\n');
  const result = f.check();
  assert.equal(result.status, 0, result.stdout + result.stderr);
  assert.match(result.stdout, /backend=false frontend=false/);
  assert.match(result.stdout, /\[hook-tests\]/);
  assert.doesNotMatch(result.stdout, /\[backend-quality\]|\[frontend-quality\]/);
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
  assert.match(result.stdout, /check-quality.sh backend/);
  assert.doesNotMatch(result.stdout, /test-changed.sh/);
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
  copyFileSync(join(source, 'check-backend-quality.sh'), join(f.root, 'scripts/check-backend-quality.sh'));
  f.write('limited/dirname', '#!/bin/bash\nprintf "%s\\n" "${1%/*}"\n');
  f.write('limited/gofmt', '#!/bin/bash\nexit 0\n');
  symlinkSync('/bin/bash', join(f.root, 'limited/bash'));
  const result = f.run('/bin/bash', ['scripts/check-quality.sh', 'backend'], {
    env: { ...f.env, PATH: join(f.root, 'limited') },
  });
  assert.equal(result.status, 127);
  assert.match(result.stderr, /go: command not found/);
});

test('successful backend checks survive a docs-only follow-up but vulnerabilities still run', t => {
  const f = fixture(t);
  f.commit('backend/example.go', 'package example\n');
  assert.equal(f.check().status, 0);
  f.commit('README.md', 'docs-only follow-up\n');
  const result = f.check();
  assert.equal(result.status, 0, result.stdout + result.stderr);
  for (const stage of ['backend-quality', 'backend-lint', 'architecture']) assert.match(result.stdout, new RegExp(`\\[${stage}\\] cache hit`));
  assert.match(result.stdout, /vulnerabilities/);
});

test('failures never create successful cache records', t => {
  const f = fixture(t);
  f.commit('backend/example.go', 'package example\n');
  for (let attempt = 0; attempt < 2; attempt++) {
    const result = f.check({ FAIL_QUALITY: '1' });
    assert.equal(result.status, 9);
    assert.match(result.stdout, /\[backend-quality\] cache miss/);
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
  assert.match(result.stdout, /\[backend-quality\] cache miss/);
});

test('changing a checker invalidates its success', t => {
  const f = fixture(t);
  f.commit('backend/example.go', 'package example\n');
  assert.equal(f.check().status, 0);
  f.commit('scripts/check-quality.sh', '#!/bin/bash\n# changed checker\nexit 0\n');
  const result = f.check({ FAIL_QUALITY: '1' });
  assert.equal(result.status, 9);
});

test('advancing the base invalidates cached comparisons even if the merge-base is unchanged', t => {
  const f = fixture(t);
  f.commit('backend/example.go', 'package example\n');
  assert.equal(f.check().status, 0);
  const next = f.git('commit-tree', f.git('rev-parse', 'base^{tree}'), '-p', f.git('rev-parse', 'base'), '-m', 'advance base');
  f.git('branch', '-f', 'base', next);
  const result = f.check({ FAIL_QUALITY: '1' });
  assert.equal(result.status, 9);
});

test('changed installed tools invalidate results; missing tools also fail on a warm cache', t => {
  const f = fixture(t);
  f.commit('backend/example.go', 'package example\n');
  assert.equal(f.check().status, 0);
  f.write('.devbox/nix/profile/default/bin/gofmt', '#!/bin/bash\n# updated tool\nexit 0\n');
  assert.equal(f.check({ FAIL_QUALITY: '1' }).status, 9);
  rmSync(join(f.root, '.devbox/nix/profile/default/bin/gofmt'));
  assert.match(f.check().stderr, /Pinned gofmt is missing/);
});

test('a warm cache cannot hide dirty source or source changes during validation', t => {
  const f = fixture(t);
  f.commit('backend/example.go', 'package example\n');
  const mutated = f.check({ MUTATE_SOURCE: '1' });
  assert.notEqual(mutated.status, 0);
  assert.match(mutated.stderr, /Source changed during pre-push/);
  f.git('restore', 'backend/example.go');
  assert.match(f.check().stdout, /\[backend-quality\] cache miss/);
  f.write('backend/example.go', 'package dirty\n');
  assert.match(f.check().stderr, /requires a clean working tree/);
});

test('frontend-only quality changes do not run backend analysis', t => {
  const f = fixture(t);
  f.commit('scripts/check-frontend-quality.sh', '#!/bin/bash\nexit 0\n');
  const result = f.check();
  assert.equal(result.status, 0, result.stderr);
  assert.match(result.stdout, /backend=false frontend=true/);
  assert.doesNotMatch(result.stdout, /\[backend-quality\]/);
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
