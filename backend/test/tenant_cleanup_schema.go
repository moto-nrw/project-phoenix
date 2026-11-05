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
