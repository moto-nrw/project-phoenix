package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version: "1.15.489", Description: "Read-only administrative role for internal tenant return (#3838, ADR 0092)",
		DependsOn: []string{"1.15.479"},
	})
	Migrations.MustRegister(tenantExportReaderUp, tenantExportReaderDown)
}

// NOLOGIN and no application-role membership keep this out of HTTP sessions.
// pg_read_all_data supplies SELECT/USAGE, never writes; BYPASSRLS is needed
// for the explicitly scoped administrative snapshot, including tenantChild
// and shared account rows. The executor must still project catalog fields.
func tenantExportReaderUp(ctx context.Context, db *bun.DB) error {
	_, err := db.ExecContext(ctx, `
		DO $$ BEGIN
			IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'phoenix_tenant_export') THEN
				CREATE ROLE phoenix_tenant_export NOLOGIN BYPASSRLS;
			END IF;
		END $$;
		GRANT pg_read_all_data TO phoenix_tenant_export;
	`)
	if err != nil {
		return fmt.Errorf("create tenant export reader: %w", err)
	}
	return nil
}

func tenantExportReaderDown(ctx context.Context, db *bun.DB) error {
	// PostgreSQL roles are cluster-wide. Other migrated databases may still
	// use this reader; rollback must not revoke or drop a shared role.
	return nil
}
