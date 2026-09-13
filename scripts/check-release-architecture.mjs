#!/usr/bin/env node
import { execFileSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import { resolve } from 'node:path';

const run = (command, args) => execFileSync(command, args, { encoding: 'utf8' }).trim();

export function checkRelease(context, execute = run) {
  const { event, baseRef, headRef, repository, headRepository, baseSha, headSha, candidate = 'HEAD' } = context;
  if (event !== 'pull_request' || baseRef !== 'main' || headRef !== 'development' ||
      !repository || repository !== headRepository) {
    throw new Error('Release promotion requires same-repository development -> main');
  }
  for (const sha of [baseSha, headSha]) {
    if (!/^[a-f0-9]{40}$/.test(sha ?? '')) throw new Error('Release commits must be full immutable SHAs');
  }
  execute('git', ['merge-base', '--is-ancestor', baseSha, headSha]);
  const expected = execute('git', ['rev-parse', `${headSha}^{tree}`]);
  if (execute('git', ['rev-parse', `${candidate}^{tree}`]) !== expected) {
    throw new Error('Release tree differs from the tested development commit; merge main into development first');
  }
  // A successful PR check is not evidence for the complete development push suite.
  const response = JSON.parse(execute('gh', ['api', '--method', 'GET',
    `repos/${repository}/actions/workflows/main.yml/runs`,
    '-f', 'event=push', '-f', 'branch=development', '-f', `head_sha=${headSha}`, '-f', 'per_page=100']));
  const accepted = response.workflow_runs?.some(value =>
    value.event === 'push' && value.head_branch === 'development' && value.head_sha === headSha &&
    value.head_repository?.full_name === repository && value.path === '.github/workflows/main.yml' &&
    value.status === 'completed' && value.conclusion === 'success');
  if (!accepted) throw new Error('No successful full development CI run for the exact release commit');
  return headSha;
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  try {
    const headSha = checkRelease({
      event: process.env.EVENT_NAME, baseRef: process.env.BASE_REF, headRef: process.env.HEAD_REF,
      repository: process.env.REPOSITORY, headRepository: process.env.HEAD_REPO,
      baseSha: process.env.BASE_SHA, headSha: process.env.HEAD_SHA,
    });
    // Also reject tracked working-tree changes before evaluating the current graph.
    execFileSync('git', ['diff', '--exit-code', headSha, '--'], { stdio: 'inherit' });
    execFileSync('bash', ['scripts/backend-architecture.sh', 'check', '--base-ref', headSha], { stdio: 'inherit' });
  } catch (error) {
    console.error(error.message);
    process.exitCode = 1;
  }
}
