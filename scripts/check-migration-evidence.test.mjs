import assert from 'node:assert/strict';
import test from 'node:test';
import { readFileSync } from 'node:fs';
import { matchesGlob } from 'node:path';
import { checkMigrationEvidence } from './check-migration-evidence.mjs';

const base = 'a'.repeat(40);
const head = 'b'.repeat(40);
const repository = 'moto-nrw/project-phoenix';

for (const [name, event] of [
  ['pull_request', { pull_request: { base: { sha: base, ref: 'development' }, head: { sha: head, ref: 'feature', repo: { full_name: repository } } } }],
  ['merge_group', { merge_group: { base_sha: base } }],
  ['push', { before: base, ref: 'refs/heads/development' }],
]) {
  test(`${name} validates the full inventory against its immutable event base`, async () => {
    const calls = [];
    await checkMigrationEvidence(name, event, repository, (command, args) => { calls.push([command, args]); return ''; });
    assert.deepEqual(calls.at(-1), ['bash', ['scripts/backend-architecture.sh', 'validate-ticket', '--all', '--base-ref', base]]);
    assert.ok(calls.some(([command, args]) => command === 'git' && args.includes(`${base}^{commit}`)));
  });
}

for (const value of ['', 'HEAD', '0'.repeat(40), 'abc123']) {
  test(`missing or mutable push base fails closed: ${value}`, async () => {
    await assert.rejects(() => checkMigrationEvidence('push', { before: value, ref: 'refs/heads/development' }, repository, () => assert.fail('must not execute')), /immutable base SHA/);
  });
}

test('unavailable history cannot fall back to comparing HEAD with itself', async () => {
  await assert.rejects(() => checkMigrationEvidence('push', { before: base, ref: 'refs/heads/development' }, repository, () => { throw new Error('missing Git object'); }), /missing Git object/);
});

test('release PR reuses only successful development evidence for the exact tree', async () => {
  const calls = [];
  const event = { pull_request: { base: { sha: base, ref: 'main' }, head: { sha: head, ref: 'development', repo: { full_name: repository } } } };
  await checkMigrationEvidence('pull_request', event, repository, (command, args) => {
    calls.push([command, args]);
    if (command === 'gh') return JSON.stringify({ workflow_runs: [{ event:'push',head_branch:'development',head_sha:head,head_repository:{full_name:repository},path:'.github/workflows/main.yml',status:'completed',conclusion:'success' }] });
    if (command === 'git' && args[0] === 'rev-parse') return 'same-tree';
    return '';
  });
  assert.deepEqual(calls.at(-1), ['bash', ['scripts/backend-architecture.sh', 'validate-ticket', '--all']]);
  assert.ok(calls.some(([command,args]) => command === 'git' && args[0] === 'diff' && args.includes(head)));
});

test('first release push reuses an exact tested development parent, not pre-gate history', async () => {
  const merge = 'c'.repeat(40);
  const calls = [];
  await checkMigrationEvidence('push', { before: base, after: merge, ref: 'refs/heads/main' }, repository, (command, args) => {
    calls.push([command, args]);
    if (command === 'git' && args[0] === 'cat-file' && args[2].includes(':scripts/')) throw new Error('file absent before rollout');
    if (command === 'git' && args[0] === 'rev-list') return `${merge} ${base} ${head}`;
    if (command === 'git' && args[0] === 'rev-parse') return 'same-tree';
    if (command === 'gh') return JSON.stringify({ workflow_runs: [{ event:'push',head_branch:'development',head_sha:head,head_repository:{full_name:repository},path:'.github/workflows/main.yml',status:'completed',conclusion:'success' }] });
    return '';
  });
  assert.deepEqual(calls.at(-1), ['bash', ['scripts/backend-architecture.sh', 'validate-ticket', '--all']]);
  assert.ok(calls.some(([cmd,args]) => cmd === 'git' && args[0] === 'merge-base' && args.includes(head)));
});

for (const failure of ['CI', 'tree', 'ancestry', 'working tree']) {
  test(`first release push rejects failed ${failure} proof`, async () => {
    const merge = 'c'.repeat(40);
    await assert.rejects(() => checkMigrationEvidence('push', { before:base,after:merge,ref:'refs/heads/main' }, repository, (command,args) => {
      if (command === 'git' && args[0] === 'rev-list') return `${merge} ${base} ${head}`;
      if (command === 'git' && args[0] === 'rev-parse') return failure === 'tree' && args[1] === 'HEAD^{tree}' ? 'different' : 'tree';
      if (command === 'git' && args[0] === 'merge-base' && failure === 'ancestry') throw new Error('not ancestor');
      if (command === 'git' && args[0] === 'diff' && failure === 'working tree') throw new Error('dirty tree');
      if (command === 'gh') return JSON.stringify({ workflow_runs:[{event:'push',head_branch:'development',head_sha:head,head_repository:{full_name:repository},path:'.github/workflows/main.yml',status:'completed',conclusion:failure === 'CI' ? 'failure':'success'}] });
      if (command === 'bash') assert.fail('must not accept release');
      return '';
    }));
  });
}

test('source-only edits route to the evidence gate', () => {
  const workflow = readFileSync(new URL('../.github/workflows/main.yml', import.meta.url), 'utf8');
  const block = workflow.split('            backend-architecture:\n')[1].split('            backend-lint-full:\n')[0];
  const patterns = [...block.matchAll(/- '([^']+)'/g)].map(match => match[1]);
  for (const path of ['docs/runtime-checkpoints/account-sessions-2720.md', 'docs/operations/student-owner-storage-backfill.md', 'backend/architecture/wave.json', 'scripts/check-migration-evidence.mjs']) {
    assert.ok(patterns.some(pattern => matchesGlob(path, pattern)), `${path} must trigger the gate`);
  }
  assert.match(workflow, /run: node scripts\/check-migration-evidence\.mjs/);
});
