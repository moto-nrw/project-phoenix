package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	studentFieldEditsVersion     = "1.15.228"
	studentFieldEditsDescription = "Create audit.student_field_edits for per-child change history (issue #1455)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     studentFieldEditsVersion,
		Description: studentFieldEditsDescription,
		DependsOn: []string{
			"1.15.130", // students table + allowed departure modes exist
			"1.15.225", // audit schema defaults are append-only
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createStudentFieldEditsTable(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return dropStudentFieldEditsTable(ctx, db)
		},
	)
}

func createStudentFieldEditsTable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.228: Creating audit.student_field_edits...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// edited_by_name is a denormalized snapshot of the editor's display name at
	// change time: an audit log must survive renames/deletions and render
	// without a join. edited_by has no FK on purpose so the history outlives a
	// deleted account.
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS audit.student_field_edits (
			id              BIGSERIAL PRIMARY KEY,
			tenant_id       BIGINT NOT NULL REFERENCES platform.schools(id),
			student_id      BIGINT NOT NULL REFERENCES users.students(id) ON DELETE CASCADE,
			edited_by       BIGINT NOT NULL,
			edited_by_name  TEXT NOT NULL,
			field_name      VARCHAR(50) NOT NULL,
			old_value       TEXT,
			new_value       TEXT,
			created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);

		CREATE INDEX IF NOT EXISTS idx_sfe_student_created
			ON audit.student_field_edits (student_id, created_at DESC);
		CREATE INDEX IF NOT EXISTS idx_sfe_tenant_created
			ON audit.student_field_edits (tenant_id, created_at DESC);

		ALTER TABLE audit.student_field_edits ENABLE ROW LEVEL SECURITY;
		ALTER TABLE audit.student_field_edits FORCE ROW LEVEL SECURITY;

		DROP POLICY IF EXISTS tenant_isolation_audit_student_field_edits
			ON audit.student_field_edits;
		CREATE POLICY tenant_isolation_audit_student_field_edits
			ON audit.student_field_edits
			FOR ALL
			USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
			WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);

		-- Audit rows are append-only except for the tenant-scoped retention job.
		-- Migration 1.15.225 revoked the schema default UPDATE/DELETE grants;
		-- restore only the DELETE privilege that DeleteOlderThan requires.
		GRANT SELECT, INSERT, DELETE ON audit.student_field_edits TO phoenix_tenant;
		REVOKE UPDATE, TRUNCATE ON audit.student_field_edits FROM phoenix_tenant;
		GRANT USAGE ON SEQUENCE audit.student_field_edits_id_seq TO phoenix_tenant;
	`)
	if err != nil {
		return fmt.Errorf("error creating audit.student_field_edits: %w", err)
	}

	return tx.Commit()
}

func dropStudentFieldEditsTable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.228: Dropping audit.student_field_edits...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	_, err = tx.ExecContext(ctx, `DROP TABLE IF EXISTS audit.student_field_edits CASCADE;`)
	if err != nil {
		return fmt.Errorf("error dropping audit.student_field_edits: %w", err)
	}

	return tx.Commit()
}
