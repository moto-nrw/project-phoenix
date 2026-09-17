package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	workSessionEditsVersion     = "1.10.5"
	workSessionEditsDescription = "Create audit.work_session_edits table for time tracking audit trail"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     workSessionEditsVersion,
		Description: workSessionEditsDescription,
		DependsOn:   []string{"1.10.1"}, // Depends on work_sessions table
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createWorkSessionEdits(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return dropWorkSessionEdits(ctx, db)
		},
	)
}

func createWorkSessionEdits(ctx context.Context, db *bun.DB) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			logRollbackFailure(ctx, err)
		}
	}()

	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS audit.work_session_edits (
			id              BIGSERIAL PRIMARY KEY,
			session_id      BIGINT NOT NULL REFERENCES active.work_sessions(id) ON DELETE CASCADE,
			staff_id        BIGINT NOT NULL,
			edited_by       BIGINT NOT NULL,
			field_name      VARCHAR(50) NOT NULL,
			old_value       TEXT,
			new_value       TEXT,
			notes           TEXT,
			created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);

		CREATE INDEX IF NOT EXISTS idx_wse_session_id ON audit.work_session_edits(session_id);
		CREATE INDEX IF NOT EXISTS idx_wse_staff_id ON audit.work_session_edits(staff_id);
		CREATE INDEX IF NOT EXISTS idx_wse_created_at ON audit.work_session_edits(created_at);
	`)
	if err != nil {
		return fmt.Errorf("error creating work_session_edits table: %w", err)
	}

	return tx.Commit()
}

func dropWorkSessionEdits(ctx context.Context, db *bun.DB) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			logRollbackFailure(ctx, err)
		}
	}()

	_, err = tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS audit.work_session_edits CASCADE;
	`)
	if err != nil {
		return fmt.Errorf("error dropping work_session_edits table: %w", err)
	}

	return tx.Commit()
}
