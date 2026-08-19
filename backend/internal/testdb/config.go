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
	db := sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(dsn)))
	db.SetMaxOpenConns(2)
	db.SetMaxIdleConns(1)
	return db
}

// acquireLifecycleLock pins the cross-process lifecycle lock to one
// maintenance connection. PostgreSQL advisory locks are session-scoped, so
// every protected DDL/query must use this returned connection until unlock.
func acquireLifecycleLock(ctx context.Context, db *sql.DB) (conn *sql.Conn, unlock func(), err error) {
	conn, err = db.Conn(ctx)
	if err != nil {
		return nil, nil, err
	}
	if _, err := conn.ExecContext(ctx, `SELECT pg_advisory_lock($1)`, lifecycleAdvisoryLockKey); err != nil {
		_ = conn.Close()
		return nil, nil, err
	}
	return conn, func() {
		_, _ = conn.ExecContext(context.Background(), `SELECT pg_advisory_unlock($1)`, lifecycleAdvisoryLockKey)
		_ = conn.Close()
	}, nil
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
