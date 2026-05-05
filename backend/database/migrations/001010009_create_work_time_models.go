package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	workTimeModelsVersion     = "1.10.9"
	workTimeModelsDescription = "Create config.work_time_models, model entries, and rotation columns on staff + schedules"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     workTimeModelsVersion,
		Description: workTimeModelsDescription,
		DependsOn:   []string{"1.10.8"},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createWorkTimeModels(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return dropWorkTimeModels(ctx, db)
		},
	)
}

func createWorkTimeModels(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.10.9: Creating work-time model tables and rotation columns...")

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
		CREATE TABLE IF NOT EXISTS config.work_time_models (
			id                    BIGSERIAL PRIMARY KEY,
			tenant_id             BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			name                  VARCHAR(100) NOT NULL,
			rotation_length       SMALLINT NOT NULL DEFAULT 1,
			rotation_anchor_date  DATE NOT NULL,
			created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT chk_wtm_rotation CHECK (rotation_length BETWEEN 1 AND 4),
			CONSTRAINT uq_wtm_tenant_name UNIQUE (tenant_id, name)
		);

		CREATE INDEX IF NOT EXISTS idx_wtm_tenant ON config.work_time_models(tenant_id);

		CREATE TABLE IF NOT EXISTS config.work_time_model_entries (
			id              BIGSERIAL PRIMARY KEY,
			model_id        BIGINT NOT NULL REFERENCES config.work_time_models(id) ON DELETE CASCADE,
			week_index      SMALLINT NOT NULL,
			day_of_week     SMALLINT NOT NULL,
			target_minutes  INT NOT NULL,
			created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT chk_wtme_week CHECK (week_index BETWEEN 0 AND 3),
			CONSTRAINT chk_wtme_day CHECK (day_of_week BETWEEN 0 AND 6),
			CONSTRAINT chk_wtme_minutes CHECK (target_minutes >= 0 AND target_minutes <= 720),
			CONSTRAINT uq_wtme_slot UNIQUE (model_id, week_index, day_of_week)
		);

		CREATE INDEX IF NOT EXISTS idx_wtme_model ON config.work_time_model_entries(model_id);

		ALTER TABLE users.staff
			ADD COLUMN IF NOT EXISTS work_time_model_id BIGINT REFERENCES config.work_time_models(id) ON DELETE SET NULL;

		ALTER TABLE users.staff
			ADD COLUMN IF NOT EXISTS rotation_anchor_date DATE;

		ALTER TABLE config.staff_work_schedules
			ADD COLUMN IF NOT EXISTS week_index SMALLINT NOT NULL DEFAULT 0;

		ALTER TABLE config.staff_work_schedules
			ADD CONSTRAINT chk_sws_week CHECK (week_index BETWEEN 0 AND 3);

		ALTER TABLE config.staff_work_schedules
			ADD COLUMN IF NOT EXISTS rotation_length SMALLINT NOT NULL DEFAULT 1;

		ALTER TABLE config.staff_work_schedules
			ADD CONSTRAINT chk_sws_rotation CHECK (rotation_length BETWEEN 1 AND 4);
	`)
	if err != nil {
		return fmt.Errorf("error creating work-time model tables: %w", err)
	}

	fmt.Println("Migration 1.10.9: Successfully created work-time model tables")
	return tx.Commit()
}

func dropWorkTimeModels(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.10.9: Dropping work-time model tables...")

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
		ALTER TABLE config.staff_work_schedules DROP CONSTRAINT IF EXISTS chk_sws_rotation;
		ALTER TABLE config.staff_work_schedules DROP COLUMN IF EXISTS rotation_length;
		ALTER TABLE config.staff_work_schedules DROP CONSTRAINT IF EXISTS chk_sws_week;
		ALTER TABLE config.staff_work_schedules DROP COLUMN IF EXISTS week_index;
		ALTER TABLE users.staff DROP COLUMN IF EXISTS rotation_anchor_date;
		ALTER TABLE users.staff DROP COLUMN IF EXISTS work_time_model_id;
		DROP TABLE IF EXISTS config.work_time_model_entries CASCADE;
		DROP TABLE IF EXISTS config.work_time_models CASCADE;
	`)
	if err != nil {
		return fmt.Errorf("error dropping work-time model tables: %w", err)
	}

	fmt.Println("Migration 1.10.9: Successfully rolled back")
	return tx.Commit()
}
