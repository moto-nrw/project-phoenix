import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { copyFileSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import test from 'node:test';

const script = new URL('./worktree-teardown.sh', import.meta.url).pathname;
const wtconfig = new URL('../.wtconfig', import.meta.url).pathname;
const project = 'wt-fix-1-moto-nrw-project-phoenix';

// Docker state as "kind name compose-project" lines: this worktree's project,
// the main checkout's stack, the shared test database server and caches with
// a fixed name. The fake docker filters by project label like the real one,
// deletes removed lines and logs every call.
const state = [
  `container ${project}-postgres-1 ${project}`,
  `container ${project}-server-1 ${project}`,
  `container ${project}-postgres-test-1 ${project}`,
  `network ${project}_default ${project}`,
  `volume ${project}_postgres ${project}`,
  `volume ${project}_postgres_test ${project}`,
  `volume phoenix_go_build_cache ${project}`,
  `network phoenix-fixed ${project}`,
  'container project-phoenix-postgres-1 project-phoenix',
  'network project-phoenix_default project-phoenix',
  'volume project-phoenix_postgres project-phoenix',
  'container project-phoenix-testdb-5433-postgres-test-1 project-phoenix-testdb-5433',
  'network project-phoenix-testdb-5433_default project-phoenix-testdb-5433',
  'volume project-phoenix-testdb-5433_postgres_test project-phoenix-testdb-5433',
  'volume phoenix_go_mod_cache project-phoenix',
  '',
].join('\n');

const fakeDocker = `#!/bin/sh
echo "$*" >>"$DOCKER_LOG"
case "$1 $2" in
  "ps -a") kind=container; filter=$4 ;;
  "network ls"|"volume ls") kind=$1; filter=$4 ;;
  "rm --force") shift 3; for n in "$@"; do grep -v "^container $n " "$DOCKER_STATE" >"$DOCKER_STATE.next"; mv "$DOCKER_STATE.next" "$DOCKER_STATE"; done; exit 0 ;;
  "network rm"|"volume rm") grep -v "^$1 $3 " "$DOCKER_STATE" >"$DOCKER_STATE.next"; mv "$DOCKER_STATE.next" "$DOCKER_STATE"; exit 0 ;;
  *) echo "unexpected docker call: $*" >&2; exit 9 ;;
esac
awk -v kind="$kind" -v project="\${filter#label=com.docker.compose.project=}" '$1 == kind && $3 == project { print $2 }' "$DOCKER_STATE"
`;

function fixture(t, wtEnv = `COMPOSE_PROJECT_NAME=${project}\nFRONTEND_HOST_PORT=3010\n`) {
  const base = mkdtempSync(join(tmpdir(), 'phoenix-worktree-teardown-'));
  t.after(() => rmSync(base, { recursive: true, force: true }));
  const root = join(base, 'worktree');
  const bin = join(base, 'bin');
  mkdirSync(join(root, 'scripts'), { recursive: true });
  mkdirSync(bin);
  copyFileSync(script, join(root, 'scripts/worktree-teardown.sh'));
  if (wtEnv !== null) writeFileSync(join(root, '.wt.env'), wtEnv);
  writeFileSync(join(root, '.env'), 'COMPOSE_PROJECT_NAME=project-phoenix\nTEST_DB_PORT=5433\n');
  writeFileSync(join(bin, 'docker'), fakeDocker, { mode: 0o755 });
  const files = { DOCKER_STATE: join(base, 'state'), DOCKER_LOG: join(base, 'log') };
  writeFileSync(files.DOCKER_STATE, state);
  writeFileSync(files.DOCKER_LOG, '');
  const run = (...args) => spawnSync('bash', ['scripts/worktree-teardown.sh', ...args], {
    cwd: root, encoding: 'utf8', env: { ...process.env, ...files, PATH: `${bin}:${process.env.PATH}` },
  });
  const remaining = () => readFileSync(files.DOCKER_STATE, 'utf8').split('\n').filter(Boolean);
  const calls = () => readFileSync(files.DOCKER_LOG, 'utf8').split('\n').filter(Boolean);
  return { run, remaining, calls };
}

const foreign = state.split('\n').filter(line => line && !line.includes(`${project}-`) && !line.includes(`${project}_`));

test('removes only resources Compose created for the worktree project', t => {
  const f = fixture(t);
  const result = f.run();
  assert.equal(result.status, 0, result.stderr);
  assert.deepEqual(f.remaining(), foreign);
  assert.ok(f.remaining().includes(`volume phoenix_go_build_cache ${project}`), 'fixed-name caches stay');
  assert.ok(f.remaining().includes(`network phoenix-fixed ${project}`), 'fixed-name networks stay');
  assert.ok(f.remaining().includes('volume project-phoenix-testdb-5433_postgres_test project-phoenix-testdb-5433'), 'the shared test database stays');
  for (const call of f.calls().filter(c => / rm /.test(` ${c} `))) {
    assert.ok(call.includes(project), `foreign removal: ${call}`);
  }
});

test('a dry run lists the removals and changes nothing', t => {
  const f = fixture(t);
  const result = f.run('--dry-run');
  assert.equal(result.status, 0, result.stderr);
  assert.equal(result.stdout, [
    `would remove container ${project}-postgres-1`,
    `would remove container ${project}-server-1`,
    `would remove container ${project}-postgres-test-1`,
    `would remove network ${project}_default`,
    `would remove volume ${project}_postgres`,
    `would remove volume ${project}_postgres_test`,
    '',
  ].join('\n'));
  assert.deepEqual(f.remaining(), state.split('\n').filter(Boolean));
});

test('without a wt-assigned project nothing is touched', t => {
  const f = fixture(t, null);
  const result = f.run();
  assert.equal(result.status, 0, result.stderr);
  assert.match(result.stderr, /no COMPOSE_PROJECT_NAME in \.wt\.env; nothing removed/);
  assert.deepEqual(f.calls(), []);
});

test('a shared test database project or an invalid name is refused', t => {
  for (const name of ['project-phoenix-testdb-5433', 'Bad Name', 'x;rm']) {
    const f = fixture(t, `COMPOSE_PROJECT_NAME=${name}\n`);
    const result = f.run();
    assert.equal(result.status, 1, `${name}: ${result.stderr}`);
    assert.deepEqual(f.calls(), [], name);
  }
});

test('wt leaves the shared test database to the test harness', () => {
  const config = readFileSync(wtconfig, 'utf8');
  assert.doesNotMatch(config, /^\s*port\s*=\s*"?TEST_DB_PORT\b/m, 'TEST_DB_PORT is not a worktree lease');
  const teardowns = [...config.matchAll(/^\s*teardown\s*=\s*(.*)$/gm)].map(m => m[1].trim());
  assert.deepEqual(teardowns, ['scripts/worktree-teardown.sh']);
});
