package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	workSessionsVersion     = "1.10.1"
	workSessionsDescription = "Create work sessions table for staff time tracking"
)

func init() {
	MigrationRegistry[workSessionsVersion] = &Migration{
		Version:     workSessionsVersion,
		Description: workSessionsDescription,
		DependsOn:   []string{"1.0.1"}, // Depends on schema creation
	}

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createWorkSessionsTable(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return dropWorkSessionsTable(ctx, db)
		},
	)
}

func createWorkSessionsTable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.10.1: Creating active.work_sessions table...")

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
		CREATE TABLE IF NOT EXISTS active.work_sessions (
			id            BIGSERIAL PRIMARY KEY,
			staff_id      BIGINT NOT NULL REFERENCES users.staff(id),
			date          DATE NOT NULL DEFAULT CURRENT_DATE,
			status        VARCHAR(20) NOT NULL DEFAULT 'present',
			check_in_time TIMESTAMPTZ NOT NULL,
			check_out_time TIMESTAMPTZ,
			break_minutes INT NOT NULL DEFAULT 0,
			notes         TEXT,
			auto_checked_out BOOLEAN NOT NULL DEFAULT FALSE,
			created_by    BIGINT NOT NULL REFERENCES users.staff(id),
			updated_by    BIGINT REFERENCES users.staff(id),
			created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			
			CONSTRAINT uq_work_sessions_staff_date UNIQUE(staff_id, date),
			CONSTRAINT chk_work_sessions_status CHECK (status IN ('present', 'home_office')),
			CONSTRAINT chk_work_sessions_break_minutes CHECK (break_minutes >= 0),
			CONSTRAINT chk_work_sessions_checkout_after_checkin CHECK (
				check_out_time IS NULL OR check_out_time > check_in_time
			)
		);

		CREATE INDEX idx_work_sessions_staff_date ON active.work_sessions(staff_id, date DESC);
		CREATE INDEX idx_work_sessions_date ON active.work_sessions(date);
	`)
	if err != nil {
		return fmt.Errorf("error creating active.work_sessions table: %w", err)
	}

	fmt.Println("Migration 1.10.1: Successfully created active.work_sessions table")
	return tx.Commit()
}

func dropWorkSessionsTable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.10.1: Dropping active.work_sessions table...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	_, err = tx.ExecContext(ctx, `DROP TABLE IF EXISTS active.work_sessions CASCADE;`)
	if err != nil {
		return fmt.Errorf("error dropping active.work_sessions table: %w", err)
	}

	fmt.Println("Migration 1.10.1: Successfully dropped active.work_sessions table")
	return tx.Commit()
}
