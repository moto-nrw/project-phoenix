#!/usr/bin/env node
// Prints why the latest PR check run of a branch failed, compactly enough for
// an agent's context: the failed jobs and steps, the error lines of each failed
// step, and the local command that reproduces the job. It also warns when the
// same job already failed on the previous push, because pushing a CI failure
// again without a local reproduction cost the most round trips (journal,
// docs/research/moto-iteration-hill-climb-journal.md, rounds 6 and 14).
//
// Usage: node scripts/ci-failures.mjs [branch | PR number]   (default: current branch)
// Exit: 0 green or still running, 1 failed, 2 usage or lookup error.
import { spawnSync } from 'node:child_process';
import { pathToFileURL } from 'node:url';

// Reproduction for the jobs that failed most often. Jobs not listed fall back
// to the failed step's own CI command, printed with every failure.
const localCommands = {
  'backend-quality': 'scripts/run-go-toolchain.sh bash scripts/check-quality.sh backend  (deadcode alone: scripts/run-go-toolchain.sh bash scripts/check-deadcode.sh)',
  'backend-lint': 'scripts/run-go-toolchain.sh scripts/check-backend-quality.sh lint origin/development',
  'frontend-quality': 'bash scripts/check-quality.sh frontend',
  'frontend-react-doctor': 'cd frontend && bash ../scripts/check-quality.sh react-doctor origin/development',
  'frontend-build': 'cd frontend && pnpm run build && pnpm run perf:bundle-ratchet',
  'test-frontend': 'node scripts/test-frontend-changed.mjs --check-guards; node scripts/test-frontend-changed.mjs origin/development',
  'test-backend': 'scripts/run-go-toolchain.sh scripts/test-changed.sh origin/development',
  'seed-smoke': 'cd backend && ../scripts/run-go-toolchain.sh go test ./internal/devtools/seedapi/ -run TestSeedCoverageRatchet',
  'env-sync-check': 'bash scripts/env-check.sh',
};
const ignoredJobs = new Set(['ci-gate']);
// Case-sensitive on purpose: tools print findings as FAIL, Error, Unused, while
// lower-case "fail" and "unused" also occur in passing output and migration names.
const annotation = /^##\[error\](?!Process completed)/;
const errorLine = /\bFAIL\b|panic:|Error\b|ERROR|undefined:|[Vv]iolation|Verstö|überschreitet|[Dd]ead code|reachable only|Unused|exceed|✗|✖/;
const passing = /^\s*(?:PASS|ok\s|✔|ℹ)/;
const maxErrorLines = 30;
const tailLines = 12;

// "lint / frontend-quality" -> "frontend-quality"; "test / test-frontend (dom-2)" -> "test-frontend".
export function jobKey(name) {
  return name.split(' / ').at(-1).replace(/\s*\(.*\)$/, '').trim();
}

export function localCommand(name) {
  return localCommands[jobKey(name)];
}

// GitHub prefixes every line with an ISO timestamp; tools add ANSI colours.
export function cleanLog(raw) {
  return raw.split(/\r?\n/).map(line => line
    .replace(/\x1b\[[0-9;]*[A-Za-z]/g, '')
    .replace(/^﻿?\d{4}-\d\d-\d\dT[\d:.]+Z ?/, ''));
}

// The failed step is the last "Run ..." group before the first non-zero exit,
// or before the first error annotation of an action that reports no exit code.
export function failedStep(lines) {
  const exit = lines.findIndex(line => /^##\[error\]Process completed with exit code [1-9]/.test(line));
  const end = exit >= 0 ? exit : lines.findIndex(line => line.startsWith('##[error]'));
  if (end < 0) return undefined;
  let start = -1;
  for (let i = end - 1; i >= 0; i -= 1) {
    if (lines[i].startsWith('##[group]Run ')) { start = i; break; }
  }
  if (start < 0) return undefined;
  let body = start + 1;
  while (body < end && lines[body] !== '##[endgroup]') body += 1;
  return { command: lines[start].slice('##[group]Run '.length), output: lines.slice(body + 1, exit >= 0 ? end : end + 1) };
}

// Error-looking lines (bounded) plus the step's last lines, without repeats.
export function digest(output) {
  const kept = new Set();
  const errors = [
    ...output.filter(line => annotation.test(line)),
    ...output.filter(line => !annotation.test(line) && !passing.test(line) && errorLine.test(line)),
  ].filter(line => !kept.has(line) && kept.add(line)).slice(0, maxErrorLines);
  const tail = output.slice(-tailLines).filter(line => line.trim() && !kept.has(line));
  const skipped = output.length - errors.length - tail.length;
  return [...errors, ...(tail.length && skipped > 0 ? [`… ${skipped} more lines …`] : []), ...tail];
}

export function repeatedFailures(latest, previous) {
  const before = new Set(previous.map(jobKey));
  return [...new Set(latest.map(jobKey))].filter(key => before.has(key));
}

function gh(args) {
  const result = spawnSync('gh', args, { encoding: 'utf8', maxBuffer: 512 * 1024 * 1024 });
  if (result.error) throw new Error(`gh: ${result.error.message}`);
  if (result.status !== 0) throw new Error(`gh ${args.slice(0, 3).join(' ')}: ${result.stderr.trim()}`);
  return result.stdout;
}

function git(args) {
  const result = spawnSync('git', args, { encoding: 'utf8' });
  return result.status === 0 ? result.stdout.trim() : '';
}

function failedJobs(runId) {
  const jobs = JSON.parse(gh(['api', '--paginate', '--slurp', `repos/{owner}/{repo}/actions/runs/${runId}/jobs`]))
    .flatMap(page => page.jobs);
  return jobs.filter(job => job.conclusion === 'failure' && !ignoredJobs.has(jobKey(job.name)));
}

function main(argv) {
  if (argv.length > 1 || argv[0] === '-h' || argv[0] === '--help') {
    console.error('Usage: node scripts/ci-failures.mjs [branch | PR number]');
    return 2;
  }
  let branch = argv[0] ?? git(['branch', '--show-current']);
  if (/^\d+$/.test(branch)) branch = gh(['pr', 'view', branch, '--json', 'headRefName', '--jq', '.headRefName']).trim();
  if (!branch) {
    console.error('No branch: check out a branch or pass a branch or PR number.');
    return 2;
  }
  const runs = JSON.parse(gh(['run', 'list', '--workflow', 'main.yml', '--branch', branch, '--event', 'pull_request',
    '--limit', '10', '--json', 'databaseId,status,conclusion,headSha,url']));
  const completed = runs.filter(run => run.status === 'completed' && run.conclusion !== 'cancelled');
  if (runs[0] && runs[0].status !== 'completed') console.log(`Run for ${runs[0].headSha.slice(0, 10)} still in progress: ${runs[0].url}`);
  const latest = completed[0];
  if (!latest) {
    console.log(`No completed PR check run for ${branch}.`);
    return 0;
  }
  const local = argv[0] === undefined ? git(['rev-parse', 'HEAD']) : '';
  const stale = local && local !== latest.headSha ? ` (local HEAD is ${local.slice(0, 10)})` : '';
  if (latest.conclusion === 'success') {
    console.log(`CI green for ${latest.headSha.slice(0, 10)}${stale}.`);
    return 0;
  }
  console.log(`CI ${latest.conclusion} for ${latest.headSha.slice(0, 10)}${stale}: ${latest.url}`);
  const jobs = failedJobs(latest.databaseId);
  for (const job of jobs) {
    const step = job.steps?.find(candidate => candidate.conclusion === 'failure')?.name ?? 'unknown step';
    console.log(`\n✗ ${job.name} / ${step}`);
    let section;
    try {
      section = failedStep(cleanLog(gh(['api', '--allow-escape-sequences', `repos/{owner}/{repo}/actions/jobs/${job.id}/logs`])));
    } catch (error) {
      console.log(`  log unavailable: ${error.message}`);
    }
    if (section) {
      console.log(`  CI ran: ${section.command}`);
      for (const line of digest(section.output)) console.log(`  | ${line}`);
    }
    const command = localCommand(job.name);
    console.log(`  Reproduce locally: ${command ?? (section ? section.command : 'see the job in .github/workflows')}`);
  }
  const previous = completed.find(run => run.headSha !== latest.headSha);
  if (previous?.conclusion === 'failure') {
    const repeated = repeatedFailures(jobs.map(job => job.name), failedJobs(previous.databaseId).map(job => job.name));
    if (repeated.length) {
      console.log(`\nThe previous push (${previous.headSha.slice(0, 10)}) failed in the same job(s): ${repeated.join(', ')}.`);
      console.log('Reproduce the failure locally and see it pass before pushing again.');
    }
  }
  return 1;
}

if (import.meta.url === pathToFileURL(process.argv[1]).href) {
  try {
    process.exitCode = main(process.argv.slice(2));
  } catch (error) {
    console.error(error.message);
    process.exitCode = 2;
  }
}
