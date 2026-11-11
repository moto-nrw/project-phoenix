import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { existsSync, mkdirSync, mkdtempSync, readdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import test from 'node:test';

import { architectureInputChanged, concurrencyFor, inputsKey, runSteps, stepsFor, summarize, verifyRate } from './check-changed.mjs';
import { gitTestEnvironment, initGitFixture } from './test-helpers/git-fixture.mjs';
import { changedPaths, sourceRatchets, sourceRatchetPattern } from './backend-source-ratchets.mjs';

// Preserve declaration boundaries while ignoring Go snippets in scanner fixtures.
function goCode(source) {
  return source.replace(/\/\*[\s\S]*?\*\/|\/\/[^\n]*|`[^`]*`|"(?:\\.|[^"\\])*"|'(?:\\.|[^'\\])*'/g,
    token => token.replace(/[^\n]/g, ' '));
}

function unassignedSourceTests(files, selected, exceptions = []) {
  const assigned = new Set(selected.flatMap(rule => rule.tests.map(name => `${rule.file}:${name}`)));
  const mappedFiles = new Set(selected.map(rule => rule.file));
  const excluded = new Map(exceptions.map(({ file, name, reason }) => [`${file}:${name}`, reason]));
  const seen = new Set();
  const problems = [];
  for (const [file, source] of files) {
    const code = goCode(source);
    // Include direct filesystem/parser access and the shared repository readers.
    // Mapped files stay guarded even if a new check calls an opaque helper.
    const readsSource = /\b(?:findBackendRoot|readGoSource|parseGoSourceCached|goSourceIndex|walkGoFiles\w*|walkModuleRatchetSources|productionGoSources)\b|\.\s*(?:ReadFile|ReadDir|Open|OpenFile|Walk|WalkDir|Glob|ParseFile|ParseDir)\b/.test(code);
    if (!readsSource && !mappedFiles.has(file)) continue;
    for (const [, name] of code.matchAll(/\bfunc\s+(Test(?!\p{Ll})[\p{L}\p{N}_]*)\s*\(/gu)) {
      const key = `${file}:${name}`;
      seen.add(key);
      if (!assigned.has(key) && !excluded.get(key)?.trim()) problems.push(`inventory the repository inputs of ${key}`);
      if (assigned.has(key) && excluded.has(key)) problems.push(`selected test also excluded: ${key}`);
    }
  }
  for (const key of assigned) if (!seen.has(key)) problems.push(`stale source-test selection: ${key}`);
  for (const key of excluded.keys()) if (!seen.has(key)) problems.push(`stale source-test exception: ${key}`);
  return problems;
}

const fixtureExceptions = (file, names, reason = 'Synthetic temporary fixtures only; no repository input.') =>
  names.map(name => ({ file, name, reason }));
const sourceTestExceptions = [
  ...fixtureExceptions('calendar_date_alias_test.go', [
    'TestCalendarDateAliasTypeIdentity',
    'TestCanonicalCalendarDateTypeIdentity',
  ]),
  ...fixtureExceptions('calendar_fixture_ratchet_test.go', [
    'TestCalendarFixtureRatchetDetectsEnrollmentPattern',
    'TestCalendarFixtureRatchetDetectsWorkSessionPattern',
    'TestCalendarFixtureRatchetDetectsLiveDateRange',
    'TestCalendarFixtureRatchetDetectsLiveEffectiveBoundaries',
    'TestCalendarFixtureRatchetFollowsLiveDateHelperIntoAssertion',
    'TestCalendarFixtureRatchetRequiresReviewedExceptionReason',
    'TestCalendarFixtureRatchetIgnoresFixedAndNonCodePatterns',
    'TestCalendarFixtureRatchetIgnoresUnixNanoUsedForFixtureUniqueness',
    'TestCalendarFixtureRatchetDetectsUnixNanoUsedForCalendarOffset',
    'TestCalendarFixtureRatchetDetectsInstantReconstructedFromLiveUnixScalar',
    'TestCalendarFixtureRatchetDetectsReconstructedInstantInWeeklyFixture',
    'TestCalendarFixtureRatchetFollowsUnixScalarHelpersAcrossFiles',
    'TestCalendarFixtureRatchetKeepsScopesAndNowMethodsSeparate',
    'TestCalendarFixtureRatchetQualifiesNowMethodsByReceiver',
  ]),
  ...fixtureExceptions('composition_inventory_serialization_test.go', [
    'TestWriteCompositionInventoryUsesCanonicalTopLevelOrder',
    'TestReplaceCompositionFieldsPreservesUnownedFields',
    'TestReplaceCompositionFieldsLeavesInvalidManifestUntouched',
  ]),
  ...fixtureExceptions('durable_side_effects_ratchet_test.go', [
    'TestSideEffectScannerFindsNestedAndPackageLevelOperations',
    'TestSideEffectNoticeRegisterRejectsEqualSizedReplacements',
  ]),
  ...fixtureExceptions('gdpr_log_pii_ratchet_test.go', [
    'TestGDPRLogPIIRatchetCoversExplicitLevelsAndFullNames',
  ]),
  ...fixtureExceptions('helper_consolidation_ratchet_test.go', [
    'TestHandRolledParseIntScanCoversHandlerPackages',
  ]),
  ...fixtureExceptions('hermetic_verification_test.go', [
    'TestParallelGlobalStateGateFindsDirectMutation',
    'TestHermeticSetupScanIgnoresProseButRequiresRealSetup',
    'TestIgnoredCleanupDeletesFindsDiscardedDeletesInCleanupCode',
  ]),
  ...fixtureExceptions('isolated_subtest_scanner_test.go', [
    'TestIsolatedParallelSubtestScanner',
  ]),
  ...fixtureExceptions('http_error_identity_ratchet_test.go', [
    'TestHTTPErrorTextComparisons',
  ]),
  ...fixtureExceptions('module_compose_alias_ratchet_test.go', [
    'TestModuleComposeAliasScanCountsForeignAliasesOnly',
  ]),
  ...fixtureExceptions('module_contract_inventory_test.go', [
    'TestModuleContractInventoryRejectsStaleOrPrivateEvidence',
    'TestModuleContractInventoryAcceptsObservablePublicEvidence',
  ]),
  ...fixtureExceptions('module_facade_method_set_scanner_test.go', [
    'TestModuleFacadeMethodSetCatchesCompositeGrowth',
    'TestModuleFacadeMethodSetRejectsUnresolvedEmbeddings',
  ]),
  ...fixtureExceptions('module_source_ratchet_test.go', [
    'TestModuleRootSourceRatchetsRejectNewViolations',
  ]),
  ...fixtureExceptions('permission_catalog_test.go', [
    'TestPermissionCatalogMatchesMigratedDatabase',
  ], 'Runtime database/schema comparison; covered by the full affected suite.'),
  ...fixtureExceptions('production_source_scan_test.go', [
    'TestHandlerScannersUseProductionBuildNotHookFileNames',
  ]),
  ...fixtureExceptions('register_git_contract_test.go', [
    'TestRegisterRatchetsRejectBranchGrowthAfterTargetAdvance',
    'TestRegisterRatchetsMergePreservesTargetRetirements',
    'TestRegisterRatchetsMissingHistoryFailsClosed',
    'TestRegisterMergeBasePinsTargetBeforeRefMoves',
    'TestRegisterRatchetsRelocationsStillRequireOriginalIdentity',
    'TestRegisterRatchetsAllowRetirement',
  ], 'Synthetic Git repositories; no live repository input.'),
  ...fixtureExceptions('school_class_label_ratchet_test.go', [
    'TestSchoolClassLabelScannerPattern',
  ]),
  ...fixtureExceptions('source_index_test.go', [
    'TestGoSourceIndexReportsWalkErrors',
    'TestParseGoSourceCachedReparsesChangedContent',
  ]),
  ...fixtureExceptions('test_skip_ratchet_test.go', [
    'TestTestSkipRatchetFindsSkipsOnTestHandlesOnly',
  ]),
  ...fixtureExceptions('traeger_view_scope_ratchet_test.go', [
    'TestTraegerViewScopeRatchetRejectsUnfilteredQueries',
  ]),
  ...fixtureExceptions('transaction_boundaries_ratchet_test.go', [
    'TestTransactionScannerIncludesAliasesAndNestedStarts',
    'TestTransactionScannerIncludesRestrictedDemoScope',
    'TestTransactionScannerIncludesAdministrativeMigrationCalls',
  ]),
  ...fixtureExceptions('trigger_ownership_test.go', [
    'TestDatabaseTriggerOwnership',
  ], 'Runtime database/schema comparison; covered by the full affected suite.'),
  ...fixtureExceptions('trigger_ownership_test.go', [
    'TestTriggerOwnershipFollowsFunctionsAndRejectsDynamicWrites',
  ], 'Synthetic SQL routine bodies only; no repository input.'),
  ...fixtureExceptions('wall_clock_verification_test.go', [
    'TestActivityInstanceWallClockRatchetDetectsAliasedInstantSources',
  ]),
];

test('source inventory rejects an unmapped function added to an inventoried file', () => {
  const selected = [{ file: 'known_test.go', tests: ['TestKnown'] }];
  const source = 'package test\nfunc TestKnown(t *testing.T) { findBackendRoot() }\n';
  assert.deepEqual(unassignedSourceTests(new Map([['known_test.go', source]]), selected), []);
  const added = source + 'func TestNewSourceCheck(t *testing.T) { os.ReadFile("../modules/new.go") }\n';
  assert.deepEqual(unassignedSourceTests(new Map([['known_test.go', added]]), selected),
    ['inventory the repository inputs of known_test.go:TestNewSourceCheck']);
  const helper = source + 'func TestThroughHelper(t *testing.T) { scanOpaqueInputs() }\n';
  assert.deepEqual(unassignedSourceTests(new Map([['known_test.go', helper]]), selected),
    ['inventory the repository inputs of known_test.go:TestThroughHelper']);
  const unicode = source + 'func TestÄnderung(t *testing.T) { scanOpaqueInputs() }\n';
  assert.deepEqual(unassignedSourceTests(new Map([['known_test.go', unicode]]), selected),
    ['inventory the repository inputs of known_test.go:TestÄnderung']);
  const direct = 'package test\nfunc TestDirect(t *testing.T) { os.ReadFile("../modules/new.go") }\n';
  assert.deepEqual(unassignedSourceTests(new Map([['direct_test.go', direct]]), []),
    ['inventory the repository inputs of direct_test.go:TestDirect']);
  const fixture = source + '\nvar fixture = `\nfunc TestFake(t *testing.T) { os.ReadFile("fake.go") }\n`\n' +
    '// func TestComment(t *testing.T) {}\n/* func TestBlock(t *testing.T) {} */\n' +
    'var quoted = "func TestQuoted(t *testing.T) {}"\n';
  assert.deepEqual(unassignedSourceTests(new Map([['known_test.go', fixture]]), selected), []);
  assert.deepEqual(unassignedSourceTests(new Map([['known_test.go', 'var fixture = `func TestKnown(t *testing.T) {}`']]), selected),
    ['stale source-test selection: known_test.go:TestKnown']);
  const exceptions = [{ file: 'direct_test.go', name: 'TestDirect', reason: 'Synthetic temporary fixture only.' }];
  assert.deepEqual(unassignedSourceTests(new Map([['direct_test.go', direct]]), [], exceptions), []);
  assert.match(unassignedSourceTests(new Map([['direct_test.go', direct]]), [], [{ ...exceptions[0], reason: '' }])[0], /inventory/);
  assert.match(unassignedSourceTests(new Map(), [], exceptions)[0], /stale source-test exception/);
});

test('source ratchets follow filesystem inputs, not imports or test-name substrings', () => {
  const selected = (...paths) => new RegExp(sourceRatchetPattern(paths) || '^$');
  const production = selected('backend/modules/example/internal/application/new.go');
  for (const name of ['TestTransactionBoundaryRatchet', 'TestModuleFacadeMethodSetRatchet', 'TestHermeticTestPatterns', 'TestTenantFiltersRequireExplicitScope']) {
    assert.match(name, production);
  }
  const tests = selected('backend/modules/example/internal/application/new_test.go');
  assert.match('TestHermeticTestPatterns', tests);
  assert.match('TestCalendarFixtureClockRatchet', tests);
  assert.match('TestQueryBudgetRatchet', tests);
  assert.doesNotMatch('TestModuleFileSizeRatchet', tests);
  assert.doesNotMatch('TestTenantIsolation_StudentVisibility', production);
  assert.doesNotMatch('TestTransactionBoundaryRatchetExtra', production, 'the Go -run filter is anchored');
  assert.equal(sourceRatchetPattern(['frontend/src/page.tsx', 'backend/modules/example/README.md', 'backend/architecture/WORKPLAN.md']), '');
  assert.equal(sourceRatchetPattern([]), '');
});

test('source ratchets include data, policy, manifests and configuration read outside Go imports', () => {
  const cases = [
    ['backend/test/database_slots.go', 'TestIsolatedDatabaseSlotsBreakParallelSubtestSaturation'],
    ['backend/test/database_scope.go', 'TestIsolatedDatabaseSlotsBoundWaitAndRelease'],
    ['backend/architecture/policy/new-owner.json', 'TestTransactionBoundaryRatchet'],
    ['backend/architecture/composition.json', 'TestCompositionInventory'],
    ['backend/architecture/passthrough/reviews/owner.json', 'TestMethodPassthroughRatchet'],
    ['backend/test/transaction_boundaries.json', 'TestTransactionBoundaryRatchet'],
    ['scripts/backend-architecture.sh', 'TestTransactionBoundaryRatchet'],
    ['scripts/backend-architecture/relocations.go', 'TestTransactionBoundaryRatchet'],
    ['scripts/backend-architecture/testdata/relocations/input.go', 'TestTransactionBoundaryRatchet'],
    ['backend/test/durable_side_effect_notices.json', 'TestDurableSideEffectsRatchet'],
    ['backend/test/module_contract_inventory.json', 'TestModuleContractInventoryRatchet'],
    ['backend/test/query-budgets/new.json', 'TestQueryBudgetRatchet'],
    ['backend/auth/authorize/permissions/catalog.json', 'TestPermissionCatalogNamesAreUnique'],
    ['backend/tools/go.mod', 'TestToolchainPinsRatchet'],
    ['devbox.lock', 'TestToolchainPinsRatchet'],
    ['error-registry/new.json', 'TestErrorCodeGenerationRatchetsAllowAdditionalCode'],
  ];
  for (const [path, name] of cases) {
    const pattern = sourceRatchetPattern([path]);
    assert.ok(pattern, `no source check selected for ${path}`);
    assert.match(name, new RegExp(pattern), path);
  }
  const files = new Map(readdirSync(new URL('../backend/test/', import.meta.url))
    .filter(file => file.endsWith('_test.go'))
    .map(file => [file, readFileSync(new URL(`../backend/test/${file}`, import.meta.url), 'utf8')]));
  assert.deepEqual(unassignedSourceTests(files, sourceRatchets, sourceTestExceptions), []);
});

test('source input discovery keeps old and new rename paths, deletions, staged and untracked files', t => {
  const root = mkdtempSync(join(tmpdir(), 'phoenix-ratchet-inputs-'));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  const env = gitTestEnvironment();
  const git = (...args) => {
    const result = spawnSync('git', args, { cwd: root, env, encoding: 'utf8' });
    assert.equal(result.status, 0, result.stderr);
    return result.stdout.trim();
  };
  initGitFixture(git);
  mkdirSync(join(root, 'backend'), { recursive: true });
  for (const name of ['changed.go', 'deleted.go', 'renamed.go', 'staged.go']) writeFileSync(join(root, 'backend', name), 'package example\n');
  git('add', '.');
  git('commit', '-qm', 'base');
  const base = git('rev-parse', 'HEAD');
  git('mv', 'backend/renamed.go', 'outside.go');
  git('rm', '-q', 'backend/deleted.go');
  writeFileSync(join(root, 'backend/staged.go'), '// staged\npackage example\n');
  git('add', 'backend/staged.go');
  writeFileSync(join(root, 'backend/changed.go'), '// dirty\npackage example\n');
  writeFileSync(join(root, 'backend/new file.go'), 'package example\n');
  writeFileSync(join(root, '.gitignore'), 'ignored.go\n');
  writeFileSync(join(root, 'ignored.go'), 'package ignored\n');
  const paths = changedPaths(root, base, env);
  assert.deepEqual(paths.filter(path => path.endsWith('.go')), ['backend/changed.go', 'backend/deleted.go', 'backend/new file.go', 'backend/renamed.go', 'backend/staged.go', 'outside.go']);
  git('add', '.');
  git('commit', '-qm', 'branch changes');
  assert.deepEqual(changedPaths(root, base, env), paths, 'committed and uncommitted inputs have the same selection');
  assert.throws(() => changedPaths(root, 'missing-ref', env), /git merge-base/);
});

test('the architecture check follows the paths that start CI\'s architecture ratchet', () => {
  for (const path of ['backend/api/parent_meal_message_write_rig_test.go', 'backend/modules/example/internal/application/new.go',
    'backend/go.mod', 'backend/go.sum', 'backend/architecture/policy/new-owner.json', 'backend/architecture/legacy.jsonl', 'backend/internal/architecture/testdata/policy.json',
    'scripts/backend-architecture.sh', 'scripts/backend-architecture/main.go']) {
    assert.equal(architectureInputChanged([path]), true, path);
  }
  for (const path of ['frontend/src/page.tsx', 'backend/architecture/WORKPLAN.md', 'backend/modules/example/README.md',
    'backend/dev.env', 'scripts/backend-architecture.shx', 'docs/agents/operations.md']) {
    assert.equal(architectureInputChanged([path]), false, path);
  }
  assert.equal(architectureInputChanged([]), false);
});

test('the architecture step runs the existing checker on the working tree, only for its inputs', () => {
  const names = steps => steps.map(step => step.name);
  assert.deepEqual(names(stepsFor('/repo', 'origin/development', ['frontend/src/page.tsx'])), ['static', 'tests']);
  const steps = stepsFor('/repo', 'origin/development', ['backend/api/untracked_test.go']);
  assert.deepEqual(names(steps), ['static', 'arch', 'tests']);
  assert.deepEqual(steps[1].command, ['bash', 'scripts/backend-architecture.sh', 'check', '--baseline', '/repo/backend/architecture/legacy.jsonl']);
});

test('steps run side by side only on a machine with room', () => {
  assert.equal(concurrencyFor(16, 64 * 1024 ** 3, 3), 2);
  assert.equal(concurrencyFor(4, 64 * 1024 ** 3, 0), 1);
  assert.equal(concurrencyFor(16, 8 * 1024 ** 3, 0), 1);
  assert.equal(concurrencyFor(16, 64 * 1024 ** 3, 12), 1, 'other agents already load the machine');
});

test('the cache verification rate defaults to 5 % and rejects nonsense', () => {
  assert.equal(verifyRate({}), 0.05);
  assert.equal(verifyRate({ PHX_CACHE_VERIFY_RATE: '0' }), 0);
  assert.equal(verifyRate({ PHX_CACHE_VERIFY_RATE: '1' }), 1);
  assert.throws(() => verifyRate({ PHX_CACHE_VERIFY_RATE: 'often' }), /from 0 to 1/);
});

test('steps print whole blocks, report failures and stop a step past its timeout with its children', async () => {
  const dir = mkdtempSync(join(tmpdir(), 'phoenix-check-changed-'));
  try {
    let printed = '';
    const results = await runSteps([
      { name: 'fail', command: ['sh', '-c', 'echo first; sleep 0.2; echo second; exit 3'] },
      { name: 'slow', command: ['sh', '-c', `sh -c 'sleep 5; touch ${join(dir, 'survived')}' & wait`] },
      { name: 'ok', command: ['sh', '-c', 'echo fine'] },
    ], { concurrency: 2, timeoutMs: 1000, cwd: dir, write: text => { printed += text; } });
    assert.deepEqual(results.map(result => result.status), ['FAILED', 'TIMEOUT', 'ok']);
    assert.equal(results[0].code, 3);
    assert.match(printed, /===== fail: FAILED \([\d.]+s\) =====\nfirst\nsecond\n/);
    assert.match(summarize(results), /^FAILED  fail/m);
    await new Promise(done => setTimeout(done, 5500));
    assert.equal(spawnSync('test', ['-e', join(dir, 'survived')]).status, 1, 'the timed-out step left a child running');
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
});

test('the inputs key follows untracked files and the base, not the order of runs', t => {
  const root = mkdtempSync(join(tmpdir(), 'phoenix-check-changed-key-'));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  const env = gitTestEnvironment();
  const git = (...args) => {
    const result = spawnSync('git', args, { cwd: root, env, encoding: 'utf8' });
    assert.equal(result.status, 0, result.stderr);
    return result.stdout.trim();
  };
  initGitFixture(git);
  writeFileSync(join(root, 'a.txt'), 'a\n');
  git('add', '.');
  git('commit', '-qm', 'base');
  const base = git('rev-parse', 'HEAD');
  const first = inputsKey(root, base, env);
  assert.equal(inputsKey(root, base, env), first);
  writeFileSync(join(root, 'untracked.go'), 'package x\n');
  const withUntracked = inputsKey(root, base, env);
  assert.notEqual(withUntracked, first);
  assert.notEqual(inputsKey(root, 'other-base', env), withUntracked);
  // Same final tree, different history: a credential committed and removed.
  writeFileSync(join(root, 'a.txt'), 'TOKEN=committed-then-removed\n');
  git('commit', '-qam', 'add');
  writeFileSync(join(root, 'a.txt'), 'a\n');
  git('commit', '-qam', 'remove');
  assert.notEqual(inputsKey(root, base, env), withUntracked, 'branch history is an input of the secret scan');
  git('reset', '-q', '--hard', base);
  assert.equal(git('status', '--porcelain'), '?? untracked.go', 'the key leaves the real index alone');
  assert.notEqual(inputsKey(root, base, { ...env, GOFLAGS: '-p=1' }), withUntracked, 'Go flags change outcomes');
  assert.notEqual(inputsKey(root, base, { ...env, PHX_PREPUSH: 'skip' }), withUntracked, 'a skipped gate is another input');
  assert.equal(inputsKey(root, base, { ...env, PHX_CACHE_VERIFY_RATE: '1' }), withUntracked, 'the sampling rate is no input');
  for (const name of ['GOAMD64', 'GOARM64', 'GO386']) {
    assert.notEqual(inputsKey(root, base, { ...env, [name]: 'v3' }), withUntracked, `${name} changes the build`);
  }
  mkdirSync(join(root, 'frontend/.next/types'), { recursive: true });
  writeFileSync(join(root, '.gitignore'), 'frontend/.next/\n');
  const ignoredBase = inputsKey(root, base, env);
  writeFileSync(join(root, 'frontend/.next/types/routes.d.ts'), 'export type Broken = ;\n');
  assert.notEqual(inputsKey(root, base, env), ignoredBase, 'ignored generated route types feed the typecheck');
});

test('Ctrl-C reaches the running step\'s process group and no further step starts', async () => {
  const dir = mkdtempSync(join(tmpdir(), 'phoenix-check-changed-int-'));
  try {
    setTimeout(() => process.emit('SIGINT', 'SIGINT'), 500);
    const results = await runSteps([
      { name: 'long', command: ['sh', '-c', `sh -c 'sleep 3; touch ${join(dir, 'survived')}' & wait`] },
      { name: 'later', command: ['sh', '-c', `touch ${join(dir, 'started')}`] },
    ], { concurrency: 1, timeoutMs: 60_000, graceMs: 200, cwd: dir, write: () => {} });
    assert.deepEqual(results.map(result => result.status), ['FAILED', 'SKIPPED']);
    await new Promise(done => setTimeout(done, 3500));
    assert.equal(existsSync(join(dir, 'survived')), false, 'the interrupted step left a child running');
    assert.equal(existsSync(join(dir, 'started')), false);
    assert.equal(results.interrupted, 'SIGINT');
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
});
