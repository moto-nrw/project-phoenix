#!/usr/bin/env node
// Fixed workloads and execution inventories over the existing resource meter.
// See docs/research/test-performance-hill-climb.md for the acceptance protocol.
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { spawn, spawnSync, execFileSync } from 'node:child_process';
import { createReadStream, existsSync, mkdirSync, mkdtempSync, readFileSync, readdirSync, writeFileSync } from 'node:fs';
import { homedir, loadavg, totalmem, cpus, platform, arch, release } from 'node:os';
import { basename, dirname, isAbsolute, join, relative, resolve } from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const workloadFile = join(here, 'test-perf-workloads.json');
const workloads = JSON.parse(readFileSync(workloadFile, 'utf8'));
const sha = value => createHash('sha256').update(value).digest('hex');
const hashFile = file => existsSync(file) ? sha(readFileSync(file)) : null;
const json = file => JSON.parse(readFileSync(file, 'utf8'));
const save = (file, value) => writeFileSync(file, JSON.stringify(value, null, 2) + '\n');

export function inventory(report, frontend) {
  assert.equal(report.success, true, 'Vitest report is not successful');
  assert.equal(report.numFailedTests, 0, 'Failed tests');
  assert.equal(report.numFailedTestSuites, 0, 'Failed suites');
  assert.ok(report.numPassedTests > 0, 'No tests passed');
  assert.ok(report.testResults.length > 0, 'No files ran');
  const entries = report.testResults.flatMap(file => {
    assert.equal(file.status, 'passed', `Failed file: ${file.name}`);
    const name = relative(frontend, file.name).split('\\').join('/');
    assert.ok(name && name !== '..' && !name.startsWith('../') && !isAbsolute(name), 'Test outside frontend');
    return file.assertionResults.map(test => {
      assert.ok(['passed', 'skipped', 'pending', 'todo', 'disabled'].includes(test.status), 'Unfinished/failed test');
      return [name, test.fullName, test.status];
    });
  }).sort((a, b) => JSON.stringify(a).localeCompare(JSON.stringify(b)));
  assert.equal(entries.length, report.numTotalTests, 'Incomplete test inventory');
  assert.equal(entries.filter(entry => entry[2] === 'passed').length, report.numPassedTests, 'Passed count mismatch');
  // Include empty files and multiplicity: a repeated/removed project must not
  // silently look equivalent just because filenames or test names coincide.
  const files = report.testResults.map(file => relative(frontend, file.name).split('\\').join('/')).sort();
  return { files, entries };
}

// Both adapters share validation and retain duplicate internal/external names.
function goInventoryCollector() {
  const packages = new Map();
  const tests = [];
  const runs = new Map();
  let events = 0;
  const collect = line => {
    if (!line.trim()) return;
    const event = JSON.parse(line);
    events++;
    assert.ok(typeof event.Package === 'string' && event.Package.length, 'Missing package');
    assert.ok(typeof event.Action === 'string', 'Missing action');
    assert.notEqual(event.Action, 'fail', 'Failed package or test');
    assert.ok(!String(event.Output ?? '').includes('(cached)'), 'Cached result is not fresh execution');
    const key = JSON.stringify([event.Package, event.Test]);
    if (event.Test) {
      if (event.Action === 'run') runs.set(key, (runs.get(key) ?? 0) + 1);
      if (event.Action === 'pass' || event.Action === 'skip') {
        assert.ok((runs.get(key) ?? 0) > 0, 'Terminal test without run');
        runs.set(key, runs.get(key) - 1);
        tests.push([event.Package, event.Test, event.Action]);
      }
    } else {
      if (event.Action === 'start') {
        assert.ok(!packages.has(event.Package), 'Duplicate package start');
        packages.set(event.Package, null);
      }
      if (event.Action === 'pass' || event.Action === 'skip') {
        assert.ok(packages.has(event.Package), 'Terminal package without start');
        assert.equal(packages.get(event.Package), null, 'Duplicate terminal package');
        packages.set(event.Package, event.Action);
      }
    }
  };
  const finish = () => {
    assert.ok(events > 0 && packages.size > 0, 'No packages executed');
    assert.ok([...packages.values()].every(Boolean), 'Unfinished package');
    assert.ok(tests.length > 0 && tests.some(test => test[2] === 'pass'), 'No tests passed');
    assert.ok([...runs.values()].every(count => count === 0), 'Unfinished tests');
    const entries = tests.sort((a,b) => JSON.stringify(a).localeCompare(JSON.stringify(b)));
    return { packages: [...packages].sort(([a],[b]) => a.localeCompare(b)), entries };
  };
  return { collect, finish };
}

export function goInventory(text) {
  const collector = goInventoryCollector();
  let start = 0, end;
  while ((end = text.indexOf('\n', start)) !== -1) {
    collector.collect(text.slice(start, end));
    start = end + 1;
  }
  collector.collect(text.slice(start));
  return collector.finish();
}

export async function goInventoryFile(file, { highWaterMark = 64 * 1024 } = {}) {
  const collector = goInventoryCollector();
  // The stream decoder preserves UTF-8 characters across byte chunks. Retain
  // only an unfinished event, never the full report or its output events.
  const input = createReadStream(file, { encoding: 'utf8', highWaterMark });
  let pending = '';
  try {
    for await (const chunk of input) {
      let start = 0, end;
      while ((end = chunk.indexOf('\n', start)) !== -1) {
        collector.collect(pending + chunk.slice(start, end));
        pending = '';
        start = end + 1;
      }
      pending += chunk.slice(start);
    }
    collector.collect(pending);
    return collector.finish();
  } finally {
    input.destroy();
  }
}

function sourceState(root) {
  const git = (...args) => execFileSync('git', ['-C', root, ...args], { maxBuffer: 32 * 1024 * 1024 });
  const untracked = git('ls-files', '--others', '--exclude-standard', '-z').toString().split('\0').filter(Boolean).sort();
  return {
    revision: git('rev-parse', 'HEAD').toString().trim(),
    diff: sha(git('diff', '--binary', 'HEAD')),
    untracked: untracked.map(file => [file, hashFile(join(root, file))]),
  };
}

function context(root, cacheState) {
  const local = ['.env', '.wt.env', 'backend/dev.env', 'frontend/.env', 'frontend/.env.local',
    'frontend/.env.test', 'frontend/.env.test.local', 'frontend/.env.development', 'frontend/.env.development.local'];
  return {
    platform: platform(), arch: arch(), os: release(), cpus: cpus().map(cpu => cpu.model), ram: totalmem(),
    node: process.version,
    vitest: json(join(root, 'frontend/node_modules/vitest/package.json')).version,
    pnpm: execFileSync('pnpm', ['--version'], { cwd: root, encoding: 'utf8' }).trim(),
    lock: hashFile(join(root, 'frontend/pnpm-lock.yaml')),
    installedLock: hashFile(join(root, 'frontend/node_modules/.pnpm/lock.yaml')),
    nodeOptions: sha(process.env.NODE_OPTIONS ?? ''),
    ci: process.env.CI ?? '', lang: process.env.LANG ?? '', tz: process.env.TZ ?? '',
    environment: sha(JSON.stringify(Object.entries(process.env).filter(([key]) =>
      /^(?:VITEST_|PHX_|NEXT_PUBLIC_|API_URL$|LC_)/.test(key)).sort(([a], [b]) => a.localeCompare(b)))),
    resourceGate: hashFile(process.env.AGENT_HEAVY || join(homedir(), 'bin/agent-heavy')),
    localConfiguration: local.map(file => [file, hashFile(join(root, file))]),
    cacheState,
  };
}

// Only this versioned, full-suite experiment classifies the worker policy as
// treatment. All other profiles keep it in their strict shared wrapper context.
const WORKER3_PROFILE = 'backend-full-worker3';
const WORKER3_FILE = 'scripts/go-package-workers.sh';
const WORKER3_HASHES = Object.freeze({
  B: '05abbd3c24bf528f05b0f0bdab3980ee59d65150aae54a2066270104ef652dcf',
  W: '81d4b08115e9df87b12cc936fa75db77fb5799deb4ec926d6b8970b9aa6df5b6',
});
const WORKER3_WORKLOAD = Object.freeze({ runner: 'go', args: ['-count=1', './...'], treatment: 'darwin-worker3-v1' });

function worker3Workload(profile, workload) {
  if (profile !== WORKER3_PROFILE) {
    assert.ok(!workload.treatment, 'Treatment is restricted to backend-full-worker3');
    return false;
  }
  assert.deepEqual(workload, WORKER3_WORKLOAD, 'Worker3 requires the exact versioned full workload; no overrides');
  return true;
}

function worker3Observation(root, evidence) {
  const hash = hashFile(join(root, WORKER3_FILE));
  const side = Object.keys(WORKER3_HASHES).find(side => WORKER3_HASHES[side] === hash);
  evidence.sha256 = hash;
  assert.ok(side, 'Unknown worker3 policy source');
  // Use the same pinned PATH/environment as test-backend.sh, not the caller's
  // possibly different getconf/sysctl. This is a read-only default resolution.
  const output = execFileSync(join(root, 'scripts/run-go-toolchain.sh'), [join(root, WORKER3_FILE)], {
    cwd: root, encoding: 'utf8',
  }).trim();
  evidence.defaultWorkers = /^[1-9][0-9]*$/.test(output) ? Number(output) : null;
  assert.equal(hashFile(join(root, WORKER3_FILE)), hash, 'Worker3 policy changed while resolving');
  assert.equal(output, side === 'B' ? '2' : '3', 'Worker3 requires a host resolving B=2 and W=3');
  return { version: 'darwin-worker3-v1', file: WORKER3_FILE, side, sha256: hash, defaultWorkers: Number(output) };
}

function validateWorker3(sample) {
  assert.deepEqual(sample.treatmentBefore, sample.treatmentAfter, 'Treatment changed during trial');
  const observation = sample.treatmentBefore;
  assert.ok(observation && Object.hasOwn(WORKER3_HASHES, observation.side), 'Missing worker3 treatment');
  assert.deepEqual(observation, {
    version: 'darwin-worker3-v1', file: WORKER3_FILE, side: observation.side,
    sha256: WORKER3_HASHES[observation.side], defaultWorkers: observation.side === 'B' ? 2 : 3,
  }, 'Invalid worker3 treatment');
  assert.equal(sample.workload, sha(JSON.stringify(WORKER3_WORKLOAD)), 'Non-equivalent worker3 workload');
  assert.ok(!sample.context.wrappers.some(([file]) => file === WORKER3_FILE), 'Worker3 policy misclassified');
}

function backendContext(root, cacheState, worker3 = false) {
  const files = ['backend/go.mod', 'backend/go.sum', 'backend/tools/go.mod', 'backend/tools/go.sum',
    'go.work', 'go.work.sum', 'backend/go.work', 'backend/go.work.sum', 'devbox.json', 'devbox.lock'];
  const wrappers = ['scripts/run-go-toolchain.sh', 'scripts/test-backend.sh', 'scripts/go-test-workspace.sh',
    'scripts/test-run-id.sh', 'scripts/go-package-workers.sh', 'scripts/total-memory-gb.sh'];
  return {
    platform: platform(), arch: arch(), os: release(), cpus: cpus().map(cpu => cpu.model), ram: totalmem(),
    node: process.version,
    go: execFileSync(join(root, 'scripts/run-go-toolchain.sh'), ['go', 'version'], {
      cwd: join(root, 'backend'), encoding: 'utf8',
    }).trim(),
    locks: files.map(file => [file, hashFile(join(root, file))]),
    wrappers: wrappers.filter(file => !worker3 || file !== WORKER3_FILE).map(file => [file, hashFile(join(root, file))]),
    // Both ratchets and the architecture wrapper consume the shared base,
    // not the moving target tip. Preserve that semantic input for comparison.
    gitReferenceInputs: backendGitReferenceInputs(root).basis,
    nodeOptions: sha(process.env.NODE_OPTIONS ?? ''),
    ci: process.env.CI ?? '', lang: process.env.LANG ?? '', tz: process.env.TZ ?? '',
    // Values (including TEST_DB_DSN) never enter the command or stored context.
    environment: sha(JSON.stringify(Object.entries(process.env).filter(([key]) =>
      /^(?:GO|CGO_|PHX_|TEST_DB(?:_|$)|LC_|CC$|CXX$|PKG_CONFIG|TMPDIR$|PATH$)/.test(key))
      .sort(([a], [b]) => a.localeCompare(b)))),
    resourceGate: hashFile(process.env.AGENT_HEAVY || join(homedir(), 'bin/agent-heavy')),
    localConfiguration: ['.env', '.wt.env', 'backend/dev.env'].map(file => [file, hashFile(join(root, file))]),
    cacheState,
  };
}

function backendGitReferenceInputs(root) {
  const read = (...args) => {
    const result = spawnSync('git', ['-C', root, ...args], { encoding: 'utf8' });
    if (result.error) throw result.error;
    assert.equal(result.signal, null, 'Git reference observation interrupted');
    // Preserve missing refs/unrelated histories explicitly. The real test
    // commands still reject missing required bases; no fallback is supplied.
    return { exit: result.status, value: result.status === 0 ? result.stdout.trim() : null };
  };
  const target = read('rev-parse', '--verify', 'origin/development^{commit}');
  const mergeBases = target.exit === 0
    ? read('merge-base', '--all', 'HEAD', target.value)
    : { exit: target.exit, value: null };
  const selectedBase = target.exit === 0
    ? read('merge-base', 'HEAD', target.value)
    : { exit: target.exit, value: null };
  if (mergeBases.exit === 0) mergeBases.value = mergeBases.value.split('\n').sort().join('\n');
  return { target, basis: { targetResolutionExit: target.exit, mergeBases, selectedBase } };
}

async function execute(command, args, cwd, env = process.env) {
  const child = spawn(command, args, { cwd, env, stdio: 'inherit' });
  const forward = signal => child.kill(signal);
  const interrupt = () => forward('SIGINT');
  const terminate = () => forward('SIGTERM');
  process.on('SIGINT', interrupt);
  process.on('SIGTERM', terminate);
  try {
    return await new Promise((resolve, reject) => {
      child.once('error', reject);
      child.once('exit', (code, signal) => resolve({ code, signal }));
    });
  } finally {
    process.off('SIGINT', interrupt);
    process.off('SIGTERM', terminate);
  }
}

async function run(profile, label, root, cacheState) {
  assert.ok(Object.hasOwn(workloads, profile), `Unknown workload: ${profile}`);
  assert.match(label ?? '', /^[a-zA-Z0-9][a-zA-Z0-9_-]*$/, 'Label: use letters, digits, _ or -');
  assert.ok(['warm', 'fresh-worktree', 'uncontrolled'].includes(cacheState), 'Unknown cache state');
  const base = join(root, 'tmp/test-perf');
  mkdirSync(base, { recursive: true });
  const out = mkdtempSync(join(base, `${label}-${profile}-`));
  const workload = workloads[profile];
  const backend = !Array.isArray(workload) && workload.runner === 'go';
  const worker3 = worker3Workload(profile, workload);
  assert.ok(Array.isArray(workload) || (backend && Array.isArray(workload.args)), 'Unsupported workload');
  const report = join(out, backend ? 'go.jsonl' : 'vitest.json');
  const args = backend ? workload.args : ['run', ...workload, '--cache=false', '--reporter=default', '--reporter=json', `--outputFile.json=${report}`];
  const trialContext = () => backend ? backendContext(root, cacheState, worker3) : context(root, cacheState);
  const state = sourceState(root);
  const metadata = {
    schema: 1, id: basename(out), profile, label, root, started: new Date().toISOString(), state,
    evaluator: sha([readFileSync(fileURLToPath(import.meta.url)), readFileSync(workloadFile), readFileSync(join(here, 'dev-bench.sh'))].join('\0')),
    workload: sha(JSON.stringify(workloads[profile])), context: trialContext(),
    loadBefore: loadavg(), command: backend
      ? [join(root, 'scripts/run-go-toolchain.sh'), join(root, 'scripts/test-backend.sh'), ...args]
      : ['pnpm', '--dir', join(root, 'frontend'), 'exec', 'vitest', ...args],
    ...(backend ? { rawReport: report, gitReferenceObservation: backendGitReferenceInputs(root) } : {}),
  };
  save(join(out, 'metadata.json'), metadata);
  console.log(`test-perf: artifacts ${out}`);
  const meter = join(out, 'measure');
  const gate = process.env.AGENT_HEAVY || join(homedir(), 'bin/agent-heavy');
  // One trial per meter invocation keeps logs distinct even for paired runs.
  const bench = ['--root', root, '--out', meter, ...(existsSync(gate) ? [profile === 'quick' || (backend && profile !== 'backend-full' && !worker3) ? '--light' : '--heavy'] : []), '--', profile, ...metadata.command];
  const summary = { ...metadata, valid: false };
  try {
    if (backend) assert.ok(process.env.TEST_DB_DSN?.trim(), 'Backend trials require inherited TEST_DB_DSN');
    if (worker3) {
      summary.treatmentBeforeRaw = {};
      summary.treatmentBefore = worker3Observation(root, summary.treatmentBeforeRaw);
      metadata.treatmentBefore = summary.treatmentBefore;
      save(join(out, 'metadata.json'), metadata);
    }
    const exit = await execute('bash', [join(here, 'dev-bench.sh'), ...bench], root,
      backend ? { ...process.env, GOTESTSUM_JSONFILE: report } : process.env);
    summary.exit = exit;
    if (worker3) {
      summary.treatmentAfterRaw = {};
      summary.treatmentAfter = worker3Observation(root, summary.treatmentAfterRaw);
      assert.deepEqual(summary.treatmentAfter, summary.treatmentBefore, 'Treatment changed during trial');
    }
    assert.equal(exit.code, 0, `Measurement failed (${exit.code ?? exit.signal})`);
    assert.deepEqual(sourceState(root), state, 'Checkout changed during trial');
    if (backend) summary.gitReferenceObservationAfter = backendGitReferenceInputs(root);
    assert.deepEqual(trialContext(), metadata.context, 'Environment changed during trial');
    const table = readFileSync(join(meter, readdirSync(meter).find(file => file.endsWith('.tsv'))), 'utf8').trim().split('\n');
    assert.equal(table.length, 2, 'Expected exactly one measurement');
    const [stage, wall, queue, cpu, rss, , , status] = table[1].split('\t');
    assert.equal(stage, profile);
    assert.equal(status, 'ok');
    summary.metrics = { wallSeconds: +wall, queueSeconds: +queue, cpuSeconds: +cpu, peakTreeMB: +rss };
    assert.ok(Object.values(summary.metrics).every(n => Number.isFinite(n) && n >= 0), 'Invalid metrics');
    assert.ok(summary.metrics.wallSeconds > 0, 'Trial below meter resolution');
    if (backend) {
      // The wrapper may fall back to plain go test. Exit 0 without a fresh
      // gotestsum report is not execution evidence and must remain invalid.
      assert.ok(existsSync(report), 'Missing gotestsum JSONL report');
      summary.inventory = await goInventoryFile(report);
      summary.counts = {
        packages: summary.inventory.packages.length, tests: summary.inventory.entries.length,
        passed: summary.inventory.entries.filter(entry => entry[2] === 'pass').length,
        skipped: summary.inventory.entries.filter(entry => entry[2] === 'skip').length,
      };
    } else {
      const result = json(report);
      summary.inventory = inventory(result, join(root, 'frontend'));
      summary.counts = { files: result.testResults.length, passed: result.numPassedTests, pending: result.numPendingTests, todo: result.numTodoTests };
    }
    summary.valid = true;
  } catch (error) {
    summary.error = error.message;
  }
  summary.loadAfter = loadavg();
  save(join(out, 'summary.json'), summary);
  console.log(`test-perf: ${summary.valid ? 'valid' : 'INVALID'} ${join(out, 'summary.json')}${summary.error ? `: ${summary.error}` : ''}`);
  return summary.valid ? 0 : 1;
}

const median = values => {
  const sorted = [...values].sort((a, b) => a - b);
  const middle = Math.floor(sorted.length / 2);
  return sorted.length % 2 ? sorted[middle] : (sorted[middle - 1] + sorted[middle]) / 2;
};

export function compare(baselines, candidates) {
  assert.ok(baselines.length > 0 && baselines.length === candidates.length, 'Supply equal, nonzero paired sample counts');
  const reference = baselines[0];
  const all = [...baselines, ...candidates];
  assert.ok(all.every(sample => typeof sample.id === 'string'), 'Missing trial identity');
  assert.equal(new Set(all.map(sample => sample.id)).size, all.length, 'Reused trial; measure independent executions');
  for (const sample of all) {
    assert.equal(sample.schema, 1, 'Unknown result schema');
    assert.equal(sample.valid, true, 'Invalid trial');
    assert.notEqual(sample.context.cacheState, 'uncontrolled', 'Declare comparable cache state before timing');
    if (sample.profile === WORKER3_PROFILE) validateWorker3(sample);
    else assert.ok(!sample.treatmentBefore && !sample.treatmentAfter, 'Treatment is restricted to backend-full-worker3');
    for (const key of ['profile', 'evaluator', 'workload', 'context', 'inventory']) {
      assert.deepEqual(sample[key], reference[key], `Non-equivalent ${key}; inspect/rebaseline, do not count missing work as a win`);
    }
    assert.ok(['wallSeconds', 'queueSeconds', 'cpuSeconds', 'peakTreeMB']
      .every(key => Number.isFinite(sample.metrics[key]) && sample.metrics[key] >= 0), 'Invalid metrics');
    assert.ok(sample.metrics.wallSeconds > 0, 'Invalid wall time');
  }
  for (const group of [baselines, candidates]) {
    for (const sample of group) {
      assert.deepEqual(sample.state, group[0].state, 'Source state changed within sample group');
      if (reference.profile === WORKER3_PROFILE) assert.deepEqual(sample.treatmentBefore, group[0].treatmentBefore, 'Treatment changed within sample group');
    }
  }
  if (reference.profile === WORKER3_PROFILE) {
    assert.equal(baselines[0].treatmentBefore.side, 'B', 'Worker3 baseline must be B');
    assert.equal(candidates[0].treatmentBefore.side, 'W', 'Worker3 candidate must be W');
  }
  const metrics = {};
  for (const key of ['wallSeconds', 'queueSeconds', 'cpuSeconds', 'peakTreeMB']) {
    const stats = samples => {
      const values = samples.map(sample => sample.metrics[key]);
      return { median: median(values), min: Math.min(...values), max: Math.max(...values) };
    };
    metrics[key] = { baseline: stats(baselines), candidate: stats(candidates) };
  }
  return {
    comparable: true, samplesPerSide: baselines.length, counts: reference.counts,
    ...(reference.profile === WORKER3_PROFILE ? { treatment: { baseline: baselines[0].treatmentBefore, candidate: candidates[0].treatmentBefore } } : {}),
    speedup: metrics.wallSeconds.baseline.median / metrics.wallSeconds.candidate.median,
    pairedSpeedups: baselines.map((sample, i) => sample.metrics.wallSeconds / candidates[i].metrics.wallSeconds),
    metrics,
    load: all.map(sample => ({ label: sample.label, before: sample.loadBefore, after: sample.loadAfter })),
    verdict: 'Comparable execution inventories, not proof of equal assertion strength. Review noise, failure-detection evidence and CI before accepting.',
  };
}

async function main(args) {
  if (args[0] === 'compare') {
    const separator = args.indexOf('--');
    assert.ok(separator > 1 && separator < args.length - 1, 'compare BASE_DIR... -- CANDIDATE_DIR...');
    const read = directory => json(join(resolve(directory), 'summary.json'));
    console.log(JSON.stringify(compare(args.slice(1, separator).map(read), args.slice(separator + 1).map(read)), null, 2));
    return 0;
  }
  if (args[0] === 'run') {
    const [profile, label, ...options] = args.slice(1);
    let root = resolve(here, '..'), cache = 'uncontrolled';
    while (options.length) {
      const option = options.shift();
      const value = options.shift();
      assert.ok(value, `Missing value for ${option}`);
      if (option === '--root') root = resolve(value);
      else if (option === '--cache-state') cache = value;
      else throw new Error(`Unknown option: ${option}`);
    }
    return run(profile, label, root, cache);
  }
  console.log('Usage: node scripts/test-perf.mjs run PROFILE LABEL [--root DIR] [--cache-state warm|fresh-worktree|uncontrolled]\n       node scripts/test-perf.mjs compare BASE_DIR... -- CANDIDATE_DIR...\nProfiles: ' + Object.keys(workloads).join(', ') + '\nBackend profiles use scripts/test-backend.sh through the pinned toolchain with inherited TEST_DB_DSN (never a CLI argument).\nbackend-full: ./...; backend-architecture: ./internal/architecture; backend-export: ./modules/organizationtenancy/compose; backend-lifecycle: ./internal/testdb; backend-smoke: ./modules/identityaccess/internal/domain. All use -count=1.');
  return args.length && !['--help', '-h'].includes(args[0]) ? 1 : 0;
}

if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
  try { process.exitCode = await main(process.argv.slice(2)); }
  catch (error) { console.error(`test-perf: ${error.message}`); process.exitCode = 1; }
}
