#!/usr/bin/env node
// Weekly watch on the pull request runtime target (.claude/rules/ci-test-performance.md).
//
//   GITHUB_REPOSITORY=moto-ogs/project-phoenix GITHUB_TOKEN=$(gh auth token) \
//     node scripts/ci-runtime-watch.mjs [--dry-run] [--days 7]
//
// Measures the green first attempts of the CI workflow (main.yml) on pull
// requests: first job start to the end of ci-gate. When the median of the runs
// with backend tests is above the target's upper bound, it opens one issue or
// comments on the open one. It never closes the issue: a fast week does not
// prove the cause is gone. --dry-run prints the issue text instead of writing.
import { appendFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { githubApi } from '../.github/actions/ci-cost/ci-cost.mjs';

export const TITLE = 'CI-Laufzeit: Median der PR-Läufe mit Backend-Tests über 5 Minuten';
export const LABEL = 'dev-ops';
// Upper bound of the 3–5 min target for a whole pull request run.
export const THRESHOLD_SECONDS = 300;
// Fewer runs than this say more about the week's traffic than about CI.
export const MIN_RUNS = 10;
// Each sampled run costs one job-list request; GITHUB_TOKEN allows 1000 an hour.
export const MAX_RUNS = 250;
const WORKFLOW = 'main.yml';
const GATE = 'ci-gate';
const BACKEND_JOB = /^Test \/ Backend tests\b/;
const RULE = '.claude/rules/ci-test-performance.md';
const MAX_PAGES = 10;

// One-day slices: a filtered run list returns at most 1000 runs, a busy week has more.
export function windowSlices(now, days) {
  const day = 24 * 60 * 60 * 1000;
  const end = now.getTime();
  return Array.from({ length: days }, (_, i) => ({
    from: new Date(end - (days - i) * day).toISOString().replace(/\.\d{3}Z$/, 'Z'),
    to: new Date(end - (days - i - 1) * day - 1000).toISOString().replace(/\.\d{3}Z$/, 'Z'),
  }));
}

// Green first attempts of pull request runs, oldest first.
export function selectRuns(runs) {
  const seen = new Set();
  return runs
    .filter(run => run.event === 'pull_request' && run.conclusion === 'success' && run.run_attempt === 1)
    .filter(run => !seen.has(run.id) && seen.add(run.id))
    .sort((a, b) => Date.parse(a.created_at) - Date.parse(b.created_at));
}

// Evenly spread over the window, so a sample does not favour one day.
export function sampleRuns(runs, max) {
  if (runs.length <= max) return runs;
  return Array.from({ length: max }, (_, i) => runs[Math.floor((i * runs.length) / max)]);
}

const seconds = (from, to) => (Date.parse(to) - Date.parse(from)) / 1000;
// A job that got a runner. The "CI cost" check run (ci-cost.yml) joins the
// run's check suite as a job without runner after ci-gate.
const ran = job => job.runner_name && job.started_at && job.completed_at && job.conclusion !== 'skipped';

// null when ci-gate did not pass in this attempt.
export function measureRun(run, jobs) {
  const gate = jobs.find(job => job.name === GATE && job.conclusion === 'success' && job.completed_at);
  const worked = jobs.filter(job => job.name !== GATE && ran(job));
  if (!gate || worked.length === 0) return null;
  const start = worked.reduce((min, job) => (job.started_at < min ? job.started_at : min), worked[0].started_at);
  const last = worked.reduce((a, b) => (b.completed_at > a.completed_at ? b : a));
  const backend = jobs.find(job => BACKEND_JOB.test(job.name) && job.conclusion === 'success' && ran(job));
  return {
    id: run.id,
    url: run.html_url,
    branch: run.head_branch,
    seconds: Math.round(seconds(start, gate.completed_at)),
    backendSeconds: backend ? Math.round(seconds(backend.started_at, backend.completed_at)) : null,
    lastJob: last.name,
  };
}

export function median(values) {
  if (values.length === 0) return null;
  const sorted = [...values].sort((a, b) => a - b);
  const mid = Math.floor(sorted.length / 2);
  return sorted.length % 2 ? sorted[mid] : (sorted[mid - 1] + sorted[mid]) / 2;
}

export function summarize(measured, { listed, sampled }) {
  const backend = measured.filter(run => run.backendSeconds !== null);
  const lastJobs = new Map();
  for (const run of measured) lastJobs.set(run.lastJob, (lastJobs.get(run.lastJob) ?? 0) + 1);
  return {
    listed,
    sampled,
    all: { runs: measured.length, median: median(measured.map(run => run.seconds)) },
    backend: { runs: backend.length, median: median(backend.map(run => run.seconds)) },
    backendJob: { median: median(backend.map(run => run.backendSeconds)) },
    lastJobs: [...lastJobs].sort(([a, x], [b, y]) => y - x || a.localeCompare(b)).slice(0, 3),
    slowest: [...measured].sort((a, b) => b.seconds - a.seconds).slice(0, 5),
  };
}

export function decide(stats, { threshold = THRESHOLD_SECONDS, minRuns = MIN_RUNS } = {}) {
  if (stats.backend.runs < minRuns) return 'insufficient';
  return stats.backend.median > threshold ? 'over' : 'ok';
}

const fmt = value => (value === null ? '–' : `${Math.round(value)} s`);
const cell = text => String(text).replace(/[\r\n|`]/g, ' ');

export function renderReport(stats, decision, { repo, from, to, threshold = THRESHOLD_SECONDS, minRuns = MIN_RUNS }) {
  const verdict = {
    over: `**Über dem Ziel:** Der Median der Läufe mit Backend-Tests liegt bei ${fmt(stats.backend.median)}, die Obergrenze ist ${threshold} s.`,
    ok: `Im Ziel: Der Median der Läufe mit Backend-Tests liegt bei ${fmt(stats.backend.median)}, die Obergrenze ist ${threshold} s.`,
    insufficient: `Keine Entscheidung: nur ${stats.backend.runs} Läufe mit Backend-Tests, nötig sind ${minRuns}.`,
  }[decision];
  const sample = stats.sampled < stats.listed ? ` (gleichmäßige Stichprobe von ${stats.sampled} aus ${stats.listed})` : '';
  return [
    '### CI-Laufzeit der Pull Requests',
    '',
    `Zeitraum ${from.slice(0, 16).replace('T', ' ')} bis ${to.slice(0, 16).replace('T', ' ')} UTC. Gemessen: grüne erste Versuche des Workflows CI (\`${WORKFLOW}\`), vom Start des ersten Jobs bis zum Ende von \`${GATE}\`${sample}.`,
    '',
    verdict,
    '',
    '| Auswahl | Läufe | Median |',
    '|---|---:|---:|',
    `| Alle Läufe | ${stats.all.runs} | ${fmt(stats.all.median)} |`,
    `| Läufe mit Backend-Tests | ${stats.backend.runs} | ${fmt(stats.backend.median)} |`,
    `| Job \`Test / Backend tests\` | ${stats.backend.runs} | ${fmt(stats.backendJob.median)} |`,
    '',
    'Zuletzt fertig vor `ci-gate`:',
    '',
    ...stats.lastJobs.map(([name, count]) => `- ${cell(name)}: ${count} Läufe`),
    '',
    'Langsamste Läufe:',
    '',
    ...stats.slowest.map(run => `- [${run.id}](${run.url}) ${run.seconds} s${run.backendSeconds === null ? '' : `, Backend-Tests ${run.backendSeconds} s`}`),
    '',
    `Ziele und Vorgehen: [\`${RULE}\`](https://github.com/${repo}/blob/development/${RULE}).`,
    '',
  ].join('\n');
}

export function issueBody(report) {
  return [
    'Der wöchentliche Laufzeit-Wächter (`.github/workflows/ci-runtime-watch.yml`) hat das Laufzeitziel gerissen.',
    'Suche die Pakete und Jobs, die gewachsen sind (Job-Summary „Backend package time budgets“ und „CPU seconds“), und mache sie günstiger in der Reihenfolge der Regel.',
    'Der Wächter schließt dieses Issue nicht; schließe es, wenn die Ursache behoben ist.',
    '',
    report,
  ].join('\n');
}

async function listRuns(api, repo, slices) {
  const runs = [];
  for (const { from, to } of slices) {
    const query = new URLSearchParams({ event: 'pull_request', status: 'success', created: `${from}..${to}`, per_page: '100' });
    let url = `/repos/${repo}/actions/workflows/${WORKFLOW}/runs?${query}`;
    for (let page = 1; page <= MAX_PAGES; page++) {
      const body = await api.get(`${url}&page=${page}`);
      runs.push(...body.workflow_runs);
      if (body.workflow_runs.length < 100) break;
    }
  }
  return runs;
}

async function openIssue(api, repo) {
  for (let page = 1; page <= MAX_PAGES; page++) {
    const issues = await api.get(`/repos/${repo}/issues?state=open&labels=${encodeURIComponent(LABEL)}&per_page=100&page=${page}`);
    const found = issues.find(issue => !issue.pull_request && issue.title === TITLE);
    if (found || issues.length < 100) return found ?? null;
  }
  return null;
}

export async function main(env, api, args) {
  const repo = env.GITHUB_REPOSITORY;
  if (!repo) throw new Error('GITHUB_REPOSITORY is not set');
  const dryRun = args.includes('--dry-run');
  const daysArg = args.indexOf('--days');
  const days = daysArg >= 0 ? Number(args[daysArg + 1]) : 7;
  if (!Number.isInteger(days) || days < 1 || days > 30) throw new Error('--days must be 1 to 30');

  const slices = windowSlices(new Date(), days);
  const listed = selectRuns(await listRuns(api, repo, slices));
  const sampled = sampleRuns(listed, MAX_RUNS);
  const measured = [];
  for (const run of sampled) {
    const { jobs } = await api.get(`/repos/${repo}/actions/runs/${run.id}/attempts/1/jobs?per_page=100`);
    const row = measureRun(run, jobs);
    if (row) measured.push(row);
  }
  const stats = summarize(measured, { listed: listed.length, sampled: sampled.length });
  const decision = decide(stats);
  const report = renderReport(stats, decision, { repo, from: slices[0].from, to: slices.at(-1).to });
  console.log(report);
  if (env.GITHUB_STEP_SUMMARY) appendFileSync(env.GITHUB_STEP_SUMMARY, `${report}\n`);
  if (decision !== 'over') return { decision, action: 'none' };

  const existing = await openIssue(api, repo);
  if (dryRun) {
    console.log(existing
      ? `Dry-run: würde #${existing.number} kommentieren:\n\n${report}`
      : `Dry-run: würde ein Issue „${TITLE}“ mit Label ${LABEL} öffnen:\n\n${issueBody(report)}`);
    return { decision, action: existing ? 'comment (dry-run)' : 'create (dry-run)' };
  }
  if (existing) {
    await api.send('POST', `/repos/${repo}/issues/${existing.number}/comments`, { body: report });
    console.log(`Kommentar an #${existing.number}`);
    return { decision, action: 'comment' };
  }
  const created = await api.send('POST', `/repos/${repo}/issues`, { title: TITLE, body: issueBody(report), labels: [LABEL, 'needs-triage'] });
  console.log(`Issue #${created.number} geöffnet`);
  return { decision, action: 'create' };
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  const env = process.env;
  await main(env, githubApi({ token: env.GITHUB_TOKEN, baseUrl: env.GITHUB_API_URL }), process.argv.slice(2));
}
