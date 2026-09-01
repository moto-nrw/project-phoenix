package test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database"
	"github.com/moto-nrw/project-phoenix/internal/testdb"
	"github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"
)

type authEventCommand struct{ repo audit.AuthEventRepository }

func (c authEventCommand) Append(ctx context.Context, event any) error {
	authEvent, ok := event.(*audit.AuthEvent)
	if !ok {
		return fmt.Errorf("test auth event command: unsupported event %T", event)
	}
	return c.repo.Create(ctx, authEvent)
}

func NewAuthEventCommand(repo audit.AuthEventRepository) audit.Command {
	return authEventCommand{repo: repo}
}

func NewAuditAuthEvent(accountID int64, ipAddress string) any {
	return audit.NewAuthEvent(accountID, audit.EventTypeLogin, true, ipAddress)
}

func SetAuditEventTenant(event any, tenantID int64) {
	event.(*audit.AuthEvent).SetTenantID(tenantID)
}

func AuditEventIdentity(event any) (tenantID, eventID, accountID int64) {
	authEvent := event.(*audit.AuthEvent)
	return authEvent.TenantID, authEvent.ID, authEvent.AccountID
}

// Database aliases and constructors keep migration tests on the shared test
// support boundary instead of adding test-only dependencies to production
// migration files.
type Tx = bun.Tx
type TxOptions = sql.TxOptions
type NullInt64 = sql.NullInt64
type NullString = sql.NullString
type NullTime = sql.NullTime

func DBList(value any) any { return bun.List(value) }

func NewBunDB(database *sql.DB, options ...bun.DBOption) *bun.DB {
	return bun.NewDB(database, pgdialect.New(), options...)
}

func OpenPostgresSQL(dsn string) *sql.DB {
	return sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(dsn)))
}

// SetupTestDB returns the package-wide test database pool. The first call in
// a test binary initializes the whole test-database lifecycle (server up,
// template current, package clone created and bootstrapped — see db_clone.go
// and internal/testdb) and opens one shared pool against the package clone;
// every later call returns that same pool.
//
// Tests MUST NOT close the returned handle — it is shared by every test in
// the binary and dies with the process. The per-test path performs no
// t.Setenv or viper writes, so tests using it may call t.Parallel().
//
// This is the preferred way to get a database connection in tests:
//
//	func TestSomething(t *testing.T) {
//	    db := testpkg.SetupTestDB(t)
//	    // ... test code
//	}
func SetupTestDB(t testing.TB) *bun.DB {
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

	packageTestDBOnce.Do(func() {
		packageTestDBErr = initPackageTestDB()
	})
	if packageTestDBErr != nil {
		t.Fatalf("setup package test database: %v", packageTestDBErr)
	}

	// Self-heal after tests that call viper.Reset() (factory config tests):
	// without this, every later test in the package would lose test_db_dsn.
	// viper's maps are not thread-safe, so the read and the repair are taken
	// under one lock — otherwise a package that both resets viper and runs
	// parallel tests would race here under -race.
	viperHealMu.Lock()
	if viper.GetString("test_db_dsn") != packageClone.DSN {
		applyViperTestConfig()
	}
	viperHealMu.Unlock()

	if leftoverMode() == "test" {
		perTestLeftoverCheck(t)
	}

	return sharedTestDB
}

// WithAfterCommitHooks queues realtime side effects until the returned commit
// function is called. It keeps service-package tests on the shared test seam
// instead of importing tenant runtime internals directly.
func WithAfterCommitHooks(ctx context.Context) (context.Context, func()) {
	return tenant.WithAfterCommitHooksForTest(ctx)
}

// SetupClosableTestDB returns a PRIVATE pool against the package clone for
// tests that deliberately close their database to provoke errors. Closing
// the shared SetupTestDB pool would kill every later test in the binary;
// closing this one affects nobody else. The caller owns the close.
func SetupClosableTestDB(t *testing.T) *bun.DB {
	t.Helper()

	SetupTestDB(t) // ensure the lifecycle ran and viper points at the clone

	db, err := database.DBConn()
	require.NoError(t, err, "Failed to open private test database pool")
	return db
}

// SetupServeTestDB returns a private pool connected as the least-privilege
// phoenix_auth role. Use it for tests that must exercise SET ROLE and database
// capabilities through the same connection identity as the HTTP server.
func SetupServeTestDB(t *testing.T) *bun.DB {
	t.Helper()

	SetupTestDB(t) // ensure the package clone and test role password are ready
	db, err := database.DBConnForServe()
	require.NoError(t, err, "Failed to open private phoenix_auth test database pool")
	return db
}

// ============================================================================
// Context Helpers
// ============================================================================

// TenantContext returns a context with tenant_id set.
// Use this in service and repository tests that call methods requiring tenant context.
// Without tenant context, EnsureTenantID silently leaves tenant_id=0, which violates
// FK constraints on tenant-scoped tables.
func TenantContext(tenantID int64) context.Context {
	ctx := tenant.WithTenantID(WithPackageTenantRuntime(context.Background()), tenantID)
	return audit.WithTenantID(ctx, tenantID)
}

// TenantScope owns a unique tenant for a test and provides the matching context.
type TenantScope struct {
	TenantID int64
}

// NewTenantScope creates a unique tenant and returns a scope for tests that
// assert counts or shared-table state and therefore must not use tenant_id=1.
func NewTenantScope(tb testing.TB, db *bun.DB) TenantScope {
	tb.Helper()

	// JWT-safe band, not UniqueTestTenantID: a scope tenant travels through
	// minted test JWTs, and JSON decodes numbers as float64 — a nanosecond
	// timestamp ID exceeds 2^53 and comes back corrupted.
	tenantID := uniqueJWTSafeTenantID()
	EnsureTestTenant(tb, db, tenantID)

	return TenantScope{TenantID: tenantID}
}

// Context returns a background context scoped to the test tenant.
func (s TenantScope) Context() context.Context {
	return TenantContext(s.TenantID)
}

var uniqueTestTenantCounter int64

const (
	// Tenant IDs handed out here live in [testTenantIDBase,
	// testTenantIDCeiling). The values come from internal/testdb because the
	// leftover gate reads the same floor to tell a test's own tenant from
	// shared state — two copies of the number would drift apart silently.
	testTenantIDBase    = testdb.TenantIDBase
	testTenantIDCeiling = testdb.TenantIDCeiling
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
// assert aggregate counts and therefore must not share tenant_id=1. It hands
// out IDs from the JWT-safe band: a nanosecond-timestamp ID exceeds 2^53 and
// comes back corrupted once it travels through a minted test token, because
// JSON decodes numbers as float64.
func UniqueTestTenantID(tb testing.TB) int64 {
	tb.Helper()
	return uniqueJWTSafeTenantID()
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
