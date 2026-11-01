#!/usr/bin/env node
// Estimates the runner cost of all CI runs of a pull request and keeps it in
// one comment, created once and then edited. Runs on a workflow_run event and
// reads GITHUB_EVENT_PATH, GITHUB_REPOSITORY, GITHUB_TOKEN and GITHUB_API_URL.
// The comment caches the minutes of every completed run, so an event fetches
// jobs only for new or re-run runs.
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

// List prices in USD per minute, checked 2026-11-01 on blacksmith.sh/pricing
// and docs.blacksmith.sh/blacksmith-runners/overview: 2 vCPU cost the base
// rate, larger runners scale with their vCPU count.
export const BLACKSMITH_X64_2VCPU = 0.004;
export const BLACKSMITH_ARM_2VCPU = 0.0025;
const MARKER = 'ci-cost-state';
const BOT = 'github-actions[bot]';

// GitHub-hosted standard runners are free for public repositories. Larger
// runners and self-hosted labels have no price here and stay out of the sum.
export function pricePerMinute(label, { publicRepo }) {
  const blacksmith = /^blacksmith-(\d+)vcpu-ubuntu-\d{4}(-arm)?$/.exec(label);
  if (blacksmith) {
    return (Number(blacksmith[1]) / 2) * (blacksmith[2] ? BLACKSMITH_ARM_2VCPU : BLACKSMITH_X64_2VCPU);
  }
  if (publicRepo && /^((ubuntu|windows|macos)-(latest|[\d.]+)(-arm)?|ubuntu-slim)$/.test(label)) return 0;
  return null;
}

// Billed minutes of one job, rounded up per job like GitHub bills. Skipped
// jobs and jobs that never got a runner cost nothing.
export function billJob(job, prices) {
  if (!job.runner_name || !job.started_at || !job.completed_at) return null;
  const seconds = (Date.parse(job.completed_at) - Date.parse(job.started_at)) / 1000;
  if (!(seconds > 0)) return null;
  const labels = job.labels ?? [];
  const label = labels.find(l => pricePerMinute(l, prices) !== null) ?? (labels.join(',') || 'unbekannt');
  return { label, minutes: Math.ceil(seconds / 60) };
}

export function encodeState(runs) {
  const workflows = [];
  const labels = [];
  const index = (list, value) => {
    const found = list.indexOf(value);
    return found >= 0 ? found : list.push(value) - 1;
  };
  const r = {};
  for (const [id, run] of Object.entries(runs)) {
    r[id] = [run.attempt, index(workflows, run.workflow),
      Object.entries(run.minutes).map(([label, minutes]) => [index(labels, label), minutes])];
  }
  const json = JSON.stringify({ v: 1, w: workflows, l: labels, r });
  return `<!-- ${MARKER} ${Buffer.from(json).toString('base64')} -->`;
}

export function decodeState(body) {
  const match = new RegExp(`<!-- ${MARKER} ([A-Za-z0-9+/=]+) -->`).exec(body ?? '');
  if (!match) return {};
  try {
    const state = JSON.parse(Buffer.from(match[1], 'base64').toString('utf8'));
    if (state.v !== 1) return {};
    const runs = {};
    for (const [id, [attempt, workflow, minutes]] of Object.entries(state.r)) {
      runs[id] = {
        attempt,
        workflow: state.w[workflow],
        minutes: Object.fromEntries(minutes.map(([label, n]) => [state.l[label], n])),
      };
    }
    return runs;
  } catch {
    return {};
  }
}

export async function findPullRequest(api, repo, run) {
  const number = run.pull_requests?.[0]?.number;
  if (number) return api.get(`/repos/${repo}/pulls/${number}`);
  // Fork pull requests leave pull_requests empty.
  const head = encodeURIComponent(`${run.head_repository.owner.login}:${run.head_branch}`);
  const [pr] = await api.get(`/repos/${repo}/pulls?state=open&head=${head}`);
  return pr ?? null;
}

// Minutes per completed pull_request run of the PR. A run that is still in
// progress keeps its cached minutes until its new attempt completes.
export async function collectRuns(api, repo, pr, cached, prices) {
  const query = new URLSearchParams({
    event: 'pull_request',
    branch: pr.head.ref,
    created: `>=${pr.created_at}`,
    per_page: '100',
  });
  const listed = await api.paginate(`/repos/${repo}/actions/runs?${query}`, 'workflow_runs');
  // Another fork can use the same branch name.
  const own = listed.filter(run => !pr.head.repo || run.head_repository?.id === pr.head.repo.id);
  // run.name is the run-name, which can differ per run ("Build <sha>").
  let names;
  const workflowName = async run => {
    names ??= new Map((await api.paginate(`/repos/${repo}/actions/workflows?per_page=100`, 'workflows'))
      .map(workflow => [workflow.id, workflow.name]));
    return names.get(run.workflow_id) ?? run.path;
  };
  const runs = {};
  let running = 0;
  for (const run of own) {
    const hit = cached[run.id];
    if (run.status !== 'completed') {
      running++;
      if (hit) runs[run.id] = hit;
      continue;
    }
    if (hit && hit.attempt === run.run_attempt) {
      runs[run.id] = hit;
      continue;
    }
    // filter=all includes the jobs of earlier attempts; re-runs cost too.
    const jobs = await api.paginate(`/repos/${repo}/actions/runs/${run.id}/jobs?filter=all&per_page=100`, 'jobs');
    const minutes = {};
    for (const job of jobs) {
      const billed = billJob(job, prices);
      if (billed) minutes[billed.label] = (minutes[billed.label] ?? 0) + billed.minutes;
    }
    runs[run.id] = { attempt: run.run_attempt, workflow: await workflowName(run), minutes };
  }
  return { runs, running };
}

const number = (value, digits = 0) =>
  value.toLocaleString('de-DE', { minimumFractionDigits: digits, maximumFractionDigits: digits });
const usd = value => (value > 0 && value < 0.005 ? '< 0,01 $' : `${number(value, 2)} $`);
const approx = value => (value > 0 && value < 0.005 ? usd(value) : `≈ ${usd(value)}`);
// Workflow names come from the pull request's own workflow files.
const cell = text => String(text).replace(/[\r\n]+/g, ' ').slice(0, 80)
  .replace(/[\\`*_[\]<>|]/g, '\\$&').replace(/@/g, '@\u200b');

function costOf(minutes, prices) {
  let cost = 0;
  for (const [label, n] of Object.entries(minutes)) cost += n * (pricePerMinute(label, prices) ?? 0);
  return cost;
}

export function render({ runs, running, prices, trigger }) {
  const workflows = new Map();
  const runners = new Map();
  for (const run of Object.values(runs)) {
    const row = workflows.get(run.workflow) ?? { runs: 0, cost: 0 };
    row.runs++;
    row.cost += costOf(run.minutes, prices);
    workflows.set(run.workflow, row);
    for (const [label, n] of Object.entries(run.minutes)) runners.set(label, (runners.get(label) ?? 0) + n);
  }
  const total = [...workflows.values()].reduce((sum, row) => sum + row.cost, 0);
  const paidMinutes = [...runners].filter(([label]) => pricePerMinute(label, prices) > 0)
    .reduce((sum, [, n]) => sum + n, 0);
  const unpriced = [...runners.keys()].filter(label => pricePerMinute(label, prices) === null);
  const count = Object.keys(runs).length;

  const lines = [
    '### CI-Kosten dieses PRs',
    '',
    `**${approx(total)}** für ${count} ${count === 1 ? 'Run' : 'Runs'}, davon ${number(paidMinutes)} bezahlte Runner-Minuten.`
      + (running ? ` ${running} ${running === 1 ? 'Run läuft' : 'Runs laufen'} noch.` : ''),
    '',
    '| Workflow | Runs | Ø pro Run | Kosten |',
    '| --- | ---: | ---: | ---: |',
    ...[...workflows].sort(([a, x], [b, y]) => y.cost - x.cost || a.localeCompare(b))
      .map(([name, row]) => `| ${cell(name)} | ${row.runs} | ${usd(row.cost / row.runs)} | ${usd(row.cost)} |`),
    '',
    '<details><summary>Nach Runner</summary>',
    '',
    '| Runner | Minuten | $ pro Minute | Kosten |',
    '| --- | ---: | ---: | ---: |',
    ...[...runners].map(([label, n]) => [label, n, pricePerMinute(label, prices)])
      .sort(([a, , x], [b, , y]) => (y ?? 0) - (x ?? 0) || a.localeCompare(b))
      .map(([label, n, price]) => `| ${cell(label)} | ${number(n)} | ${price === null ? '?' : price.toLocaleString('de-DE', { maximumFractionDigits: 4 })} | ${price === null ? '?' : usd(n * price)} |`),
    '',
    '</details>',
    '',
  ];
  const latest = trigger && runs[trigger.id];
  if (latest) lines.push(`Letzter Run: [${cell(latest.workflow)} #${trigger.run_number}](${trigger.html_url}) ${approx(costOf(latest.minutes, prices))}`, '');
  lines.push(
    '<sub>Schätzung zum Listenpreis: Laufzeit je Job auf volle Minuten aufgerundet, Wiederholungen eingeschlossen. '
      + `Blacksmith ${number(BLACKSMITH_X64_2VCPU, 3)} $ pro Minute je 2 vCPU (ARM ${number(BLACKSMITH_ARM_2VCPU, 4)} $). `
      + (prices.publicRepo ? 'GitHub-Runner sind für öffentliche Repos kostenlos. ' : '')
      + 'Ohne Freiminuten und Speicher.'
      + (unpriced.length ? ' Runner mit „?“ haben keinen Preis und fehlen in der Summe.' : '')
      + '</sub>',
    '',
    encodeState(runs),
  );
  return `${lines.join('\n')}\n`;
}

export function githubApi({ token, baseUrl = 'https://api.github.com', fetchImpl = fetch }) {
  async function request(url, method = 'GET', body) {
    const response = await fetchImpl(url.startsWith('https://') ? url : baseUrl + url, {
      method,
      headers: {
        accept: 'application/vnd.github+json',
        authorization: `Bearer ${token}`,
        'x-github-api-version': '2022-11-28',
      },
      body: body && JSON.stringify(body),
    });
    if (!response.ok) throw new Error(`GitHub API ${response.status} for ${url}: ${await response.text()}`);
    return response;
  }
  return {
    get: async path => (await request(path)).json(),
    send: async (method, path, body) => (await request(path, method, body)).json(),
    async paginate(path, key) {
      const items = [];
      for (let url = path; url;) {
        const response = await request(url);
        const body = await response.json();
        items.push(...(key ? body[key] : body));
        url = /<([^>]+)>;\s*rel="next"/.exec(response.headers.get('link') ?? '')?.[1];
      }
      return items;
    },
  };
}

export async function main(env, api) {
  const event = JSON.parse(readFileSync(env.GITHUB_EVENT_PATH, 'utf8'));
  const run = event.workflow_run;
  const repo = env.GITHUB_REPOSITORY;
  const prices = { publicRepo: event.repository?.private === false };
  const pr = await findPullRequest(api, repo, run);
  if (!pr) {
    console.log(`No open pull request for ${run.head_branch}.`);
    return null;
  }
  const comments = await api.paginate(`/repos/${repo}/issues/${pr.number}/comments?per_page=100`);
  // Only our own comment may seed the cache; anyone can post a marker.
  const previous = comments.findLast(c => c.user?.login === BOT && c.body?.includes(MARKER));
  const { runs, running } = await collectRuns(api, repo, pr, decodeState(previous?.body), prices);
  const body = render({ runs, running, prices, trigger: run });
  if (!previous) await api.send('POST', `/repos/${repo}/issues/${pr.number}/comments`, { body });
  else if (previous.body !== body) await api.send('PATCH', `/repos/${repo}/issues/comments/${previous.id}`, { body });
  return pr.number;
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  const env = process.env;
  await main(env, githubApi({ token: env.GITHUB_TOKEN, baseUrl: env.GITHUB_API_URL }));
}
