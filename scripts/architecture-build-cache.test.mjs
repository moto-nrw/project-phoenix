// "Warm architecture build cache" (backend-build-cache.yml) is the only writer
// of the build cache "Backend architecture ratchet" (main.yml) restores. Go
// reuses a compile action only in the same environment, so a drift in the
// key, the path, GOFLAGS, the toolchain or the runner image turns every pull
// request back into a cold compile without failing anything (#6962).
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const read = name => readFileSync(new URL(`../.github/workflows/${name}`, import.meta.url), 'utf8');

// Lines of one job under `jobs:`, up to the next job.
function job(source, name) {
  const lines = source.split('\n');
  const start = lines.indexOf(`  ${name}:`);
  assert.notEqual(start, -1, `job ${name} exists`);
  let end = start + 1;
  while (end < lines.length && !/^ {2}\S/.test(lines[end]) && !/^\S/.test(lines[end])) end++;
  return lines.slice(start, end);
}

// Steps of a job as text blocks; comments dropped, they may differ.
function steps(lines) {
  const result = [];
  for (const line of lines) {
    if (/^\s*#/.test(line)) continue;
    if (/^ {6}- /.test(line)) result.push([]);
    else if (!/^ {7,}\S/.test(line) && line.trim() !== '') continue;
    result.at(-1)?.push(line.replace(/\s+#.*$/, ''));
  }
  return result.map(block => block.join('\n').trimEnd());
}

const step = (all, name) => {
  const found = all.find(block => block.includes(`name: ${name}\n`) || block.endsWith(`name: ${name}`));
  assert.ok(found, `step ${name} exists`);
  return found;
};

// The value of `key:` in a block, at the given indentation.
const field = (block, key, indent) => block.match(new RegExp(`^ {${indent}}${key}: (.*)$`, 'm'))?.[1];

// Everything below `with:` or `env:` of a step, as text.
const section = (block, name) => {
  const match = block.match(new RegExp(`^ {8}${name}:\\n((?: {10,}.*\\n?)*)`, 'm'));
  return match?.[1].trimEnd();
};

// GOFLAGS as the job sees it: job env over workflow env, unset as "".
function goflags(source, lines) {
  const ownEnv = lines.join('\n').match(/^ {4}env:\n((?: {6,}.*\n?)*)/m)?.[1] ?? '';
  const workflowEnv = source.match(/^env:\n((?: {2,}.*\n?)*)/m)?.[1] ?? '';
  const value = text => text.match(/^ +GOFLAGS: (.*)$/m)?.[1];
  const raw = value(ownEnv) ?? value(workflowEnv) ?? '""';
  return raw.replace(/^(["'])(.*)\1$/, '$2');
}

const main = read('main.yml');
const warmSource = read('backend-build-cache.yml');
const ratchet = job(main, 'backend-architecture-ratchet');
const warm = job(warmSource, 'warm-architecture');
const ratchetSteps = steps(ratchet);
const warmSteps = steps(warm);

test('warm job and ratchet build in the same environment', () => {
  assert.equal(field(warm.join('\n'), 'runs-on', 4), field(ratchet.join('\n'), 'runs-on', 4), 'runner image');
  assert.equal(goflags(warmSource, warm), goflags(main, ratchet), 'GOFLAGS');
  assert.equal(goflags(main, ratchet), '', 'the check adds -trimpath and the policy tags itself');
  const toolchain = source => source.match(/^env:\n(?: {2,}.*\n)*? {2}GOTOOLCHAIN: (.*)$/m)?.[1];
  assert.equal(toolchain(warmSource), toolchain(main), 'GOTOOLCHAIN');
  const setup = section(step(ratchetSteps, 'Setup Go'), 'with');
  assert.equal(section(step(warmSteps, 'Setup Go'), 'with'), setup, 'Go version and setup-go cache');
  // setup-go's own cache would restore a second build cache beside this one;
  // the modules come from the architecture cache (#6962).
  assert.equal(field(setup, 'cache', 10), 'false', 'no setup-go cache');
});

// The cached paths of a `with:` section: a single `path:` or a block of them.
function paths(withSection) {
  const block = withSection.match(/^ {10}path: \|\n((?: {12}.*\n?)*)/m)?.[1];
  if (block !== undefined) return block.split('\n').map(line => line.trim()).filter(Boolean);
  const single = field(withSection, 'path', 10);
  return single === undefined ? [] : [single];
}

test('warm job saves the key the ratchet restores', () => {
  const keyStep = 'Compute architecture build cache key';
  assert.equal(section(step(warmSteps, keyStep), 'env'), section(step(ratchetSteps, keyStep), 'env'), 'key inputs');
  assert.equal(
    step(warmSteps, keyStep).split('run: |')[1],
    step(ratchetSteps, keyStep).split('run: |')[1],
    'key composition',
  );
  const restore = 'Restore architecture build cache';
  assert.equal(field(step(warmSteps, restore), 'uses', 8), field(step(ratchetSteps, restore), 'uses', 8));
  assert.equal(section(step(warmSteps, restore), 'with'), section(step(ratchetSteps, restore), 'with'), 'restore path and keys');
  const save = step(warmSteps, 'Save architecture build cache');
  assert.match(field(save, 'uses', 8), /^actions\/cache\/save@/);
  const restored = section(step(ratchetSteps, restore), 'with');
  assert.deepEqual(paths(section(save, 'with')), paths(restored), 'saved paths');
  assert.deepEqual(paths(restored), ['~/.cache/go-build', '~/go/pkg/mod'], 'build and module cache');
  assert.equal(field(save, 'key', 10), field(restored, 'key', 10), 'saved key');
  // The module cache carries no old versions forward only while go.sum of
  // both modules is part of the prefix it is restored by.
  assert.match(section(step(ratchetSteps, keyStep), 'env'), /ARCH_CACHE_PREFIX: .*hashFiles\('backend\/go\.sum', 'scripts\/backend-architecture\/go\.sum'\) \}\}-$/m);
});

test('pull requests only restore the architecture build cache', () => {
  const uses = ratchetSteps.map(block => field(block, 'uses', 8) ?? field(block, '- uses', 6)).filter(Boolean);
  assert.ok(uses.some(value => value.startsWith('actions/cache/restore@')));
  assert.deepEqual(uses.filter(value => /^actions\/cache(\/save)?@/.test(value)), [], 'no step of the ratchet saves');
});

test('warm job sweeps to one check and runs on every change of the ratchet build', () => {
  const marker = '"$RUNNER_TEMP/architecture-cache-marker" ~/.cache/go-build';
  assert.match(step(warmSteps, 'Mark restored architecture build cache entries'), new RegExp(`go-cache-sweep\\.sh mark ${marker.replace(/[$.~"/]/g, '\\$&')}`));
  const sweep = step(warmSteps, 'Drop architecture build cache entries this check did not use');
  assert.match(sweep, /go-cache-sweep\.sh sweep "\$RUNNER_TEMP\/architecture-cache-marker" ~\/\.cache\/go-build/);
  assert.match(sweep, /steps\.check\.outcome == 'success'/, 'a check that stopped early sweeps nothing');
  assert.match(step(warmSteps, 'Check this commit against itself'), /scripts\/backend-architecture\.sh check --base-ref "\$GITHUB_SHA"/);
  const paths = warmSource.match(/^ {4}paths:\n((?: {6}.*\n)*)/m)?.[1] ?? '';
  for (const input of ['backend/**', 'scripts/backend-architecture/**', 'scripts/go-cache-sweep.sh', '.github/workflows/backend-build-cache.yml']) {
    assert.match(paths, new RegExp(`- '${input.replace(/[.*/]/g, '\\$&')}'`), `push trigger covers ${input}`);
  }
});
