package testdb

import (
	"context"
	"database/sql"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/uptrace/bun/driver/pgdriver"
)

const testConnectionReadTimeout = time.Minute
const testConnectionTimezone = "Europe/Berlin"

const (
	// lifecycleAdvisoryLockKey serializes all test-database DDL (template
	// rebuild, clone create, GC, sweep) across processes AND worktrees. Every
	// session takes it while connected to the maintenance database, so the
	// scope is the whole postgres-test server. Same key as the pre-ADR-0004
	// clone lock so old and new binaries never interleave DDL.
	lifecycleAdvisoryLockKey = int64(914735692)

	// authRoleAdvisoryLockKey serializes the phoenix_auth password pin alone.
	// ALTER ROLE writes a pg_authid tuple, and two sessions doing it at the
	// same moment fail with "tuple concurrently updated" (XX000) — which
	// takes the whole package binary down, because the pin runs during
	// SetupTestDB. It needs its own key rather than the lifecycle lock: the
	// pin sits on the lock-free fast path precisely so it does not queue
	// behind clone creation.
	authRoleAdvisoryLockKey = int64(914735693)

	// ClonePrefix names every package clone. The generation GC drops any
	// database with this prefix that belongs to no living run.
	ClonePrefix = "phx_test_pkg_"

	// TenantIDBase is the floor of the tenant IDs the suite hands out to
	// tests (test.UniqueTestTenantID), and the leftover gate's ownership
	// test: a row whose tenant is at or above it belongs to a tenant some
	// test created and dies with the clone; a row below it — or with no
	// tenant at all — is shared state a test wrote into and did not take
	// back.
	//
	// "At or above", not "inside a band": tenant fixtures also push the
	// platform.schools sequence clear of the band, so a school created
	// through the ordinary service path lands ABOVE the ceiling and is just
	// as test-owned as one the fixture placed inside it.
	//
	// The floor sits above the literal IDs older fixtures hardcode (1, 42, …)
	// and the band below TenantIDCeiling stays under 2^53, so an ID survives
	// a JWT round trip — JSON decodes numbers as float64, exact only below
	// that.
	TenantIDBase    int64 = 1_000_000_000
	TenantIDCeiling int64 = 2_000_000_000
)

// Config derives every DSN the lifecycle needs from TEST_DB_DSN, whose
// database name (conventionally phoenix_test) is the BASE of the template
// name. The effective template is migration-scoped — see ForMigrations.
type Config struct {
	templateURL *url.URL
	// templateName is the effective template database name. Empty means the
	// base name from TEST_DB_DSN (an unresolved config).
	templateName string
}

// NewConfig parses the TEST_DB_DSN pointing at the template database.
func NewConfig(dsn string) (*Config, error) {
	parsed, err := parsePostgresDSN(dsn)
	if err != nil {
		return nil, err
	}
	query := parsed.Query()
	query.Set("read_timeout", testConnectionReadTimeout.String())
	query.Set("timezone", testConnectionTimezone)
	parsed.RawQuery = query.Encode()
	return &Config{templateURL: parsed}, nil
}

// BaseTemplateName returns the database name from TEST_DB_DSN (e.g.
// phoenix_test). It is the prefix every migration-scoped template shares.
func (c *Config) BaseTemplateName() string {
	return databaseNameFromURL(c.templateURL)
}

// TemplateName returns the effective template database name: the
// migration-scoped one once the config was resolved via ForMigrations,
// otherwise the base name from TEST_DB_DSN.
func (c *Config) TemplateName() string {
	if c.templateName != "" {
		return c.templateName
	}
	return c.BaseTemplateName()
}

// TemplateDSN returns the DSN of the effective template database.
func (c *Config) TemplateDSN() string {
	return c.DatabaseDSN(c.TemplateName())
}

// TemplateEnv carries the template name the once-per-run bootstrap resolved,
// so package binaries can skip EnsureServer and EnsureTemplate. Unset means
// "resolve it yourself" — what a naked `go test` does.
const TemplateEnv = "PHX_TEST_TEMPLATE"

// WithTemplate returns a copy of c pinned to an already-resolved template
// name (the bootstrap's answer), without deriving it from a migrations hash.
func (c *Config) WithTemplate(name string) *Config {
	return &Config{templateURL: c.templateURL, templateName: name}
}

// ForMigrations returns a copy of c whose template database is scoped to the
// given migrations hash (<base>_<12 hex>). Two worktrees on different
// migration states therefore build two templates side by side instead of
// tearing each other's down.
func (c *Config) ForMigrations(hash string) *Config {
	return &Config{
		templateURL:  c.templateURL,
		templateName: templateNameForHash(c.BaseTemplateName(), hash),
	}
}

// MaintenanceDSN returns the DSN of the `postgres` maintenance database on
// the same server, used for all CREATE/DROP DATABASE work.
func (c *Config) MaintenanceDSN() string {
	return withDatabaseName(c.templateURL, "postgres").String()
}

// DatabaseDSN returns the DSN for an arbitrary database on the same server.
func (c *Config) DatabaseDSN(name string) string {
	return withDatabaseName(c.templateURL, name).String()
}

// openSQL opens a small throwaway pool for lifecycle DDL and checks.
func openSQL(dsn string) *sql.DB {
	db := sql.OpenDB(newLifecycleConnector(dsn))
	db.SetMaxOpenConns(2)
	db.SetMaxIdleConns(1)
	return db
}

func newLifecycleConnector(dsn string) *pgdriver.Connector {
	return pgdriver.NewConnector(
		pgdriver.WithDSN(dsn),
		// Lifecycle DDL can legitimately wait behind another worktree's clone
		// or forced checkpoint. Every caller already supplies a bounded context,
		// so the driver's shorter 10-second socket default must not preempt it.
		pgdriver.WithReadTimeout(0),
	)
}

// acquireLifecycleLock pins the cross-process lifecycle lock to one
// maintenance connection. PostgreSQL advisory locks are session-scoped, so
// every protected DDL/query must use this returned connection until unlock.
func acquireLifecycleLock(ctx context.Context, db *sql.DB) (conn *sql.Conn, unlock func(), err error) {
	return acquireLock(ctx, db, `SELECT pg_advisory_lock($1)`, `SELECT pg_advisory_unlock($1)`, lifecycleAdvisoryLockKey)
}

// acquireLock is the shared body of the lifecycle lock helpers: take the
// advisory lock on one dedicated connection, hand back that connection, and
// release both on unlock.
func acquireLock(ctx context.Context, db *sql.DB, lockSQL, unlockSQL string, key int64) (*sql.Conn, func(), error) {
	conn, err := db.Conn(ctx)
	if err != nil {
		return nil, nil, err
	}
	if _, err := conn.ExecContext(ctx, lockSQL, key); err != nil {
		_ = conn.Close()
		return nil, nil, err
	}
	return conn, func() {
		_, _ = conn.ExecContext(context.Background(), unlockSQL, key)
		_ = conn.Close()
	}, nil
}

// acquireLifecycleLockShared pins the lifecycle lock in SHARED mode. Cloning
// is the one lifecycle step that several processes may do at the same time:
// CREATE DATABASE ... TEMPLATE only takes a ShareLock on the source database
// (it needs the template to stay put and unconnected, which every cloner
// wants too), and each clone's name is unique to its run and package, so two
// cloners never touch the same target. What the shared mode still keeps out
// is the exclusive holder — a template rebuild, which drops the very database
// being copied, and the generation GC, which drops clones.
//
// Measured motive: CREATE DATABASE cost 6,0s summed over 93 package binaries
// (median 61ms), all of it serialized behind one exclusive lock (#2419).
func acquireLifecycleLockShared(ctx context.Context, db *sql.DB) (conn *sql.Conn, unlock func(), err error) {
	return acquireLifecycleLockSharedForKey(ctx, db, lifecycleAdvisoryLockKey)

}

func acquireLifecycleLockSharedForKey(ctx context.Context, db *sql.DB, key int64) (*sql.Conn, func(), error) {
	return acquireLock(ctx, db, `SELECT pg_advisory_lock_shared($1)`, `SELECT pg_advisory_unlock_shared($1)`, key)
}

// tryAcquireLifecycleLock takes the exclusive lifecycle lock if it is free
// right now, and reports ok=false instead of waiting. The generation GC uses
// it: collecting dead runs' clones is housekeeping, and making every package
// binary queue for it — behind, and ahead of, the shared cloners — would
// rebuild exactly the convoy the shared mode removes. A skipped GC is picked
// up by the next binary, the run's sweep, or the next run.
func tryAcquireLifecycleLock(ctx context.Context, db *sql.DB) (conn *sql.Conn, unlock func(), ok bool, err error) {
	conn, err = db.Conn(ctx)
	if err != nil {
		return nil, nil, false, err
	}
	var got bool
	if err := conn.QueryRowContext(ctx,
		`SELECT pg_try_advisory_lock($1)`, lifecycleAdvisoryLockKey).Scan(&got); err != nil {
		_ = conn.Close()
		return nil, nil, false, err
	}
	if !got {
		_ = conn.Close()
		return nil, nil, false, nil
	}
	return conn, func() {
		_, _ = conn.ExecContext(context.Background(), `SELECT pg_advisory_unlock($1)`, lifecycleAdvisoryLockKey)
		_ = conn.Close()
	}, true, nil
}

// ProjectRoot returns the repository root (the parent of the backend module
// root), where .env and docker-compose.yml live.
func ProjectRoot() (string, error) {
	backend, err := backendRoot()
	if err != nil {
		return "", err
	}
	return filepath.Dir(backend), nil
}

// backendRoot walks up from the current working directory to the directory
// containing go.mod (the backend module root). Test binaries run with their
// package directory as cwd; the sweep command runs from backend/ or the repo
// root's backend subdirectory.
func backendRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", os.ErrNotExist
		}
		dir = parent
	}
}
