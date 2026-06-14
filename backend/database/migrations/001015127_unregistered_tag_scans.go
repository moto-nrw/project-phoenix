package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	unregisteredTagScansVersion     = "1.15.127"
	unregisteredTagScansDescription = "Track unregistered RFID tag scans for operator visibility"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     unregisteredTagScansVersion,
		Description: unregisteredTagScansDescription,
		DependsOn:   []string{"1.15.124"},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createUnregisteredTagScansTable(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return dropUnregisteredTagScansTable(ctx, db)
		},
	)
}

func createUnregisteredTagScansTable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.127: Creating audit.unregistered_tag_scans...")

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
		CREATE TABLE IF NOT EXISTS audit.unregistered_tag_scans (
			id BIGSERIAL PRIMARY KEY,
			tenant_id BIGINT NOT NULL REFERENCES platform.schools(id),
			tag_uid TEXT NOT NULL,
			device_id BIGINT NULL REFERENCES iot.devices(id) ON DELETE SET NULL,
			scanned_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			resolved_at TIMESTAMPTZ NULL,
			resolved_by_operator_id BIGINT NULL REFERENCES platform.operators(id) ON DELETE SET NULL,
			resolution_note TEXT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);

		CREATE INDEX IF NOT EXISTS idx_unregistered_tag_scans_tenant_unresolved
			ON audit.unregistered_tag_scans (tenant_id, scanned_at DESC)
			WHERE resolved_at IS NULL;
		CREATE INDEX IF NOT EXISTS idx_unregistered_tag_scans_tenant_tag
			ON audit.unregistered_tag_scans (tenant_id, tag_uid, scanned_at DESC);
		CREATE INDEX IF NOT EXISTS idx_unregistered_tag_scans_device
			ON audit.unregistered_tag_scans (device_id, scanned_at DESC)
			WHERE device_id IS NOT NULL;

		DROP TRIGGER IF EXISTS update_unregistered_tag_scans_updated_at ON audit.unregistered_tag_scans;
		CREATE TRIGGER update_unregistered_tag_scans_updated_at
		BEFORE UPDATE ON audit.unregistered_tag_scans
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();

		ALTER TABLE audit.unregistered_tag_scans ENABLE ROW LEVEL SECURITY;
		ALTER TABLE audit.unregistered_tag_scans FORCE ROW LEVEL SECURITY;

		DROP POLICY IF EXISTS tenant_isolation_audit_unregistered_tag_scans
			ON audit.unregistered_tag_scans;
		CREATE POLICY tenant_isolation_audit_unregistered_tag_scans
			ON audit.unregistered_tag_scans
			FOR ALL
			USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
			WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);

		GRANT SELECT, INSERT, UPDATE, DELETE ON audit.unregistered_tag_scans TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE audit.unregistered_tag_scans_id_seq TO phoenix_tenant;
	`)
	if err != nil {
		return fmt.Errorf("error creating audit.unregistered_tag_scans: %w", err)
	}

	return tx.Commit()
}

func dropUnregisteredTagScansTable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.127: Dropping audit.unregistered_tag_scans...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	_, err = tx.ExecContext(ctx, `DROP TABLE IF EXISTS audit.unregistered_tag_scans CASCADE;`)
	if err != nil {
		return fmt.Errorf("error dropping audit.unregistered_tag_scans: %w", err)
	}

	return tx.Commit()
}
