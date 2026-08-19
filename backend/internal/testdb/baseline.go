package testdb

import (
	"context"
	"fmt"
)

// The leftover gate compares a clone's END state against its START state
// (#2419 goal 2). The start state is captured here, in the test binary,
// right after the clone is bootstrapped and before the first test runs; the
// sweep reads it back after the run.
//
// Taking the snapshot inside the clone rather than diffing against the
// template is what makes the gate exact: whatever the bootstrap puts into a
// fresh clone — today a tenant, a room and a system staff row — is part of
// "start" by construction, so the gate needs no special case for it and does
// not change when those fixtures finally disappear.
//
// What the snapshot counts is SHARED state only: rows outside the test-tenant
// band (see TenantIDBase). A row a test writes into its own tenant is not a
// leftover — nothing else can see it, and it dies with the clone. A row a
// test writes into tenant-less shared state (platform operators, global
// tokens, migration bookkeeping) is, because the next test in the same binary
// runs on top of it.

const (
	// baselineSchema holds the lifecycle's own bookkeeping inside a clone.
	// Its own schema, so it can be excluded from the tables under audit
	// without pattern-matching an app table name.
	baselineSchema = "phx_test"
	baselineTable  = "phx_test.shared_baseline"
)

// SnapshotSharedBaseline records the clone's shared-row counts as the state
// every test run must return to. Called once per clone, after bootstrap.
func SnapshotSharedBaseline(ctx context.Context, dsn string) error {
	owners, err := tableOwnerColumns(ctx, dsn)
	if err != nil {
		return fmt.Errorf("resolve owner columns for baseline snapshot: %w", err)
	}
	counts, err := countSharedRows(ctx, dsn, owners)
	if err != nil {
		return fmt.Errorf("count baseline rows: %w", err)
	}

	db := openSQL(dsn)
	defer func() { _ = db.Close() }()

	if _, err := db.ExecContext(ctx, `CREATE SCHEMA IF NOT EXISTS `+quoteIdentifier(baselineSchema)); err != nil {
		return fmt.Errorf("create baseline schema: %w", err)
	}
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS `+baselineTable+` (
		table_name text PRIMARY KEY,
		owner_column text NOT NULL,
		rows bigint NOT NULL)`); err != nil {
		return fmt.Errorf("create baseline table: %w", err)
	}
	if _, err := db.ExecContext(ctx, `TRUNCATE `+baselineTable); err != nil {
		return fmt.Errorf("reset baseline table: %w", err)
	}

	for table, n := range counts {
		if _, err := db.ExecContext(ctx,
			`INSERT INTO `+baselineTable+` (table_name, owner_column, rows) VALUES ($1, $2, $3)`,
			table, owners[table], n); err != nil {
			return fmt.Errorf("record baseline for %s: %w", table, err)
		}
	}
	return nil
}

// readSharedBaseline returns the recorded start state of a clone. ok is false
// when the clone carries no snapshot — a binary that predates this file, or
// one that failed before bootstrap finished; the sweep then has nothing to
// compare against and reports no leftovers rather than a false alarm.
func readSharedBaseline(ctx context.Context, dsn string) (counts map[string]int64, owners map[string]string, ok bool, err error) {
	db := openSQL(dsn)
	defer func() { _ = db.Close() }()

	var exists bool
	if err := db.QueryRowContext(ctx,
		`SELECT to_regclass($1) IS NOT NULL`, baselineTable).Scan(&exists); err != nil {
		return nil, nil, false, fmt.Errorf("probe baseline table: %w", err)
	}
	if !exists {
		return nil, nil, false, nil
	}

	rows, err := db.QueryContext(ctx, `SELECT table_name, owner_column, rows FROM `+baselineTable)
	if err != nil {
		return nil, nil, false, fmt.Errorf("read baseline: %w", err)
	}
	defer func() { _ = rows.Close() }()

	counts = make(map[string]int64)
	owners = make(map[string]string)
	for rows.Next() {
		var table, owner string
		var n int64
		if err := rows.Scan(&table, &owner, &n); err != nil {
			return nil, nil, false, err
		}
		counts[table] = n
		owners[table] = owner
	}
	if err := rows.Err(); err != nil {
		return nil, nil, false, err
	}
	return counts, owners, true, nil
}
