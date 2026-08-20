// Package test provides test utilities and hermetic test verification.
package test

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/testdb"
)

// TestHermeticTestPatterns verifies that test files follow hermetic testing patterns.
// This test scans all *_test.go files in the backend directory and checks for:
// 1. Hardcoded small integer IDs (int64(1), int64(2), etc.) that indicate non-hermetic tests
// 2. Test files with DB operations that don't use SetupTestDB
//
// Hermetic tests should:
// - Create their own test data using fixtures
// - Never rely on hardcoded IDs that may not exist in the database
// - Clean up after themselves
// - Be runnable in any order and in parallel
func TestHermeticTestPatterns(t *testing.T) {
	// Find the backend root directory
	backendRoot, err := findBackendRoot()
	if err != nil {
		t.Skipf("Could not find backend root: %v", err)
		return
	}

	t.Run("no_hardcoded_integer_ids", func(t *testing.T) {
		violations := checkHardcodedIDs(t, backendRoot)
		if len(violations) > 0 {
			t.Errorf("Found %d hardcoded ID violation(s):\n\n%s\n\n"+
				"Fix: Use test fixtures instead of hardcoded IDs.\n"+
				"Example:\n"+
				"  // Before (non-hermetic):\n"+
				"  result, err := repo.FindByID(ctx, int64(1))\n\n"+
				"  // After (hermetic):\n"+
				"  student := testpkg.CreateTestStudent(t, db, \"First\", \"Last\", \"1a\")\n"+
				"  result, err := repo.FindByID(ctx, student.ID)",
				len(violations), strings.Join(violations, "\n"))
		}
	})

	t.Run("db_operations_use_setup_test_db", func(t *testing.T) {
		violations := checkMissingSetupTestDB(t, backendRoot)
		if len(violations) > 0 {
			t.Errorf("Found %d test file(s) with DB operations missing SetupTestDB:\n\n%s\n\n"+
				"Fix: Add SetupTestDB to initialize the test database.\n"+
				"Example:\n"+
				"  func TestExample(t *testing.T) {\n"+
				"      db := testpkg.SetupTestDB(t)\n"+
				"      // ... test code\n"+
				"  }",
				len(violations), strings.Join(violations, "\n"))
		}
	})

	t.Run("no_new_cleanup_calls", func(t *testing.T) {
		violations := checkCleanupCallRatchet(t, backendRoot)
		if len(violations) > 0 {
			t.Errorf("Cleanup-call ratchet violated (per-package counts may only shrink):\n\n%s\n\n"+
				"Since #2419 the package clone owns cleanup: every test binary gets a\n"+
				"fresh database clone, so per-row Cleanup* calls are redundant. New\n"+
				"tests must not add them. When you remove cleanup calls from a package,\n"+
				"lower its baseline in cleanupCallBaseline — never raise one.",
				strings.Join(violations, "\n"))
		}
	})

	t.Run("no_shared_pool_close", func(t *testing.T) {
		violations := checkSharedPoolClose(t, backendRoot)
		if len(violations) > 0 {
			t.Errorf("Found %d close(s) of the shared package test pool:\n\n%s\n\n"+
				"SetupTestDB returns ONE pool shared by every test in the binary —\n"+
				"closing it kills all later tests. Do not close it. Tests that close\n"+
				"their database on purpose (error-path injection) use the private pool:\n"+
				"  db := testpkg.SetupClosableTestDB(t)\n"+
				"  require.NoError(t, db.Close())",
				len(violations), strings.Join(violations, "\n"))
		}
	})

	t.Run("bootstrap_tenant_ratchet", func(t *testing.T) {
		violations := checkBootstrapTenantRatchet(t, backendRoot)
		if len(violations) > 0 {
			t.Errorf("Bootstrap-tenant ratchet violated (per-package counts may only shrink):\n\n%s\n\n"+
				"The fixed bootstrap tenant (TenantContext(1)) is being phased out\n"+
				"(#2419): new tests create their own tenant via testpkg.NewTenantScope\n"+
				"so they cannot collide with parallel tests. When you migrate a package,\n"+
				"lower its baseline in tenantContext1Baseline — never raise one.",
				strings.Join(violations, "\n"))
		}
	})

	t.Run("db_packages_opt_into_per_test_tenants", func(t *testing.T) {
		violations := checkPerTestTenantsOptIn(t, backendRoot)
		if len(violations) > 0 {
			t.Errorf("Found %d test package(s) that touch the database without per-test tenants:\n\n%s\n\n"+
				"A package that opens the test database must give each of its tests\n"+
				"its own tenant (#2419), otherwise every test writes into the shared\n"+
				"bootstrap tenant and tenant-wide assertions become order-dependent.\n\n"+
				"Add a TestMain to the package:\n"+
				"  func TestMain(m *testing.M) {\n"+
				"      testpkg.PerTestTenants()\n"+
				"      m.Run()\n"+
				"  }",
				len(violations), strings.Join(violations, "\n"))
		}
	})

	t.Run("db_packages_run_the_leftover_gate", func(t *testing.T) {
		violations := checkLeftoverGateOptIn(t, backendRoot)
		if len(violations) > 0 {
			t.Errorf("Found %d test package(s) that open the database without the leftover gate:\n\n%s\n\n"+
				"The gate compares the package clone's end state against its start\n"+
				"state and fails the package that left rows in shared state (#2419\n"+
				"goal 2). It runs from TestMain, so a package that does not call it\n"+
				"is simply not gated:\n"+
				"  func TestMain(m *testing.M) {\n"+
				"      testpkg.PerTestTenants()\n"+
				"      testpkg.Run(m)\n"+
				"  }",
				len(violations), strings.Join(violations, "\n"))
		}
	})

	t.Run("no_parallel_test_touching_global_state", func(t *testing.T) {
		violations := checkParallelGlobalState(t, backendRoot)
		if len(violations) > 0 {
			t.Errorf("Found %d parallel test(s) that reach process-global state:\n\n%s\n\n"+
				"viper keys, environment variables and the default logger are shared by\n"+
				"the whole test binary. A helper that sets one and restores it in\n"+
				"t.Cleanup will yank it out from under any test running beside it —\n"+
				"the failure surfaces as an unrelated assertion, and only under load.\n\n"+
				"Fix: drop t.Parallel() from this test, or make the helper stop mutating\n"+
				"global state (inject the value instead).",
				len(violations), strings.Join(violations, "\n"))
		}
	})

	t.Run("tests_run_in_parallel", func(t *testing.T) {
		counts, unexplained := checkSerialTestRatchet(t, backendRoot)
		if v := shrinkOnlyViolations(unexplained, serialUnexplainedBaseline); len(v) > 0 {
			t.Errorf("Serial tests without a reason (per-package counts may only shrink):\n\n%s\n\n"+
				"A test without t.Parallel() carries a comment directly above it that\n"+
				"starts with %q and names which of the five reasons applies\n"+
				"(process-global state, schema mutation, a sweep that runs across\n"+
				"tenants, a query budget on the shared pool, deliberate lock\n"+
				"contention). The counts below are what predates the rule; a new\n"+
				"serial test either explains itself or replaces one that does not.",
				strings.Join(v, "\n"), serialReasonPrefix)
		}
		violations := shrinkOnlyViolations(counts, serialTestBaseline)
		if len(violations) > 0 {
			t.Errorf("Serial-test ratchet violated (per-package counts may only shrink):\n\n%s\n\n"+
				"Since #2419 every test runs in its own tenant inside a per-package\n"+
				"database clone, so tests are parallel by default:\n\n"+
				"  func TestSomething(t *testing.T) {\n"+
				"      t.Parallel()\n"+
				"      db := testpkg.SetupTestDB(t)\n\n"+
				"A test that genuinely cannot (it writes process-global state, changes\n"+
				"the schema, measures a query budget, or exercises a sweep that runs\n"+
				"across tenants) stays serial WITH a comment above it saying which of\n"+
				"those it is — and raises nothing: lower another entry in\n"+
				"serialTestBaseline first, or fix the reason instead.",
				strings.Join(violations, "\n"))
		}
	})

	t.Run("no_broken_cleanup_model_pattern", func(t *testing.T) {
		violations := checkBrokenCleanupPattern(t, backendRoot)
		if len(violations) > 0 {
			t.Errorf("Found %d use(s) of the broken Model((*interface{})(nil)) cleanup pattern:\n\n%s\n\n"+
				"This pattern is silently rejected by bun (interface{} is not a struct),\n"+
				"causing every Exec() to short-circuit without running SQL — the cleanup\n"+
				"becomes a no-op and tests rely on stale data from previous runs (#1296).\n\n"+
				"Fix: replace with TableExpr(...) — the same pattern CleanupTableRecords uses.\n"+
				"  // Before (no-op):\n"+
				"  db.NewDelete().Model((*interface{})(nil)).Table(\"users.students\").Where(...)\n\n"+
				"  // After (correct):\n"+
				"  db.NewDelete().TableExpr(\"users.students\").Where(...)",
				len(violations), strings.Join(violations, "\n"))
		}
	})
}

// findBackendRoot returns the backend module root. internal/testdb already
// resolves it for the lifecycle; one walk-up implementation is enough.
func findBackendRoot() (string, error) {
	root, err := testdb.ProjectRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "backend"), nil
}

// checkHardcodedIDs scans test files for hardcoded small integer IDs.
func checkHardcodedIDs(t *testing.T, root string) []string {
	t.Helper()

	var violations []string

	// Pattern matches int64(1) through int64(9)
	hardcodedIDPattern := regexp.MustCompile(`int64\([1-9]\)`)

	// Patterns that indicate legitimate uses (not IDs)
	// Note: Some patterns use simple substring matching, others use word boundaries
	legitimatePatterns := []string{
		"//",             // Comments
		"i :=",           // Loop variables
		"i =",            // Loop variables
		"offset",         // Pagination
		"limit",          // Pagination
		"page",           // Pagination
		"Weekday",        // Day of week
		"weekday",        // Day of week
		"day",            // Time-related
		"hour",           // Time-related
		"minute",         // Time-related
		"second",         // Time-related
		"duration",       // Time-related
		"timeout",        // Time-related
		"retry",          // Retry counts
		"max",            // Limits
		"min",            // Limits
		"size",           // Sizes
		"len",            // Lengths
		"cap",            // Capacities
		"index",          // Array indices
		"999999",         // Non-existent ID patterns (intentional)
		"GreaterOrEqual", // Assertions checking >= 1
		"LessOrEqual",    // Assertions checking <= n
		"Greater",        // Assertions checking > n
		"Less",           // Assertions checking < n
		"func()",         // Inline functions creating pointers
		"return &id",     // Pointer helpers in model tests
		"tenant_id",      // Tenant ID in raw SQL or map literals (required for multi-tenancy)
		"TenantContext",  // Test helper setting tenant context
	}

	// Patterns that require word boundary matching to avoid false negatives.
	// For example, "count" should NOT match "AccountID" (Acc-ount-ID).
	wordBoundaryPatterns := []*regexp.Regexp{
		regexp.MustCompile(`\bcount\b`),    // Counts (word boundary to avoid matching "AccountID")
		regexp.MustCompile(`\baffected\b`), // Affected row counts from UPDATE/DELETE operations
	}

	// Files to skip (mock tests, model unit tests without DB)
	skipPatterns := []string{
		"_internal_test.go",                                      // Internal tests often use mocks
		"_mock_test.go",                                          // Mock tests
		"models/",                                                // Model unit tests don't hit DB (Unix)
		"models\\",                                               // Model unit tests don't hit DB (Windows)
		"invitation_service_test.go",                             // Uses mocks
		"reminder_notifications_test.go",                         // Pure in-memory fakes, no DB (#1624 follow-up)
		"password_reset_integration_test.go",                     // Uses mocks (sqlmock + stubs)
		"handlers_unit_test.go",                                  // Unit tests for converters (no DB)
		"http_middleware_test.go",                                // Uses nil *bun.DB for unit testing middleware
		"operator_provisioning_service_test.go",                  // Uses mocks (sqlmock + stubs)
		"operator_summaries_test.go",                             // Uses mockSummariesRepo + mockOrganizationRepo (no DB)
		"operator_invitation_test.go",                            // Uses mocks (sqlmock + stubs)
		"operator_invitation_dispatch_test.go",                   // Uses mocks for email dispatch tests
		"invitations_test.go",                                    // Uses mocks for handler tests
		"error_helpers_test.go",                                  // Internal unit tests for helper functions (no DB)
		"auth/authorize/role_grant_test.go",                      // Pure policy unit tests on stack-allocated roles (no DB); int64 literals are throwaway IDs
		"api/iot/api_test.go",                                    // Uses mock SchoolRepo for unit testing handler
		"api/iot/checkin/capacity_errors_test.go",                // Pure JSON-marshal regression tests for capacity/conflict renderers (ex api/iot/common); int64 literals are throwaway IDs, not DB rows
		"api/iot/checkin/wire_format_test.go",                    // Pure render-to-recorder wire goldens (issue #575 B0, ex api/iot/common); int64 literals are throwaway IDs, not DB rows
		"services/iot/checkin/checkin_pure_test.go",              // Pure/stub CheckinService unit tests (issue #575 B8); int64 literals are throwaway IDs in stack-allocated structs, not DB rows
		"api/iot/config_test.go",                                 // Uses mock settings service for unit testing config endpoint
		"enrich_pickup_times_test.go",                            // Uses mock PickupScheduleService for unit testing enrichment
		"api/timetable/api_test.go",                              // Uses mock CalendarPeriodService for unit testing handlers
		"api/timetable/closing_days_test.go",                     // Uses mock ClosingDayService for unit testing handlers; int64 literals are fake IDs, not DB rows
		"api/timetable/instances_test.go",                        // Uses mock InstanceService + PersonService for unit testing handlers
		"api/timetable/understaffed_test.go",                     // Uses mock InstanceService for unit testing the acknowledge-understaffed handler (no DB); int64 literals are fake instance IDs, not DB rows
		"api/timetable/instance_students_unit_test.go",           // Uses fake repo for unit testing attendance PATCH handler
		"services/schedule/attendance_sync_service_unit_test.go", // Uses fake repos for unit testing graceful-degradation branches
		"services/schedule/timetable_cleanup_service_test.go",    // Uses failingAuditRepo mock for audit-write-failure rollback coverage (WP-B14)
		"services/schedule/substitute_conflict_test.go",          // Pure unit test with in-memory structs; int64(1)/int64(2) are fake IDs, not DB rows (WP-B12)
		"realtime/hub_broadcast_to_tenant_test.go",               // Pure SSE-hub unit test; tenant IDs are in-memory channel routing keys, not DB rows
		"services/config/sideeffects/registry_test.go",           // Pure registry unit test; tenant IDs are pass-through arguments, not DB rows
		"services/facilities/settings_sideeffects_test.go",       // Pure side-effect dispatch unit test against fake services; tenant IDs are not DB rows
		"api/config/settings_broadcast_test.go",                  // Pure unit test for scheduleSettingsBroadcast; tenant IDs are pass-through arguments to a fake broadcaster
		"api/students/response_helpers_test.go",                  // Pure unit tests on populatePhotoFields; int64 literals are fake IDs in stack-allocated structs
		"auth/authorize/student_access_test.go",                  // Pure-logic tests against stub user-context + settings; int64 literals are fake group IDs in stack-allocated structs (no DB)
		"api/students/photo_error_mappers_test.go",               // Table-driven mapper tests with httptest.NewRecorder; no DB
		"api/students/photo_unlinker_test.go",                    // Pure file-system unit test for the unlinker; uses a temp dir, no DB
		"services/users/student_photo_service_broadcast_test.go", // Pure unit tests for broadcast helpers + side-effect registry binding; tenant IDs are pass-through arguments, no DB
		"api/common/trusted_device_dto_test.go",                  // Pure DTO-mapper unit tests against stack-allocated TrustedDeviceRow values; int64 literals are sentinel IDs in in-memory structs, not DB rows
		"services/platform/outbox_worker_test.go",                // Uses sqlmock + in-memory stubOutboxRepo to drive the worker poll-loop state machine without a real DB
		"api/enrollment/export_handlers_test.go",                 // Pure unit test for the phase-export builders against an in-memory PhaseExport; int64 literals are sentinel schema/grade values, not DB rows
		"guardian_related_accounts_errors_test.go",               // Pure mock-injection unit tests for the related-accounts error/best-effort branches; int64 literals are fake IDs in stack-allocated mocks, not DB rows
		"services/parentmessaging/parentmessaging_test.go",       // Pure unit tests for the messaging core against narrow fakes (no DB); int64 literals are in-memory sentinel IDs (thread/account/ref), not DB rows
		"services/enrollment/class_day_service_test.go",          // Pure unit tests against the classRosterTestService fakes (no DB); int64 literals are fake student/phase IDs in stack-allocated structs, not DB rows
	}

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files we can't access
		}

		// Only check test files
		if info.IsDir() || !strings.HasSuffix(path, "_test.go") {
			return nil
		}

		// Skip this verification test itself
		if strings.Contains(path, "hermetic_verification_test.go") {
			return nil
		}

		// Skip files matching skip patterns (mocks, model unit tests, etc.)
		// Normalize path to forward slashes for cross-platform matching
		normalizedPath := filepath.ToSlash(path)
		for _, pattern := range skipPatterns {
			if strings.Contains(normalizedPath, pattern) {
				return nil
			}
		}

		file, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer func() { _ = file.Close() }()

		scanner := bufio.NewScanner(file)
		lineNum := 0

		for scanner.Scan() {
			lineNum++
			line := scanner.Text()

			// Check if line contains hardcoded ID pattern
			if !hardcodedIDPattern.MatchString(line) {
				continue
			}

			// Check if it's a legitimate use
			isLegitimate := false
			for _, pattern := range legitimatePatterns {
				if strings.Contains(line, pattern) {
					isLegitimate = true
					break
				}
			}

			// Also check word-boundary patterns if not already flagged as legitimate
			if !isLegitimate {
				for _, pattern := range wordBoundaryPatterns {
					if pattern.MatchString(line) {
						isLegitimate = true
						break
					}
				}
			}

			if !isLegitimate {
				relPath, _ := filepath.Rel(root, path)
				violations = append(violations,
					formatViolation(relPath, lineNum, strings.TrimSpace(line)))
			}
		}

		return nil
	})

	if err != nil {
		t.Logf("Warning: error walking directory: %v", err)
	}

	return violations
}

// checkMissingSetupTestDB finds test files with DB operations that don't use SetupTestDB.
func checkMissingSetupTestDB(t *testing.T, root string) []string {
	t.Helper()

	var violations []string

	// Patterns indicating DB operations
	dbPatterns := []string{
		"bun.DB",
		".NewSelect()",
		".NewInsert()",
		".NewUpdate()",
		".NewDelete()",
		"repositories.",
	}

	// Patterns indicating SetupTestDB or SetupAPITest usage
	setupPatterns := []string{
		"SetupTestDB",
		"setupTestDB",
		"SetupAPITest",
		"setupAPITest",
		"setupTestContext",                // Indirect setup via shared helper (calls SetupAPITest)
		"newScenario",                     // E2E timetable flows — shared_setup.go wraps SetupAPITest
		"setupRolloverTest",               // services/enrollment rollover integration tests — wraps SetupTestDB
		"setupRequestTest",                // services/enrollment request-service integration tests — wraps SetupTestDB
		"setupDecisionTest",               // services/enrollment decision integration tests — wraps setupRolloverTest
		"setupCareTest",                   // services/enrollment care-offering integration tests — wraps SetupTestDB
		"setupAutoApproveIntegrationEnv",  // services/enrollment auto-approve integration tests — wraps setupRolloverTest
		"setupGuardianInvitationTest",     // services/auth guardian invitation + related-accounts tests — wraps SetupTestDB
		"makeScenario",                    // services/schedule materialization/split integration tests — wraps SetupTestDB
		"makeRosterChain",                 // services/schedule split-series roster tests (#2187) — wraps makeSeriesChain → makeScenario
		"makeMoveSetup",                   // services/schedule staff-pool/move tests (#1884) — wraps SetupTestDB
		"buildDevSetup",                   // api/timetable deviations/protocol tests — wraps SetupTestDB
		"setupCheckinServiceTest",         // services/iot/checkin CheckinService tests — wraps SetupAPITest (issue #575 B8)
		"setupAbsenceAdminTest",           // api/staff absence question tests (#1419) — wraps setupTestContext
		"newOverviewFixture",              // services/active overview/export integration tests (#1417) — wraps SetupTestDB
		"setupOverviewAPI",                // api/staff overview/export tests (#1417) — wraps setupTestContext
		"setupGradeTransitionServiceTest", // services/education grade-transition tests — wraps SetupTestDB
		"buildLifecycle",                  // services/schedule instance-lifecycle tests — wraps SetupTestDB
		"newCareFixture",                  // services/schedule care-request tests — wraps SetupTestDB
		"buildAbsenceService",             // services/absence excused-request tests — wraps SetupTestDB
	}

	// Patterns indicating mock-based testing (legitimate alternative)
	mockPatterns := []string{
		"sqlmock",
		"Mock",
		"mock",
		"Stub",
		"stub",
		"fake",
		"Fake",
	}

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if info.IsDir() || !strings.HasSuffix(path, "_test.go") {
			return nil
		}

		// Skip this verification test
		if strings.Contains(path, "hermetic_verification_test.go") {
			return nil
		}

		// Skip files that reference DB types but don't perform real DB operations
		normalizedPath := filepath.ToSlash(path)
		skipFiles := []string{
			"http_middleware_test.go",                           // Uses nil *bun.DB for unit testing middleware
			"parent_message_hooks_test.go",                      // Pure base.Model accessor unit test; no real DB
			"parent_announcement_model_test.go",                 // Pure validator/derivation unit tests; no real DB
			"role_management_internal_test.go",                  // Uses hand-rolled stub repos injected via repositories.Factory, no real DB
			"database/repositories/schedule/created_by_test.go", // Shared fixture helper; caller tests own DB setup
			"test/architecture_ratchet_test.go",                 // Source-scanning ratchet; regex literals look like DB ops but no DB is used
			"test/handler_layer_ratchet_test.go",                // Source-scanning ratchet (issue #584); same as above, no DB is used
			"api/timetable/timetable_data_test_helpers_test.go", // Shared fixture helper; caller tests own DB setup (mirrors created_by_test.go)
			"services/messaging/apply_export_internal_test.go",  // Test-support wrappers exposing unexported apply funcs; the *bun.DB is injected, caller (requests_test.go) owns SetupTestDB
		}
		skip := false
		for _, sf := range skipFiles {
			if strings.Contains(normalizedPath, sf) {
				skip = true
				break
			}
		}
		if skip {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		contentStr := string(content)

		// Check if file has DB operations
		hasDBOps := false
		for _, pattern := range dbPatterns {
			if strings.Contains(contentStr, pattern) {
				hasDBOps = true
				break
			}
		}

		if !hasDBOps {
			return nil
		}

		// Check if file uses SetupTestDB
		usesSetup := false
		for _, pattern := range setupPatterns {
			if strings.Contains(contentStr, pattern) {
				usesSetup = true
				break
			}
		}

		// Check if file uses mocks
		usesMocks := false
		for _, pattern := range mockPatterns {
			if strings.Contains(contentStr, pattern) {
				usesMocks = true
				break
			}
		}

		// Flag files with DB ops that don't use SetupTestDB and aren't mock-based
		if hasDBOps && !usesSetup && !usesMocks {
			relPath, _ := filepath.Rel(root, path)
			violations = append(violations, "  - "+relPath)
		}

		return nil
	})

	if err != nil {
		t.Logf("Warning: error walking directory: %v", err)
	}

	return violations
}

// cleanupCallBaseline is the shrink-only per-package baseline of Cleanup*
// fixture calls in _test.go files (#2419). The clone-per-package lifecycle
// makes them redundant; packages are migrated tranche by tranche (leftover-
// tolerant packages are already at zero). Counts may only go DOWN. A package
// not listed here must stay at zero.
var cleanupCallBaseline = map[string]int{
	"api/groups":                        93,
	"api/staff":                         84,
	"api/students":                      435,
	"api/timetable":                     295,
	"database/migrations":               106,
	"database/repositories/active":      289,
	"database/repositories/activities":  196,
	"database/repositories/audit":       95,
	"database/repositories/auth":        157,
	"database/repositories/config":      7,
	"database/repositories/education":   90,
	"database/repositories/enrollment":  1,
	"database/repositories/facilities":  19,
	"database/repositories/feedback":    33,
	"database/repositories/iot":         45,
	"database/repositories/parent":      17,
	"database/repositories/schedule":    336,
	"database/repositories/suggestions": 33,
	"database/repositories/users":       315,
	"models/base":                       3,
	"services/absence":                  21,
	"services/active":                   383,
	"services/activities":               98,
	"services/auth":                     273,
	"services/calendar":                 175,
	"services/config":                   2,
	"services/education":                262,
	"services/enrollment":               101,
	"services/messaging":                11,
	"services/parent":                   252,
	"services/schedule":                 380,
	"services/slotlists":                146,
	"services/users":                    304,
	"test":                              22,
	"test/e2e/calendar":                 13,
	"test/e2e/timetable":                28,
}

// tenantContext1Baseline is the shrink-only per-package baseline of
// bootstrap-tenant call sites (#2419). Every package that opens the test
// database now runs on per-test tenants (see PerTestTenants and the
// db_packages_opt_into_per_test_tenants gate), so what is left here is the
// residue that does NOT come from a test writing into the shared tenant.
// Counts may only go DOWN. A package not listed here must stay at zero.
//
// Why each entry survives — check the reason before touching a number:
//
//	api/testutil        the values TestClaimsFollowTheTestIntoItsOwnTenant
//	                    feeds in: the bootstrap tenant IS the input whose
//	                    rebase that test pins.
//	auth/jwt            test imports auth/jwt, so auth/jwt's own internal
//	                    tests cannot import test — no way to reach a
//	                    per-test tenant from there.
//	services/calendar   in-memory structs in pure unit tests (no DB, no rows).
//	services/auth       an in-memory stub row standing in for "some other
//	                    tenant" in a cross-tenant invalidation test.
//	services/active     a comment naming the column in a UNIQUE constraint.
//	services/enrollment a comment.
//	tenant              an internal context test with no database at all.
var tenantContext1Baseline = map[string]int{
	"api/testutil":        3,
	"auth/jwt":            2,
	"services/active":     1,
	"services/auth":       1,
	"services/calendar":   3,
	"services/enrollment": 1,
	"tenant":              1,
}

// walkGoFiles feeds every Go file under root to visit, as (rel, pkg, code):
// the backend-relative path with forward slashes, its package directory, and
// the file with line comments stripped.
//
// One walker for every gate. When each check brought its own, the exclusion
// lists drifted apart — one skipped this file by base name, another by
// substring, a third not at all — and a gate that silently scans its own
// error messages counts its own prose as violations. Comments are stripped
// for the same reason: writing down WHY something is the way it is must not
// make a ratchet grow.
//
// internal/testdb is excluded because it owns the lifecycle the gates check;
// this file is excluded because it contains every pattern by definition.
func walkGoFiles(root string, testOnly bool, visit func(rel, pkg string, code []byte)) error {
	return walkGoFilesRaw(root, func(rel, pkg string, content []byte) {
		if testOnly && !strings.HasSuffix(rel, "_test.go") {
			return
		}
		visit(rel, pkg, []byte(stripLineComments(string(content))))
	})
}

// walkGoFilesRaw is walkGoFiles without the comment stripping, for the checks
// that need the comments themselves (the serial ratchet reads the reason
// written above a test).
func walkGoFilesRaw(root string, visit func(rel, pkg string, content []byte)) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil || info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		switch {
		case !strings.HasSuffix(rel, ".go"),
			strings.HasPrefix(rel, "internal/testdb/"),
			strings.HasSuffix(rel, "hermetic_verification_test.go"):
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		visit(rel, filepath.ToSlash(filepath.Dir(rel)), content)
		return nil
	})
}

// countMatchesPerPackage counts regex matches per package directory.
// testOnly restricts the scan to _test.go files.
func countMatchesPerPackage(root string, re *regexp.Regexp, testOnly bool) (map[string]int, error) {
	counts := make(map[string]int)
	err := walkGoFiles(root, testOnly, func(_, pkg string, code []byte) {
		counts[pkg] += len(re.FindAll(code, -1))
	})
	return counts, err
}

// shrinkOnlyViolations compares current per-package counts against a
// shrink-only baseline and reports every package above its allowance.
// A package absent from the baseline is allowed zero.
func shrinkOnlyViolations(current, baseline map[string]int) []string {
	var violations []string
	for dir, n := range current {
		if allowed := baseline[dir]; n > allowed {
			violations = append(violations,
				fmt.Sprintf("  %s: %d (baseline %d)", dir, n, allowed))
		}
	}
	sort.Strings(violations)
	return violations
}

// cleanupCallPattern matches a fixture-cleanup call in any spelling the
// codebase uses: qualified (testpkg.CleanupX), deferred, or — inside package
// test itself — unqualified in statement position. The leading `\w+\.` guard
// on the last alternative keeps production calls like svc.CleanupExpiredX()
// out of it.
var cleanupCallPattern = regexp.MustCompile(
	`(?m)testpkg\.Cleanup\w+\(|(?:^|\s)defer Cleanup\w+\(|^\s*Cleanup\w+\(`)

// checkCleanupCallRatchet enforces the shrink-only Cleanup*-call baseline.
func checkCleanupCallRatchet(t *testing.T, root string) []string {
	t.Helper()
	re := cleanupCallPattern
	current, err := countMatchesPerPackage(root, re, true)
	if err != nil {
		t.Logf("Warning: error walking directory: %v", err)
	}
	return shrinkOnlyViolations(current, cleanupCallBaseline)
}

// bootstrapTenantPattern matches every spelling of "pin this to the fixed
// bootstrap tenant" that turned up during the #2419 migration. Each one was
// found the hard way, by a test that kept failing after the obvious spellings
// were gone:
//
//	testpkg.TenantContext(1) / testutil.TenantContext(1)
//	tenant.WithTenantID(context.Background(), 1)   (nested parens on the left)
//	TenantID: 1                                    (struct literal)
//	x.SetTenantID(1)                               (local fixtures)
//	CreateTestRoomForTenant(t, db, 1, ...)         (tenant as an argument)
//	Where(`x.tenant_id = ?`, 1) / tenant_id = 1    (raw SQL)
//
// Counting only some of them is what makes a ratchet decoration: the next
// test reaches the bootstrap tenant through a spelling nobody counted and
// the gate stays green.
var bootstrapTenantPattern = regexp.MustCompile(
	`TenantContext\(1\)` +
		`|WithTenantID\(.*,\s*1\)` +
		`|TenantID:\s*1\b` +
		`|SetTenantID\(1\)` +
		`|ForTenant\([^)\n]*,\s*1\s*[,)]` +
		`|tenant_id\s*=\s*\?[^,\n]*,\s*1\s*\)` +
		`|tenant_id\s*=\s*1\b`)

// checkBootstrapTenantRatchet enforces the shrink-only bootstrap-tenant baseline.
func checkBootstrapTenantRatchet(t *testing.T, root string) []string {
	t.Helper()
	re := bootstrapTenantPattern
	// Test files only. The bootstrap tenant is a test construct; the same
	// literal in production code means something else entirely — the
	// historical migrations that lifted the single-tenant era onto tenant 1
	// are immutable history, and api/testutil's default claims carry it on
	// purpose as the value the per-test rebase maps away from.
	current, err := countMatchesPerPackage(root, re, true)
	if err != nil {
		t.Logf("Warning: error walking directory: %v", err)
	}
	return shrinkOnlyViolations(current, tenantContext1Baseline)
}

// globalStateMutation matches the process-global writes that make a test
// unsafe to run beside another: viper keys, environment variables, the
// default logger, the working directory.
// Each entry was found by a test that failed once the suite went parallel:
// the settings registry (a package-level map every settings test reads),
// os.Stdout/os.Stderr reassignment (how the cmd tests capture output), and
// os.Unsetenv (the other half of os.Setenv).
var globalStateMutation = regexp.MustCompile(
	`viper\.(Set|SetDefault|Reset)\b|t\.Setenv|os\.(Set|Unset)env|log\.SetOutput|slog\.SetDefault` +
		`|os\.Chdir|config\.(Register|ResetRegistry)\(|SeedTestJWTConfig\(|os\.(Stdout|Stderr)\s*=`)

// stripLineComments blanks out // comments so a pattern match means code, not
// prose about the code.
func stripLineComments(text string) string {
	var b strings.Builder
	b.Grow(len(text))
	for line := range strings.SplitSeq(text, "\n") {
		if i := strings.Index(line, "//"); i >= 0 {
			line = line[:i]
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String()
}

// goFuncDef matches any top-level func declaration, capturing its name.
var goFuncDef = regexp.MustCompile(`(?m)^func\s+(?:\([^)]*\)\s*)?(\w+)\s*\(`)

// goFunc is one top-level function of a test file.
type goFunc struct {
	name string
	body string
}

// splitGoFuncs slices a file into its top-level functions.
func splitGoFuncs(text string) []goFunc {
	locs := goFuncDef.FindAllStringSubmatchIndex(text, -1)
	funcs := make([]goFunc, 0, len(locs))
	for i, loc := range locs {
		end := len(text)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		funcs = append(funcs, goFunc{name: text[loc[2]:loc[3]], body: text[loc[0]:end]})
	}
	return funcs
}

// callsAny reports whether body calls any of the named functions.
func callsAny(body string, names map[string]bool) bool {
	for name := range names {
		if regexp.MustCompile(`\b` + regexp.QuoteMeta(name) + `\s*\(`).MatchString(body) {
			return true
		}
	}
	return false
}

// checkParallelGlobalState finds tests that call t.Parallel() while reaching
// process-global state — directly, or through a helper anywhere in the same
// package. Resolving helpers per package (not per file) is the point: the
// #2419 sweep parallelised a whole file whose JWT-secret helper lived in a
// sibling file, and the restore in that helper's t.Cleanup broke unrelated
// tests running beside it.
func checkParallelGlobalState(t *testing.T, root string) []string {
	t.Helper()

	type fileEntry struct {
		rel  string
		text string
	}
	pkgFuncs := make(map[string]map[string]string)
	pkgFiles := make(map[string][]fileEntry)

	err := filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil || info.IsDir() || !strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if strings.HasPrefix(rel, "internal/testdb/") ||
			strings.Contains(rel, "hermetic_verification_test.go") {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		// Comments are stripped before matching: a test that only NAMES
		// t.Setenv (the parallel-safety guard in db_clone_test.go says why the
		// per-test path must not call it) is not a test that calls it.
		text := stripLineComments(string(content))
		pkg := filepath.ToSlash(filepath.Dir(rel))
		if pkgFuncs[pkg] == nil {
			pkgFuncs[pkg] = make(map[string]string)
		}
		for _, f := range splitGoFuncs(text) {
			pkgFuncs[pkg][f.name] = f.body
		}
		pkgFiles[pkg] = append(pkgFiles[pkg], fileEntry{rel: rel, text: text})
		return nil
	})
	if err != nil {
		t.Logf("Warning: error walking directory: %v", err)
	}

	var violations []string
	for pkg, funcs := range pkgFuncs {
		// Seed with helpers that mutate global state directly, then close over
		// helpers that call them.
		tainted := make(map[string]bool)
		for name, body := range funcs {
			if !strings.HasPrefix(name, "Test") && globalStateMutation.MatchString(body) {
				tainted[name] = true
			}
		}
		for range 5 {
			grew := false
			for name, body := range funcs {
				if tainted[name] || strings.HasPrefix(name, "Test") {
					continue
				}
				if callsAny(body, tainted) {
					tainted[name] = true
					grew = true
				}
			}
			if !grew {
				break
			}
		}
		if len(tainted) == 0 {
			continue
		}
		for _, fe := range pkgFiles[pkg] {
			for _, f := range splitGoFuncs(fe.text) {
				if !strings.HasPrefix(f.name, "Test") || !strings.Contains(f.body, "t.Parallel()") {
					continue
				}
				if globalStateMutation.MatchString(f.body) || callsAny(f.body, tainted) {
					violations = append(violations, "  "+fe.rel+": "+f.name)
				}
			}
		}
	}
	sort.Strings(violations)
	return violations
}

// sharedPoolAssign captures the variable a file binds the SHARED pool to —
// `db := testpkg.SetupTestDB(t)` or `db, svc := testutil.SetupAPITest(t)`.
// Only the first identifier can hold the pool in either signature.
var sharedPoolAssign = regexp.MustCompile(
	`(?m)^\s*(\w+)\s*(?:,\s*\w+\s*)?:?=\s*(?:\w+\.)?(?:SetupTestDB|SetupAPITest)\(`)

// privatePoolAssign captures variables bound to a PRIVATE pool, which the
// owning test is allowed (and expected) to close: SetupClosableTestDB, plus
// hand-built sqlmock/bun handles whose Close is part of the mock contract.
var privatePoolAssign = regexp.MustCompile(
	`(?m)^\s*(\w+)\s*(?:,\s*[\w,\s]+)?:?=\s*(?:\w+\.)?(?:SetupClosableTestDB|bun\.NewDB|sqlmock\.New)\(`)

// checkSharedPoolClose flags closes of the shared SetupTestDB pool in any
// spelling. It resolves the pool variable per file rather than matching three
// literal one-liners, so `defer testDB.Close()` and
// `require.NoError(t, db.Close())` are caught too.
func checkSharedPoolClose(t *testing.T, root string) []string {
	t.Helper()

	var violations []string

	err := filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil || info.IsDir() || !strings.HasSuffix(path, "_test.go") {
			return nil
		}
		normalized := filepath.ToSlash(path)
		if strings.Contains(normalized, "internal/testdb/") ||
			strings.Contains(normalized, "hermetic_verification_test.go") {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		text := string(content)

		shared := make(map[string]bool)
		for _, m := range sharedPoolAssign.FindAllStringSubmatch(text, -1) {
			shared[m[1]] = true
		}
		if len(shared) == 0 {
			return nil
		}
		// A name rebound to a private pool anywhere in the file is ambiguous;
		// leave it to the reviewer rather than reporting a false positive.
		for _, m := range privatePoolAssign.FindAllStringSubmatch(text, -1) {
			delete(shared, m[1])
		}

		relPath, _ := filepath.Rel(root, path)
		for i, line := range strings.Split(text, "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "//") {
				continue
			}
			for name := range shared {
				if regexp.MustCompile(`\b` + regexp.QuoteMeta(name) + `\.Close\(\)`).MatchString(line) {
					violations = append(violations,
						formatViolation(filepath.ToSlash(relPath), i+1, strings.TrimSpace(line)))
					break
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Logf("Warning: error walking directory: %v", err)
	}
	return violations
}

// checkBrokenCleanupPattern scans the test helper package for the
// Model((*interface{})(nil)) cleanup pattern that bun silently rejects.
// See issue #1296 — every call site logged an error via tb.Logf but never
// failed the test, so cleanup was a no-op for months.
func checkBrokenCleanupPattern(t *testing.T, root string) []string {
	t.Helper()

	var violations []string

	brokenPattern := regexp.MustCompile(`Model\(\(\*interface\{\}\)\(nil\)\)`)

	// Only scan the test helper package — production code uses concrete types
	// like Model((*MyStruct)(nil)) which is valid.
	testDir := filepath.Join(root, "test")

	err := filepath.Walk(testDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		// Skip this file — it intentionally contains the pattern in error messages.
		if filepath.Base(path) == "hermetic_verification_test.go" {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer func() { _ = file.Close() }()

		relPath, _ := filepath.Rel(root, path)

		scanner := bufio.NewScanner(file)
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			line := scanner.Text()

			if strings.HasPrefix(strings.TrimSpace(line), "//") {
				continue
			}

			if brokenPattern.MatchString(line) {
				violations = append(violations,
					formatViolation(relPath, lineNum, strings.TrimSpace(line)))
			}
		}

		return nil
	})

	if err != nil {
		t.Logf("Warning: error walking directory: %v", err)
	}

	return violations
}

// formatViolation formats a violation message for display.
func formatViolation(file string, line int, content string) string {
	return fmt.Sprintf("  %s:%d\n    %s", file, line, content)
}

// perTestTenantsOptOut lists packages that cannot call PerTestTenants. Only
// structural reasons belong here, and each one names its reason.
var perTestTenantsOptOut = map[string]string{
	// test/ imports auth/jwt, so auth/jwt's internal tests cannot import test/.
	"auth/jwt": "import cycle: test imports auth/jwt",
}

// checkLeftoverGateOptIn reports test packages that open the test database but
// never run the leftover gate, i.e. whose TestMain does not end in
// testpkg.Run(m).
func checkLeftoverGateOptIn(t *testing.T, root string) []string {
	t.Helper()

	usesDB, gated := make(map[string]bool), make(map[string]bool)
	err := walkGoFilesRaw(root, func(rel, pkg string, content []byte) {
		if !strings.HasSuffix(rel, "_test.go") || strings.HasPrefix(rel, "internal/testdb/") {
			return
		}
		if bytes.Contains(content, []byte("SetupTestDB(")) || bytes.Contains(content, []byte("SetupAPITest(")) {
			usesDB[pkg] = true
		}
		if bytes.Contains(content, []byte("testpkg.Run(m)")) || bytes.Contains(content, []byte("\tRun(m)")) {
			gated[pkg] = true
		}
	})
	if err != nil {
		t.Logf("Warning: error walking directory: %v", err)
	}

	var violations []string
	for pkg := range usesDB {
		if !gated[pkg] {
			violations = append(violations, "  "+pkg)
		}
	}
	sort.Strings(violations)
	return violations
}

// checkPerTestTenantsOptIn reports test packages that open the test database
// but never switch to per-test tenants. It is the forward-looking half of the
// bootstrap-tenant ratchet: that one counts what is left, this one keeps a new
// package from starting out on the shared tenant.
func checkPerTestTenantsOptIn(t *testing.T, root string) []string {
	t.Helper()

	usesDB := make(map[string]bool)
	optedIn := make(map[string]bool)

	err := filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil || info.IsDir() || !strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if strings.HasPrefix(rel, "internal/testdb/") {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		pkg := filepath.ToSlash(filepath.Dir(rel))
		if bytes.Contains(content, []byte("SetupTestDB(")) || bytes.Contains(content, []byte("SetupAPITest(")) {
			usesDB[pkg] = true
		}
		if bytes.Contains(content, []byte("PerTestTenants()")) {
			optedIn[pkg] = true
		}
		return nil
	})
	if err != nil {
		t.Logf("Warning: error walking directory: %v", err)
	}

	var violations []string
	for pkg := range usesDB {
		if optedIn[pkg] {
			continue
		}
		if reason, ok := perTestTenantsOptOut[pkg]; ok {
			_ = reason
			continue
		}
		violations = append(violations, "  "+pkg)
	}
	sort.Strings(violations)
	return violations
}

// serialTestBaseline is the shrink-only per-package count of top-level tests
// that do NOT call t.Parallel() (#2419 goal 4, "voll parallel"). Everything
// else in the suite runs in parallel; what is counted here is the residue
// that cannot, and every one of those carries a comment above the test
// saying why.
//
// The reasons that survive fall into four families:
//
//	process-global state   viper keys, environment variables, the default
//	                       logger, the settings registry — a test that
//	                       writes one cannot run beside a test that reads it
//	                       (the no_parallel_test_touching_global_state gate
//	                       is the forward-looking half of this).
//	schema mutation        migration tests, and the handful of tests that
//	                       drop and restore a column: they change the clone
//	                       every test in the binary shares.
//	unscoped sweeps        code that queries across tenants (a deadline
//	                       worker, a global cleanup) called from a service
//	                       test with a plain tenant context, so RLS never
//	                       narrows it.
//	measurement            query-budget tests, which install a hook on the
//	                       shared pool and count what flows through it.
//	lock contention        the handful of tests that take a row lock in one
//	                       transaction and expect a second one to block on
//	                       it; beside another test that touches those rows,
//	                       the contention becomes a deadlock.
//
// Counts may only go DOWN. A package not listed here must have every test
// parallel. Lower a number when you fix the underlying reason; never raise
// one to make a new test fit.
//
// One deliberate exception, in the same change that introduced this note
// (#2419): five packages went UP because tests that HAD t.Parallel() were
// shown not to survive it — every one of them an unscoped sweep or a write to
// process-global state, each now serial with its reason above it. They were
// measured, not guessed: services/auth failed roughly one shuffled run in
// three (CleanupExpiredTokens deletes orphaned push rows across every account
// and tenant), and the four others were caught in full-suite runs.
// database/repositories/platform paid for it by making 17 outbox tests
// parallel.
var serialTestBaseline = map[string]int{
	"api":                               20,
	"api/active":                        2,
	"api/auth":                          1,
	"api/enrollment":                    1,
	"api/guardians":                     1,
	"api/iot":                           7,
	"api/iot/checkin":                   14,
	"api/operator":                      30,
	"api/schedules":                     2,
	"api/staff":                         15,
	"api/staff-shifts":                  1,
	"api/students":                      14,
	"api/suggestions":                   42,
	"api/timetable":                     2,
	"api/work-time-models":              1,
	"applog":                            1,
	"auth/device":                       34,
	"auth/jwt":                          38,
	"cmd":                               190,
	"database":                          8,
	"database/migrations":               88,
	"database/repositories/audit":       1,
	"database/repositories/auth":        4,
	"database/repositories/enrollment":  3,
	"database/repositories/platform":    34,
	"database/repositories/suggestions": 2,
	"database/repositories/users":       1,
	"email":                             12,
	"integration/phoenixapi":            25,
	"models/config":                     12,
	"observability":                     4,
	"seed/api":                          1,
	"services":                          15,
	"services/active":                   1,
	"services/auth":                     25,
	"services/config":                   147,
	"services/education":                1,
	"services/enrollment":               32,
	"services/facilities":               1,
	"services/iot/checkin":              38,
	"services/parent":                   4,
	"services/platform":                 41,
	"services/scheduler":                59,
	"services/usercontext":              3,
	"services/users":                    12,
	"test/e2e/calendar":                 4,
	"test/e2e/timetable":                3,
}

// serialUnexplainedBaseline is the shrink-only per-package count of serial
// tests that give no reason. These predate the rule; the #2419 sweep wrote a
// reason above every test IT made serial, but hundreds were serial long
// before and nobody ever said why. Retro-documenting them is its own job —
// what this baseline buys is that the number can only fall.
var serialUnexplainedBaseline = map[string]int{
	"api":                    20,
	"api/guardians":          1,
	"api/iot":                2,
	"api/iot/checkin":        3,
	"api/operator":           30,
	"api/staff":              1,
	"api/students":           6,
	"api/suggestions":        42,
	"applog":                 1,
	"auth/device":            12,
	"database":               8,
	"integration/phoenixapi": 25,
	"seed/api":               1,
	"services":               15,
	"services/enrollment":    13,
	"services/iot/checkin":   14,
	"services/scheduler":     45,
}

// topLevelTestDecl matches a top-level test function declaration.
var topLevelTestDecl = regexp.MustCompile(`(?m)^func (Test\w+)\(\w+ \*testing\.T\) \{`)

// topLevelParallelCall matches the Parallel() call of the TEST ITSELF: one
// tab of indentation, nothing else on the line. Searching for ".Parallel()"
// anywhere in the body would also count a test whose SUBTESTS are parallel
// while the test itself is not — precisely the case this ratchet exists to
// count.
var topLevelParallelCall = regexp.MustCompile(`(?m)^\t\w+\.Parallel\(\)$`)

// serialReasonPrefix is the sentence a serial test opens its reason with.
// The ratchet requires it, so "why is this one serial" is answered in the
// file and not only in this baseline's doc comment.
const serialReasonPrefix = "// Deliberately NOT parallel:"

// serialPackagePrefix says it once for a whole package. Some packages are
// serial end to end for a single reason — cmd drives cobra and the viper
// singleton, and repeating the same four lines above 134 tests told the
// reader nothing the package could not say once.
const serialPackagePrefix = "// Deliberately NOT parallel (whole package):"

// checkSerialTestRatchet counts, per package, the top-level tests that do not
// call t.Parallel(), and reports every one of them that gives no reason.
func checkSerialTestRatchet(t *testing.T, root string) (counts, unexplained map[string]int) {
	t.Helper()

	counts, unexplained = make(map[string]int), make(map[string]int)
	serialPackages := make(map[string]bool)
	if err := walkGoFilesRaw(root, func(_, pkg string, content []byte) {
		if bytes.Contains(content, []byte(serialPackagePrefix)) {
			serialPackages[pkg] = true
		}
	}); err != nil {
		t.Logf("Warning: error walking directory: %v", err)
	}

	err := walkGoFilesRaw(root, func(_, pkg string, content []byte) {
		text := string(content)
		for _, f := range splitGoFuncs(text) {
			if !topLevelTestDecl.MatchString(f.body) || topLevelParallelCall.MatchString(f.body) {
				continue
			}
			counts[pkg]++
			if !serialPackages[pkg] && !strings.Contains(precedingComment(text, f), serialReasonPrefix) {
				unexplained[pkg]++
			}
		}
	})
	if err != nil {
		t.Logf("Warning: error walking directory: %v", err)
	}
	return counts, unexplained
}

// precedingComment returns the comment block directly above f.
func precedingComment(text string, f goFunc) string {
	idx := strings.Index(text, f.body)
	if idx <= 0 {
		return ""
	}
	before := text[:idx]
	var block []string
	for _, line := range reverse(strings.Split(strings.TrimRight(before, "\n"), "\n")) {
		if !strings.HasPrefix(strings.TrimSpace(line), "//") {
			break
		}
		block = append(block, line)
	}
	return strings.Join(block, "\n")
}

func reverse(lines []string) []string {
	out := make([]string, 0, len(lines))
	for i := len(lines) - 1; i >= 0; i-- {
		out = append(out, lines[i])
	}
	return out
}
