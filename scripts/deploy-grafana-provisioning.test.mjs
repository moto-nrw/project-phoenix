import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { chmodSync, existsSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import test from 'node:test';

const script = new URL('./deploy-grafana-provisioning.sh', import.meta.url).pathname;
const dashboard = 'dashboards/json/phoenix-error-codes.json';

// Runs the script with a fake ssh on PATH. ssh logs the remote
// command and answers it from `remote`: the current file contents, whether
// Grafana becomes healthy, and the Grafana log after the restart.
function deploy(t, args, { remote = {} } = {}) {
  const directory = mkdtempSync(join(tmpdir(), 'moto-grafana-deploy-'));
  t.after(() => rmSync(directory, { recursive: true, force: true }));
  const calls = join(directory, 'calls');
  const files = join(directory, 'remote-files.json');
  writeFileSync(files, JSON.stringify(remote.files ?? {}));
  writeFileSync(join(directory, 'grafana.log'), remote.log ?? '');
  writeFileSync(join(directory, 'ssh'), `#!/usr/bin/env bash
host=$1; shift
printf 'ssh %s %s\\n' "$host" "$*" >> "${calls}"
case "$*" in
  *'cat > '*) cat > "${join(directory, 'uploaded')}" ;;
  *'test -f '*)
    file=\${*##*test -f /root/monitoring/grafana/provisioning/}; file=\${file%% *}
    node -e 'const f=require(process.argv[1]); const v=f[process.argv[2]]; if (v===undefined) process.exit(1); process.stdout.write(v)' "${files}" "$file" ;;
  *'date -u'*) echo 2026-10-23T10:00:00Z ;;
  *'/api/health'*) exit ${remote.unhealthy ? 1 : 0} ;;
  *'docker compose logs'*) cat "${join(directory, 'grafana.log')}" ;;
esac
`);
  chmodSync(join(directory, 'ssh'), 0o755);
  const result = spawnSync('bash', [script, ...args], {
    env: { PATH: `${directory}:${process.env.PATH}` },
    encoding: 'utf8', timeout: 10000,
  });
  const lines = existsSync(calls) ? readFileSync(calls, 'utf8').trim().split('\n') : [];
  const uploadedFile = join(directory, 'uploaded');
  const uploaded = existsSync(uploadedFile) ? readFileSync(uploadedFile, 'utf8') : undefined;
  return { ...result, calls: lines, uploaded };
}

test('copies only the named file, restarts Grafana and waits for health', t => {
  const result = deploy(t, ['monitoring-host', dashboard]);
  assert.equal(result.status, 0, result.stderr);
  const commands = result.calls;
  assert.match(commands.find(c => c.includes('cp -p --parents')), /grafana-provisioning-\d{8}T\d{6}Z/);
  const target = `/root/monitoring/grafana/provisioning/${dashboard}`;
  assert.ok(commands.some(c => c.includes(`cat > ${target}.tmp`) && c.includes(`chmod 644 ${target}.tmp && mv ${target}.tmp ${target}`)), commands.join('\n'));
  assert.equal(result.uploaded, readFileSync(new URL(`../monitoring/grafana/provisioning/${dashboard}`, import.meta.url), 'utf8'));
  const order = ['cp -p --parents', 'cat > ', 'docker compose restart grafana', '/api/health', 'docker compose logs'];
  const positions = order.map(part => commands.findIndex(c => c.includes(part)));
  assert.deepEqual([...positions].sort((a, b) => a - b), positions, commands.join('\n'));
  assert.ok(positions.every(position => position >= 0), commands.join('\n'));
  assert.match(result.stdout, /new file/);
  assert.match(result.stdout, /Grafana is healthy/);
});

test('a dry run shows the diff and changes nothing', t => {
  const result = deploy(t, ['--dry-run', 'monitoring-host', dashboard], {
    remote: { files: { [dashboard]: '{"title":"old"}\n' } },
  });
  assert.equal(result.status, 0, result.stderr);
  assert.match(result.stdout, /-\{"title":"old"\}/);
  assert.ok(!result.calls.some(c => /cat > |restart|cp -p/.test(c)), result.calls.join('\n'));
});

test('rejects input before it touches the host', t => {
  for (const args of [
    ['monitoring-host'],
    ['monitoring-host', '../../.env'],
    ['monitoring-host', 'dashboards/json/missing.json'],
    ['bad host;rm', dashboard],
  ]) {
    const result = deploy(t, args);
    assert.equal(result.status, 1, args.join(' '));
    assert.deepEqual(result.calls, [], args.join(' '));
  }
});

test('fails with the restore command when provisioning logs an error', t => {
  const result = deploy(t, ['monitoring-host', dashboard], {
    remote: { log: 'logger=provisioning.dashboard level=error msg="failed to load dashboard"\nlogger=plugins level=error msg="unrelated"\n' },
  });
  assert.equal(result.status, 1);
  assert.match(result.stderr, /failed to load dashboard/);
  assert.doesNotMatch(result.stderr, /unrelated/);
  assert.match(result.stderr, /Restore: ssh monitoring-host .*grafana-provisioning-/);
});

test('accepts a missing optional provisioning directory', t => {
  const result = deploy(t, ['monitoring-host', dashboard], {
    remote: { log: 'logger=provisioning.plugins level=error msg="Failed to read plugin provisioning files from directory" path=/etc/grafana/provisioning/plugins error="open /etc/grafana/provisioning/plugins: no such file or directory"\n' },
  });
  assert.equal(result.status, 0, result.stderr);
});

test('fails when Grafana does not become healthy', t => {
  const result = deploy(t, ['monitoring-host', dashboard], { remote: { unhealthy: true } });
  assert.equal(result.status, 1);
  assert.match(result.stderr, /not healthy/);
});
