package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	staffEmploymentVersion     = "1.10.8"
	staffEmploymentDescription = "Add employment_type to users.staff and create config.staff_work_schedules"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     staffEmploymentVersion,
		Description: staffEmploymentDescription,
		DependsOn:   []string{"1.10.7"},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return addStaffEmploymentFields(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return dropStaffEmploymentFields(ctx, db)
		},
	)
}

func addStaffEmploymentFields(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.10.8: Adding employment fields to users.staff...")

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
		ALTER TABLE users.staff
			ADD COLUMN IF NOT EXISTS employment_type VARCHAR(20);

		ALTER TABLE users.staff
			ADD CONSTRAINT chk_staff_employment_type
				CHECK (employment_type IS NULL OR employment_type IN ('full_time', 'part_time', 'minijob'));

		CREATE TABLE IF NOT EXISTS config.staff_work_schedules (
			id              BIGSERIAL PRIMARY KEY,
			tenant_id       BIGINT NOT NULL,
			staff_id        BIGINT NOT NULL REFERENCES users.staff(id) ON DELETE CASCADE,
			day_of_week     SMALLINT NOT NULL,
			target_minutes  INT NOT NULL,
			valid_from      DATE NOT NULL,
			valid_until     DATE,
			created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT chk_sws_day CHECK (day_of_week BETWEEN 0 AND 6),
			CONSTRAINT chk_sws_minutes CHECK (target_minutes >= 0 AND target_minutes <= 720),
			CONSTRAINT chk_sws_dates CHECK (valid_until IS NULL OR valid_until >= valid_from)
		);

		CREATE INDEX IF NOT EXISTS idx_sws_staff_id ON config.staff_work_schedules(staff_id);
		CREATE INDEX IF NOT EXISTS idx_sws_staff_valid ON config.staff_work_schedules(staff_id, valid_from, valid_until);
	`)
	if err != nil {
		return fmt.Errorf("error adding staff employment fields: %w", err)
	}

	fmt.Println("Migration 1.10.8: Successfully added employment fields to users.staff")
	return tx.Commit()
}

func dropStaffEmploymentFields(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.10.8: Removing employment fields from users.staff...")

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
		DROP TABLE IF EXISTS config.staff_work_schedules CASCADE;
		ALTER TABLE users.staff DROP CONSTRAINT IF EXISTS chk_staff_employment_type;
		ALTER TABLE users.staff DROP COLUMN IF EXISTS employment_type;
	`)
	if err != nil {
		return fmt.Errorf("error removing staff employment fields: %w", err)
	}

	fmt.Println("Migration 1.10.8: Successfully rolled back")
	return tx.Commit()
}
