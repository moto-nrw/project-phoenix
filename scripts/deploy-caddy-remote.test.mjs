import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { spawnSync } from 'node:child_process';
import { chmodSync, existsSync, mkdirSync, mkdtempSync, readFileSync, readdirSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import test from 'node:test';

const script = new URL('./deploy-caddy-remote.sh', import.meta.url).pathname;
const OLD = 'old.example {\n\trespond "old"\n}\n';
const NEW = 'new.example {\n\trespond "new"\n}\n';
const sha = text => createHash('sha256').update(text).digest('hex');

// Runs the script against a fake /etc/caddy with stub caddy, systemctl and
// curl on PATH. Each stub logs its call; `fail` names the ones that fail.
// systemctl reload fails only while the live file holds NEW, so a restored
// backup reloads cleanly.
function deploy(t, { live = OLD, known = [OLD], fail = {} } = {}) {
  const root = mkdtempSync(join(tmpdir(), 'moto-caddy-deploy-'));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  const caddyDir = join(root, 'etc-caddy');
  const bin = join(root, 'bin');
  mkdirSync(caddyDir);
  mkdirSync(bin);
  writeFileSync(join(caddyDir, 'Caddyfile'), live);
  writeFileSync(join(caddyDir, 'cloudflare.env'), 'CLOUDFLARE_API_TOKEN=fixture-not-a-token\n');
  writeFileSync(join(root, 'Caddyfile.new'), NEW);
  writeFileSync(join(root, 'Caddyfile.previous'), OLD);
  writeFileSync(join(root, 'known'), known.map(text => sha(text) + '\n').join(''));
  const calls = join(root, 'calls');
  const stub = (name, body) => {
    writeFileSync(join(bin, name), `#!/usr/bin/env bash\nprintf '%s %s\\n' ${name} "$*" >> "${calls}"\n${body}\n`);
    chmodSync(join(bin, name), 0o755);
  };
  stub('caddy', `[ "$CLOUDFLARE_API_TOKEN" = fixture-not-a-token ] || { echo 'token not loaded' >&2; exit 3; }
${fail.validate ? 'exit 1' : 'exit 0'}`);
  stub('systemctl', `case "$1" in
  reload) ${fail.reload ? `grep -q '"new"' "${caddyDir}/Caddyfile" && exit 1` : ':'}; ${fail.restore ? 'exit 1' : 'exit 0'} ;;
  is-active) exit 0 ;;
esac`);
  stub('curl', fail.admin ? `grep -q '"new"' "${caddyDir}/Caddyfile" && exit 7; exit 0` : 'exit 0');
  const result = spawnSync('bash', [script, join(root, 'Caddyfile.new'), join(root, 'Caddyfile.previous'), join(root, 'known')], {
    env: { PATH: `${bin}:${process.env.PATH}`, CADDY_DIR: caddyDir },
    encoding: 'utf8', timeout: 10000,
  });
  return {
    ...result,
    calls: existsSync(calls) ? readFileSync(calls, 'utf8').trim().split('\n') : [],
    live: readFileSync(join(caddyDir, 'Caddyfile'), 'utf8'),
    backups: readdirSync(caddyDir).filter(name => name.startsWith('Caddyfile.bak-')),
  };
}

test('validates with the token, backs up, installs, reloads and checks health', t => {
  const result = deploy(t);
  assert.equal(result.status, 0, result.stderr);
  assert.equal(result.live, NEW);
  assert.equal(result.backups.length, 1);
  assert.match(result.backups[0], /^Caddyfile\.bak-\d{8}T\d{6}Z$/);
  assert.deepEqual(result.calls.map(call => call.split(' ').slice(0, 2).join(' ')),
    ['caddy validate', 'systemctl reload', 'systemctl is-active', 'curl -fsS']);
  assert.match(result.calls.at(-1), /http:\/\/localhost:2019\/config\//);
});

test('an identical live file is a no-op', t => {
  const result = deploy(t, { live: NEW });
  assert.equal(result.status, 0, result.stderr);
  assert.match(result.stdout, /already current/);
  assert.deepEqual(result.calls, []);
  assert.deepEqual(result.backups, []);
});

test('accepts an older repository version, as left by an earlier failed rollout', t => {
  const older = 'older.example {\n}\n';
  const result = deploy(t, { live: older, known: [older, OLD] });
  assert.equal(result.status, 0, result.stderr);
  assert.equal(result.live, NEW);
});

test('a hand-edited live file aborts with the diff and stays untouched', t => {
  const edited = OLD.replace('"old"', '"hotfix"');
  const result = deploy(t, { live: edited });
  assert.equal(result.status, 1);
  assert.match(result.stderr, /edited on the server/);
  assert.match(result.stderr, /-\trespond "old"\n\+\trespond "hotfix"/);
  assert.equal(result.live, edited);
  assert.deepEqual(result.calls, []);
  assert.deepEqual(result.backups, []);
});

test('an invalid config aborts without touching the live file', t => {
  const result = deploy(t, { fail: { validate: true } });
  assert.equal(result.status, 1);
  assert.match(result.stderr, /caddy validate rejected/);
  assert.equal(result.live, OLD);
  assert.deepEqual(result.backups, []);
  assert.ok(!result.calls.some(call => call.startsWith('systemctl')), result.calls.join('\n'));
});

test('a failed reload restores the backup and reloads it', t => {
  const result = deploy(t, { fail: { reload: true } });
  assert.equal(result.status, 10, result.stderr);
  assert.equal(result.live, OLD);
  assert.equal(result.calls.filter(call => call === 'systemctl reload caddy').length, 2);
  assert.match(result.stderr, /Previous Caddyfile restored/);
});

test('a failed health check after reload restores the backup', t => {
  const result = deploy(t, { fail: { admin: true } });
  assert.equal(result.status, 10, result.stderr);
  assert.equal(result.live, OLD);
});

test('reports a failed restore with exit 11', t => {
  const result = deploy(t, { fail: { reload: true, restore: true } });
  assert.equal(result.status, 11, result.stderr);
  assert.equal(result.live, OLD);
  assert.match(result.stderr, /Restoring .* failed/);
});

test('rejects missing inputs before it touches anything', t => {
  const result = spawnSync('bash', [script, '/nonexistent/new', '/nonexistent/previous', '/nonexistent/known'], {
    env: { PATH: process.env.PATH, CADDY_DIR: '/nonexistent' }, encoding: 'utf8',
  });
  assert.equal(result.status, 1);
  assert.match(result.stderr, /missing/);
  assert.equal(deploy(t, { known: [] }).status, 1);
});
