import assert from 'node:assert/strict';
import { mkdtempSync, readFileSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import test from 'node:test';
import { billJob, collectRuns, decodeState, encodeState, main, pricePerMinute, render } from './ci-cost.mjs';

const prices = { publicRepo: true };
const repo = 'org/repo';
const pr = { number: 7, head: { ref: 'feat/x', repo: { id: 1 } }, created_at: '2026-10-01T00:00:00Z' };
const job = (labels, start, end, runner = 'runner-1') =>
  ({ labels, runner_name: runner, started_at: `2026-10-02T10:${start}Z`, completed_at: `2026-10-02T10:${end}Z` });
const run = (id, attempt = 1, status = 'completed', repoId = 1) =>
  ({ id, run_attempt: attempt, status, workflow_id: 9, path: '.github/workflows/ci.yml', head_repository: { id: repoId } });

// Answers paginate/get by path prefix and records every requested path.
function fakeApi(routes) {
  const calls = [];
  const answer = path => {
    calls.push(path);
    const route = Object.keys(routes).find(prefix => path.startsWith(prefix));
    assert.ok(route, `unexpected request ${path}`);
    return typeof routes[route] === 'function' ? routes[route](path) : routes[route];
  };
  return { calls, get: async path => answer(path), paginate: async path => answer(path) };
}

test('prices Blacksmith by vCPU and GitHub-hosted runners as free only in public repos', () => {
  assert.equal(pricePerMinute('blacksmith-2vcpu-ubuntu-2404', prices), 0.004);
  assert.equal(pricePerMinute('blacksmith-8vcpu-ubuntu-2404', prices), 0.016);
  assert.equal(pricePerMinute('blacksmith-4vcpu-ubuntu-2404-arm', prices), 0.005);
  assert.equal(pricePerMinute('ubuntu-latest', prices), 0);
  assert.equal(pricePerMinute('ubuntu-24.04-arm', prices), 0);
  assert.equal(pricePerMinute('ubuntu-slim', prices), 0);
  assert.equal(pricePerMinute('ubuntu-latest-4-cores', prices), null);
  assert.equal(pricePerMinute('ubuntu-latest', { publicRepo: false }), null);
  assert.equal(pricePerMinute('self-hosted', prices), null);
});

test('bills each job in whole minutes and skips jobs without a runner', () => {
  assert.deepEqual(billJob(job(['blacksmith-8vcpu-ubuntu-2404'], '00:00', '01:01'), prices),
    { label: 'blacksmith-8vcpu-ubuntu-2404', minutes: 2 });
  assert.deepEqual(billJob(job(['self-hosted', 'linux'], '00:00', '00:30'), prices),
    { label: 'self-hosted,linux', minutes: 1 });
  assert.equal(billJob(job(['ubuntu-latest'], '00:00', '00:30', null), prices), null);
  assert.equal(billJob(job(['ubuntu-latest'], '00:02', '00:01'), prices), null);
});

test('state survives the rendered comment', () => {
  const runs = {
    11: { attempt: 2, workflow: 'CI', minutes: { 'blacksmith-8vcpu-ubuntu-2404': 5, 'ubuntu-latest': 3 } },
    12: { attempt: 1, workflow: 'PR body', minutes: {} },
  };
  assert.deepEqual(decodeState(render({ runs, running: 0, prices })), runs);
  assert.deepEqual(decodeState('<!-- ci-cost-state bm90IGpzb24= -->'), {});
  assert.deepEqual(decodeState(undefined), {});
});

test('fetches jobs only for new and re-run runs, keeps cached runs in progress', async () => {
  const cached = {
    1: { attempt: 1, workflow: 'CI', minutes: { 'blacksmith-4vcpu-ubuntu-2404': 3 } },
    2: { attempt: 1, workflow: 'CI', minutes: { 'blacksmith-4vcpu-ubuntu-2404': 3 } },
    3: { attempt: 1, workflow: 'CI', minutes: { 'blacksmith-4vcpu-ubuntu-2404': 3 } },
  };
  const api = fakeApi({
    [`/repos/${repo}/actions/runs?`]: [run(1), run(2, 2), run(3, 2, 'in_progress'), run(4), run(5, 1, 'queued'), run(6, 1, 'completed', 99)],
    [`/repos/${repo}/actions/runs/`]: [job(['blacksmith-8vcpu-ubuntu-2404'], '00:00', '04:00')],
    [`/repos/${repo}/actions/workflows`]: [{ id: 9, name: 'CI' }],
  });
  const { runs, running } = await collectRuns(api, repo, pr, cached, prices);

  assert.deepEqual(Object.keys(runs), ['1', '2', '3', '4']);
  assert.deepEqual(runs[2], { attempt: 2, workflow: 'CI', minutes: { 'blacksmith-8vcpu-ubuntu-2404': 4 } });
  assert.equal(runs[3], cached[3]);
  assert.equal(running, 2);
  assert.deepEqual(api.calls.filter(path => path.includes('/jobs')), [
    `/repos/${repo}/actions/runs/2/jobs?filter=all&per_page=100`,
    `/repos/${repo}/actions/runs/4/jobs?filter=all&per_page=100`,
  ]);
  const list = new URL(api.calls[0], 'https://x').searchParams;
  assert.equal(list.get('branch'), 'feat/x');
  assert.equal(list.get('created'), '>=2026-10-01T00:00:00Z');
});

test('renders totals per workflow and marks unpriced runners', () => {
  const body = render({
    runs: {
      1: { attempt: 1, workflow: 'CI', minutes: { 'blacksmith-8vcpu-ubuntu-2404': 10, 'ubuntu-latest': 4 } },
      2: { attempt: 1, workflow: 'CI', minutes: { 'blacksmith-4vcpu-ubuntu-2404': 5 } },
      3: { attempt: 1, workflow: 'x|<b>@team', minutes: { 'self-hosted': 3 } },
    },
    running: 1,
    prices,
    trigger: { id: 2, run_number: 41, html_url: 'https://github.com/org/repo/actions/runs/2' },
  });
  assert.match(body, /\*\*≈ 0,20 \$\*\* für 3 Runs, davon 15 bezahlte Runner-Minuten\. 1 Run läuft noch\./);
  assert.match(body, /\| CI \| 2 \| 0,10 \$ \| 0,20 \$ \|/);
  assert.match(body, /\| x\\\|\\<b\\>@​team \| 1 \| 0,00 \$ \| 0,00 \$ \|/);
  assert.match(body, /\| self-hosted \| 3 \| \? \| \? \|/);
  assert.match(body, /Letzter Run: \[CI #41\]\(https:\/\/github\.com\/org\/repo\/actions\/runs\/2\) ≈ 0,04 \$/);
  assert.match(body, /fehlen in der Summe/);
});

test('main seeds the cache only from the bot comment and resolves fork pull requests by head', async () => {
  const dir = mkdtempSync(join(tmpdir(), 'ci-cost-'));
  const event = join(dir, 'event.json');
  writeFileSync(event, JSON.stringify({
    repository: { private: false },
    workflow_run: { id: 1, run_number: 3, pull_requests: [], head_branch: 'feat/x', head_repository: { owner: { login: 'fork' } } },
  }));
  const forged = encodeState({ 1: { attempt: 1, workflow: 'CI', minutes: { 'blacksmith-32vcpu-ubuntu-2404': 999 } } });
  const api = fakeApi({
    [`/repos/${repo}/pulls?state=open&head=fork%3Afeat%2Fx`]: [pr],
    [`/repos/${repo}/issues/7/comments`]: [{ user: { login: 'mallory' }, body: forged }],
    [`/repos/${repo}/actions/runs?`]: [run(1)],
    [`/repos/${repo}/actions/runs/1/jobs`]: [job(['blacksmith-2vcpu-ubuntu-2404'], '00:00', '01:00')],
    [`/repos/${repo}/actions/workflows`]: [{ id: 9, name: 'CI' }],
  });
  const env = { GITHUB_EVENT_PATH: event, GITHUB_REPOSITORY: repo, CI_COST_OUTPUT: join(dir, 'out.md'), GITHUB_OUTPUT: join(dir, 'gh-out') };

  assert.equal(await main(env, api), 7);
  assert.match(readFileSync(env.CI_COST_OUTPUT, 'utf8'), /\*\*< 0,01 \$\*\* für 1 Run, davon 1 bezahlte Runner-Minuten/);
  assert.equal(readFileSync(env.GITHUB_OUTPUT, 'utf8'), 'pr=7\n');
});
