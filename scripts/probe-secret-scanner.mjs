#!/usr/bin/env node
// Live acceptance probe. Requires ggshield authentication; never runs in unit CI.
import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { generateKeyPairSync } from 'node:crypto';
import { copyFileSync, mkdirSync, mkdtempSync, readFileSync, rmSync, symlinkSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const temporary = mkdtempSync(join(tmpdir(), 'phoenix-secret-probe-'));
const scanner = join(root, '.devbox/nix/profile/default/bin/ggshield');
const env = { ...process.env, GITGUARDIAN_EXIT_ZERO: 'false', GITGUARDIAN_SHOW_SECRETS: 'false' };
for (const key of ['GIT_DIR', 'GIT_WORK_TREE', 'GIT_INDEX_FILE', 'GIT_COMMON_DIR', 'GIT_PREFIX']) delete env[key];
env.GIT_CONFIG_GLOBAL = '/dev/null';
env.GIT_CONFIG_NOSYSTEM = '1';

function git(cwd, ...args) {
  const result = spawnSync('git', args, { cwd, env, encoding: 'utf8' });
  assert.equal(result.status, 0, 'Temporary probe repository operation failed');
}

function hook(cwd, args, expectedStatus) {
  const result = spawnSync('bash', ['scripts/check-secrets.sh', ...args],
    { cwd, env, encoding: 'utf8', maxBuffer: 16 * 1024 * 1024, timeout: 120_000 });
  assert.equal(result.status, expectedStatus,
    `Hook exited ${result.status}; expected ${expectedStatus}. No scanner payload printed.`);
}

function scan(path, expectedStatus) {
  const output = join(temporary, 'report.json');
  rmSync(output, { force: true });
  const result = spawnSync(scanner, ['--config-path', join(root, '.gitguardian.yaml'), '--no-check-for-updates',
    'secret', 'scan', 'path', '--recursive', '--yes', '--json', '--output', output, path],
  { cwd: root, env, encoding: 'utf8', maxBuffer: 16 * 1024 * 1024, timeout: 120_000 });
  // Never print scanner payloads: they can contain source snippets or matches.
  if (result.status === 1 && expectedStatus === 0) {
    const report = JSON.parse(readFileSync(output, 'utf8'));
    function summaries(collection) {
      return [...(collection.entities_with_incidents ?? []).map(entity => ({
        filename: entity.filename, detectors: entity.incidents.map(incident => incident.type),
      })), ...(collection.scans ?? []).flatMap(summaries)];
    }
    process.stderr.write(`Unexpected findings (metadata only): ${JSON.stringify(summaries(report))}\n`);
  }
  assert.equal(result.status, expectedStatus,
    `Scanner exited ${result.status}; expected ${expectedStatus}. Check authentication, connectivity and detection settings. No payload printed.`);
  const report = JSON.parse(readFileSync(output, 'utf8'));
  assert.equal(report.errors?.length ?? 0, 0, 'Scanner reported an incomplete scan');
  assert.equal(report.total_incidents > 0, expectedStatus === 1, 'Unexpected incident count');
  return report;
}

try {
  const clean = join(temporary, 'clean');
  mkdirSync(clean);
  for (const [index, file] of [
    'backend/test/credentials.go',
    'backend/test/fixtures.go',
    'backend/modules/identityaccess/behavior/auth_core_test.go',
    'backend/modules/identityaccess/behavior/auth_login_test.go',
    'backend/modules/identityaccess/behavior/guardian_invitation_service_test.go',
    'backend/modules/identityaccess/behavior/invitation_flow_test.go',
    'backend/modules/exporttransfer/internal/adapters/sftp/sftptest/server.go',
    'backend/modules/exporttransfer/compose/settings_resolver_test.go',
    'backend/modules/delivery/internal/notifications/vapid_internal_test.go',
    'backend/modules/delivery/subscriptions/subscription_test.go',
  ].entries()) writeFileSync(join(clean, `${index}-${file.split('/').at(-1)}`), readFileSync(join(root, file)));
  scan(clean, 0);
  process.stdout.write('PASS: generated credential fixtures and explicit mapping placeholders are clean.\n');

  git(clean, 'init', '-q');
  git(clean, 'config', 'user.name', 'Secret scanner probe');
  git(clean, 'config', 'user.email', 'scanner@example.invalid');
  mkdirSync(join(clean, 'scripts'));
  copyFileSync(join(root, 'scripts/check-secrets.sh'), join(clean, 'scripts/check-secrets.sh'));
  copyFileSync(join(root, '.gitguardian.yaml'), join(clean, '.gitguardian.yaml'));
  mkdirSync(join(clean, '.devbox/nix/profile/default/bin'), { recursive: true });
  symlinkSync(scanner, join(clean, '.devbox/nix/profile/default/bin/ggshield'));
  writeFileSync(join(clean, '.gitignore'), '.devbox/\n');
  git(clean, 'add', '.');
  hook(clean, ['pre-commit'], 0);
  git(clean, 'commit', '-qm', 'clean fixtures');
  git(clean, 'branch', 'base');
  process.stdout.write('PASS: staged clean fixtures pass the actual pre-commit wrapper.\n');

  const seeded = join(temporary, 'seeded');
  mkdirSync(seeded);
  // A fresh, undeployed signing key is deliberately embedded only in disposable
  // files. Each extension must remain scanned, especially test extensions.
  const { privateKey } = generateKeyPairSync('rsa', { modulusLength: 2048,
    privateKeyEncoding: { type: 'pkcs8', format: 'pem' }, publicKeyEncoding: { type: 'spki', format: 'pem' } });
  const names = ['fixture.go', 'fixture_test.go', 'fixture.test.ts', 'fixture.spec.tsx', 'fixture.test.mjs'];
  for (const name of names) writeFileSync(join(seeded, name), `const signingKey = \`${privateKey}\`;\n`);
  const report = scan(seeded, 1);
  function findings(collection) {
    return [...(collection.entities_with_incidents ?? []), ...(collection.scans ?? []).flatMap(findings)];
  }
  const found = findings(report).filter(entity => entity.incidents.some(incident => !incident.ignore_reason))
    .map(entity => entity.filename);
  for (const name of names) assert.ok(found.some(path => path.endsWith(name)), `No finding for ${name}`);
  process.stdout.write(`PASS: ephemeral private-key probes blocked in all ${names.length} source/test extensions.\n`);

  copyFileSync(join(seeded, 'fixture_test.go'), join(clean, 'leaked_test.go'));
  git(clean, 'add', 'leaked_test.go');
  hook(clean, ['pre-commit'], 1);
  git(clean, 'commit', '-qm', 'temporary probe only');
  git(clean, 'rm', '-q', 'leaked_test.go');
  git(clean, 'commit', '-qm', 'remove temporary probe');
  hook(clean, ['pre-push', 'base'], 1);
  process.stdout.write('PASS: hooks block a staged test key and its history after removal at the tip.\n');
} finally {
  rmSync(temporary, { recursive: true, force: true });
}
