package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	workTimeStartTimesVersion     = "1.15.163"
	workTimeStartTimesDescription = "Add optional planned start times to staff work schedules"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     workTimeStartTimesVersion,
		Description: workTimeStartTimesDescription,
		DependsOn:   []string{"1.15.102"},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return addWorkTimeStartTimes(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return dropWorkTimeStartTimes(ctx, db)
		},
	)
}

func addWorkTimeStartTimes(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.163: Adding planned start times to work-time schedules...")

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
		ALTER TABLE config.staff_work_schedules
			ADD COLUMN IF NOT EXISTS start_time TIME WITHOUT TIME ZONE;

		ALTER TABLE config.work_time_model_entries
			ADD COLUMN IF NOT EXISTS start_time TIME WITHOUT TIME ZONE;
	`)
	if err != nil {
		return fmt.Errorf("error adding work-time start times: %w", err)
	}

	fmt.Println("Migration 1.15.163: Successfully added work-time start times")
	return tx.Commit()
}

func dropWorkTimeStartTimes(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.163: Removing work-time start times...")

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
		ALTER TABLE config.work_time_model_entries
			DROP COLUMN IF EXISTS start_time;

		ALTER TABLE config.staff_work_schedules
			DROP COLUMN IF EXISTS start_time;
	`)
	if err != nil {
		return fmt.Errorf("error removing work-time start times: %w", err)
	}

	fmt.Println("Migration 1.15.163: Successfully rolled back")
	return tx.Commit()
}
