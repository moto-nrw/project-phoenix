package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	parentMessageStaffNameVisibilityVersion     = "1.15.158"
	parentMessageStaffNameVisibilityDescription = "Per-message frozen flag for showing the staff sender's name to guardians (#1672)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     parentMessageStaffNameVisibilityVersion,
		Description: parentMessageStaffNameVisibilityDescription,
		DependsOn:   []string{parentMessagingRequestsVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return parentMessageStaffNameVisibilityUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return parentMessageStaffNameVisibilityDown(ctx, db)
		},
	)
}

func parentMessageStaffNameVisibilityUp(ctx context.Context, db *bun.DB) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			logRollbackFailure(ctx, err)
		}
	}()

	// DEFAULT false backfills every EXISTING row to "keep the OGS label". Those
	// replies were written while individual staff names never left the backend, so
	// they must stay anonymous even after a school enables the setting — the
	// "reveal only from activation onwards" rule. New staff messages get their
	// value stamped explicitly by the send path (frozen at write time), so the
	// column default only ever applies to this backfill.
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE users.parent_messages
			ADD COLUMN IF NOT EXISTS staff_name_visible BOOLEAN NOT NULL DEFAULT false;
	`)
	if err != nil {
		return fmt.Errorf("error adding users.parent_messages.staff_name_visible: %w", err)
	}

	return tx.Commit()
}

func parentMessageStaffNameVisibilityDown(ctx context.Context, db *bun.DB) error {
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
		ALTER TABLE users.parent_messages
			DROP COLUMN IF EXISTS staff_name_visible;
	`)
	if err != nil {
		return fmt.Errorf("error removing users.parent_messages.staff_name_visible: %w", err)
	}

	return tx.Commit()
}
