package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	vacationHalfDayEndpointsVersion     = "1.15.106"
	vacationHalfDayEndpointsDescription = "Add start_half_day + end_half_day to staff_absences for Personio-style half-day boundaries"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     vacationHalfDayEndpointsVersion,
		Description: vacationHalfDayEndpointsDescription,
		DependsOn:   []string{"1.15.105"},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return addVacationHalfDayEndpoints(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return removeVacationHalfDayEndpoints(ctx, db)
		},
	)
}

func addVacationHalfDayEndpoints(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.106: Adding start_half_day/end_half_day columns...")
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// New columns default false. Legacy half_day column stays for now —
	// services prefer the new columns when set, fall back to half_day so
	// rows from before this migration keep their semantic. A follow-up
	// migration may drop half_day once we are sure no caller writes it.
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE active.staff_absences
			ADD COLUMN IF NOT EXISTS start_half_day BOOLEAN NOT NULL DEFAULT false,
			ADD COLUMN IF NOT EXISTS end_half_day   BOOLEAN NOT NULL DEFAULT false;
	`)
	if err != nil {
		return fmt.Errorf("error adding half-day columns: %w", err)
	}

	// Backfill: if legacy half_day=true on an existing row, treat it as
	// "both ends halfed" so the working_days math stays equivalent for a
	// single-day request (half_day=true → 0.5 days, which equals start=true
	// + end=true on a 1-day range).
	_, err = tx.ExecContext(ctx, `
		UPDATE active.staff_absences
		SET start_half_day = true, end_half_day = true
		WHERE half_day = true AND start_half_day = false AND end_half_day = false;
	`)
	if err != nil {
		return fmt.Errorf("error backfilling half-day endpoints: %w", err)
	}

	fmt.Println("Migration 1.15.106: half-day endpoints ready")
	return tx.Commit()
}

func removeVacationHalfDayEndpoints(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.106...")
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
			DROP COLUMN IF EXISTS start_half_day,
			DROP COLUMN IF EXISTS end_half_day;
	`)
	if err != nil {
		return fmt.Errorf("error dropping half-day columns: %w", err)
	}
	return tx.Commit()
}
