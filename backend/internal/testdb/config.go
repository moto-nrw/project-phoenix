package testdb

import (
	"context"
	"database/sql"
	"net/url"
	"os"
	"path/filepath"

	"github.com/uptrace/bun/driver/pgdriver"
)

const (
	// lifecycleAdvisoryLockKey serializes all test-database DDL (template
	// rebuild, clone create, GC, sweep) across processes AND worktrees. Every
	// session takes it while connected to the maintenance database, so the
	// scope is the whole postgres-test server. Same key as the pre-ADR-0004
	// clone lock so old and new binaries never interleave DDL.
	lifecycleAdvisoryLockKey = int64(914735692)

	// ClonePrefix names every package clone. The generation GC drops any
	// database with this prefix that belongs to no living run.
	ClonePrefix = "phx_test_pkg_"
)

// Config derives every DSN the lifecycle needs from TEST_DB_DSN, which points
// at the template database (conventionally phoenix_test).
type Config struct {
	templateURL *url.URL
}

// NewConfig parses the TEST_DB_DSN pointing at the template database.
func NewConfig(dsn string) (*Config, error) {
	parsed, err := parsePostgresDSN(dsn)
	if err != nil {
		return nil, err
	}
	return &Config{templateURL: parsed}, nil
}

// TemplateName returns the template database name (e.g. phoenix_test).
func (c *Config) TemplateName() string {
	return databaseNameFromURL(c.templateURL)
}

// TemplateDSN returns the DSN of the template database.
func (c *Config) TemplateDSN() string {
	return c.templateURL.String()
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
	db := sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(dsn)))
	db.SetMaxOpenConns(2)
	db.SetMaxIdleConns(1)
	return db
}

// acquireLifecycleLock takes the cross-process lifecycle lock on conn. The
// lock is session-scoped; it releases when conn closes (or via the returned
// unlock func).
func acquireLifecycleLock(ctx context.Context, db *sql.DB) (unlock func(), err error) {
	if _, err := db.ExecContext(ctx, `SELECT pg_advisory_lock($1)`, lifecycleAdvisoryLockKey); err != nil {
		return nil, err
	}
	return func() {
		_, _ = db.ExecContext(context.Background(), `SELECT pg_advisory_unlock($1)`, lifecycleAdvisoryLockKey)
	}, nil
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
