package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	studentParentNotesVersion     = "1.15.112"
	studentParentNotesDescription = "Create users.student_parent_notes for parent-submitted free-text notes"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     studentParentNotesVersion,
		Description: studentParentNotesDescription,
		DependsOn: []string{
			"1.3.5", // users.students
			"1.0.1", // auth.accounts
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return studentParentNotesUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return studentParentNotesDown(ctx, db)
		},
	)
}

func studentParentNotesUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.112: Creating users.student_parent_notes...")

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
		CREATE TABLE IF NOT EXISTS users.student_parent_notes (
			id                  BIGSERIAL PRIMARY KEY,
			tenant_id           BIGINT NOT NULL REFERENCES platform.schools(id),
			student_id          BIGINT NOT NULL REFERENCES users.students(id) ON DELETE CASCADE,
			guardian_account_id BIGINT NOT NULL REFERENCES auth.accounts(id),
			body                TEXT NOT NULL,
			created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT chk_student_parent_notes_body_not_blank CHECK (length(btrim(body)) > 0)
		);

		CREATE INDEX IF NOT EXISTS idx_student_parent_notes_student_created
			ON users.student_parent_notes (tenant_id, student_id, created_at DESC);

		DROP TRIGGER IF EXISTS update_student_parent_notes_updated_at ON users.student_parent_notes;
		CREATE TRIGGER update_student_parent_notes_updated_at
		BEFORE UPDATE ON users.student_parent_notes
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();

		ALTER TABLE users.student_parent_notes ENABLE ROW LEVEL SECURITY;
		ALTER TABLE users.student_parent_notes FORCE ROW LEVEL SECURITY;

		CREATE POLICY tenant_isolation_users_student_parent_notes ON users.student_parent_notes
			FOR ALL
			USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
			WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);

		GRANT SELECT, INSERT, UPDATE, DELETE ON users.student_parent_notes TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE users.student_parent_notes_id_seq TO phoenix_tenant;
	`)
	if err != nil {
		return fmt.Errorf("error creating users.student_parent_notes: %w", err)
	}

	return tx.Commit()
}

func studentParentNotesDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.112: Dropping users.student_parent_notes...")

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
		DROP TRIGGER IF EXISTS update_student_parent_notes_updated_at ON users.student_parent_notes;
		DROP TABLE IF EXISTS users.student_parent_notes CASCADE;
	`)
	if err != nil {
		return fmt.Errorf("error dropping users.student_parent_notes: %w", err)
	}

	return tx.Commit()
}
