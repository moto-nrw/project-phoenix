// Package test provides test utilities and hermetic test verification.
package test

import (
	"bufio"
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/testdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHermeticTestPatterns verifies that test files follow hermetic testing patterns.
// This test scans all *_test.go files in the backend directory and checks for:
// 1. Hardcoded small integer IDs (int64(1), int64(2), etc.) that indicate non-hermetic tests
// 2. Test files with DB operations that don't use SetupTestDB
//
// Hermetic tests should:
// - Create their own test data using fixtures
// - Never rely on hardcoded IDs that may not exist in the database
// - Let the package clone or fixture builder own their rows
// - Be runnable in any order and in parallel
func TestHermeticTestPatterns(t *testing.T) {
	t.Parallel()
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
				"Fix: make the helper stop mutating global state; inject the value instead.",
				len(violations), strings.Join(violations, "\n"))
		}
	})

	t.Run("tests_run_in_parallel", func(t *testing.T) {
		violations := shrinkOnlyViolations(checkSerialTests(t, backendRoot), serialTestBaseline)
		if len(violations) > 0 {
			t.Errorf("Found top-level tests without t.Parallel():\n\n%s\n\n"+
				"Every top-level backend test runs in parallel. Isolate process globals,\n"+
				"schema changes, sweeps, measurements, and locks instead of adding an exemption:\n\n"+
				"  func TestSomething(t *testing.T) {\n"+
				"      t.Parallel()\n"+
				"      db := testpkg.SetupTestDB(t)\n",
				strings.Join(violations, "\n"))
		}
	})

	t.Run("no_ignored_cleanup_deletes", func(t *testing.T) {
		violations := checkIgnoredCleanupDeletes(t, backendRoot)
		if len(violations) > 0 {
			t.Errorf("Found %d DB delete(s) with a discarded error in test cleanup code:\n\n%s\n\n"+
				"The package clone and per-test tenants own every tenant row, so a\n"+
				"delete in t.Cleanup, a deferred call, a returned cleanup closure or a\n"+
				"wipe*/cleanup* helper is redundant — and with its error thrown away it\n"+
				"cannot even report that it failed.\n\n"+
				"Fix: delete the cleanup. Tenantless rows (auth.accounts, users.persons)\n"+
				"belong to the fixture helpers that create them; schema-migration tests\n"+
				"use testpkg.OwnTenantRows. Do not add an exemption.",
				len(violations), strings.Join(violations, "\n"))
		}
	})

	t.Run("no_broken_cleanup_model_pattern", func(t *testing.T) {
		violations := checkBrokenCleanupPattern(t, backendRoot)
		if len(violations) > 0 {
			t.Errorf("Found %d use(s) of the broken Model((*interface{})(nil)) cleanup pattern:\n\n%s\n\n"+
				"This pattern is silently rejected by bun (interface{} is not a struct),\n"+
				"causing every Exec() to short-circuit without running SQL — the cleanup\n"+
				"becomes a no-op and tests rely on stale data from previous runs (#1296).\n\n"+
				"Fix: replace Model((*interface{})(nil)).Table(...) with TableExpr(...).\n"+
				"  // Before (no-op):\n"+
				"  db.NewDelete().Model((*interface{})(nil)).Table(\"users.students\").Where(...)\n\n"+
				"  // After (correct):\n"+
				"  db.NewDelete().TableExpr(\"users.students\").Where(...)",
				len(violations), strings.Join(violations, "\n"))
		}
	})
}

func TestParallelGlobalStateGateFindsDirectMutation(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	probe := []byte(`package probe

import (
	"testing"
	"github.com/spf13/viper"
)

func TestDirectMutation(t *testing.T) {
	t.Parallel()
	viper.` + "Set" + `("key", "value")
}
`)
	if err := os.WriteFile(filepath.Join(root, "direct_test.go"), probe, 0o600); err != nil {
		t.Fatal(err)
	}

	violations := checkParallelGlobalState(t, root)
	if len(violations) != 1 || !strings.Contains(violations[0], "TestDirectMutation") {
		t.Fatalf("direct process-global mutation not detected: %v", violations)
	}
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
		"_internal_test.go",              // Internal tests often use mocks
		"_mock_test.go",                  // Mock tests
		"models/",                        // Model unit tests don't hit DB (Unix)
		"models\\",                       // Model unit tests don't hit DB (Windows)
		"invitation_service_test.go",     // Uses mocks
		"reminder_notifications_test.go", // Pure in-memory fakes, no DB (#1624 follow-up)
		"handlers_unit_test.go",          // Unit tests for converters (no DB)
		"invitations_test.go",            // Uses mocks for handler tests
		"error_helpers_test.go",          // Internal unit tests for helper functions (no DB)
		"api/iot/api_test.go",            // Uses mock SchoolRepo for unit testing handler
		"modules/devicescan/http/checkin/capacity_errors_test.go",              // Pure JSON-marshal regression tests for capacity/conflict renderers (ex api/iot/common); int64 literals are throwaway IDs, not DB rows
		"modules/devicescan/http/checkin/wire_test.go",                         // Wire goldens over a fake device-scan workflow (#2698); int64 literals are throwaway IDs, not DB rows
		"api/iot/config_test.go",                                               // Uses mock settings service for unit testing config endpoint
		"enrich_pickup_times_test.go",                                          // Uses mock PickupScheduleService for unit testing enrichment
		"modules/timetable/http/api_test.go",                                   // Uses mock CalendarPeriodService for unit testing handlers
		"modules/timetable/http/closing_days_test.go",                          // Uses mock ClosingDayService for unit testing handlers; int64 literals are fake IDs, not DB rows
		"modules/timetable/http/instances_test.go",                             // Uses mock InstanceService + PersonService for unit testing handlers
		"modules/timetable/http/understaffed_test.go",                          // Uses mock InstanceService for unit testing the acknowledge-understaffed handler (no DB); int64 literals are fake instance IDs, not DB rows
		"modules/timetable/http/instance_students_unit_test.go",                // Uses fake repo for unit testing attendance PATCH handler
		"timetable/substitute_conflicts_test.go",                               // Pure unit test with in-memory structs; int64(1)/int64(2) are fake IDs, not DB rows (WP-B12)
		"realtime/hub_broadcast_to_tenant_test.go",                             // Pure SSE-hub unit test; tenant IDs are in-memory channel routing keys, not DB rows
		"modules/presenceprojection/http/response_helpers_test.go",             // Pure unit tests on populatePhotoFields; int64 literals are fake IDs in stack-allocated structs
		"modules/peopledirectory/inbound/students/photo_error_mappers_test.go", // Table-driven mapper tests with httptest.NewRecorder; no DB
		"api/common/trusted_device_dto_test.go",                                // Pure DTO-mapper unit tests against stack-allocated TrustedDeviceRow values; int64 literals are sentinel IDs in in-memory structs, not DB rows
		"modules/enrollment/http/change_request_handlers_test.go",              // Pure handler unit tests against a service mock; int64 literals are URL parser sentinels, not DB rows
		"modules/enrollment/http/export_handlers_test.go",                      // Pure unit test for the phase-export builders against an in-memory PhaseExport; int64 literals are sentinel schema/grade values, not DB rows
	}

	files, err := goSourceIndex(root)
	if err != nil {
		t.Fatalf("walk source files: %v", err)
	}
	for _, f := range files {
		// Only check test files
		if !strings.HasSuffix(f.rel, "_test.go") {
			continue
		}

		// Skip this verification test itself
		if strings.Contains(f.rel, "hermetic_verification_test.go") {
			continue
		}

		// Skip files matching skip patterns (mocks, model unit tests, etc.)
		skipped := false
		for _, pattern := range skipPatterns {
			if strings.Contains(f.rel, pattern) {
				skipped = true
				break
			}
		}
		if skipped {
			continue
		}

		scanner := bufio.NewScanner(bytes.NewReader(f.content))
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
				violations = append(violations,
					formatViolation(f.rel, lineNum, strings.TrimSpace(line)))
			}
		}
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

	// Patterns indicating direct or indirect shared test DB setup.
	setupPatterns := []string{
		"SetupTestDB",
		"SetupIsolatedTestDB",
		"setupTestDB",
		"SetupAPITest",
		"setupAPITest",
		"setupTestContext",               // Indirect setup via shared helper (calls SetupAPITest)
		"setupRolloverTest",              // modules/enrollment/http rollover integration tests — wraps SetupTestDB
		"setupRequestTest",               // modules/enrollment/http intake integration tests — wraps SetupTestDB
		"setupDecisionTest",              // modules/enrollment/http and careplan contracttest decision integration tests — wraps setupRolloverTest
		"setupCareTest",                  // careplan contracttest care-offering integration tests — wraps SetupTestDB
		"setupAutoApproveIntegrationEnv", // modules/enrollment/http auto-approve integration tests — wraps setupRolloverTest
		"setupGuardianInvitationTest",    // services/auth guardian invitation + related-accounts tests — wraps SetupTestDB
		"makeScenario",                   // modules/timetable/compose/httpintegration materialization/split/lifecycle integration tests — wraps SetupTestDB
		"makeRosterChain",                // modules/timetable/compose/httpintegration split-series roster tests (#2187) — wraps makeSeriesChain → makeScenario
		"makeMoveSetup",                  // modules/timetable/compose/httpintegration staff-move tests (#1884) — wraps SetupTestDB
		"buildDevSetup",                  // modules/timetable/http deviations/protocol tests — wraps SetupTestDB
		"setupAbsenceAdminTest",          // api/staff absence question tests (#1419) — wraps setupTestContext
		"newOverviewFixture",             // modules/studentpresence/internal/application/presence overview/export integration tests (#1417) — wraps SetupTestDB
		"setupOverviewAPI",               // api/staff overview/export tests (#1417) — wraps setupTestContext
		"newTransitionFixture",           // services/education grade-transition workflow tests — wraps SetupTestDB
		"buildLifecycle",                 // modules/timetable/compose/httpintegration instance-lifecycle tests — wraps SetupTestDB
		"newCareFixture",                 // modules/careplan/contracttest care-request tests — wraps SetupTestDB
		"setupDashboardContext",          // api/active supervision-dashboard tests — wraps SetupActiveModule → SetupTestDB
		"setupStudentsRoute",             // modules/peopledirectory/inbound/students route suites — wraps testutil.SetupStudentModule → SetupTestDB
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

	files, err := goSourceIndex(root)
	if err != nil {
		t.Fatalf("walk source files: %v", err)
	}
	for _, f := range files {
		if !strings.HasSuffix(f.rel, "_test.go") {
			continue
		}

		// Skip this verification test
		if strings.Contains(f.rel, "hermetic_verification_test.go") {
			continue
		}

		// Skip files that reference DB types but don't perform real DB operations
		skipFiles := []string{
			"parent_message_hooks_test.go",                               // Pure base.Model accessor unit test; no real DB
			"test/handler_layer_ratchet_test.go",                         // Source-scanning ratchet (issue #584); same as above, no DB is used
			"test/route_sized_api_builders_analyzer_test.go",             // Source-scanning gate fixtures; *bun.DB is a fixture return type, no DB is used
			"modules/timetable/http/timetable_data_test_helpers_test.go", // Shared fixture helper; caller tests own DB setup
			"test/module_file_size_ratchet_test.go",                      // Source-scanning ratchet (#2580); allowlist keys name *_repositories.go files, no DB is used
			"test/module_complexity_ratchet_test.go",                     // Source-scanning ratchet (#2580); same as above, no DB is used
			"modules/identityaccess/behavior/owner_contracts_test.go",    // Shared row readers of the behaviour suites (#3446); caller tests own DB setup
		}
		skip := false
		for _, sf := range skipFiles {
			if strings.Contains(f.rel, sf) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}

		contentStr := string(f.content)

		// Check if file has DB operations
		hasDBOps := false
		for _, pattern := range dbPatterns {
			if strings.Contains(contentStr, pattern) {
				hasDBOps = true
				break
			}
		}

		if !hasDBOps {
			continue
		}

		// Check if file uses SetupTestDB
		usesSetup := false
		for _, pattern := range setupPatterns {
			if strings.Contains(contentStr, pattern) {
				usesSetup = true
				break
			}
		}
		if !usesSetup && routeSizedSetupCall.MatchString(contentStr) {
			usesSetup = true
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
			violations = append(violations, "  - "+f.rel)
		}
	}

	return violations
}

// tenantContext1Baseline is empty: tests must generate or locally own tenant
// identity instead of relying on the fixed bootstrap tenant. A package absent
// from the baseline is allowed zero matches.
var tenantContext1Baseline = map[string]int{}

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
// written above a test). It iterates the shared goSourceIndex instead of
// walking the tree itself; the exclusions are unchanged.
func walkGoFilesRaw(root string, visit func(rel, pkg string, content []byte)) error {
	files, err := goSourceIndex(root)
	if err != nil {
		return err
	}
	for _, f := range files {
		if strings.HasPrefix(f.rel, "internal/testdb/") ||
			strings.HasSuffix(f.rel, "hermetic_verification_test.go") {
			continue
		}
		visit(f.rel, f.pkg, f.content)
	}
	return nil
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
		t.Fatalf("walk source files: %v", err)
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

type globalStateFile struct {
	rel  string
	text string
}

// checkParallelGlobalState finds tests that call t.Parallel() while reaching
// process-global state — directly, or through a helper anywhere in the same
// package. Resolving helpers per package (not per file) is the point: the
// #2419 sweep parallelised a whole file whose JWT-secret helper lived in a
// sibling file, and the restore in that helper's t.Cleanup broke unrelated
// tests running beside it.
func checkParallelGlobalState(t *testing.T, root string) []string {
	t.Helper()
	pkgFuncs := make(map[string]map[string]string)
	pkgFiles := make(map[string][]globalStateFile)
	err := walkGoFiles(root, true, func(rel, pkg string, code []byte) {
		text := string(code)
		if pkgFuncs[pkg] == nil {
			pkgFuncs[pkg] = make(map[string]string)
		}
		for _, f := range splitGoFuncs(text) {
			pkgFuncs[pkg][f.name] = f.body
		}
		pkgFiles[pkg] = append(pkgFiles[pkg], globalStateFile{rel: rel, text: text})
	})
	if err != nil {
		t.Fatalf("walk source files: %v", err)
	}

	var violations []string
	for pkg, funcs := range pkgFuncs {
		violations = append(violations, parallelGlobalStateViolations(pkgFiles[pkg], taintedGlobalHelpers(funcs))...)
	}
	sort.Strings(violations)
	return violations
}

func taintedGlobalHelpers(funcs map[string]string) map[string]bool {
	tainted := make(map[string]bool)
	for name, body := range funcs {
		if !strings.HasPrefix(name, "Test") && globalStateMutation.MatchString(body) {
			tainted[name] = true
		}
	}
	for range 5 {
		grew := false
		for name, body := range funcs {
			if !tainted[name] && !strings.HasPrefix(name, "Test") && callsAny(body, tainted) {
				tainted[name] = true
				grew = true
			}
		}
		if !grew {
			break
		}
	}
	return tainted
}

func parallelGlobalStateViolations(files []globalStateFile, tainted map[string]bool) []string {
	var violations []string
	for _, file := range files {
		for _, f := range splitGoFuncs(file.text) {
			parallel := strings.HasPrefix(f.name, "Test") && strings.Contains(f.body, "t.Parallel()")
			if parallel && (globalStateMutation.MatchString(f.body) || callsAny(f.body, tainted)) {
				violations = append(violations, "  "+file.rel+": "+f.name)
			}
		}
	}
	return violations
}

// sharedPoolAssign captures the variable a file binds the SHARED pool to —
// `db := testpkg.SetupTestDB(t)` or a two-result api/testutil setup builder.
// Only the first identifier can hold the pool in either signature.
var sharedPoolAssign = regexp.MustCompile(
	`(?m)^\s*(\w+)\s*(?:,\s*\w+\s*)?:?=\s*(?:\w+\.)?(?:SetupTestDB|SetupAPITest|Setup\w+Module)\(`)

// routeSizedSetupCall matches an actual builder invocation at the start of a
// statement. It deliberately does not match function declarations or test
// names that merely contain "Route(t".
var routeSizedSetupCall = regexp.MustCompile(
	`(?m)^\s*(?:[\w,\s]+:?=\s*|return\s+)?(?:\w+\.)?(?:setup|Setup|build)\w+(?:Route|Router|Module)\(t(?:,|\))`)

func usesSharedDBSetup(content []byte) bool {
	return bytes.Contains(content, []byte("SetupTestDB(")) ||
		bytes.Contains(content, []byte("SetupAPITest(")) ||
		routeSizedSetupCall.Match(content)
}

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
	err := walkGoFilesRaw(root, func(rel, _ string, content []byte) {
		if strings.HasSuffix(rel, "_test.go") {
			violations = append(violations, sharedPoolCloseViolations(rel, string(content))...)
		}
	})
	if err != nil {
		t.Fatalf("walk source files: %v", err)
	}
	return violations
}

func sharedPoolCloseViolations(rel, text string) []string {
	shared := make(map[string]bool)
	for _, match := range sharedPoolAssign.FindAllStringSubmatch(text, -1) {
		shared[match[1]] = true
	}
	// A name rebound to a private pool is ambiguous; avoid a false positive.
	for _, match := range privatePoolAssign.FindAllStringSubmatch(text, -1) {
		delete(shared, match[1])
	}

	var violations []string
	for i, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "//") {
			continue
		}
		for name := range shared {
			closeCall := regexp.MustCompile(`\b` + regexp.QuoteMeta(name) + `\.Close\(\)`)
			if closeCall.MatchString(line) {
				violations = append(violations, formatViolation(rel, i+1, strings.TrimSpace(line)))
				break
			}
		}
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
	files, err := goSourceIndex(root)
	if err != nil {
		t.Fatalf("walk source files: %v", err)
	}
	for _, f := range files {
		if !strings.HasPrefix(f.rel, "test/") {
			continue
		}
		// Skip this file — it intentionally contains the pattern in error messages.
		if filepath.Base(f.rel) == "hermetic_verification_test.go" {
			continue
		}

		scanner := bufio.NewScanner(bytes.NewReader(f.content))
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			line := scanner.Text()

			if strings.HasPrefix(strings.TrimSpace(line), "//") {
				continue
			}

			if brokenPattern.MatchString(line) {
				violations = append(violations,
					formatViolation(f.rel, lineNum, strings.TrimSpace(line)))
			}
		}
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
	// test/ imports the JWT adapter, so its internal tests cannot import test/.
	"modules/identityaccess/sessiontoken": "import cycle: test imports modules/identityaccess/sessiontoken",
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
		if usesSharedDBSetup(content) {
			usesDB[pkg] = true
		}
		if bytes.Contains(content, []byte("testpkg.Run(m)")) || bytes.Contains(content, []byte("\tRun(m)")) {
			gated[pkg] = true
		}
	})
	if err != nil {
		t.Fatalf("walk source files: %v", err)
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
	usesDB, optedIn := make(map[string]bool), make(map[string]bool)
	err := walkGoFilesRaw(root, func(rel, pkg string, content []byte) {
		if !strings.HasSuffix(rel, "_test.go") {
			return
		}
		if usesSharedDBSetup(content) {
			usesDB[pkg] = true
		}
		if bytes.Contains(content, []byte("PerTestTenants()")) {
			optedIn[pkg] = true
		}
	})
	if err != nil {
		t.Fatalf("walk source files: %v", err)
	}

	var violations []string
	for pkg := range usesDB {
		if optedIn[pkg] {
			continue
		}
		if _, ok := perTestTenantsOptOut[pkg]; ok {
			continue
		}
		violations = append(violations, "  "+pkg)
	}
	sort.Strings(violations)
	return violations
}

// The baseline is intentionally empty: serial top-level tests are forbidden.
var serialTestBaseline = map[string]int{}

// topLevelTestDecl matches a top-level test function declaration.
var topLevelTestDecl = regexp.MustCompile(`(?m)^func (Test\w+)\(\w+ \*testing\.T\) \{`)

// topLevelParallelCall matches the Parallel() call of the TEST ITSELF: one
// tab of indentation, nothing else on the line. Searching for ".Parallel()"
// anywhere in the body would also count a test whose SUBTESTS are parallel
// while the test itself is not — precisely the case this ratchet exists to
// count.
var topLevelParallelCall = regexp.MustCompile(`(?m)^\t\w+\.Parallel\(\)$`)

func checkSerialTests(t *testing.T, root string) map[string]int {
	t.Helper()

	counts := make(map[string]int)
	err := walkGoFilesRaw(root, func(_, pkg string, content []byte) {
		for _, f := range splitGoFuncs(string(content)) {
			if !topLevelTestDecl.MatchString(f.body) || topLevelParallelCall.MatchString(f.body) {
				continue
			}
			counts[pkg]++
		}
	})
	if err != nil {
		t.Fatalf("walk source files: %v", err)
	}
	return counts
}

func TestIgnoredCleanupDeletesFindsDiscardedDeletesInCleanupCode(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		code string
		want []int
	}{
		{
			name: "t.Cleanup closure",
			code: `package sample
func TestX(t *testing.T) {
	t.Cleanup(func() {
		_, _ = db.NewDelete().TableExpr("enrollment.phases").Where("id = ?", id).Exec(ctx)
	})
}`,
			want: []int{4},
		},
		{
			name: "deferred closure with raw delete",
			code: `package sample
func TestX(t *testing.T) {
	defer func() {
		_, _ = db.NewRaw("DELETE FROM enrollment.phases WHERE id = ?", id).Exec(ctx)
	}()
}`,
			want: []int{4},
		},
		{
			name: "returned cleanup closure with bare exec",
			code: `package sample
func setup(t *testing.T) (*env, func()) {
	cleanup := func() {
		db.NewDelete().TableExpr("enrollment.requests").Where("phase_id = ?", id).Exec(ctx)
	}
	return &env{}, cleanup
}`,
			want: []int{4},
		},
		{
			name: "closure returned directly",
			code: `package sample
func setup(t *testing.T) func() {
	return func() {
		_, _ = db.NewDelete().TableExpr("enrollment.requests").Exec(ctx)
	}
}`,
			want: []int{4},
		},
		{
			name: "wipe helper",
			code: `package sample
func wipePhases(db *bun.DB, tenantID int64) {
	_, _ = db.NewDelete().TableExpr("enrollment.phases").Where("tenant_id = ?", tenantID).Exec(context.Background())
}`,
			want: []int{3},
		},
		{
			name: "deferred delete call",
			code: `package sample
func TestX(t *testing.T) {
	defer db.NewDelete().TableExpr("users.persons").Where("id = ?", id).Exec(ctx)
}`,
			want: []int{3},
		},
		{
			name: "exec context delete in cleanup",
			code: `package sample
func TestX(t *testing.T) {
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, "DELETE FROM auth.accounts WHERE id = $1", id) })
}`,
			want: []int{3},
		},
		{
			name: "named function registered as cleanup",
			code: `package sample
func dropRows() { _, _ = db.NewDelete().TableExpr("users.persons").Exec(ctx) }
func TestX(t *testing.T) {
	t.Cleanup(dropRows)
}`,
			want: []int{2},
		},
		{
			name: "deferred named helper",
			code: `package sample
func dropRows(id int64) { _, _ = db.NewDelete().TableExpr("users.persons").Exec(ctx) }
func TestX(t *testing.T) {
	defer dropRows(1)
}`,
			want: []int{2},
		},
		{
			name: "helper called from cleanup closure",
			code: `package sample
func dropRows(id int64) {
	_, _ = db.NewDelete().TableExpr("users.persons").Where("id = ?", id).Exec(ctx)
}
func TestX(t *testing.T) {
	t.Cleanup(func() { dropRows(1) })
}`,
			want: []int{3},
		},
		{
			name: "trailing delete at the end of a subtest",
			code: `package sample
func TestX(t *testing.T) {
	t.Run("case", func(t *testing.T) {
		assert.True(t, ok)
		// Cleanup
		_, _ = db.NewDelete().TableExpr("schedule.student_pickup_schedules").Where("id = ?", id).Exec(ctx)
	})
}`,
			want: []int{6},
		},
		{
			name: "trailing delete at the end of a test",
			code: `package sample
func TestX(t *testing.T) {
	assert.True(t, ok)
	_, _ = db.NewDelete().TableExpr("users.persons").Where("id = ?", a).Exec(ctx)
	_, _ = db.NewDelete().TableExpr("users.persons").Where("id = ?", b).Exec(ctx)
}`,
			want: []int{4, 5},
		},
		{
			name: "closure appended to a cleanup list",
			code: `package sample
func setup(s *scenario) {
	s.extraCleanups = append([]func(){func() {
		_, _ = s.db.NewRaw("DELETE FROM schedule.instance_students WHERE instance_id = ?", id).Exec(ctx)
	}}, s.extraCleanups...)
}`,
			want: []int{4},
		},
		{
			name: "trailing delete inside a final if block",
			code: `package sample
func TestX(t *testing.T) {
	assert.True(t, ok)
	if id, ok := data["id"].(float64); ok {
		_, _ = db.NewDelete().TableExpr("auth.invitation_tokens").Where("id = ?", int64(id)).Exec(ctx)
	}
}`,
			want: []int{5},
		},
		{
			name: "method called from cleanup",
			code: `package sample
func (e *env) dropRows() { _, _ = e.db.NewDelete().TableExpr("users.persons").Exec(ctx) }
func TestX(t *testing.T) {
	t.Cleanup(func() { e.dropRows() })
}`,
			want: []int{2},
		},
		{
			name: "delete SQL in a constant",
			code: `package sample
const dropPersons = "DELETE FROM users.persons WHERE id = $1"
func TestX(t *testing.T) {
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, dropPersons, id) })
}`,
			want: []int{4},
		},
		{
			name: "delete in the middle of a test is arrangement",
			code: `package sample
func TestX(t *testing.T) {
	_, _ = db.NewDelete().TableExpr("users.persons").Where("id = ?", id).Exec(ctx)
	assert.True(t, ok)
}`,
		},
		{
			name: "trailing delete in a plain helper",
			code: `package sample
func removePerson(id int64) {
	_, _ = db.NewDelete().TableExpr("users.persons").Where("id = ?", id).Exec(ctx)
}`,
		},
		{
			name: "truncate in cleanup",
			code: `package sample
func TestX(t *testing.T) {
	t.Cleanup(func() { _, _ = db.NewTruncateTable().TableExpr("x").Exec(ctx) })
}`,
			want: []int{3},
		},
		{
			name: "checked delete in cleanup",
			code: `package sample
func TestX(t *testing.T) {
	t.Cleanup(func() {
		_, err := db.NewDelete().TableExpr("auth.accounts").Where("id = ?", id).Exec(ctx)
		require.NoError(t, err)
	})
}`,
		},
		{
			name: "ignored non-delete in cleanup",
			code: `package sample
func TestX(t *testing.T) {
	t.Cleanup(func() {
		_, _ = db.NewUpdate().TableExpr("users.persons").Set("x = 1").Exec(ctx)
		_, _ = db.ExecContext(ctx, "SELECT pg_advisory_unlock(1)")
		_ = server.Close()
	})
}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := ignoredCleanupDeletes("sample_test.go", []byte(tc.code))
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

// cleanupHelperName matches helpers whose name says they exist to tear rows
// down again. Their bodies count as cleanup code wherever they are called.
var cleanupHelperName = regexp.MustCompile(`(?i)^(wipe|cleanup|teardown)`)

// cleanupVariableName matches closures bound to a name that marks them as
// teardown, typically returned by a setup helper and deferred by its caller.
var cleanupVariableName = regexp.MustCompile(`(?i)(cleanup|teardown|wipe)`)

// rawDeleteSQL matches SQL text that deletes rows.
var rawDeleteSQL = regexp.MustCompile(`(?i)^\s*(DELETE|TRUNCATE)\b`)

// checkIgnoredCleanupDeletes reports discarded DB deletes in test cleanup
// code. The package clone and per-test tenants already own every tenant row;
// a delete whose error is thrown away in t.Cleanup, a deferred call or a
// wipe* helper either does nothing useful or hides that it failed.
func checkIgnoredCleanupDeletes(t *testing.T, root string) []string {
	t.Helper()
	var violations []string
	err := walkGoFilesRaw(root, func(rel, _ string, content []byte) {
		if !strings.HasSuffix(rel, "_test.go") {
			return
		}
		lines, err := ignoredCleanupDeletes(rel, content)
		if err != nil {
			violations = append(violations, fmt.Sprintf("  %s: parse: %v", rel, err))
			return
		}
		source := strings.Split(string(content), "\n")
		for _, line := range lines {
			violations = append(violations, formatViolation(rel, line, strings.TrimSpace(source[line-1])))
		}
	})
	if err != nil {
		t.Fatalf("walk source files: %v", err)
	}
	return violations
}

// ignoredCleanupDeletes returns the lines of DB deletes whose result is
// discarded inside cleanup code: a closure or function registered with
// Cleanup, anything deferred, a closure returned or bound to a cleanup name,
// wipe*/cleanup*/teardown* helpers, same-file functions and methods those
// call, and the trailing statements of a test or subtest body.
//
// Known limits: helpers in other files are not followed, SQL is recognised
// only as a string literal or a file-level string constant, and a delete
// whose error is merely logged counts as handled.
func ignoredCleanupDeletes(filename string, code []byte) ([]int, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, code, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}

	var bodies []ast.Node
	flagged := make(map[int]bool)
	cleanupNames := make(map[string]bool)
	consts := fileStringConstants(file)
	isDelete := func(call *ast.CallExpr) bool { return isDeleteExec(call, consts) }
	var flagTrailing func(body *ast.BlockStmt)
	flagTrailing = func(body *ast.BlockStmt) {
		for i := len(body.List) - 1; i >= 0; i-- {
			if ifStmt, ok := body.List[i].(*ast.IfStmt); ok && i == len(body.List)-1 && ifStmt.Else == nil {
				flagTrailing(ifStmt.Body)
				return
			}
			call := discardedCall(body.List[i])
			if call == nil || !isDelete(call) {
				return
			}
			flagged[fset.Position(call.Pos()).Line] = true
		}
	}

	register := func(expr ast.Expr) {
		switch fn := expr.(type) {
		case *ast.FuncLit:
			bodies = append(bodies, fn.Body)
		case *ast.Ident:
			cleanupNames[fn.Name] = true
		case *ast.SelectorExpr:
			cleanupNames[fn.Sel.Name] = true
		}
	}

	ast.Inspect(file, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.CallExpr:
			sel, ok := n.Fun.(*ast.SelectorExpr)
			if ok && sel.Sel.Name == "Cleanup" && len(n.Args) == 1 {
				register(n.Args[0])
			}
			if ok && sel.Sel.Name == "Run" && len(n.Args) == 2 {
				if fn, ok := n.Args[1].(*ast.FuncLit); ok {
					flagTrailing(fn.Body)
				}
			}
		case *ast.DeferStmt:
			if isDelete(n.Call) {
				flagged[fset.Position(n.Call.Pos()).Line] = true
			}
			register(n.Call.Fun)
		case *ast.ReturnStmt:
			for _, result := range n.Results {
				if fn, ok := result.(*ast.FuncLit); ok {
					bodies = append(bodies, fn.Body)
				}
			}
		}
		return true
	})

	funcs := make(map[string]*ast.BlockStmt)
	methods := make(map[string]*ast.BlockStmt)
	ast.Inspect(file, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.FuncDecl:
			if n.Body == nil {
				return true
			}
			if n.Recv == nil {
				funcs[n.Name.Name] = n.Body
				if strings.HasPrefix(n.Name.Name, "Test") {
					flagTrailing(n.Body)
				}
			} else {
				methods[n.Name.Name] = n.Body
			}
			if cleanupHelperName.MatchString(n.Name.Name) || cleanupNames[n.Name.Name] {
				bodies = append(bodies, n.Body)
			}
		case *ast.AssignStmt:
			for i, lhs := range n.Lhs {
				if i >= len(n.Rhs) {
					continue
				}
				var name string
				switch target := lhs.(type) {
				case *ast.Ident:
					name = target.Name
				case *ast.SelectorExpr:
					name = target.Sel.Name
				default:
					continue
				}
				if !cleanupNames[name] && !cleanupVariableName.MatchString(name) {
					continue
				}
				// Every closure bound to a cleanup name, including those
				// appended to a list of cleanups, is cleanup code.
				ast.Inspect(n.Rhs[i], func(rhs ast.Node) bool {
					if fn, ok := rhs.(*ast.FuncLit); ok {
						bodies = append(bodies, fn.Body)
						return false
					}
					return true
				})
			}
		}
		return true
	})

	// Same-file functions and methods that cleanup code calls are cleanup
	// code too.
	visited := make(map[ast.Node]bool)
	for i := 0; i < len(bodies); i++ {
		body := bodies[i]
		if visited[body] {
			continue
		}
		visited[body] = true
		ast.Inspect(body, func(node ast.Node) bool {
			if call, ok := node.(*ast.CallExpr); ok {
				switch fn := call.Fun.(type) {
				case *ast.Ident:
					if funcs[fn.Name] != nil {
						bodies = append(bodies, funcs[fn.Name])
					}
				case *ast.SelectorExpr:
					if methods[fn.Sel.Name] != nil {
						bodies = append(bodies, methods[fn.Sel.Name])
					}
				}
			}
			if stmt, ok := node.(ast.Stmt); ok {
				if call := discardedCall(stmt); call != nil && isDelete(call) {
					flagged[fset.Position(call.Pos()).Line] = true
				}
			}
			return true
		})
	}

	lines := make([]int, 0, len(flagged))
	for line := range flagged {
		lines = append(lines, line)
	}
	sort.Ints(lines)
	if len(lines) == 0 {
		return nil, nil
	}
	return lines, nil
}

// discardedCall returns the call a statement runs while discarding its
// result: a bare call statement or an assignment to blanks only.
func discardedCall(stmt ast.Stmt) *ast.CallExpr {
	var expr ast.Expr
	switch n := stmt.(type) {
	case *ast.ExprStmt:
		expr = n.X
	case *ast.AssignStmt:
		if len(n.Rhs) == 1 && allBlank(n.Lhs) {
			expr = n.Rhs[0]
		}
	}
	call, _ := expr.(*ast.CallExpr)
	return call
}

// fileStringConstants maps file-level constants and variables bound to a
// string literal to their text, so SQL kept in a named constant is still
// recognised.
func fileStringConstants(file *ast.File) map[string]string {
	consts := make(map[string]string)
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || (gen.Tok != token.CONST && gen.Tok != token.VAR) {
			continue
		}
		for _, spec := range gen.Specs {
			value, ok := spec.(*ast.ValueSpec)
			if !ok || len(value.Names) != len(value.Values) {
				continue
			}
			for i, name := range value.Names {
				lit, ok := value.Values[i].(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					continue
				}
				if text, err := strconv.Unquote(lit.Value); err == nil {
					consts[name.Name] = text
				}
			}
		}
	}
	return consts
}

func allBlank(exprs []ast.Expr) bool {
	for _, expr := range exprs {
		if ident, ok := expr.(*ast.Ident); !ok || ident.Name != "_" {
			return false
		}
	}
	return true
}

// isDeleteExec reports whether call executes a query that deletes rows: an
// Exec/ExecContext whose bun chain starts with NewDelete or NewTruncateTable,
// or whose SQL text (directly or through NewRaw) starts with DELETE/TRUNCATE.
func isDeleteExec(call *ast.CallExpr, consts map[string]string) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || (sel.Sel.Name != "Exec" && sel.Sel.Name != "ExecContext") {
		return false
	}
	if hasDeleteSQL(call.Args, consts) {
		return true
	}
	for expr := sel.X; ; {
		inner, ok := expr.(*ast.CallExpr)
		if !ok {
			return false
		}
		innerSel, ok := inner.Fun.(*ast.SelectorExpr)
		if !ok {
			return false
		}
		switch innerSel.Sel.Name {
		case "NewDelete", "NewTruncateTable":
			return true
		case "NewRaw":
			return hasDeleteSQL(inner.Args, consts)
		}
		expr = innerSel.X
	}
}

func hasDeleteSQL(args []ast.Expr, consts map[string]string) bool {
	for _, arg := range args {
		var text string
		switch value := arg.(type) {
		case *ast.BasicLit:
			if value.Kind != token.STRING {
				continue
			}
			unquoted, err := strconv.Unquote(value.Value)
			if err != nil {
				continue
			}
			text = unquoted
		case *ast.Ident:
			text = consts[value.Name]
		default:
			continue
		}
		if rawDeleteSQL.MatchString(text) {
			return true
		}
	}
	return false
}
