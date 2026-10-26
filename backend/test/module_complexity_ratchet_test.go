package test

import (
	"fmt"
	"go/ast"
	"sort"
	"strings"
	"testing"

	"github.com/uudashr/gocognit"
)

// TestModuleComplexityRatchet extends .claude/rules/backend-conventions.md
// Rule 4 — gocognit cognitive complexity ≤ 15 — from api/ to the capability
// modules, application workflows, roots (ADR 0082), and all backend trees
// (ADR 0087).
//
// Why a second ratchet instead of a wider api/ one: migration #2580 moved
// 228,306 of 638,717 production LOC into modules/ and workflows/, and the
// api/ ratchet stopped at its directory boundary. Moved code left the
// ratchet's reach without a single line changing. This
// test closes that gap by freezing today's state under the new roots.
//
// TestHandlerComplexityRatchet keeps the default production api/ build at
// zero. This ratchet includes test hooks too and scores the module handlers
// (modules/*/http, modules/*/inbound, workflows/*/http) against the same
// threshold; they have no register of their own.
//
// Allowlist semantics (per function, not per file), identical to the api/
// ratchet:
//
//   - A function NOT in the allowlist must score ≤ 15.
//   - An allowlisted function may never exceed its recorded score.
//   - When refactoring lowers a score (or drops it to ≤ 15), the test fails
//     until the entry is lowered/removed — the ratchet only turns one way.
//     Additions or raises require a new-numbered ADR cited by the entry
//     (ADR 0070).
//
// Keys are "relative/file.go:FuncName" with methods rendered exactly like the
// gocognit CLI ("(*service).ListOptions"), so CLI output maps 1:1 onto
// entries here. walkModuleRatchetSources scans all backend trees recursively,
// including legacy, migrations, tools, generated code, hooks and test support.
// Only _test.go and hidden directories are excluded. ADR 0087 records the
// category audit and exact additions measured at 8aca1925f7. SQL inside
// strings adds no Go cognitive complexity.
const moduleComplexityThreshold = 15

// Seeded 2026-09-18 at merge commit ecf0003369, the state of modules/ and
// workflows/ right after the #2580 move, measured with:
//
//	cd backend && ../scripts/run-go-toolchain.sh \
//	    go run github.com/uudashr/gocognit/cmd/gocognit -over 15 ./modules ./workflows
//
// (_test.go hits filtered out). 444 functions. The first seed was taken at
// 19feca2822 and re-measured here after the merge of origin/development
// (a47f77b2c3): PR #3408 (issue #3350) moved eleven functions over the limit
// into modules/, which the ratchet cannot absorb without a fresh measurement.
// The entries record the debt as it was on that day; every one of them may
// only shrink or disappear.
//
// Re-measured on 2026-10-19 at 148e5aeb86 for the gocognit 1.2.1 → 1.3.0
// upgrade (ADR 0072). The new version scores else nesting, parenthesized
// logical operators and recursion differently; no code changed. Thirteen
// entries rose, three functions entered at 16, and two entries fell: the
// register's sum went from 5,460 to 5,527 over 228 entries. This is a tool
// rebaseline, not new debt; ADR 0072 records why the functions were not
// refactored instead. The module register drained; ADR 0082 adds the root
// baseline without headroom, measured at 722c5aa147.
var moduleComplexityAllowlist = map[string]int{
	// ADR 0087.
	"analytics/core_actions_middleware.go:CoreActionMiddleware":                                               19,
	"api/active_test_helpers.go:buildActiveTestModule":                                                        31,
	"api/auth_test_helpers.go:NewAuthTestModule":                                                              25,
	"api/enrollment_booking_repository_test_helpers.go:(EnrollmentBookingFixture).InsertRequestChildOffering": 18,
	"api/enrollment_booking_repository_test_helpers.go:(EnrollmentBookingFixture).bookingCommandValues":       22,
	"api/student_owner_http_test_helpers.go:NewStudentHTTPTestResources":                                      19,
	"api/student_test_helpers.go:newStudentTestModule":                                                        32,
	"api/timetable_test_helpers.go:NewTimetableTestModule":                                                    27,
	"cmd/devtools/demo.go:provisionDemoSchool":                                                                18,
	"cmd/devtools/demo_runtime.go:NewDemoRuntime":                                                             16,
	"cmd/devtools/demo_scheduler.go:(*demoScheduler).drain":                                                   20,
	"cmd/serve.go:scrubSentryEvent":                                                                           20,
	"cmd/serve.go:validateServeConfig":                                                                        18,
	// ADR 0087.
	"database/migrations/001006002_create_admin_account.go:createAdminAccount": 16,
	// ADR 0087.
	"database/migrations/001008003_activities_groups_created_by.go:migrateActivitiesGroupsCreatedBy": 19,
	// ADR 0087.
	"database/migrations/001014002_add_tenant_id_to_all_tables.go:addTenantIDToAllTables": 17,
	// ADR 0087.
	"database/migrations/001014003_migrate_unique_constraints.go:migrateUniqueConstraints": 37,
	// ADR 0087.
	"database/migrations/001014003_migrate_unique_constraints.go:rollbackUniqueConstraints": 37,
	// ADR 0087.
	"database/migrations/001014005_create_tenant_indexes.go:createTenantIndexes": 18,
	// ADR 0087.
	"database/migrations/001014005_create_tenant_indexes.go:rollbackTenantIndexes": 18,
	// ADR 0087.
	"database/migrations/001015032_create_arrival_schedules.go:createArrivalSchedulesUp": 17,
	// ADR 0087.
	"database/migrations/001015036_create_activity_instances.go:createActivityInstancesUp": 29,
	// ADR 0087.
	"database/migrations/001015073_request_children_renewal_columns.go:init": 16,
	// ADR 0087.
	"database/migrations/001015074_form_schemas_add_name.go:init": 22,
	// ADR 0087.
	"database/migrations/001015149_parent_messaging.go:parentMessagingUp": 21,
	// ADR 0087.
	"database/migrations/001015166_form_schemas_migrate_legacy_departure_fields.go:convertLegacyDepartureFields": 50,
	// ADR 0087.
	"database/migrations/001015166_form_schemas_migrate_legacy_departure_fields.go:formSchemasMigrateLegacyDepartureUp": 33,
	// ADR 0087.
	"database/migrations/001015168_add_is_system_flags.go:init": 18,
	// ADR 0087.
	"database/migrations/001015192_staff_shift_flexible_changes.go:init": 18,
	// ADR 0087.
	"database/migrations/00_migrations.go:DetectVersionCollisions": 23,
	// ADR 0087.
	"database/migrations/00_migrations.go:ValidateMigrations": 43,
	// ADR 0087.
	"database/migrations/form_schema_validation_166.go:validateConvertedDepartureSchema": 21,
	// ADR 0087.
	"database/migrations/form_schema_validation_166.go:validateMigration166Field": 21,
	// ADR 0087.
	"database/migrations/form_schema_validation_166.go:validateMigration166LegalBlock": 32,
	// ADR 0087.
	"database/migrations/form_schema_validation_166.go:validateMigration166Visibility": 19,
	// ADR 0087.
	"database/migrations/guardian_owner_backfill.go:(*guardianOwnerTenantRun).copyBatch": 23,
	// ADR 0087.
	"database/migrations/guardian_owner_backfill.go:(*guardianOwnerTenantRun).tryCopyBatch": 20,
	// ADR 0087.
	"database/migrations/guardian_owner_backfill.go:runGuardianOwnerTenant": 17,
	// ADR 0087.
	"database/migrations/guardian_owner_contract.go:contractGuardianOwnerStorageChecked": 19,
	// ADR 0087.
	"database/migrations/presence_backfill_metrics.go:monitorBackfillLocks": 19,
	// ADR 0087.
	"database/migrations/presence_contract.go:contractPresenceStorageChecked": 18,
	// ADR 0087.
	"database/migrations/request_child_storage_backfill.go:(*tenantBackfillRun).reconcile": 22,
	// ADR 0087.
	"database/migrations/request_child_storage_backfill.go:RunRequestChildStorageBackfill": 20,
	// ADR 0087.
	"database/migrations/request_child_storage_contract.go:contractRequestChildStorageChecked": 19,
	// ADR 0087.
	"database/migrations/staff_owner_backfill.go:(*staffOwnerTenantRun).copyBatch": 23,
	// ADR 0087.
	"database/migrations/staff_owner_backfill.go:(*staffOwnerTenantRun).tryCopyBatch": 20,
	// ADR 0087.
	"database/migrations/staff_owner_backfill.go:runStaffOwnerTenant": 17,
	// ADR 0087.
	"database/migrations/staff_owner_contract.go:contractStaffOwnerStorageChecked": 19,
	// ADR 0087.
	"database/migrations/student_owner_backfill.go:(*studentOwnerTenantRun).copyBatch": 23,
	// ADR 0087.
	"database/migrations/student_owner_backfill.go:(*studentOwnerTenantRun).tryCopyBatch": 20,
	// ADR 0087.
	"database/migrations/student_owner_backfill.go:runStudentOwnerTenant": 17,
	// ADR 0087.
	"database/migrations/student_owner_backfill_verify.go:studentOwnerCutoverPrecondition": 23,
	// ADR 0087.
	"database/migrations/student_owner_contract.go:contractStudentOwnerStorageChecked": 21,
	// ADR 0087.
	"internal/architecture/composition_owners.go:(*compositionOwnership).held": 17,
	// ADR 0087.
	"internal/architecture/composition_owners.go:(*rootStructWalk).lexicalName": 59,
	// ADR 0087.
	"internal/architecture/composition_owners.go:rootStructInventoryWith": 39,
	// ADR 0087.
	"internal/architecture/composition_ratchet.go:(*compositionSnapshot).add": 27,
	// ADR 0087.
	"internal/architecture/composition_ratchet.go:(*compositionSnapshot).collectSetters": 23,
	// ADR 0087.
	"internal/architecture/composition_ratchet.go:(*compositionSnapshot).dependencyType": 16,
	// ADR 0087.
	"internal/architecture/composition_ratchet.go:(*compositionSnapshot).guardedStructureField": 21,
	// ADR 0087.
	"internal/architecture/composition_ratchet.go:(*compositionSnapshot).inventory": 40,
	// ADR 0087.
	"internal/architecture/composition_ratchet.go:CompareCompositionSurface": 36,
	// ADR 0087.
	"internal/architecture/composition_ratchet.go:compositionTarget": 19,
	// ADR 0087.
	"internal/architecture/composition_ratchet.go:loadCompositionExternalTypes": 16,
	// ADR 0087.
	"internal/architecture/composition_ratchet.go:readCompositionBase": 20,
	// ADR 0087.
	"internal/architecture/composition_removals.go:packageDeclarations": 33,
	// ADR 0087.
	"internal/architecture/cycles.go:AnalyzeCycles": 17,
	// ADR 0087.
	"internal/architecture/cycles.go:cycleEdges": 16,
	// ADR 0087.
	"internal/architecture/cycles.go:cyclicComponents": 25,
	// ADR 0087.
	"internal/architecture/import_debt.go:CompareCandidateLegacyBaselines": 20,
	// ADR 0087.
	"internal/architecture/import_debt.go:basePackageImports": 29,
	// ADR 0087.
	"internal/architecture/migration_additions.go:migrationCreateDataObjects": 19,
	// ADR 0087.
	"internal/architecture/migration_queries.go:analyzeMigrationQueries": 48,
	// ADR 0087.
	"internal/architecture/migration_sql.go:migrationSQLBodyDataObjects": 85,
	// ADR 0087.
	"internal/architecture/migration_sql.go:migrationSQLTokens": 40,
	// ADR 0087.
	"internal/architecture/owner_http_rule.go:reviewedOwnerHTTPRule": 51,
	// ADR 0087.
	"internal/architecture/policy.go:(*Policy).validate": 16,
	// ADR 0087.
	"internal/architecture/policy.go:(*Policy).validateHeader": 16,
	// ADR 0087.
	"internal/architecture/policy.go:(*Policy).validateLegacyComposition": 16,
	// ADR 0087.
	"internal/architecture/policy.go:(*Policy).validatePackages": 17,
	// ADR 0087.
	"internal/architecture/policy.go:(*Policy).validateReadProjections": 18,
	// ADR 0087.
	"internal/architecture/policy.go:(*Policy).validateRules": 20,
	// ADR 0087.
	"internal/architecture/policy.go:decodePolicy": 21,
	// ADR 0087.
	"internal/architecture/policy_compare.go:classificationLoosenings": 20,
	// ADR 0087.
	"internal/architecture/policy_compare.go:ownershipLoosenings": 26,
	// ADR 0087.
	"internal/architecture/ratchet_budgets.go:(*ratchetBudgets).finalize": 39,
	// ADR 0087.
	"internal/architecture/ratchet_budgets.go:(*ratchetBudgets).measureRegister": 22,
	// ADR 0087.
	"internal/architecture/ratchet_budgets.go:CompareRatchetBudgets": 22,
	// ADR 0087.
	"internal/architecture/ratchet_budgets.go:compareRatchetBudgets": 70,
	// ADR 0087.
	"internal/architecture/relocation.go:(*Policy).validateRelocations": 22,
	// ADR 0087.
	"internal/architecture/relocation.go:relocatedPackages": 16,
	// ADR 0087.
	"internal/architecture/rule_packages.go:(*Policy).validateRulePackages": 16,
	// ADR 0087.
	"internal/architecture/semantic.go:(semanticAnalyzer).analyzeFile": 18,
	// ADR 0087.
	"internal/architecture/semantic.go:bunModelDataObject": 32,
	// ADR 0087.
	"internal/architecture/semantic.go:contractDeclarationPositions": 25,
	// ADR 0087.
	"internal/architecture/semantic.go:contractMethodViolationsSeen": 17,
	// ADR 0087.
	"internal/architecture/semantic.go:ormTagViolations": 22,
	// ADR 0087.
	"internal/architecture/target_rule.go:validateTargetOnlyRule": 25,
	// ADR 0087.
	"internal/devtools/seedapi/client.go:extractResponseSummary": 22,
	// ADR 0087.
	"internal/devtools/seedapi/demo_roles.go:(*FixedSeeder).seedDemoRoles": 16,
	// ADR 0087.
	// ADR 0087.
	"internal/devtools/seedapi/enrollment_weekly_profile.go:seedWeeklyProfileContents": 29,
	// ADR 0087.
	"internal/devtools/seedapi/enrollment_weekly_profile.go:verifyEnrollmentProfileCare": 23,
	// ADR 0087.
	"internal/devtools/seedapi/parent_enrollment.go:(parentEnrollmentSeedStep).seedEnrollment": 53,
	// ADR 0087.
	"internal/devtools/seedapi/parent_enrollment.go:(parentEnrollmentSeedStep).seedParentAccounts": 25,
	// ADR 0087.
	"internal/devtools/seedapi/parent_enrollment.go:(parentRequestsSeedStep).Run": 16,
	// ADR 0087.
	"internal/devtools/seedapi/seed_extras.go:(seedStatisticsDemoStep).Run": 19,
	// ADR 0087.
	"internal/devtools/seedapi/seed_phase_periods.go:seedPhaseSchoolYear": 30,
	// ADR 0087.
	"internal/devtools/seedapi/seed_planning.go:selectRecurringPlanningInstances": 16,
	// ADR 0087.
	"internal/devtools/seedapi/seed_time_tracking.go:(seedTimeTrackingHistoryStep).Run": 35,
	// ADR 0087.
	"internal/devtools/seedapi/seed_time_tracking.go:(timeTrackingHistoryPlan).seedStaff": 31,
	// ADR 0087.
	"internal/devtools/seedapi/seeder.go:(*Seeder).Seed": 21,
	// ADR 0087.
	"internal/devtools/seedapi/seeder.go:(*Seeder).printSuccessSummary": 17,
	// ADR 0087.
	"internal/devtools/seedapi/stammdaten.go:(*FixedSeeder).Seed": 22,
	// ADR 0087.
	"internal/devtools/seedapi/stammdaten.go:(*FixedSeeder).seedGuardianPayments": 16,
	// ADR 0087.
	"internal/devtools/seedapi/stammdaten.go:(*FixedSeeder).seedStaffAccounts": 29,
	// ADR 0087.
	"internal/devtools/seedapi/stammdaten.go:(*FixedSeeder).seedStudents": 18,
	// ADR 0087.
	"internal/devtools/simulate/demo.go:(*DemoTicker).Tick": 31,
	// ADR 0087.
	"internal/devtools/simulate/demo.go:(*DemoTicker).closeLeftovers": 17,
	// ADR 0087.
	"internal/devtools/simulate/demo.go:(*DemoTicker).ensureSessions": 19,
	// ADR 0087.
	"internal/devtools/simulate/demo.go:(*DemoTicker).rebuild": 16,
	// ADR 0087.
	"internal/devtools/simulate/demo_day.go:(*demoDay).move": 37,
	// ADR 0087.
	"internal/devtools/simulate/demo_day.go:(*demoDay).planActivities": 23,
	// ADR 0087.
	"internal/devtools/simulate/demo_day.go:(*demoDay).sync": 37,
	// ADR 0087.
	"internal/devtools/simulate/demo_day.go:demoDayShift": 17,
	// ADR 0087.
	"internal/devtools/simulate/live.go:RunLive": 24,
	// ADR 0087.
	"internal/devtools/simulate/live.go:runLiveTick": 19,
	// ADR 0087.
	"internal/testdb/cmd/bootstrap/main.go:main": 18,
	// ADR 0087.
	"internal/testdb/idle_unix.go:HoldServer": 31,
	// ADR 0087.
	"internal/testdb/idle_unix.go:IdleStatus": 17,
	// ADR 0087.
	"internal/testdb/idle_unix.go:watchIdle": 17,
	// ADR 0087.
	"internal/testdb/server.go:inspectTestContainer": 18,
	// ADR 0087.
	"internal/testdb/template.go:migrationsComplete": 17,
	// ADR 0087.
	"observability/sentry_envelope.go:TagSentryEnvelope": 16,
	// ADR 0087.
	"realtime/hub.go:(*Hub).Unregister": 24,
	// ADR 0087.
	"test/fixtures.go:cleanupTenantTestData": 39,
	// ADR 0087.
	"test/runtime_checkpoint.go:MeasureDeliveryCheckpoint": 27,
	// ADR 0087.
	"test/runtime_checkpoint.go:MeasureTimetableCheckpoint": 17,
	// ADR 0087.
	"test/side_effect_register.go:sideEffectRegisterSubset": 16,
	// ADR 0087.
	"test/tenant_runtime.go:TenantTxMiddleware": 17,
	// ADR 0087.
	"test/transaction_receiver_relocation.go:originalReceiverTransactionKey": 17,
	// ADR 0087.
	"test/transaction_receiver_relocation.go:removedTransactionReceiverMethods": 35,
	// ADR 0087.
	"test/transaction_relocation.go:transactionFingerprints": 17,
	// ADR 0087.
	"test/transaction_relocation.go:transactionFunctionFingerprints": 20,
	// ADR 0087.
	"test/transaction_relocation.go:transactionScopeSubset": 20,
}

func TestModuleComplexityRatchet(t *testing.T) {
	t.Parallel()

	backendRoot, err := findBackendRoot()
	if err != nil {
		t.Skipf("Could not find backend root: %v", err)
		return
	}

	scores, err := moduleComplexityScan(backendRoot)
	if err != nil {
		t.Fatalf("complexity scan failed: %v", err)
	}

	violations := moduleComplexityViolations(scores, moduleComplexityAllowlist)
	if len(violations) > 0 {
		sort.Strings(violations)
		t.Errorf("Module-complexity ratchet check failed (%d issue(s)):\n\n%s",
			len(violations), strings.Join(violations, "\n\n"))
	}
}

// moduleComplexityViolations compares per-function scores over the threshold
// against the allowlist. Like its api/ counterpart it cannot reuse
// ratchetViolations: entries here are capped scores, not hit counts, and an
// allowlisted function dropping to or below the threshold disappears from the
// scan entirely (which is the "ratchet down" signal, not a stale-entry error
// in the per-file sense).
func moduleComplexityViolations(scores, allowlist map[string]int) []string {
	var violations []string
	for fn, got := range scores {
		allowed, ok := allowlist[fn]
		switch {
		case !ok:
			violations = append(violations, fmt.Sprintf(
				"[gocognit] %s scores %d (limit %d), and is not in the allowlist.\n  Move branching/orchestration behind a capability method (Rule 4) — do not add new entries.",
				fn, got, moduleComplexityThreshold))
		case got > allowed:
			violations = append(violations, fmt.Sprintf(
				"[gocognit] %s scores %d, allowed %d.\n  Complexity grew. Simplify the function — never raise the allowlist.", fn, got, allowed))
		case got < allowed:
			violations = append(violations, fmt.Sprintf(
				"[gocognit] %s scores %d, allowlist says %d.\n  Nice — ratchet the entry down to %d so the improvement cannot regress.", fn, got, allowed, got))
		}
	}
	for fn, allowed := range allowlist {
		if _, ok := scores[fn]; !ok {
			violations = append(violations, fmt.Sprintf(
				"[gocognit] %s is allowlisted at %d but now scores ≤ %d (or was deleted/renamed).\n  Remove the entry.",
				fn, allowed, moduleComplexityThreshold))
		}
	}
	return violations
}

// All roots retain coverage and thresholds after a move (ADRs 0064, 0082).
// moduleComplexityScan returns gocognit scores above the threshold for every
// function in the shared backend source scope, keyed
// "relpath:FuncName" with methods rendered like the gocognit CLI
// ("(*service).name"). It shares funcDisplayName with the api/ ratchet so
// both registers speak the same key language.
func moduleComplexityScan(backendRoot string) (map[string]int, error) {
	scores := make(map[string]int)
	err := walkModuleRatchetSources(backendRoot, func(path, rel string) error {
		_, file, parseErr := parseGoSourceCached(path, nil)
		if parseErr != nil {
			return fmt.Errorf("parse %s: %w", rel, parseErr)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			if score := gocognit.Complexity(fn); score > moduleComplexityThreshold {
				scores[rel+":"+funcDisplayName(fn)] = score
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return scores, nil
}
