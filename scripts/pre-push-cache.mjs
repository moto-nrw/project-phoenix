import { createHash } from 'node:crypto';
import { spawnSync } from 'node:child_process';
import { accessSync, constants, existsSync, mkdirSync, readFileSync, readdirSync, realpathSync, renameSync, statSync, writeFileSync } from 'node:fs';
import { join, resolve } from 'node:path';
import { performance } from 'node:perf_hooks';

// Only these static checks may use persistent success results. Security scans
// against changing vulnerability databases deliberately have no cache entry.
const scopes = {
  'backend-quality': ['backend', 'scripts', '.github', '.claude/hooks', 'lefthook.yml', 'devbox.json', 'devbox.lock', 'package.json', 'frontend/package.json'],
  'backend-lint': ['backend', 'scripts/check-backend-quality.sh', 'scripts/backend-affected-packages.sh', 'scripts/run-go-toolchain.sh'],
  architecture: ['backend', 'scripts/backend-architecture', 'scripts/backend-architecture.sh', 'scripts/run-go-toolchain.sh'],
  'frontend-quality': ['frontend', 'scripts/check-frontend-quality.sh', 'package.json'],
  'react-doctor': ['frontend', 'scripts/check-frontend-quality.sh', 'package.json'],
  'hook-tests': ['scripts/pre-push.test.mjs', 'scripts/pre-push.sh', 'scripts/pre-push-cache.mjs', 'scripts/check-secrets.sh', 'scripts/run-go-toolchain.sh', 'scripts/check-quality.sh', 'scripts/check-backend-quality.sh', 'scripts/check-frontend-quality.sh', 'lefthook.yml', '.github/workflows'],
};
const shared = ['scripts/pre-push-cache.mjs', 'scripts/quality-timing.sh', 'scripts/check-quality.sh', 'devbox.json', 'devbox.lock'];
const tools = {
  'backend-quality': ['go', 'gofmt'], 'backend-lint': ['go', 'golangci-lint'], architecture: ['go'],
  'frontend-quality': ['node', 'pnpm', 'npx'], 'react-doctor': ['node', 'npx'], 'hook-tests': ['node'],
};
function run(command, args, cwd) {
  const result = spawnSync(command, args, { cwd, encoding: 'utf8', maxBuffer: 32 * 1024 * 1024 });
  if (result.error || result.status !== 0) throw new Error(`${command} failed: ${result.error?.message ?? result.stderr}`);
  return result.stdout;
}
function clean(root, head) {
  if (run('git', ['rev-parse', 'HEAD'], root).trim() !== head || run('git', ['status', '--porcelain', '--untracked-files=normal'], root).trim()) {
    throw new Error('Source changed during pre-push. Commit or stash changes and retry.');
  }
}
function identity(path) {
  accessSync(path, constants.X_OK);
  const actual = realpathSync(path), info = statSync(actual);
  return [actual, info.size, info.mtimeMs];
}
function keyFor(root, stage, base, command, args) {
  const hash = createHash('sha256');
  const paths = [...shared, ...scopes[stage]];
  // Git tree entries include paths, file modes and blob IDs, including deletions
  // and symlink targets. HEAD's commit ID is intentionally not part of the key.
  hash.update(run('git', ['ls-tree', '-r', '-z', 'HEAD', '--', ...paths], root));
  const bin = join(root, '.devbox/nix/profile/default/bin');
  const identities = tools[stage].map(tool => {
    try { return [tool, identity(join(bin, tool))]; }
    catch { throw new Error(`Pinned ${tool} is missing or not executable. Run 'devbox install'.`); }
  });
  hash.update(JSON.stringify({ stage, base, command, args, platform: process.platform, arch: process.arch, node: process.version, identities, bash: identity('/bin/bash'), git: run('git', ['--version'], root) }));
  let cacheable = true;
  if (tools[stage].includes('go')) {
    const config = run(join(bin, 'go'), ['env', '-json', 'GOVERSION', 'GOOS', 'GOARCH', 'CGO_ENABLED', 'GOFLAGS', 'GOEXPERIMENT', 'GOWORK', 'GOTOOLCHAIN', 'GOPATH', 'GOMODCACHE'], join(root, 'backend'));
    hash.update(config);
    const values = JSON.parse(config);
    // External workspaces, alternate modfiles and replacement trees cannot be
    // proven immutable from this checkout's tree. Run without a success cache.
    cacheable = (!values.GOWORK || values.GOWORK === 'off') && !values.GOFLAGS?.includes('-modfile') && !/^\s*replace\b/m.test(readFileSync(join(root, 'backend/go.mod'), 'utf8'));
  }
  // Local configuration is never written to disk or printed: only its digest
  // contributes to the key. CI does not read or write this local cache.
  const localFiles = stage.startsWith('frontend') || stage === 'react-doctor'
    ? ['frontend/.env', 'frontend/.env.local', 'frontend/.env.development', 'frontend/.env.development.local', 'frontend/next-env.d.ts', 'frontend/node_modules/.modules.yaml', 'frontend/node_modules/.pnpm/lock.yaml']
    : ['backend/dev.env'];
  for (const file of localFiles) {
    hash.update(file);
    hash.update(existsSync(join(root, file)) ? readFileSync(join(root, file)) : '<absent>');
  }
  function generatedTypes(path) {
    if (!existsSync(path)) return;
    for (const entry of readdirSync(path, { withFileTypes: true }).sort((a, b) => a.name.localeCompare(b.name))) {
      const file = join(path, entry.name);
      hash.update(file);
      if (entry.isDirectory()) generatedTypes(file);
      else hash.update(readFileSync(file));
    }
  }
  if (stage === 'frontend-quality') {
    generatedTypes(join(root, 'frontend/.next/types'));
    generatedTypes(join(root, 'frontend/.next/dev/types'));
  }
  // Hash environment values consumed by frontend configuration without storing
  // their contents in cache manifests. Exclude shell bookkeeping and volatile
  // test-run IDs so unrelated terminal sessions can reuse static results.
  const environment = Object.entries(process.env).filter(([name]) => /^(NEXT_|NEXTAUTH_|AUTH_|API_URL$|TENANT_DOMAIN$|METRICS_|NODE_OPTIONS$|NODE_ENV$|TS_NODE_|CC$|CXX$|CGO_)/.test(name)).sort();
  hash.update(JSON.stringify(environment));
  return { key: hash.digest('hex'), cacheable };
}

const started = performance.now();
const [stage, baseRef, command, ...args] = process.argv.slice(2);
try {
  if (!scopes[stage] || !baseRef || !command) throw new Error('Usage: pre-push-cache.mjs <static-stage> <base> <command> [args...]');
  const root = run('git', ['rev-parse', '--show-toplevel']).trim();
  const head = run('git', ['rev-parse', 'HEAD'], root).trim();
  clean(root, head);
  const base = run('git', ['rev-parse', `${baseRef}^{commit}`], root).trim();
  const { key, cacheable } = keyFor(root, stage, base, command, args);
  const directory = resolve(root, run('git', ['rev-parse', '--git-path', 'pre-push-cache'], root).trim());
  const entry = join(directory, `${stage}-${key}.json`);
  if (cacheable && existsSync(entry)) {
    const record = JSON.parse(readFileSync(entry, 'utf8'));
    if (record.key !== key || record.success !== true) throw new Error(`Invalid cache record: ${stage}`);
    clean(root, head);
    console.log(`[${stage}] cache hit (${((performance.now() - started) / 1000).toFixed(2)}s)`);
  } else {
    console.log(`[${stage}] ${cacheable ? 'cache miss' : 'uncached external inputs'}; starting`);
    const result = spawnSync(command, args, { cwd: root, stdio: 'inherit' });
    if (result.error || result.status !== 0) {
      console.error(`[${stage}] failed after ${((performance.now() - started) / 1000).toFixed(2)}s${result.error ? `: ${result.error.message}` : ''}`);
      process.exit(result.status || 1);
    }
    clean(root, head);
    const after = keyFor(root, stage, base, command, args);
    if (after.key !== key) throw new Error(`Inputs changed during ${stage}; result not cached. Retry.`);
    if (cacheable) {
      mkdirSync(directory, { recursive: true });
      const temporary = `${entry}.${process.pid}.tmp`;
      writeFileSync(temporary, JSON.stringify({ key, success: true }), { mode: 0o600 });
      renameSync(temporary, entry);
    }
    console.log(`[${stage}] passed in ${((performance.now() - started) / 1000).toFixed(2)}s`);
  }
} catch (error) {
  console.error(`[${stage ?? 'cache'}] ${error.message}`);
  process.exitCode = 1;
}
