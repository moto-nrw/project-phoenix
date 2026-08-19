// Package test provides test utilities and hermetic test verification.
package test

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
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

// findBackendRoot walks up the directory tree to find the backend root.
func findBackendRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		// Check if this looks like the backend root
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", os.ErrNotExist
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
// TenantContext(1) call sites (#2419). Per-test tenants (NewTenantScope)
// replace the fixed bootstrap tenant so parallel tests cannot collide.
// Counts may only go DOWN. A package not listed here must stay at zero.
var tenantContext1Baseline = map[string]int{
	"api/active":                        5,
	"api/activities":                    16,
	"api/admin":                         2,
	"api/auth":                          4,
	"api/birthdays":                     2,
	"api/classday":                      1,
	"api/classlistentries":              1,
	"api/feedback":                      1,
	"api/groups":                        1,
	"api/guardians":                     15,
	"api/import":                        3,
	"api/iot":                           1,
	"api/iot/checkin":                   11,
	"api/shift-types":                   1,
	"api/sse":                           4,
	"api/staff":                         20,
	"api/students":                      47,
	"api/suggestions":                   4,
	"api/timetable":                     28,
	"database/repositories/active":      140,
	"database/repositories/activities":  74,
	"database/repositories/audit":       37,
	"database/repositories/auth":        155,
	"database/repositories/base":        11,
	"database/repositories/config":      17,
	"database/repositories/education":   87,
	"database/repositories/facilities":  10,
	"database/repositories/feedback":    13,
	"database/repositories/iot":         20,
	"database/repositories/mealplan":    4,
	"database/repositories/parent":      2,
	"database/repositories/platform":    49,
	"database/repositories/schedule":    191,
	"database/repositories/suggestions": 27,
	"database/repositories/users":       179,
	"models/base":                       28,
	"services/active":                   216,
	"services/activities":               99,
	"services/auth":                     162,
	"services/calendar":                 48,
	"services/config":                   3,
	"services/database":                 2,
	"services/education":                97,
	"services/enrollment":               502,
	"services/facilities":               41,
	"services/feedback":                 13,
	"services/import":                   1,
	"services/iot":                      20,
	"services/iot/checkin":              5,
	"services/parent":                   13,
	"services/reminders":                1,
	"services/schedule":                 102,
	"services/scheduler":                4,
	"services/slotlists":                41,
	"services/suggestions":              7,
	"services/users":                    146,
	"tenant":                            2,
	"test":                              7,
}

// countMatchesPerPackage walks root and counts regex matches per package
// directory (relative, forward slashes). testOnly restricts the scan to
// _test.go files. internal/testdb owns its own lifecycle and is excluded.
func countMatchesPerPackage(root string, re *regexp.Regexp, testOnly bool) (map[string]int, error) {
	counts := make(map[string]int)
	err := filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil || info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if !strings.HasSuffix(rel, ".go") || strings.HasPrefix(rel, "internal/testdb/") {
			return nil
		}
		if strings.Contains(rel, "hermetic_verification_test.go") {
			return nil
		}
		if testOnly && !strings.HasSuffix(rel, "_test.go") {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		if n := len(re.FindAll(content, -1)); n > 0 {
			counts[filepath.ToSlash(filepath.Dir(rel))] += n
		}
		return nil
	})
	return counts, err
}

// ratchetViolations compares current per-package counts against a shrink-only
// baseline and reports every package above its allowance.
func shrinkOnlyViolations(current, baseline map[string]int) []string {
	var violations []string
	for dir, n := range current {
		if allowed := baseline[dir]; n > allowed {
			violations = append(violations,
				"  "+dir+": "+itoa(n)+" (baseline "+itoa(allowed)+")")
		}
	}
	sort.Strings(violations)
	return violations
}

// checkCleanupCallRatchet enforces the shrink-only Cleanup*-call baseline.
func checkCleanupCallRatchet(t *testing.T, root string) []string {
	t.Helper()
	re := regexp.MustCompile(`testpkg\.Cleanup\w+\(|(?:^|\s)defer Cleanup\w+\(`)
	current, err := countMatchesPerPackage(root, re, true)
	if err != nil {
		t.Logf("Warning: error walking directory: %v", err)
	}
	return shrinkOnlyViolations(current, cleanupCallBaseline)
}

// checkBootstrapTenantRatchet enforces the shrink-only TenantContext(1) baseline.
func checkBootstrapTenantRatchet(t *testing.T, root string) []string {
	t.Helper()
	re := regexp.MustCompile(`TenantContext\(1\)`)
	current, err := countMatchesPerPackage(root, re, false)
	if err != nil {
		t.Logf("Warning: error walking directory: %v", err)
	}
	return shrinkOnlyViolations(current, tenantContext1Baseline)
}

// checkSharedPoolClose flags one-liner closes of the shared SetupTestDB pool.
func checkSharedPoolClose(t *testing.T, root string) []string {
	t.Helper()

	var violations []string
	closeLine := regexp.MustCompile(
		`^\s*(defer func\(\) \{ _ = db\.Close\(\) \}\(\)|defer db\.Close\(\)|t\.Cleanup\(func\(\) \{ _ = db\.Close\(\) \}\))\s*$`)

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
		// Only files on the shared pool are affected; SetupClosableTestDB
		// pools are private and may be closed.
		if !strings.Contains(text, "SetupTestDB(") && !strings.Contains(text, "SetupAPITest(") {
			return nil
		}
		relPath, _ := filepath.Rel(root, path)
		for i, line := range strings.Split(text, "\n") {
			if closeLine.MatchString(line) {
				violations = append(violations,
					formatViolation(filepath.ToSlash(relPath), i+1, strings.TrimSpace(line)))
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
	return "  " + file + ":" + itoa(line) + "\n    " + content
}

// itoa converts an int to string without importing strconv.
func itoa(i int) string {
	if i == 0 {
		return "0"
	}

	var b [20]byte
	pos := len(b)
	neg := i < 0
	if neg {
		i = -i
	}

	for i > 0 {
		pos--
		b[pos] = byte('0' + i%10)
		i /= 10
	}

	if neg {
		pos--
		b[pos] = '-'
	}

	return string(b[pos:])
}
