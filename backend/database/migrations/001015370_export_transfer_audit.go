package migrations

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/uptrace/bun"
)

const (
	exportTransferAuditVersion     = "1.15.370"
	exportTransferAuditDescription = "Create audit.export_transfers for the manual SFTP transfer of Zeitwirtschafts-/DATEV exports (#3050)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     exportTransferAuditVersion,
		Description: exportTransferAuditDescription,
		DependsOn: []string{
			"1.15.1",                     // RLS infrastructure
			createTenantRolesVersion,     // phoenix_tenant / phoenix_admin roles
			auditAppendOnlyGrantsVersion, // audit schema is append-only for phoenix_tenant
		},
	})

	Migrations.MustRegister(exportTransferAuditUp, exportTransferAuditDown)
}

// exportTransferAuditUp creates the trail of manual export transfers (#3050).
//
// A row is written for EVERY attempt, successful or not. That is the point of
// the table: a payroll file that left the school has to be traceable, and a
// transfer that failed must be visibly a failure rather than an absent row
// that reads like "nobody tried".
//
// What is recorded is who, when, which export, which file, which destination,
// and the outcome. What is NOT recorded, ever: the username, the password, the
// host key, and the file's contents. The destination host and directory are
// kept because they answer "where did our salary data go" — the credentials
// are not needed for that answer and would turn the audit trail into a second
// place to steal them from.
//
// actor_account_id carries no foreign key and the actor's name is snapshotted,
// so the trail survives an account deletion — the same shape audit.file_events
// uses. failure_reason holds a short, stable reason code (not the transport's
// error prose), so the entries stay readable and leak no internals.
func exportTransferAuditUp(ctx context.Context, db *bun.DB) error {
	slog.Info("migration 1.15.370: creating audit.export_transfers")

	if _, err := db.NewRaw(`
		CREATE TABLE IF NOT EXISTS audit.export_transfers (
			id BIGSERIAL PRIMARY KEY,
			tenant_id BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			actor_account_id BIGINT,
			actor_name TEXT NOT NULL,
			export_kind TEXT NOT NULL,
			export_format TEXT NOT NULL,
			filename TEXT NOT NULL,
			byte_size BIGINT NOT NULL,
			target_host TEXT NOT NULL,
			target_port INTEGER NOT NULL,
			target_directory TEXT NOT NULL,
			status TEXT NOT NULL CHECK (status IN ('success', 'failed')),
			failure_reason TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);

		CREATE INDEX IF NOT EXISTS idx_audit_export_transfers_tenant_created
			ON audit.export_transfers (tenant_id, created_at DESC);

		GRANT SELECT, INSERT ON audit.export_transfers TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE audit.export_transfers_id_seq TO phoenix_tenant;
		GRANT ALL ON audit.export_transfers TO phoenix_admin;
		GRANT ALL ON SEQUENCE audit.export_transfers_id_seq TO phoenix_admin;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed creating audit.export_transfers: %w", err)
	}

	if err := provisionTenantRLS(ctx, db, "audit.export_transfers"); err != nil {
		return fmt.Errorf("failed provisioning RLS for audit.export_transfers: %w", err)
	}
	return nil
}

func exportTransferAuditDown(ctx context.Context, db *bun.DB) error {
	slog.Info("migration 1.15.370: rolling back, dropping audit.export_transfers")
	if _, err := db.NewRaw(`
		DROP TABLE IF EXISTS audit.export_transfers CASCADE;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed rolling back audit.export_transfers: %w", err)
	}
	return nil
}
