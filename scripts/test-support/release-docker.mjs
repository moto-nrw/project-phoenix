#!/usr/bin/env node
// Deterministic Docker boundary for release shell failure-path tests.
import { appendFileSync, writeFileSync, readFileSync } from 'node:fs';
const args = process.argv.slice(2);
const call = args.join(' ');
appendFileSync(process.env.RELEASE_TEST_LOG, `${call}\n`);
const contractMount = args.find(value => value.endsWith(':/run/student-contract-evidence.json:ro'));
if (contractMount) {
  const source = contractMount.slice(0, -':/run/student-contract-evidence.json:ro'.length);
  appendFileSync(process.env.RELEASE_TEST_LOG, `contract-evidence-content ${JSON.stringify(readFileSync(source, 'utf8'))}\n`);
  if (call.includes('migrate preflight') && process.env.RELEASE_TEST_MUTATE_EVIDENCE) {
    writeFileSync(process.env.RELEASE_TEST_MUTATE_EVIDENCE, 'changed after preflight');
  }
}
const fail = process.env.RELEASE_TEST_FAIL;
const isCompose = args[0] === 'compose';
const input = isCompose && args.includes('psql') && !args.includes('-c') ? readFileSync(0, 'utf8') : '';
const exit = () => process.exit(9);
if ((fail === 'stop' && isCompose && args.includes('stop')) ||
    (fail === 'pull' && (args[0] === 'pull' || args.includes('pull'))) ||
    // The preflight and the migration are both `compose run migrate`; only the
    // trailing command separates them, so each failure mode names its own.
    (fail === 'migrate' && isCompose && args.includes('run') && !call.includes('migrate preflight')) ||
    (fail === 'preflight' && isCompose && args.includes('run') && call.includes('migrate preflight')) ||
    (fail === 'health' && isCompose && args.includes('up') && args.includes('frontend')) ||
    (fail === 'roles' && isCompose && args.includes('pg_dumpall')) ||
    (fail === 'restore-db' && isCompose && args.includes('pg_restore')) ||
    (fail === 'restore-roles' && input.includes('CREATE ROLE')) ||
    (fail === 'unavailable-image' && (args[0] === 'pull' || (args[0] === 'image' && !args.includes('--format')))) ||
    (fail === 'archive' && args.includes('-tzf')) ||
    (fail === 'restore-uploads' && args.includes('-c'))) exit();
if (isCompose && args.includes('ps')) console.log(args.at(-1));
else if (args[0] === 'inspect') {
  const format = args[2];
  console.log(format.includes('.State.Running') ? (fail === 'running' ? 'true' : 'false') :
    format.includes('.Mounts') ? 'test-uploads' : `sha256:${'1'.repeat(64)}`);
} else if (args[0] === 'image' && args.includes('--format')) console.log(`postgres@sha256:${'1'.repeat(64)}`);
else if (isCompose && args.includes('--services')) console.log('postgres\nserver\nfrontend\nmigrate');
else if (isCompose && args.includes('--no-interpolate')) console.log('services:\n  postgres:\n    image: postgres:test');
else if (isCompose && args.includes('pg_dumpall')) console.log('CREATE ROLE phoenix_auth;');
else if (isCompose && args.includes('pg_dump')) console.log('database snapshot');
else if (isCompose && call.includes('SELECT (SELECT count(*)')) console.log('0');
else if (args.includes('-czf')) {
  const bind = args.find(value => value.startsWith('type=bind,src='));
  const directory = bind.split(',')[1].slice(4);
  writeFileSync(`${directory}/uploads.tar.gz`, 'uploads snapshot');
}
