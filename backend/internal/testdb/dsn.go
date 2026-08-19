// Package testdb owns the lifecycle of the backend test databases (ADR 0004):
// it keeps the template `phoenix_test` current via a migrations hash, hands
// every test binary a run-stamped package clone, and sweeps clones after the
// run — including a generation GC that collects clones of dead runs across
// worktrees. The long-lived postgres-test container is never owned here; the
// suite owns the databases inside it, not the server.
//
// Callers: test.SetupTestDB (via the test helper package) and the sweep
// command (internal/testdb/cmd/sweep) invoked by the test wrapper scripts.
package testdb

import (
	"fmt"
	"net/url"
	"strings"
)

// parsePostgresDSN validates a postgres URL-style DSN and requires a database
// name in the path.
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
