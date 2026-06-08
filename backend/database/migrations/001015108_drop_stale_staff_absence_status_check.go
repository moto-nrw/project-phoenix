package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	dropStaleStaffAbsenceStatusCheckVersion     = "1.15.108"
	dropStaleStaffAbsenceStatusCheckDescription = "Drop stale chk_sa_status constraint that still blocks the new 'requested'/'canceled' statuses"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     dropStaleStaffAbsenceStatusCheckVersion,
		Description: dropStaleStaffAbsenceStatusCheckDescription,
		DependsOn:   []string{"1.15.105"},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return dropStaleStaffAbsenceStatusCheck(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return restoreStaleStaffAbsenceStatusCheck(ctx, db)
		},
	)
}

func dropStaleStaffAbsenceStatusCheck(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.108: Dropping legacy chk_sa_status constraint...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// The legacy CHECK named chk_sa_status only allows reported|approved|declined.
	// Migration 1.15.105 introduced staff_absences_status_check with the
	// expanded set (incl. requested|canceled) but tried to drop the OLD
	// constraint by the wrong name, so both ended up coexisting and the
	// stricter legacy one blocks every new vacation request.
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE active.staff_absences
			DROP CONSTRAINT IF EXISTS chk_sa_status;
	`)
	if err != nil {
		return fmt.Errorf("error dropping chk_sa_status: %w", err)
	}

	fmt.Println("Migration 1.15.108: chk_sa_status dropped")
	return tx.Commit()
}

func restoreStaleStaffAbsenceStatusCheck(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.108: Restoring legacy chk_sa_status...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// Restore the original constraint shape so rollback is symmetrical.
	// Any rows with status='requested' or 'canceled' will block this — the
	// caller is expected to clean those up first if rolling all the way back.
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE active.staff_absences
			ADD CONSTRAINT chk_sa_status
			CHECK (status IN ('reported','approved','declined'));
	`)
	if err != nil {
		return fmt.Errorf("error restoring chk_sa_status: %w", err)
	}

	return tx.Commit()
}
