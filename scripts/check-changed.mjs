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
// The steps run side by side when the machine has room (8 CPUs, 16 GB and a
// load average below half the CPUs), otherwise one after the other, static
// checks first because they fail faster. Each prints its output when it finishes; a
// step running past PHX_CHECK_TIMEOUT_MINUTES (default 30) is stopped with its
// process group and reported as TIMEOUT. A rerun with byte-identical inputs
// (every non-ignored file, the base, env files, installed dependencies, Node
// and Go) prints the last passing summary instead; PHX_CACHE_VERIFY_RATE
// (default 0.05) of such hits run anyway to catch a wrong cache key.
import { spawn, spawnSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import { copyFileSync, existsSync, mkdirSync, mkdtempSync, readdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { availableParallelism, loadavg, totalmem, tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { pathToFileURL } from 'node:url';

// Side by side only while the machine has room. Under load from other agents
// (load 18 to 30 on 16 CPUs), parallel steps starved the oxlint guard tests
// into their 5 s timeouts, so the current load average counts as well.
export function concurrencyFor(cpus = availableParallelism(), memoryBytes = totalmem(), load = loadavg()[0]) {
  return cpus >= 8 && memoryBytes >= 16 * 1024 ** 3 && load < cpus / 2 ? 2 : 1;
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
export async function runSteps(steps, { concurrency, timeoutMs, graceMs = 2000, cwd, env = process.env, write = text => process.stdout.write(text) }) {
  const results = [];
  let next = 0;
  // Steps run in their own process groups, so a Ctrl-C reaches only this
  // runner: pass it on and let each step clean up (test-changed sweeps its
  // test database clones on exit) before exiting.
  const active = new Set();
  let interrupted;
  // Non-interactive shells ignore SIGINT in background jobs, so escalate
  // like a timeout: SIGTERM after a grace period, SIGKILL after ten more seconds.
  const escalations = [];
  const signalActive = signal => { for (const pid of active) { try { process.kill(-pid, signal); } catch {} } };
  const forward = signal => {
    if (interrupted) return;
    interrupted = signal;
    signalActive(signal);
    escalations.push(setTimeout(() => signalActive('SIGTERM'), graceMs), setTimeout(() => signalActive('SIGKILL'), graceMs + 10_000));
  };
  process.on('SIGINT', forward);
  process.on('SIGTERM', forward);
  async function worker() {
    while (next < steps.length) {
      const step = steps[next++];
      const started = Date.now();
      const result = await new Promise(done => {
        if (interrupted) {
          done({ name: step.name, status: 'SKIPPED', code: null, ms: 0, output: `not started after ${interrupted}\n` });
          return;
        }
        const child = spawn(step.command[0], step.command.slice(1), { cwd, env, detached: true, stdio: ['ignore', 'pipe', 'pipe'] });
        active.add(child.pid);
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
          active.delete(child.pid);
          clearTimeout(timer);
          clearTimeout(killer);
          done({ name: step.name, status: timedOut ? 'TIMEOUT' : code === 0 ? 'ok' : 'FAILED', code, ms: Date.now() - started, output: Buffer.concat(chunks).toString('utf8') });
        });
      });
      write(`\n===== ${result.name}: ${result.status} (${formatSeconds(result.ms)}) =====\n${result.output}`);
      results.push(result);
    }
  }
  try {
    await Promise.all(Array.from({ length: Math.min(concurrency, steps.length) }, worker));
  } finally {
    process.off('SIGINT', forward);
    process.off('SIGTERM', forward);
    escalations.forEach(clearTimeout);
  }
  const ordered = steps.map(step => results.find(result => result.name === step.name));
  ordered.interrupted = interrupted;
  return ordered;
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

// Environment that can change a step's outcome: Go and Node flags, frontend
// configuration, test database and the repository's own switches.
const outcomeEnvironment = /^(GO[A-Z0-9_]*|CGO_[A-Z0-9_]+|PHX_(?!CACHE_VERIFY_RATE$|CHECK_TIMEOUT_MINUTES$)[A-Z_]+|NEXT_[A-Z_]+|NEXTAUTH_[A-Z_]+|AUTH_[A-Z_]+|API_URL|TENANT_DOMAIN|NODE_OPTIONS|NODE_ENV|TEST_DB_[A-Z_]+|TZ|CI)$/;

// Every non-ignored file (untracked included, via a scratch index), the base,
// local env files, generated types, installed dependencies, the toolchain
// versions and the outcome-relevant environment.
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
    // HEAD too: the secret scan covers branch history, which the tree alone hides.
    const head = git(root, ['rev-parse', 'HEAD']);
    const hash = createHash('sha256').update(JSON.stringify({ tree, head, mergeBase, node: process.version, go, platform: process.platform, arch: process.arch }));
    for (const file of ['.env', '.wt.env', 'backend/dev.env', 'frontend/.env', 'frontend/.env.local', 'frontend/.env.test', 'frontend/.env.test.local',
      'frontend/.env.development', 'frontend/.env.development.local', 'frontend/next-env.d.ts',
      'frontend/node_modules/.modules.yaml', 'frontend/node_modules/.pnpm/lock.yaml']) {
      hash.update(file).update(existsSync(join(root, file)) ? readFileSync(join(root, file)) : '<absent>');
    }
    // Ignored generated route types feed the typecheck, as in pre-push-cache.mjs.
    const generated = directory => {
      if (!existsSync(directory)) return;
      for (const entry of readdirSync(directory, { withFileTypes: true }).sort((a, b) => a.name.localeCompare(b.name))) {
        const path = join(directory, entry.name);
        hash.update(path);
        if (entry.isDirectory()) generated(path);
        else hash.update(readFileSync(path));
      }
    };
    generated(join(root, 'frontend/.next/types'));
    generated(join(root, 'frontend/.next/dev/types'));
    // Values stay in memory; only their digest enters the key.
    hash.update(JSON.stringify(Object.entries(env).filter(([name]) => outcomeEnvironment.test(name)).sort()));
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
  if (results.interrupted) {
    console.log(`\n===== interrupted (${results.interrupted}) =====\n${summary}`);
    return 130;
  }
  console.log(`\n===== summary =====\n${summary}`);
  const passed = results.every(result => result.status === 'ok');
  // PHX_PREPUSH=skip passes the static step without checking anything.
  if (passed && process.env.PHX_PREPUSH !== 'skip' && inputsKey(root, mergeBase) === key) {
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
