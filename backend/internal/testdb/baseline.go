package testdb

import (
	"context"
	"fmt"
	"strings"
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
	// One pool for all three steps. This sits on the startup path of every
	// test binary, so the three separate pools it used to open cost more
	// than the queries they carried.
	db := openSQL(dsn)
	defer func() { _ = db.Close() }()

	predicates, err := tableSharedPredicates(ctx, db)
	if err != nil {
		return fmt.Errorf("resolve shared-row predicates for baseline snapshot: %w", err)
	}
	counts, err := countSharedRowsDB(ctx, db, predicates)
	if err != nil {
		return fmt.Errorf("count baseline rows: %w", err)
	}
	if err := prepareBaselineTable(ctx, db); err != nil {
		return err
	}
	return insertSharedBaseline(ctx, db, counts, predicates)
}

func prepareBaselineTable(ctx context.Context, db sqlExecutor) error {
	if _, err := db.ExecContext(ctx, `CREATE SCHEMA IF NOT EXISTS `+quoteIdentifier(baselineSchema)); err != nil {
		return fmt.Errorf("create baseline schema: %w", err)
	}
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS `+baselineTable+` (
		table_name text PRIMARY KEY,
		shared_predicate text NOT NULL,
		rows bigint NOT NULL)`); err != nil {
		return fmt.Errorf("create baseline table: %w", err)
	}
	if _, err := db.ExecContext(ctx, `TRUNCATE `+baselineTable); err != nil {
		return fmt.Errorf("reset baseline table: %w", err)
	}
	return nil
}

func insertSharedBaseline(ctx context.Context, db sqlExecutor, counts map[string]int64, predicates map[string]string) error {
	// One multi-row INSERT, not one per table: ~180 round trips were the
	// single most expensive thing this function did.
	values := make([]string, 0, len(counts))
	args := make([]any, 0, 3*len(counts))
	for table, n := range counts {
		values = append(values, fmt.Sprintf("($%d,$%d,$%d)", len(args)+1, len(args)+2, len(args)+3))
		args = append(args, table, predicates[table], n)
	}
	if len(values) == 0 {
		return nil
	}
	if _, err := db.ExecContext(ctx,
		`INSERT INTO `+baselineTable+` (table_name, shared_predicate, rows) VALUES `+
			strings.Join(values, ","), args...); err != nil {
		return fmt.Errorf("record baseline: %w", err)
	}
	return nil
}

// readSharedBaseline returns the recorded start state of a clone. ok is false
// when the clone carries no snapshot — a binary that predates this file, or
// one that failed before bootstrap finished; the sweep then has nothing to
// compare against and reports no leftovers rather than a false alarm.
func readSharedBaseline(ctx context.Context, db sqlExecutor) (counts map[string]int64, predicates map[string]string, ok bool, err error) {
	var exists bool
	if err := db.QueryRowContext(ctx,
		`SELECT to_regclass($1) IS NOT NULL`, baselineTable).Scan(&exists); err != nil {
		return nil, nil, false, fmt.Errorf("probe baseline table: %w", err)
	}
	if !exists {
		return nil, nil, false, nil
	}

	rows, err := db.QueryContext(ctx, `SELECT table_name, shared_predicate, rows FROM `+baselineTable)
	if err != nil {
		return nil, nil, false, fmt.Errorf("read baseline: %w", err)
	}
	defer func() { _ = rows.Close() }()

	counts = make(map[string]int64)
	predicates = make(map[string]string)
	for rows.Next() {
		var table, predicate string
		var n int64
		if err := rows.Scan(&table, &predicate, &n); err != nil {
			return nil, nil, false, err
		}
		counts[table] = n
		predicates[table] = predicate
	}
	if err := rows.Err(); err != nil {
		return nil, nil, false, err
	}
	return counts, predicates, true, nil
}
