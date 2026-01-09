package test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/joho/godotenv"
	"github.com/moto-nrw/project-phoenix/database"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
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
	if err := godotenv.Load(envPath); err != nil {
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

	viper.Set("db_dsn", dsn)
	viper.Set("db_debug", false) // Set to true for SQL debugging

	db, err := database.DBConn()
	require.NoError(t, err, "Failed to connect to test database")

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
		Where("id IN (?)", bun.In(ids)).
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
		Where("id IN (?)", bun.In(ids)).
		Exec(ctx)
	if err != nil {
		tb.Logf("Warning: failed to cleanup %s: %v", table, err)
	}
}

// ============================================================================
// Domain-Specific Cleanup Helpers
// ============================================================================

// CleanupAuthRecords removes auth-domain records by their specific IDs.
// Unlike CleanupActivityFixtures, this ONLY deletes from auth.* tables,
// preventing accidental deletion of unrelated records.
//
// Parameters are explicit to avoid ID collision across domains:
//   - accountIDs: IDs from auth.accounts table
//   - roleIDs: IDs from auth.roles table (pass nil if none)
//   - permissionIDs: IDs from auth.permissions table (pass nil if none)
//
// Usage:
//
//	account := CreateTestAccount(t, db, "test@example.com")
//	role := CreateTestRole(t, db, "test-role")
//	defer CleanupAuthRecords(t, db, []int64{account.ID}, []int64{role.ID}, nil)
func CleanupAuthRecords(tb testing.TB, db *bun.DB, accountIDs, roleIDs, permissionIDs []int64) {
	tb.Helper()

	if len(accountIDs) == 0 && len(roleIDs) == 0 && len(permissionIDs) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 1. Delete tokens (depends on accounts)
	if len(accountIDs) > 0 {
		_, _ = db.NewDelete().
			TableExpr("auth.tokens").
			Where("account_id IN (?)", bun.In(accountIDs)).
			Exec(ctx)
	}

	// 2. Delete account_roles (depends on accounts and roles)
	if len(accountIDs) > 0 {
		_, _ = db.NewDelete().
			TableExpr("auth.account_roles").
			Where("account_id IN (?)", bun.In(accountIDs)).
			Exec(ctx)
	}
	if len(roleIDs) > 0 {
		_, _ = db.NewDelete().
			TableExpr("auth.account_roles").
			Where("role_id IN (?)", bun.In(roleIDs)).
			Exec(ctx)
	}

	// 3. Delete account_permissions (depends on accounts and permissions)
	if len(accountIDs) > 0 {
		_, _ = db.NewDelete().
			TableExpr("auth.account_permissions").
			Where("account_id IN (?)", bun.In(accountIDs)).
			Exec(ctx)
	}
	if len(permissionIDs) > 0 {
		_, _ = db.NewDelete().
			TableExpr("auth.account_permissions").
			Where("permission_id IN (?)", bun.In(permissionIDs)).
			Exec(ctx)
	}

	// 4. Delete role_permissions (depends on roles and permissions)
	if len(roleIDs) > 0 {
		_, _ = db.NewDelete().
			TableExpr("auth.role_permissions").
			Where("role_id IN (?)", bun.In(roleIDs)).
			Exec(ctx)
	}
	if len(permissionIDs) > 0 {
		_, _ = db.NewDelete().
			TableExpr("auth.role_permissions").
			Where("permission_id IN (?)", bun.In(permissionIDs)).
			Exec(ctx)
	}

	// 5. Delete roles
	if len(roleIDs) > 0 {
		CleanupTableRecords(tb, db, "auth.roles", roleIDs...)
	}

	// 6. Delete permissions
	if len(permissionIDs) > 0 {
		CleanupTableRecords(tb, db, "auth.permissions", permissionIDs...)
	}

	// 7. Delete accounts
	if len(accountIDs) > 0 {
		CleanupTableRecords(tb, db, "auth.accounts", accountIDs...)
	}
}

// ============================================================================
// Pointer Helpers
// ============================================================================

// IntPtr returns a pointer to the given int value.
func IntPtr(i int) *int { return &i }

// StrPtr returns a pointer to the given string value.
func StrPtr(s string) *string { return &s }
