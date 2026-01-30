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
		Where("email IN (?)", bun.In(emails)).
		Exec(ctx)
	if err != nil {
		tb.Logf("Warning: failed to cleanup auth.password_reset_rate_limits: %v", err)
	}
}

// ============================================================================
// Pointer Helpers
// ============================================================================

// IntPtr returns a pointer to the given int value.
func IntPtr(i int) *int { return &i }

// StrPtr returns a pointer to the given string value.
func StrPtr(s string) *string { return &s }
