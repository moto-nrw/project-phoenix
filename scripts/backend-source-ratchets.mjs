#!/usr/bin/env node
// Filesystem dependencies of backend/test that Go's import graph cannot see.
// Inputs are conservative supersets: never inspect only existing files or diff
// contents, since additions, removals and relocations can invalidate a register.
import { spawnSync } from 'node:child_process';
import { pathToFileURL } from 'node:url';

const go = /^backend\/.*\.go$/;
const production = path => go.test(path) && !path.endsWith('_test.go');
const policy = /^backend\/architecture\/policy(?:\.json|\/.*\.json)$/;
const architecture = /^backend\/architecture\/.*\.json$/;
const tests = /^backend\/.*_test\.go$/;
const moduleRoots = path => production(path) && /^backend\/(modules|workflows)\//.test(path);
const match = (input, path) => typeof input === 'function' ? input(path) : input.test(path);
const entry = (file, names, ...inputs) => ({ file: `${file}_test.go`, tests: names.split(' '), inputs });

// One entry per repository-reading test file, including tests without Ratchet
// in their name. Scanner unit tests on synthetic trees stay ordinary Go tests.
// Runtime schema comparisons (permission_catalog, trigger_ownership) remain in
// the affected/full suites; only the catalog's source-only check belongs here.
export const sourceRatchets = [
  entry('hermetic_verification', 'TestHermeticTestPatterns', go),
  entry('calendar_date_verification', 'TestDateColumnTypes', go),
  entry('calendar_fixture_ratchet', 'TestCalendarFixtureClockRatchet', go),
  entry('wall_clock_verification', 'TestActivityInstanceWallClockRatchet', go),
  entry('test_skip_ratchet', 'TestTestSkipRatchet', go),
  entry('account_credential_writer', 'TestAccountCredentialHasOneWriter', production),
  entry('operator_credential_writer', 'TestOperatorCredentialHasOneWriter TestOperatorCredentialStatementRecognizesWriters', production),
  entry('gdpr_log_pii_ratchet', 'TestGDPRLogPIIRatchet', production),
  entry('realtime_scope_ratchet', 'TestRealtimeProducersRequireSchool', production),
  entry('tenant_filters', 'TestTenantFiltersRequireExplicitScope', production),
  entry('school_class_label_ratchet', 'TestSchoolClassLabelRatchet', production),
  entry('module_file_size_ratchet', 'TestModuleFileSizeRatchet', production),
  entry('module_function_length_ratchet', 'TestModuleFunctionLengthRatchet', production),
  entry('module_complexity_ratchet', 'TestModuleComplexityRatchet', production),
  entry('module_legacy_budget_ratchet', 'TestModuleLegacyBudgetRatchet', production),
  entry('module_compose_alias_ratchet', 'TestModuleComposeAliasRatchet', production),
  entry('module_facade_width_ratchet', 'TestModuleFacadeWidthRatchet', moduleRoots),
  // Embedded interfaces can import types from anywhere in the backend.
  entry('module_facade_method_set_ratchet', 'TestModuleFacadeMethodSetRatchet', production),
  entry('module_passthrough_ratchet', 'TestModulePassthroughRatchet', moduleRoots, /^backend\/(api|roots)\/.*\.go$/),
  entry('method_passthrough_ratchet', 'TestMethodPassthroughRatchet', production, architecture),
  entry('handler_layer_ratchet', 'TestHandlerLayerRatchet', production, /^backend\/go\.(mod|sum)$/),
  entry('handler_complexity_ratchet', 'TestHandlerComplexityRatchet', production, /^backend\/go\.(mod|sum)$/),
  entry('helper_consolidation_ratchet', 'TestHelperConsolidationRatchet', production),
  entry('http_error_identity_ratchet', 'TestHTTPErrorIdentityRatchet', production, policy),
  entry('transaction_boundaries_ratchet', 'TestTransactionBoundaryRatchet', production, policy, /^backend\/test\/transaction_boundaries\.json$/,
    /^scripts\/backend-architecture(?:\.sh|\/.*)$/),
  entry('durable_side_effects_ratchet', 'TestDurableSideEffectsRatchet', production, policy, /^backend\/test\/durable_side_effect_notices\.json$/),
  entry('composition_inventory', 'TestCompositionInventory TestWorkerJobRegistryInventory', go, architecture),
  entry('module_contract_inventory', 'TestModuleContractInventoryRatchet', tests, policy, /^backend\/test\/module_contract_inventory\.json$/),
  entry('query_budget_ratchet', 'TestQueryBudgetRatchet', tests, /^backend\/test\/query-budgets\//),
  entry('route_sized_api_builders', 'TestAPITestsUseNarrowRouteBuilders', go),
  entry('repo_method_caller_ratchet', 'TestRepoInterfaceMethodCallerRatchet', production),
  entry('production_source_scan', 'TestProductionBuildExcludesTestHooks', production, /^backend\/go\.(mod|sum)$/),
  entry('fixture_deadline_order', 'TestFixtureDeadlinesStartAfterNestedFixtures', /^backend\/test\/[^/]+\.go$/),
  entry('shared_kernel_types', 'TestSharedKernelTypesAreNamedDefinitions', /^backend\/(sharedkernel\/calendar\/(date|wall_clock)|tenant\/id|observability\/correlation_id)\.go$/),
  entry('testhooks_leaf', 'TestBroadlyImportedPackagesCompileTheSameWithTestHooks', production),
  entry('traeger_view_scope_ratchet', 'TestTraegerViewQueriesFilterOnTheTraegerSchools TestAuditAccessTrailQueriesFilterOnTheSchoolSet TestHolidayAccessTrailQueriesFilterOnTheTraeger',
    /^backend\/modules\/traegerview\/internal\/adapters\/postgres\/[^/]+\.go$/,
    /^backend\/modules\/(auditlog\/internal\/adapters\/auditpostgres\/access_trail|enrollment\/internal\/adapters\/postgres\/holiday_access_trail)\.go$/),
  entry('permission_catalog', 'TestPermissionCatalogNamesAreUnique', /^backend\/auth\/authorize\/permissions\/catalog\.json$/),
  entry('error_code_generation_ratchet', 'TestErrorCodeGenerationRatchetsAllowAdditionalCode', /^error-registry\/.*\.json$/, /^scripts\/(generate-error-codes|error-registry)\.mjs$/),
  entry('toolchain_pins_ratchet', 'TestToolchainPinsRatchet',
    /^(devbox\.(json|lock)|package\.json|frontend\/(package\.json|Dockerfile(\.prod)?)|lefthook\.yml)$/,
    /^backend\/(go\.mod|tools\/go\.mod|Dockerfile(\.dev)?)$/,
    /^scripts\/(backend-architecture\/go\.mod|run-go-toolchain\.sh|pre-push\.sh)$/,
    /^\.github\/workflows\/.*\.yml$/, /^\.claude\/hooks\/format-go\.sh$/),
];

export function sourceRatchetPattern(paths) {
  const selected = sourceRatchets.filter(rule => paths.some(path =>
    path === `backend/test/${rule.file}` || rule.inputs.some(input => match(input, path))));
  const names = [...new Set(selected.flatMap(rule => rule.tests))].sort();
  return names.length ? `^(${names.join('|')})$` : '';
}

function git(root, args, env) {
  const result = spawnSync('git', args, { cwd: root, env, encoding: 'utf8', maxBuffer: 64 * 1024 * 1024 });
  if (result.error) throw result.error;
  if (result.status !== 0) throw new Error(`git ${args[0]}: ${result.stderr.trim()}`);
  return result.stdout;
}

export function changedPaths(root, base, env = process.env) {
  const mergeBase = git(root, ['merge-base', 'HEAD', base], env).trim();
  // Disable rename detection so moving a source OUT of a scanned tree still
  // selects its scanner. NUL delimiters preserve spaces and newlines in names.
  return [...new Set([
    ...git(root, ['diff', '--no-renames', '--name-only', '-z', mergeBase], env).split('\0'),
    ...git(root, ['ls-files', '--others', '--exclude-standard', '-z'], env).split('\0'),
  ].filter(Boolean))].sort();
}

if (import.meta.url === pathToFileURL(process.argv[1]).href) {
  try {
    const root = git(process.cwd(), ['rev-parse', '--show-toplevel'], process.env).trim();
    console.log(sourceRatchetPattern(changedPaths(root, process.argv[2] ?? 'origin/development')));
  } catch (error) {
    console.error(`backend source ratchets: ${error.message}`);
    process.exitCode = 1;
  }
}
