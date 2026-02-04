package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	staffAbsencesVersion     = "1.10.7"
	staffAbsencesDescription = "Create active.staff_absences table for absence tracking"
)

func init() {
	MigrationRegistry[staffAbsencesVersion] = &Migration{
		Version:     staffAbsencesVersion,
		Description: staffAbsencesDescription,
		DependsOn:   []string{"1.10.5"}, // Depends on work_session_edits
	}

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createStaffAbsences(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return dropStaffAbsences(ctx, db)
		},
	)
}

func createStaffAbsences(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.10.7: Creating active.staff_absences table...")

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
		CREATE TABLE IF NOT EXISTS active.staff_absences (
			id           BIGSERIAL PRIMARY KEY,
			staff_id     BIGINT NOT NULL REFERENCES users.staff(id),
			absence_type VARCHAR(20) NOT NULL,
			date_start   DATE NOT NULL,
			date_end     DATE NOT NULL,
			half_day     BOOLEAN NOT NULL DEFAULT false,
			note         TEXT,
			status       VARCHAR(20) NOT NULL DEFAULT 'reported',
			approved_by  BIGINT REFERENCES users.staff(id),
			approved_at  TIMESTAMPTZ,
			created_by   BIGINT NOT NULL REFERENCES users.staff(id),
			created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT chk_sa_dates CHECK (date_start <= date_end),
			CONSTRAINT chk_sa_type CHECK (absence_type IN ('sick','vacation','training','other')),
			CONSTRAINT chk_sa_status CHECK (status IN ('reported','approved','declined'))
		);

		CREATE INDEX IF NOT EXISTS idx_sa_staff_id ON active.staff_absences(staff_id);
		CREATE INDEX IF NOT EXISTS idx_sa_date_range ON active.staff_absences(date_start, date_end);
	`)
	if err != nil {
		return fmt.Errorf("error creating staff_absences table: %w", err)
	}

	fmt.Println("Migration 1.10.7: Successfully created active.staff_absences table")
	return tx.Commit()
}

func dropStaffAbsences(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.10.6: Dropping active.staff_absences table...")

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
		DROP TABLE IF EXISTS active.staff_absences CASCADE;
	`)
	if err != nil {
		return fmt.Errorf("error dropping staff_absences table: %w", err)
	}

	fmt.Println("Migration 1.10.7: Successfully rolled back")
	return tx.Commit()
}
