#!/usr/bin/env node
import { execFileSync } from 'node:child_process';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { checkRelease } from './check-release-architecture.mjs';

const run = (command, args) => command === 'bash'
  ? execFileSync(command, args, { stdio: 'inherit' })
  : execFileSync(command, args, { encoding: 'utf8' }).trim();

export async function checkMigrationEvidence(name, event, repository, execute = run) {
  const pr = event.pull_request;
  const args = ['scripts/backend-architecture.sh', 'validate-ticket', '--all'];
  if (name === 'pull_request' && pr?.base?.ref === 'main' && pr?.head?.ref === 'development' && pr?.head?.repo?.full_name === repository) {
    const tested = await checkRelease({ event: name, baseRef: pr.base.ref, headRef: pr.head.ref,
      repository, headRepository: pr.head.repo.full_name, baseSha: pr.base.sha, headSha: pr.head.sha }, execute);
    execute('git', ['diff', '--exit-code', tested, '--']);
  } else {
    const base = name === 'pull_request' ? pr?.base?.sha
      : name === 'merge_group' ? event.merge_group?.base_sha
        : name === 'push' ? event.before : undefined;
    if (!/^[a-f0-9]{40}$/.test(base ?? '') || /^0+$/.test(base)) {
      throw new Error(`Migration evidence requires an immutable base SHA for ${name}`);
    }
    execute('git', ['cat-file', '-e', `${base}^{commit}`]);
    if (name === 'push' && event.ref === 'refs/heads/main' &&
        !execute('git', ['ls-tree', '--name-only', base, '--', 'scripts/check-migration-evidence.mjs'])) {
      await verifyFirstReleasePush(event, repository, execute);
      return execute('bash', args);
    }
    args.push('--base-ref', base);
  }
  return execute('bash', args);
}

async function verifyFirstReleasePush(event, repository, execute) {
  if (!/^[a-f0-9]{40}$/.test(event.after ?? '') || /^0+$/.test(event.after)) {
    throw new Error('Release push requires an immutable after SHA');
  }
  execute('git', ['diff', '--exit-code', event.after, '--']);
  const commits = execute('git', ['rev-list', '--parents', '-n', '1', event.after]).split(/\s+/);
  // A normal merge exposes its development parent; a fast-forward is itself
  // the tested development commit. Never search unrelated commits by tree hash.
  const candidates = [...commits.slice(1).reverse(), commits[0]].filter(sha => sha !== event.before);
  for (const sha of candidates) {
    try {
      await checkRelease({ event: 'pull_request', baseRef: 'main', headRef: 'development',
        repository, headRepository: repository, baseSha: event.before, headSha: sha }, execute, { timeoutMs: 0 });
      execute('git', ['merge-base', '--is-ancestor', sha, event.after]);
      execute('git', ['diff', '--exit-code', sha, '--']);
      return;
    } catch (error) {
      // Only failed promotion proofs permit trying another direct ancestor.
      // The release is rejected if none proves successful, exact-tree CI.
      if (sha === candidates.at(-1)) throw error;
    }
  }
  throw new Error('No verified development commit for the first release push');
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  try {
    const event = JSON.parse(readFileSync(process.env.GITHUB_EVENT_PATH, 'utf8'));
    await checkMigrationEvidence(process.env.GITHUB_EVENT_NAME, event, process.env.GITHUB_REPOSITORY);
  } catch (error) {
    console.error(error.message);
    process.exitCode = 1;
  }
}
