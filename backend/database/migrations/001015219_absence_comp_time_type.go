package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	absenceCompTimeTypeVersion     = "1.15.219"
	absenceCompTimeTypeDescription = "Add 'comp_time' (Freizeitausgleich) absence type to active.staff_absences type check (#1420 5b)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     absenceCompTimeTypeVersion,
		Description: absenceCompTimeTypeDescription,
		DependsOn:   []string{staffBalanceAdjustmentsVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return absenceCompTimeTypeUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return absenceCompTimeTypeDown(ctx, db)
		},
	)
}

// absenceCompTimeTypeUp widens the absence type CHECK by 'comp_time'
// (Freizeitausgleich, #1420 5b). A comp_time day keeps its Soll but is not
// credited by the Monatskarte, so the Stundenkonto drops by the day's
// contractual target when the absence is approved.
func absenceCompTimeTypeUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.219: Adding 'comp_time' to staff_absences type check...")

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
		ALTER TABLE active.staff_absences
			DROP CONSTRAINT IF EXISTS chk_sa_type;
	`)
	if err != nil {
		return fmt.Errorf("error dropping chk_sa_type: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		ALTER TABLE active.staff_absences
			ADD CONSTRAINT chk_sa_type
			CHECK (absence_type IN ('sick','vacation','training','other','comp_time'));
	`)
	if err != nil {
		return fmt.Errorf("error adding chk_sa_type: %w", err)
	}

	return tx.Commit()
}

func absenceCompTimeTypeDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.219: Removing 'comp_time' from staff_absences type check...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// Rows still typed 'comp_time' would violate the restored constraint —
	// move them to 'other' first (mirrors the 1.15.217 rollback safety).
	_, err = tx.ExecContext(ctx, `
		UPDATE active.staff_absences
			SET absence_type = 'other'
			WHERE absence_type = 'comp_time';
	`)
	if err != nil {
		return fmt.Errorf("error reassigning comp_time absences: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		ALTER TABLE active.staff_absences
			DROP CONSTRAINT IF EXISTS chk_sa_type;
	`)
	if err != nil {
		return fmt.Errorf("error dropping chk_sa_type: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		ALTER TABLE active.staff_absences
			ADD CONSTRAINT chk_sa_type
			CHECK (absence_type IN ('sick','vacation','training','other'));
	`)
	if err != nil {
		return fmt.Errorf("error restoring chk_sa_type: %w", err)
	}

	return tx.Commit()
}
