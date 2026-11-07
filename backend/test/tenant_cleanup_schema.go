package test

import (
	"context"
	"sync"

	"github.com/uptrace/bun"
)

type tenantCleanupSchema struct {
	tables        []string
	accountTables []string
	dependents    []unscopedTenantDependent
}

var sharedTenantCleanupSchema struct {
	once   sync.Once
	schema tenantCleanupSchema
	err    error
}

// Only the package clone has an immutable schema. Isolated schema tests and
// their private pools must rediscover tables on every cleanup.
func loadTenantCleanupSchema(ctx context.Context, db *bun.DB, tx bun.Tx) (tenantCleanupSchema, error) {
	if sharedTestDB == nil || db.DB != sharedTestDB.DB {
		return readTenantCleanupSchema(ctx, tx)
	}
	sharedTenantCleanupSchema.once.Do(func() {
		sharedTenantCleanupSchema.schema, sharedTenantCleanupSchema.err = readTenantCleanupSchema(ctx, tx)
	})
	return sharedTenantCleanupSchema.schema, sharedTenantCleanupSchema.err
}

func readTenantCleanupSchema(ctx context.Context, db bun.IDB) (tenantCleanupSchema, error) {
	var schema tenantCleanupSchema
	var err error
	schema.dependents, err = discoverUnscopedTenantDependents(ctx, db)
	if err != nil {
		return schema, err
	}
	schema.tables, err = discoverTenantTables(ctx, db)
	if err != nil {
		return schema, err
	}
	schema.accountTables, err = discoverUnscopedAccountTables(ctx, db)
	return schema, err
}

// Bound each simple-query request while retaining every table's predicate
// and the caller's transaction. Any statement error aborts that transaction.
func deleteTenantTableRows(ctx context.Context, tx bun.Tx, tables []string, tenantIDs []int64) error {
	const batchSize = 32
	for start := 0; start < len(tables); start += batchSize {
		var batch []byte
		for _, table := range tables[start:min(start+batchSize, len(tables))] {
			query := tx.NewDelete().TableExpr(table).Where("tenant_id IN (?)", bun.List(tenantIDs))
			var err error
			batch, err = query.AppendQuery(query.DB().QueryGen(), batch)
			if err != nil {
				return err
			}
			batch = append(batch, ';', '\n')
		}
		if _, err := tx.ExecContext(ctx, string(batch)); err != nil {
			return err
		}
	}
	return nil
}

// The catalog reads below select what information_schema.columns joined with
// information_schema.tables (table_type 'BASE TABLE') selected before, in the
// same order, but read pg_catalog directly: the information_schema views and
// the planner's estimates for a catalog-wide constraint join cost most of a
// fresh clone's cleanup. Cleanup runs as superuser (it sets
// session_replication_role), so the views' privilege filters admitted every
// relation. TestTenantCleanupCatalogMatchesInformationSchema keeps the
// information_schema queries as the oracle.

// liveTenantColumn holds the relations with a live tenant_id column.
const liveTenantColumn = `SELECT attrelid FROM pg_attribute
	WHERE attname = 'tenant_id' AND attnum > 0 AND NOT attisdropped`

// discoverBaseTablesWithColumn lists base and partitioned tables outside the
// system schemas and temporary schemas that carry a live column.
func discoverBaseTablesWithColumn(ctx context.Context, db bun.IDB, column, extraPredicate string) ([]string, error) {
	var tables []string
	err := db.NewRaw(`SELECT format('%I.%I', n.nspname, c.relname)
	FROM pg_attribute AS a
	JOIN pg_class AS c ON c.oid = a.attrelid
	JOIN pg_namespace AS n ON n.oid = c.relnamespace
	WHERE a.attname = ? AND a.attnum > 0 AND NOT a.attisdropped
	  AND c.relkind IN ('r', 'p')
	  AND n.nspname NOT IN ('pg_catalog', 'information_schema')
	  AND n.oid <> pg_my_temp_schema() AND NOT pg_is_other_temp_schema(n.oid)
	  `+extraPredicate+`
	ORDER BY n.nspname, c.relname`, column).Scan(ctx, &tables)
	return tables, err
}

func discoverTenantTables(ctx context.Context, db bun.IDB) ([]string, error) {
	return discoverBaseTablesWithColumn(ctx, db, "tenant_id", "")
}

func discoverUnscopedAccountTables(ctx context.Context, db bun.IDB) ([]string, error) {
	return discoverBaseTablesWithColumn(ctx, db, "account_id",
		"AND c.oid NOT IN ("+liveTenantColumn+")")
}

type unscopedTenantDependent struct {
	Child         string `bun:"child"`
	Parent        string `bun:"parent"`
	JoinPredicate string `bun:"join_predicate"`
}

// discoverUnscopedTenantDependents selects the foreign keys from a relation
// without tenant_id to one with it before expanding their key columns.
func discoverUnscopedTenantDependents(ctx context.Context, db bun.IDB) ([]unscopedTenantDependent, error) {
	var dependents []unscopedTenantDependent
	err := db.NewRaw(`WITH tenant_relation AS MATERIALIZED (`+liveTenantColumn+`),
	candidate AS MATERIALIZED (
	  SELECT c.oid, c.conrelid, c.confrelid, c.conkey, c.confkey
	  FROM pg_constraint AS c
	  WHERE c.contype = 'f'
	    AND c.conrelid NOT IN (SELECT attrelid FROM tenant_relation)
	    AND c.confrelid IN (SELECT attrelid FROM tenant_relation)
	)
	SELECT format('%I.%I', child_namespace.nspname, child.relname) AS child,
	  format('%I.%I', parent_namespace.nspname, parent.relname) AS parent,
	  string_agg(format('child.%I = parent.%I', child_column.attname, parent_column.attname), ' AND ' ORDER BY key.ordinality) AS join_predicate
	FROM candidate AS c
	JOIN pg_class AS child ON child.oid = c.conrelid
	JOIN pg_namespace AS child_namespace ON child_namespace.oid = child.relnamespace
	JOIN pg_class AS parent ON parent.oid = c.confrelid
	JOIN pg_namespace AS parent_namespace ON parent_namespace.oid = parent.relnamespace
	JOIN LATERAL unnest(c.conkey, c.confkey) WITH ORDINALITY AS key(child_attnum, parent_attnum, ordinality) ON TRUE
	JOIN pg_attribute AS child_column ON child_column.attrelid = child.oid AND child_column.attnum = key.child_attnum
	JOIN pg_attribute AS parent_column ON parent_column.attrelid = parent.oid AND parent_column.attnum = key.parent_attnum
	GROUP BY child_namespace.nspname, child.relname, parent_namespace.nspname, parent.relname, c.oid
	ORDER BY child_namespace.nspname, child.relname, c.oid`).Scan(ctx, &dependents)
	return dependents, err
}
