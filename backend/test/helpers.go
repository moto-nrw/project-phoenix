package test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	"github.com/subosito/gotenv"
	"github.com/uptrace/bun"
)

// FindProjectRoot walks up the directory tree from the current working directory
// until it finds a directory containing go.mod. Returns the parent of that directory
// (the actual project root where .env lives).
//
// This approach is self-healing: it works regardless of how deep the test file is
// in the directory structure, eliminating fragile "../.." path counting.
func FindProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		// Check if go.mod exists in this directory
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			// Found backend/go.mod, return parent (project-phoenix/)
			return filepath.Dir(dir), nil
		}

		// Move up one directory
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root without finding go.mod
			return "", os.ErrNotExist
		}
		dir = parent
	}
}

// LoadTestEnv loads the .env file from the project root.
// This is the standard way to configure test database connections.
//
// Usage in test files:
//
//	func setupTestDB(t *testing.T) *bun.DB {
//	    testpkg.LoadTestEnv(t)
//	    // ... rest of setup
//	}
func LoadTestEnv(t *testing.T) {
	t.Helper()

	projectRoot, err := FindProjectRoot()
	if err != nil {
		t.Logf("Warning: Could not find project root: %v", err)
		return
	}

	envPath := filepath.Join(projectRoot, ".env")
	if err := gotenv.Load(envPath); err != nil {
		t.Logf("Warning: Could not load %s: %v", envPath, err)
	}
}

// SetupTestDB creates a test database connection using the standard configuration.
// It automatically loads .env from project root and configures the database DSN.
//
// This is the preferred way to get a database connection in tests:
//
//	func TestSomething(t *testing.T) {
//	    db := testpkg.SetupTestDB(t)
//	    defer db.Close()
//	    // ... test code
//	}
func SetupTestDB(t *testing.T) *bun.DB {
	t.Helper()

	// -short überspringt alle DB-Integrationstests: `go test -short ./...` ist
	// der schnelle Inner-Loop (Unit-/AST-/Ratchet-Tests, keine Postgres-Last).
	// -short darf NIE in CI laufen — es würde coverage.out aushöhlen und das
	// Sonar-Gate (80% on new code) bedeutungslos machen, während junit Skips
	// als grün meldet. Es überspringt auch TestRouteTableGolden und test/e2e —
	// ein Filter für die lokale Entwicklungsschleife, kein Pre-Merge-Check.
	if testing.Short() {
		t.Skip("skipping DB integration test in -short mode")
	}

	// Force test environment so database config always resolves to the test DB,
	// regardless of how `go test` was invoked (with or without APP_ENV=test).
	// t.Setenv automatically restores the original value when the test finishes.
	t.Setenv("APP_ENV", "test")

	// Load .env from project root (contains TEST_DB_DSN)
	LoadTestEnv(t)

	// Initialize viper to read environment variables
	viper.AutomaticEnv()

	// Require explicit TEST_DB_DSN - fail fast with clear instructions if missing.
	// This follows the HashiCorp pattern: test database config should be explicit,
	// not guessed from runtime config like GetDatabaseDSN().
	dsn := os.Getenv("TEST_DB_DSN")
	if dsn == "" {
		t.Fatal(`Test database not configured.

To run integration tests:
  1. Start test database: docker compose --profile test up -d postgres-test
  2. Ensure .env contains: TEST_DB_DSN=postgres://postgres:postgres@localhost:5433/phoenix_test?sslmode=disable

For CI, set TEST_DB_DSN as an environment variable.`)
	}

	dsn = packageIsolatedTestDSN(t, dsn)

	// APP_ENV=test deliberately resolves only TEST_DB_DSN. Point that key at
	// the package-isolated clone; setting db_dsn here would be ignored and
	// would silently send integration tests to the shared template database.
	viper.Set("test_db_dsn", dsn)
	viper.Set("db_debug", false) // Set to true for SQL debugging
	// Cap per-test pool size. Each call to SetupTestDB opens a fresh
	// pool; the prod default is 25 connections, which exhausts
	// Postgres's max_connections (~100) once gotestsum runs the test
	// packages in parallel. 3 conns/test fits comfortably and is more
	// than enough for the test fixture chain (1-2 statements at a
	// time).
	viper.Set("db_max_open_conns", 3)
	viper.Set("db_max_idle_conns", 1)

	db, err := database.DBConn()
	require.NoError(t, err, "Failed to connect to test database")

	// Set search_path to include all project schemas.
	// BUN's Relation() JOIN generation sometimes uses unqualified table names,
	// which fails when the target schema isn't in search_path.
	_, _ = db.ExecContext(context.Background(),
		`SET search_path TO public, platform, auth, users, education, facilities, activities, active, schedule, iot, feedback, config, meta, audit`)

	// Spread PK sequences into disjoint per-table ranges so fixture IDs from
	// different tables never collide numerically (see sequence_offsets.go).
	applySequenceOffsets(t, db)

	// Ensure the default tenant (school ID 1) exists in platform.schools.
	// All fixtures use tenant_id=1, which requires a FK target row.
	EnsureTestTenant(t, db, 1)

	// Ensure default fallback room exists (ID 1).
	// session_service.go:determineRoomIDWithStrategy uses hardcoded fallback to room 1
	// when no room is specified and no planned room exists.
	_, _ = db.ExecContext(context.Background(), `
		INSERT INTO facilities.rooms (id, tenant_id, name, building)
		VALUES (1, 1, 'Default Room', 'Default')
		ON CONFLICT (id) DO NOTHING`)

	// Ensure default system staff exists (person ID 1, staff ID 1) for legacy
	// tests that hardcode CreatedBy: 1 to satisfy created_by FK constraints on
	// schedule/activity tables. Same convention as rooms.id=1 and schools.id=1.
	// Fail fast: if the fixture insert breaks (schema drift, missing schools FK),
	// downstream tests would otherwise fail with cryptic "missing staff" errors.
	//
	// Wrap both inserts in a transaction so they're atomic from the perspective
	// of parallel test packages. Otherwise a concurrent CleanupActivityFixtures
	// could delete the persons row between the two inserts, leaving the staff
	// FK to fail. Use unconstrained ON CONFLICT DO NOTHING (rather than
	// ON CONFLICT (id)) because users.persons also carries
	// idx_persons_tenant_pk UNIQUE(tenant_id, id) and Postgres reports that
	// index first under load, so a column-targeted ON CONFLICT leaks the error.
	// After the upserts, reconcile any wrong-tenant row so the system fixture
	// always ends up at (tenant_id=1, id=1).
	ctx := context.Background()
	require.NoError(t, db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		// Serialize concurrent fixture setup across parallel test packages.
		// pg_advisory_xact_lock is released automatically on commit/rollback,
		// so other packages wait at this line instead of racing the inserts
		// below. Constant must stay identical wherever this lock is taken.
		if _, e := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(914735691)`); e != nil {
			return fmt.Errorf("acquire system fixture advisory lock: %w", e)
		}
		if _, e := tx.ExecContext(ctx, `
			INSERT INTO users.persons (id, tenant_id, first_name, last_name)
			VALUES (1, 1, 'System', 'Test')
			ON CONFLICT DO NOTHING`); e != nil {
			return fmt.Errorf("ensure system person fixture (id=1): %w", e)
		}
		if _, e := tx.ExecContext(ctx, `
			UPDATE users.persons SET tenant_id = 1, first_name = 'System', last_name = 'Test'
			WHERE id = 1 AND tenant_id <> 1`); e != nil {
			return fmt.Errorf("reconcile system person tenant_id (id=1): %w", e)
		}
		if _, e := tx.ExecContext(ctx, `
			INSERT INTO users.staff (id, tenant_id, person_id)
			VALUES (1, 1, 1)
			ON CONFLICT DO NOTHING`); e != nil {
			return fmt.Errorf("ensure system staff fixture (id=1): %w", e)
		}
		if _, e := tx.ExecContext(ctx, `
			UPDATE users.staff SET tenant_id = 1, person_id = 1
			WHERE id = 1 AND (tenant_id <> 1 OR person_id <> 1)`); e != nil {
			return fmt.Errorf("reconcile system staff tenant_id (id=1): %w", e)
		}
		return nil
	}), "SetupTestDB: failed to ensure system person/staff fixtures (id=1)")

	// Advance the BIGSERIAL sequence past any explicitly-inserted IDs.
	// Uses nextval (atomic, never goes backwards) instead of setval to avoid
	// races when parallel test packages call SetupTestDB concurrently.
	_, _ = db.ExecContext(context.Background(), `
		DO $$ DECLARE max_id bigint;
		BEGIN
			SELECT COALESCE(MAX(id), 0) INTO max_id FROM facilities.rooms;
			WHILE nextval('facilities.rooms_id_seq') < max_id LOOP END LOOP;
			SELECT COALESCE(MAX(id), 0) INTO max_id FROM users.persons;
			WHILE nextval('users.persons_id_seq') < max_id LOOP END LOOP;
			SELECT COALESCE(MAX(id), 0) INTO max_id FROM users.staff;
			WHILE nextval('users.staff_id_seq') < max_id LOOP END LOOP;
		END $$`)

	return db
}

// ============================================================================
// Generic Cleanup Helpers
// ============================================================================

// CleanupTableRecords removes records from a schema-qualified table by ID.
// Use this for simple single-table cleanup without FK dependencies.
//
// Usage:
//
//	defer testpkg.CleanupTableRecords(t, db, "facilities.rooms", room.ID)
//	defer testpkg.CleanupTableRecords(t, db, "iot.devices", device1.ID, device2.ID)
func CleanupTableRecords(tb testing.TB, db *bun.DB, table string, ids ...int64) {
	tb.Helper()
	if len(ids) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := db.NewDelete().
		TableExpr(table).
		Where("id IN (?)", bun.List(ids)).
		Exec(ctx)
	if err != nil {
		tb.Logf("Warning: failed to cleanup %s: %v", table, err)
	}
}

// CleanupTableRecordsByStringID removes records from a table by string ID.
// Use this for tables with string primary keys (e.g., RFID cards).
func CleanupTableRecordsByStringID(tb testing.TB, db *bun.DB, table string, ids ...string) {
	tb.Helper()
	if len(ids) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := db.NewDelete().
		TableExpr(table).
		Where("id IN (?)", bun.List(ids)).
		Exec(ctx)
	if err != nil {
		tb.Logf("Warning: failed to cleanup %s: %v", table, err)
	}
}

// CleanupRateLimitsByEmail removes password reset rate limit records by email.
// Use this for cleaning up after password reset rate limit tests.
// The rate limit table uses email as the primary key, not an integer ID.
//
// Usage:
//
//	defer testpkg.CleanupRateLimitsByEmail(t, db, email1, email2)
func CleanupRateLimitsByEmail(tb testing.TB, db *bun.DB, emails ...string) {
	tb.Helper()
	if len(emails) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := db.NewDelete().
		TableExpr("auth.password_reset_rate_limits").
		Where("email IN (?)", bun.List(emails)).
		Exec(ctx)
	if err != nil {
		tb.Logf("Warning: failed to cleanup auth.password_reset_rate_limits: %v", err)
	}
}

// ============================================================================
// Context Helpers
// ============================================================================

// TenantContext returns a context with tenant_id set.
// Use this in service and repository tests that call methods requiring tenant context.
// Without tenant context, EnsureTenantID silently leaves tenant_id=0, which violates
// FK constraints on tenant-scoped tables.
func TenantContext(tenantID int64) context.Context {
	return tenant.WithTenantID(context.Background(), tenantID)
}

// TenantScope owns a unique tenant for a test and provides the matching context.
type TenantScope struct {
	TenantID int64
}

// NewTenantScope creates a unique tenant and returns a scope for tests that
// assert counts or shared-table state and therefore must not use tenant_id=1.
func NewTenantScope(tb testing.TB, db *bun.DB) TenantScope {
	tb.Helper()

	tenantID := UniqueTestTenantID(tb)
	EnsureTestTenant(tb, db, tenantID)

	return TenantScope{TenantID: tenantID}
}

// Context returns a background context scoped to the test tenant.
func (s TenantScope) Context() context.Context {
	return TenantContext(s.TenantID)
}

var uniqueTestTenantCounter int64

const (
	// Tenant IDs handed out by CreateTestTenant live in
	// [testTenantIDBase, testTenantIDCeiling). The band is high enough not to
	// collide with the literal IDs older fixtures hardcode (1, 42, …) and low
	// enough that the ID survives a JWT round-trip — JSON numbers decode as
	// float64, which is exact only below 2^53.
	testTenantIDBase    int64 = 1_000_000_000
	testTenantIDCeiling int64 = 2_000_000_000
)

// uniqueJWTSafeTenantID hands out a distinct tenant ID inside the band above.
// The PID slice keeps concurrent `go test` processes sharing one database out
// of each other's way; the counter separates tests inside one process.
func uniqueJWTSafeTenantID() int64 {
	return testTenantIDBase +
		int64(os.Getpid()%10_000)*100_000 +
		atomic.AddInt64(&uniqueTestTenantCounter, 1)%100_000
}

// UniqueTestTenantID returns a high, process-local tenant ID for tests that
// assert aggregate counts and therefore must not share tenant_id=1.
func UniqueTestTenantID(tb testing.TB) int64 {
	tb.Helper()
	return time.Now().UnixNano() + int64(os.Getpid()) + atomic.AddInt64(&uniqueTestTenantCounter, 1)
}

// ============================================================================
// Pointer Helpers
// ============================================================================

// IntPtr returns a pointer to the given int value.
func IntPtr(i int) *int { return &i }

// Int16Ptr returns a pointer to the given int16 value.
func Int16Ptr(i int16) *int16 { return &i }

// StrPtr returns a pointer to the given string value.
func StrPtr(s string) *string { return &s }

// Int64Ptr returns a pointer to the given int64 value.
func Int64Ptr(i int64) *int64 { return &i }

// TimePtr returns a pointer to the given time value.
func TimePtr(t time.Time) *time.Time { return &t }
