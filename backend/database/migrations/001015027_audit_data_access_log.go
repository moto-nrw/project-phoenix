package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	auditDataAccessLogVersion     = "1.15.27"
	auditDataAccessLogDescription = "Create audit.data_access_log table for attendance history view events"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     auditDataAccessLogVersion,
		Description: auditDataAccessLogDescription,
		DependsOn:   []string{"1.15.26"},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createAuditDataAccessLogTable(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return dropAuditDataAccessLogTable(ctx, db)
		},
	)
}

func createAuditDataAccessLogTable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.27: Creating audit.data_access_log table...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	_, err = tx.ExecContext(ctx, `
		CREATE SCHEMA IF NOT EXISTS audit;

		CREATE TABLE IF NOT EXISTS audit.data_access_log (
			id                BIGSERIAL   PRIMARY KEY,
			tenant_id         BIGINT      NOT NULL REFERENCES platform.schools(id),
			actor_account_id  BIGINT      NOT NULL REFERENCES auth.accounts(id),
			actor_role        TEXT        NOT NULL,
			resource_type     TEXT        NOT NULL,
			student_id        BIGINT      NULL REFERENCES users.students(id) ON DELETE SET NULL,
			range_start       TIMESTAMPTZ NOT NULL,
			range_end         TIMESTAMPTZ NOT NULL,
			accessed_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);

		CREATE INDEX IF NOT EXISTS idx_data_access_log_tenant_student
			ON audit.data_access_log (tenant_id, student_id, accessed_at DESC);
		CREATE INDEX IF NOT EXISTS idx_data_access_log_actor
			ON audit.data_access_log (actor_account_id, accessed_at DESC);
		CREATE INDEX IF NOT EXISTS idx_data_access_log_resource
			ON audit.data_access_log (tenant_id, resource_type, accessed_at DESC);

		ALTER TABLE audit.data_access_log ENABLE ROW LEVEL SECURITY;
		ALTER TABLE audit.data_access_log FORCE ROW LEVEL SECURITY;

		CREATE POLICY tenant_isolation_audit_data_access_log ON audit.data_access_log
			FOR ALL
			USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
			WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);
	`)
	if err != nil {
		return fmt.Errorf("error creating audit.data_access_log: %w", err)
	}
	fmt.Println("  ✓ audit.data_access_log — table, indexes, RLS created")

	return tx.Commit()
}

func dropAuditDataAccessLogTable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.27: Dropping audit.data_access_log...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	_, err = tx.ExecContext(ctx, `DROP TABLE IF EXISTS audit.data_access_log CASCADE;`)
	if err != nil {
		return fmt.Errorf("error dropping audit.data_access_log: %w", err)
	}

	return tx.Commit()
}
