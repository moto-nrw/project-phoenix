package test

import (
	"context"
	"crypto/sha1"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/uptrace/bun/driver/pgdriver"
)

const (
	testDBCloneAdvisoryLockKey = int64(914735692)
	testDBClonePrefix          = "phx_test_pkg_"
)

var (
	packageTestDBOnce sync.Once
	packageTestDBDSN  string
	packageTestDBErr  error
)

func packageIsolatedTestDSN(tb testing.TB, sourceDSN string) string {
	tb.Helper()

	packageTestDBOnce.Do(func() {
		packageTestDBDSN, packageTestDBErr = ensurePackageTestDB(sourceDSN)
	})
	if packageTestDBErr != nil {
		tb.Fatalf("setup package-isolated test database: %v", packageTestDBErr)
	}
	return packageTestDBDSN
}

func ensurePackageTestDB(sourceDSN string) (string, error) {
	sourceURL, err := parsePostgresDSN(sourceDSN)
	if err != nil {
		return "", err
	}

	sourceDB := databaseNameFromURL(sourceURL)
	if sourceDB == "" {
		return "", fmt.Errorf("TEST_DB_DSN must include a database name")
	}

	cloneDB, err := packageCloneDatabaseName()
	if err != nil {
		return "", err
	}

	maintenanceDSN := withDatabaseName(sourceURL, "postgres").String()
	cloneDSN := withDatabaseName(sourceURL, cloneDB).String()

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	sqldb := sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(maintenanceDSN)))
	defer func() { _ = sqldb.Close() }()

	if err := sqldb.PingContext(ctx); err != nil {
		return "", fmt.Errorf("connect to postgres maintenance database: %w", err)
	}

	if _, err := sqldb.ExecContext(ctx, fmt.Sprintf(`SELECT pg_advisory_lock(%d)`, testDBCloneAdvisoryLockKey)); err != nil {
		return "", fmt.Errorf("acquire package test DB clone lock: %w", err)
	}
	defer func() {
		unlockCtx, unlockCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer unlockCancel()
		_, _ = sqldb.ExecContext(unlockCtx, fmt.Sprintf(`SELECT pg_advisory_unlock(%d)`, testDBCloneAdvisoryLockKey))
	}()

	if _, err := sqldb.ExecContext(ctx, `DROP DATABASE IF EXISTS `+quoteIdentifier(cloneDB)+` WITH (FORCE)`); err != nil {
		return "", fmt.Errorf("drop stale package test database %q: %w", cloneDB, err)
	}
	if _, err := sqldb.ExecContext(ctx, `CREATE DATABASE `+quoteIdentifier(cloneDB)+` TEMPLATE `+quoteIdentifier(sourceDB)); err != nil {
		return "", fmt.Errorf("clone test database %q from %q: %w", cloneDB, sourceDB, err)
	}

	return cloneDSN, nil
}

func parsePostgresDSN(dsn string) (*url.URL, error) {
	if strings.TrimSpace(dsn) == "" {
		return nil, fmt.Errorf("TEST_DB_DSN is empty")
	}

	parsed, err := url.Parse(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse TEST_DB_DSN: %w", err)
	}
	if parsed.Scheme != "postgres" && parsed.Scheme != "postgresql" {
		return nil, fmt.Errorf("TEST_DB_DSN must use postgres/postgresql scheme, got %q", parsed.Scheme)
	}
	if databaseNameFromURL(parsed) == "" {
		return nil, fmt.Errorf("TEST_DB_DSN must include a database name")
	}
	return parsed, nil
}

func packageCloneDatabaseName() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("determine package working directory: %w", err)
	}

	normalized := filepath.ToSlash(wd)
	sum := sha1.Sum([]byte(normalized))
	hash := hex.EncodeToString(sum[:])[:16]

	return testDBClonePrefix + hash, nil
}

func databaseNameFromURL(parsed *url.URL) string {
	return strings.Trim(strings.TrimPrefix(parsed.Path, "/"), "/")
}

func withDatabaseName(source *url.URL, dbName string) *url.URL {
	clone := *source
	clone.Path = "/" + dbName
	clone.RawPath = ""
	return &clone
}

func quoteIdentifier(identifier string) string {
	return `"` + strings.ReplaceAll(identifier, `"`, `""`) + `"`
}
