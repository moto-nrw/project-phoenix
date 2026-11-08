package test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The cleanup catalog reads must select exactly what the information_schema
// form selected before (#5741), including relations that form excluded.
func TestTenantCleanupCatalogMatchesInformationSchema(t *testing.T) {
	t.Parallel()

	db := SetupIsolatedTestDB(t)
	ctx := t.Context()
	_, err := db.ExecContext(ctx, `
  CREATE TABLE public.catalog_parents (id bigint, code text, tenant_id bigint, PRIMARY KEY (id, code));
  CREATE TABLE public.catalog_children (code text, parent_id bigint, account_id bigint,
    FOREIGN KEY (parent_id, code) REFERENCES public.catalog_parents (id, code));
  CREATE TABLE public.catalog_dropped_scope (id bigint PRIMARY KEY, tenant_id bigint, account_id bigint);
  ALTER TABLE public.catalog_dropped_scope DROP COLUMN tenant_id;
  CREATE TABLE public.catalog_dropped_child (parent_id bigint, parent_code text, tenant_id bigint,
    FOREIGN KEY (parent_id, parent_code) REFERENCES public.catalog_parents (id, code));
  ALTER TABLE public.catalog_dropped_child DROP COLUMN tenant_id;
  CREATE TABLE public.catalog_scoped_accounts (tenant_id bigint, account_id bigint);
  CREATE TABLE public.catalog_partitioned (tenant_id bigint, account_id bigint) PARTITION BY LIST (tenant_id);
  CREATE TABLE public.catalog_partition PARTITION OF public.catalog_partitioned FOR VALUES IN (1);
  CREATE VIEW public.catalog_view AS SELECT tenant_id, account_id FROM public.catalog_scoped_accounts;
  CREATE MATERIALIZED VIEW public.catalog_matview AS SELECT tenant_id, account_id FROM public.catalog_scoped_accounts;
  CREATE SCHEMA "Catalog Schema";
  CREATE TABLE "Catalog Schema"."Quoted Table" (tenant_id bigint, account_id bigint);`)
	require.NoError(t, err)

	// Another session's temporary table stays invisible to both forms.
	other, err := db.Conn(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, other.Close()) })
	_, err = other.ExecContext(ctx, `CREATE TEMP TABLE catalog_other_temp (tenant_id bigint, account_id bigint)`)
	require.NoError(t, err)

	err = db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		// The session's own temporary table is LOCAL TEMPORARY, not BASE TABLE.
		if _, err := tx.ExecContext(ctx, `CREATE TEMP TABLE catalog_own_temp (tenant_id bigint, account_id bigint) ON COMMIT DROP`); err != nil {
			return err
		}
		tables, err := discoverTenantTables(ctx, tx)
		require.NoError(t, err)
		require.Equal(t, informationSchemaTables(ctx, t, tx, "tenant_id", false), tables)
		require.Contains(t, tables, `"Catalog Schema"."Quoted Table"`)
		require.Contains(t, tables, "public.catalog_partitioned")
		require.Contains(t, tables, "public.catalog_partition")
		require.NotContains(t, tables, "public.catalog_view")
		require.NotContains(t, tables, "public.catalog_matview")
		require.NotContains(t, tables, "pg_temp.catalog_own_temp")
		require.NotContains(t, tables, "public.catalog_dropped_scope")

		accountTables, err := discoverUnscopedAccountTables(ctx, tx)
		require.NoError(t, err)
		require.Equal(t, informationSchemaTables(ctx, t, tx, "account_id", true), accountTables)
		require.Contains(t, accountTables, "public.catalog_children")
		require.Contains(t, accountTables, "public.catalog_dropped_scope")
		require.NotContains(t, accountTables, "public.catalog_scoped_accounts")

		dependents, err := discoverUnscopedTenantDependents(ctx, tx)
		require.NoError(t, err)
		require.Equal(t, previousUnscopedTenantDependents(ctx, t, tx), dependents)
		require.Contains(t, dependents, unscopedTenantDependent{
			Child:         "public.catalog_children",
			Parent:        "public.catalog_parents",
			JoinPredicate: "child.parent_id = parent.id AND child.code = parent.code",
		})
		require.Contains(t, dependents, unscopedTenantDependent{
			Child:         "public.catalog_dropped_child",
			Parent:        "public.catalog_parents",
			JoinPredicate: "child.parent_id = parent.id AND child.parent_code = parent.code",
		})
		return nil
	})
	require.NoError(t, err)
}

// informationSchemaTables is the discovery form used before #5741.
func informationSchemaTables(ctx context.Context, t *testing.T, db bun.IDB, column string, withoutTenant bool) []string {
	t.Helper()
	query := db.NewSelect().
		TableExpr("information_schema.columns AS c").
		ColumnExpr("format('%I.%I', c.table_schema, c.table_name)").
		Join("JOIN information_schema.tables AS t ON t.table_schema = c.table_schema AND t.table_name = c.table_name").
		Where("c.column_name = ?", column).
		Where("c.table_schema NOT IN ('pg_catalog', 'information_schema')").
		Where("t.table_type = 'BASE TABLE'").
		OrderExpr("c.table_schema, c.table_name")
	if withoutTenant {
		query = query.Where("NOT EXISTS (SELECT 1 FROM information_schema.columns tenant_column WHERE tenant_column.table_schema = c.table_schema AND tenant_column.table_name = c.table_name AND tenant_column.column_name = 'tenant_id')")
	}
	var tables []string
	require.NoError(t, query.Scan(ctx, &tables))
	return tables
}

// previousUnscopedTenantDependents is the dependent discovery used before #5741.
func previousUnscopedTenantDependents(ctx context.Context, t *testing.T, db bun.IDB) []unscopedTenantDependent {
	t.Helper()
	var dependents []unscopedTenantDependent
	require.NoError(t, db.NewSelect().
		TableExpr("pg_constraint AS c").
		ColumnExpr("format('%I.%I', child_namespace.nspname, child.relname) AS child").
		ColumnExpr("format('%I.%I', parent_namespace.nspname, parent.relname) AS parent").
		ColumnExpr("string_agg(format('child.%I = parent.%I', child_column.attname, parent_column.attname), ' AND ' ORDER BY key.ordinality) AS join_predicate").
		Join("JOIN pg_class AS child ON child.oid = c.conrelid").
		Join("JOIN pg_namespace AS child_namespace ON child_namespace.oid = child.relnamespace").
		Join("JOIN pg_class AS parent ON parent.oid = c.confrelid").
		Join("JOIN pg_namespace AS parent_namespace ON parent_namespace.oid = parent.relnamespace").
		Join("JOIN LATERAL unnest(c.conkey, c.confkey) WITH ORDINALITY AS key(child_attnum, parent_attnum, ordinality) ON TRUE").
		Join("JOIN pg_attribute AS child_column ON child_column.attrelid = child.oid AND child_column.attnum = key.child_attnum").
		Join("JOIN pg_attribute AS parent_column ON parent_column.attrelid = parent.oid AND parent_column.attnum = key.parent_attnum").
		Where("c.contype = 'f'").
		Where("NOT EXISTS (SELECT 1 FROM pg_attribute candidate_column WHERE candidate_column.attrelid = child.oid AND candidate_column.attname = 'tenant_id' AND candidate_column.attnum > 0 AND NOT candidate_column.attisdropped)").
		Where("EXISTS (SELECT 1 FROM pg_attribute candidate_column WHERE candidate_column.attrelid = parent.oid AND candidate_column.attname = 'tenant_id' AND candidate_column.attnum > 0 AND NOT candidate_column.attisdropped)").
		GroupExpr("child_namespace.nspname, child.relname, parent_namespace.nspname, parent.relname, c.oid").
		OrderExpr("child_namespace.nspname, child.relname, c.oid").
		Scan(ctx, &dependents))
	return dependents
}
