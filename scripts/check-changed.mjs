#!/usr/bin/env node
// One command for CI's fast checks on this branch's changes, uncommitted and
// untracked files included:
//   static  scripts/pre-push.sh --worktree: secrets, agent context, backend
//           lint, frontend lint/types/knip/guard inventory, as selected by
//           the changed paths
//   tests   scripts/test-changed.sh --fast: affected Go packages and their
//           direct importers, plus the frontend tests the shared selector picks
// CI still runs the full affected closure.
//
//   scripts/run-go-toolchain.sh scripts/check-changed.mjs [base]   (default origin/development)
//
// The steps run side by side when the machine has room (8 CPUs and 16 GB),
// otherwise one after the other. Each prints its output when it finishes; a
// step running past PHX_CHECK_TIMEOUT_MINUTES (default 30) is stopped with its
// process group and reported as TIMEOUT. A rerun with byte-identical inputs
// (every non-ignored file, the base, env files, installed dependencies, Node
// and Go) prints the last passing summary instead; PHX_CACHE_VERIFY_RATE
// (default 0.05) of such hits run anyway to catch a wrong cache key.
import { spawn, spawnSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import { copyFileSync, existsSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { availableParallelism, totalmem, tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { pathToFileURL } from 'node:url';

export function concurrencyFor(cpus = availableParallelism(), memoryBytes = totalmem()) {
  return cpus >= 8 && memoryBytes >= 16 * 1024 ** 3 ? 2 : 1;
}

export function verifyRate(env = process.env) {
  const raw = env.PHX_CACHE_VERIFY_RATE;
  if (raw === undefined || raw === '') return 0.05;
  const rate = Number(raw);
  if (!Number.isFinite(rate) || rate < 0 || rate > 1) throw new Error('PHX_CACHE_VERIFY_RATE must be a number from 0 to 1');
  return rate;
}

function formatSeconds(ms) {
  return `${(ms / 1000).toFixed(1)}s`;
}

// Runs each step in its own process group, at most `concurrency` at a time.
// Output is buffered per step and printed in one block when the step ends.
export async function runSteps(steps, { concurrency, timeoutMs, cwd, env = process.env, write = text => process.stdout.write(text) }) {
  const results = [];
  let next = 0;
  async function worker() {
    while (next < steps.length) {
      const step = steps[next++];
      const started = Date.now();
      const result = await new Promise(done => {
        const child = spawn(step.command[0], step.command.slice(1), { cwd, env, detached: true, stdio: ['ignore', 'pipe', 'pipe'] });
        const chunks = [];
        child.stdout.on('data', chunk => chunks.push(chunk));
        child.stderr.on('data', chunk => chunks.push(chunk));
        let timedOut = false;
        let killer;
        const timer = setTimeout(() => {
          timedOut = true;
          try { process.kill(-child.pid, 'SIGTERM'); } catch {}
          killer = setTimeout(() => { try { process.kill(-child.pid, 'SIGKILL'); } catch {} }, 10_000);
        }, timeoutMs);
        child.on('error', error => chunks.push(Buffer.from(`${error.message}\n`)));
        child.on('close', code => {
          clearTimeout(timer);
          clearTimeout(killer);
          done({ name: step.name, status: timedOut ? 'TIMEOUT' : code === 0 ? 'ok' : 'FAILED', code, ms: Date.now() - started, output: Buffer.concat(chunks).toString('utf8') });
        });
      });
      write(`\n===== ${result.name}: ${result.status} (${formatSeconds(result.ms)}) =====\n${result.output}`);
      results.push(result);
    }
  }
  await Promise.all(Array.from({ length: Math.min(concurrency, steps.length) }, worker));
  return steps.map(step => results.find(result => result.name === step.name));
}

export function summarize(results) {
  return results.map(result => `${result.status.padEnd(7)} ${result.name.padEnd(7)} ${formatSeconds(result.ms)}`).join('\n');
}

function git(root, args, env = process.env) {
  const result = spawnSync('git', args, { cwd: root, env, encoding: 'utf8', maxBuffer: 64 * 1024 * 1024 });
  if (result.error) throw result.error;
  if (result.status !== 0) throw new Error(`git ${args[0]}: ${result.stderr.trim()}`);
  return result.stdout.trim();
}

// Every non-ignored file (untracked included, via a scratch index), the base,
// local env files, installed dependencies and the toolchain versions.
export function inputsKey(root, mergeBase, env = process.env) {
  const scratch = mkdtempSync(join(tmpdir(), 'phoenix-check-changed-'));
  try {
    const index = join(scratch, 'index');
    const realIndex = resolve(root, git(root, ['rev-parse', '--git-path', 'index']));
    if (existsSync(realIndex)) copyFileSync(realIndex, index);
    const scratchEnv = { ...env, GIT_INDEX_FILE: index };
    git(root, ['add', '-A'], scratchEnv);
    const tree = git(root, ['write-tree'], scratchEnv);
    const go = spawnSync('go', ['version'], { cwd: root, env, encoding: 'utf8' }).stdout ?? '';
    const hash = createHash('sha256').update(JSON.stringify({ tree, mergeBase, node: process.version, go, platform: process.platform, arch: process.arch }));
    for (const file of ['.env', '.wt.env', 'backend/dev.env', 'frontend/.env', 'frontend/.env.local', 'frontend/.env.test', 'frontend/.env.test.local',
      'frontend/.env.development', 'frontend/.env.development.local', 'frontend/node_modules/.modules.yaml', 'frontend/node_modules/.pnpm/lock.yaml']) {
      hash.update(file).update(existsSync(join(root, file)) ? readFileSync(join(root, file)) : '<absent>');
    }
    return hash.digest('hex');
  } finally {
    rmSync(scratch, { recursive: true, force: true });
  }
}

async function main(argv) {
  if (argv.length > 1 || argv[0] === '-h' || argv[0] === '--help') {
    console.error('Usage: scripts/run-go-toolchain.sh scripts/check-changed.mjs [base]');
    return 2;
  }
  const base = argv[0] ?? 'origin/development';
  const root = git(process.cwd(), ['rev-parse', '--show-toplevel']);
  if (base === 'origin/development') git(root, ['fetch', '--quiet', 'origin', 'development']);
  const mergeBase = git(root, ['merge-base', 'HEAD', base]);
  const recordPath = resolve(root, git(root, ['rev-parse', '--git-path', 'check-changed.pass']));
  const key = inputsKey(root, mergeBase);
  if (existsSync(recordPath)) {
    const record = JSON.parse(readFileSync(recordPath, 'utf8'));
    if (record.key === key) {
      if (Math.random() >= verifyRate()) {
        console.log(`check-changed: same inputs as the last passing run (${record.at}); not rerun.\n${record.summary}`);
        return 0;
      }
      console.log('check-changed: same inputs as the last passing run; running anyway to verify the cache (PHX_CACHE_VERIFY_RATE).');
      rmSync(recordPath, { force: true });
    }
  }
  // Both steps install frontend dependencies; one install first leaves them
  // read-only checks, so the parallel steps never race on node_modules.
  const install = spawnSync('pnpm', ['install', '--frozen-lockfile'], { cwd: join(root, 'frontend'), env: { ...process.env, CI: 'true' }, stdio: 'inherit' });
  if (install.error || install.status !== 0) {
    console.error('check-changed: pnpm install --frozen-lockfile failed');
    return 1;
  }
  const minutes = Number(process.env.PHX_CHECK_TIMEOUT_MINUTES || 30);
  const concurrency = concurrencyFor();
  console.log(`check-changed: ${mergeBase.slice(0, 10)}..working tree, ${concurrency === 2 ? 'steps side by side' : 'one step at a time'}, timeout ${minutes} min per step`);
  const results = await runSteps([
    { name: 'static', command: ['bash', 'scripts/pre-push.sh', '--worktree', base] },
    { name: 'tests', command: ['bash', 'scripts/test-changed.sh', '--fast', base] },
  ], { concurrency, timeoutMs: minutes * 60_000, cwd: root });
  const summary = summarize(results);
  console.log(`\n===== summary =====\n${summary}`);
  const passed = results.every(result => result.status === 'ok');
  if (passed && inputsKey(root, mergeBase) === key) {
    mkdirSync(resolve(recordPath, '..'), { recursive: true });
    writeFileSync(recordPath, JSON.stringify({ key, at: new Date().toISOString(), summary }));
  }
  return passed ? 0 : 1;
}

if (import.meta.url === pathToFileURL(process.argv[1]).href) {
  main(process.argv.slice(2)).then(code => { process.exitCode = code; }, error => {
    console.error(`check-changed: ${error.message}`);
    process.exitCode = 2;
  });
}
