#!/usr/bin/env node
// Deterministic Docker boundary for release shell failure-path tests.
import { appendFileSync, writeFileSync, readFileSync, existsSync } from 'node:fs';
const args = process.argv.slice(2);
const call = args.join(' ');
appendFileSync(process.env.RELEASE_TEST_LOG, `${call}\n`);
const fail = process.env.RELEASE_TEST_FAIL;
const isCompose = args[0] === 'compose';
const input = isCompose && args.includes('psql') && !args.includes('-c') ? readFileSync(0, 'utf8') : '';
const exit = () => process.exit(9);
// The stack in the working directory decides whether the sidecar exists.
const sidecar = existsSync('docker-compose.yml') && readFileSync('docker-compose.yml', 'utf8').includes('demo-runtime');
if ((fail === 'stop' && isCompose && args.includes('stop')) ||
    (fail === 'pull' && (args[0] === 'pull' || args.includes('pull'))) ||
    // The preflight and the migration are both `compose run migrate`; only the
    // trailing command separates them, so each failure mode names its own.
    (fail === 'migrate' && isCompose && args.includes('run') && !call.includes('migrate preflight')) ||
    (fail === 'preflight' && isCompose && args.includes('run') && call.includes('migrate preflight')) ||
    (fail === 'health' && isCompose && args.includes('up') && args.includes('frontend')) ||
    (fail === 'sidecar' && isCompose && args.includes('up') && args.includes('demo-runtime')) ||
    (fail === 'roles' && isCompose && args.includes('pg_dumpall')) ||
    (fail === 'restore-db' && isCompose && args.includes('pg_restore')) ||
    (fail === 'restore-roles' && input.includes('CREATE ROLE')) ||
    (fail === 'unavailable-image' && (args[0] === 'pull' || (args[0] === 'image' && !args.includes('--format')))) ||
    (fail === 'archive' && args.includes('-tzf')) ||
    (fail === 'restore-uploads' && args.includes('-c'))) exit();
if (isCompose && args.includes('ps')) console.log(args.at(-1));
else if (args[0] === 'inspect') {
  const format = args[2];
  console.log(format.includes('.State.Running') ? (fail === 'running' || (fail === 'running-sidecar' && args.at(-1) === 'demo-runtime') ? 'true' : 'false') :
    format.includes('.Mounts') ? 'test-uploads' : `sha256:${'1'.repeat(64)}`);
} else if (args[0] === 'image' && args.includes('--format')) console.log(`postgres@sha256:${'1'.repeat(64)}`);
else if (isCompose && args.includes('--services')) console.log(`postgres\nserver\nfrontend\nmigrate${sidecar ? '\ndemo-runtime' : ''}`);
else if (isCompose && args.includes('--no-interpolate')) console.log(`services:\n  postgres:\n    image: postgres:test${sidecar ? '\n  demo-runtime:\n    image: server:test' : ''}`);
else if (isCompose && args.includes('pg_dumpall')) console.log('CREATE ROLE phoenix_auth;');
else if (isCompose && args.includes('pg_dump')) console.log('database snapshot');
else if (isCompose && call.includes('SELECT (SELECT count(*)')) console.log('0');
else if (args.includes('-czf')) {
  const bind = args.find(value => value.startsWith('type=bind,src='));
  const directory = bind.split(',')[1].slice(4);
  writeFileSync(`${directory}/uploads.tar.gz`, 'uploads snapshot');
}
