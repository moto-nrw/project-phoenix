package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

const (
	createPurgeTenantVersion     = "1.15.21"
	createPurgeTenantDescription = "Create platform.purge_tenant() stored procedure for dynamic tenant data deletion"
)

func init() {
	MigrationRegistry[createPurgeTenantVersion] = &Migration{
		Version:     createPurgeTenantVersion,
		Description: createPurgeTenantDescription,
		DependsOn:   []string{"1.15.20"},
	}

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			_, err := db.ExecContext(ctx, purgeTenantSQL)
			return err
		},
		func(ctx context.Context, db *bun.DB) error {
			_, err := db.ExecContext(ctx, `DROP FUNCTION IF EXISTS platform.purge_tenant(BIGINT)`)
			return err
		},
	)
}

// purgeTenantSQL creates a stored procedure that dynamically discovers all tenant-scoped
// tables via pg_constraint, topologically sorts them by FK dependency depth, and deletes
// tenant data in the correct order to avoid FK violations.
//
// The procedure:
//  1. Validates the school exists and is soft-deleted (deleted_at IS NOT NULL)
//  2. Discovers all tables with a tenant_id FK pointing to platform.schools
//  3. Discovers inter-table FKs between those tenant tables
//  4. Computes deletion order via recursive depth calculation (deepest dependents first)
//  5. Deletes from each table WHERE tenant_id = $1 in dependency-safe order
//  6. Deletes the school record itself
//  7. Returns a JSONB summary of what was deleted
//
// SECURITY DEFINER ensures the function runs as the owner (postgres superuser), bypassing RLS.
// Table names come exclusively from pg_class/pg_namespace (system catalogs), never user input.
// The tenant ID parameter is always bound via $1 parameterization.
// Dynamic EXECUTE uses format('%I.%I', schema, table) to properly quote identifiers,
// hardening against table names with special characters (defense-in-depth).
const purgeTenantSQL = `
CREATE OR REPLACE FUNCTION platform.purge_tenant(p_tenant_id BIGINT)
RETURNS JSONB
LANGUAGE plpgsql
SECURITY DEFINER
AS $$
DECLARE
    v_table_rec     RECORD;
    v_rows_deleted  BIGINT;
    v_total_deleted BIGINT := 0;
    v_result        JSONB := '[]'::JSONB;
    v_school_exists BOOLEAN;
BEGIN
    -- 1. Verify the school exists and is soft-deleted
    SELECT EXISTS(
        SELECT 1 FROM platform.schools
        WHERE id = p_tenant_id AND deleted_at IS NOT NULL
    ) INTO v_school_exists;

    IF NOT v_school_exists THEN
        RAISE EXCEPTION 'School % does not exist or is not soft-deleted', p_tenant_id
            USING ERRCODE = 'P0001';
    END IF;

    -- 2. Discover tenant tables and compute deletion order via recursive CTE.
    --    Tables are sorted by FK dependency depth (deepest first = leaf tables first).
    --    This ensures child rows are deleted before parent rows, avoiding FK violations.
    --    Schema and table names are kept separate so we can use format('%I.%I', ...) for
    --    safe identifier quoting when building dynamic DELETE statements.
    FOR v_table_rec IN
        WITH RECURSIVE
        -- Find all tables that have a single-column FK named tenant_id pointing to platform.schools
        tenant_tables AS (
            SELECT DISTINCT
                cl.oid AS table_oid,
                n.nspname AS schema_name,
                cl.relname AS table_name,
                n.nspname || '.' || cl.relname AS full_name
            FROM pg_constraint con
            JOIN pg_class cl ON cl.oid = con.conrelid
            JOIN pg_namespace n ON n.oid = cl.relnamespace
            JOIN pg_class ref_cl ON ref_cl.oid = con.confrelid
            JOIN pg_namespace ref_n ON ref_n.oid = ref_cl.relnamespace
            WHERE con.contype = 'f'
              AND ref_n.nspname = 'platform'
              AND ref_cl.relname = 'schools'
              AND EXISTS (
                  SELECT 1
                  FROM pg_attribute a
                  WHERE a.attrelid = con.conrelid
                    AND a.attnum = ANY(con.conkey)
                    AND a.attname = 'tenant_id'
              )
        ),
        -- Find FK edges between tenant-scoped tables (child → parent)
        -- Excludes self-references to prevent cycles in the depth calculation
        tenant_edges AS (
            SELECT DISTINCT
                child_t.full_name AS child_table,
                parent_t.full_name AS parent_table
            FROM pg_constraint con
            JOIN tenant_tables child_t ON child_t.table_oid = con.conrelid
            JOIN tenant_tables parent_t ON parent_t.table_oid = con.confrelid
            WHERE con.contype = 'f'
              AND child_t.table_oid != parent_t.table_oid
        ),
        -- Compute depth: tables with no outgoing FKs to other tenant tables = depth 0 (roots).
        -- Tables referencing roots = depth 1, etc. Delete from highest depth first.
        -- The depth guard (< 100) prevents infinite loops from any unexpected circular FKs.
        base_tables AS (
            SELECT t.full_name, t.schema_name, t.table_name, 0 AS depth
            FROM tenant_tables t
            WHERE NOT EXISTS (
                SELECT 1 FROM tenant_edges e
                WHERE e.child_table = t.full_name
            )
        ),
        recursive_depth AS (
            SELECT full_name, schema_name, table_name, depth FROM base_tables
            UNION ALL
            SELECT e.child_table, t.schema_name, t.table_name, rd.depth + 1
            FROM tenant_edges e
            JOIN recursive_depth rd ON rd.full_name = e.parent_table
            JOIN tenant_tables t ON t.full_name = e.child_table
            WHERE rd.depth < 100
        ),
        max_depths AS (
            SELECT full_name, schema_name, table_name, MAX(depth) AS depth
            FROM recursive_depth
            GROUP BY full_name, schema_name, table_name
        ),
        -- Include any isolated tables (no inter-table FKs) that were missed by the recursion
        all_tables_ordered AS (
            SELECT full_name, schema_name, table_name, depth FROM max_depths
            UNION
            SELECT t.full_name, t.schema_name, t.table_name, 0
            FROM tenant_tables t
            WHERE t.full_name NOT IN (SELECT full_name FROM max_depths)
        )
        SELECT schema_name, table_name, full_name
        FROM all_tables_ordered
        ORDER BY depth DESC, full_name
    LOOP
        -- Use %I (identifier quoting) to safely handle any table/schema names
        EXECUTE format('DELETE FROM %I.%I WHERE tenant_id = $1', v_table_rec.schema_name, v_table_rec.table_name)
        USING p_tenant_id;

        GET DIAGNOSTICS v_rows_deleted = ROW_COUNT;

        IF v_rows_deleted > 0 THEN
            v_total_deleted := v_total_deleted + v_rows_deleted;
            v_result := v_result || jsonb_build_object(
                'table', v_table_rec.full_name,
                'rows_deleted', v_rows_deleted
            );
        END IF;
    END LOOP;

    -- 4. Delete the school record itself
    DELETE FROM platform.schools WHERE id = p_tenant_id;
    GET DIAGNOSTICS v_rows_deleted = ROW_COUNT;
    IF v_rows_deleted > 0 THEN
        v_total_deleted := v_total_deleted + v_rows_deleted;
        v_result := v_result || jsonb_build_object(
            'table', 'platform.schools',
            'rows_deleted', v_rows_deleted
        );
    END IF;

    -- 5. Return summary
    RETURN jsonb_build_object(
        'tenant_id', p_tenant_id,
        'tables_processed', jsonb_array_length(v_result),
        'total_rows_deleted', v_total_deleted,
        'details', v_result
    );
END;
$$;

-- Restrict execution to phoenix_admin role only
REVOKE ALL ON FUNCTION platform.purge_tenant(BIGINT) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION platform.purge_tenant(BIGINT) TO phoenix_admin;
`
