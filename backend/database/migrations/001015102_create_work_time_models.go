package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	workTimeModelsVersion     = "1.15.102"
	workTimeModelsDescription = "Create config.work_time_models, model entries, and rotation columns on staff + schedules"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     workTimeModelsVersion,
		Description: workTimeModelsDescription,
		// platform.schools is the FK target for tenant_id (created in 1.13.1).
		// staff_work_schedules + users.staff (FK target for work_time_model_id)
		// live in 1.10.8.
		DependsOn: []string{"1.10.8", "1.13.1"},
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
	fmt.Println("Migration 1.15.102: Creating work-time model tables and rotation columns...")

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
				ADD COLUMN IF NOT EXISTS work_time_model_id BIGINT;

			DO $$
			DECLARE
				fk_name text;
			BEGIN
				SELECT conname INTO fk_name
				FROM pg_constraint
				WHERE conrelid = 'users.staff'::regclass
					AND contype = 'f'
					AND conkey = ARRAY[
						(
							SELECT attnum
							FROM pg_attribute
							WHERE attrelid = 'users.staff'::regclass
								AND attname = 'work_time_model_id'
						)
					]
				LIMIT 1;

				IF fk_name IS NOT NULL THEN
					EXECUTE format('ALTER TABLE users.staff DROP CONSTRAINT %I', fk_name);
				END IF;
			END $$;

			ALTER TABLE users.staff
				ADD CONSTRAINT fk_staff_work_time_model
				FOREIGN KEY (work_time_model_id)
				REFERENCES config.work_time_models(id)
				ON DELETE RESTRICT;

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

				DO $$
				BEGIN
					IF NOT EXISTS (
						SELECT 1
						FROM pg_constraint
						WHERE conrelid = 'config.staff_work_schedules'::regclass
							AND conname = 'fk_staff_work_schedules_tenant'
					) THEN
						ALTER TABLE config.staff_work_schedules
							ADD CONSTRAINT fk_staff_work_schedules_tenant
							FOREIGN KEY (tenant_id)
							REFERENCES platform.schools(id)
							ON DELETE CASCADE;
					END IF;
				END $$;

				ALTER TABLE config.work_time_models ENABLE ROW LEVEL SECURITY;
			ALTER TABLE config.work_time_models FORCE ROW LEVEL SECURITY;
			DROP POLICY IF EXISTS tenant_isolation_config_work_time_models ON config.work_time_models;
			CREATE POLICY tenant_isolation_config_work_time_models ON config.work_time_models
				FOR ALL
				USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
				WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);

			ALTER TABLE config.work_time_model_entries ENABLE ROW LEVEL SECURITY;
			ALTER TABLE config.work_time_model_entries FORCE ROW LEVEL SECURITY;
			DROP POLICY IF EXISTS tenant_isolation_config_work_time_model_entries ON config.work_time_model_entries;
			CREATE POLICY tenant_isolation_config_work_time_model_entries ON config.work_time_model_entries
				FOR ALL
				USING (
					EXISTS (
						SELECT 1
						FROM config.work_time_models AS model
						WHERE model.id = model_id
							AND model.tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint
					)
				)
				WITH CHECK (
					EXISTS (
						SELECT 1
						FROM config.work_time_models AS model
						WHERE model.id = model_id
							AND model.tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint
					)
				);

			ALTER TABLE config.staff_work_schedules ENABLE ROW LEVEL SECURITY;
			ALTER TABLE config.staff_work_schedules FORCE ROW LEVEL SECURITY;
			DROP POLICY IF EXISTS tenant_isolation_config_staff_work_schedules ON config.staff_work_schedules;
			CREATE POLICY tenant_isolation_config_staff_work_schedules ON config.staff_work_schedules
				FOR ALL
				USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
				WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);
		`)
	if err != nil {
		return fmt.Errorf("error creating work-time model tables: %w", err)
	}

	fmt.Println("Migration 1.15.102: Successfully created work-time model tables")
	return tx.Commit()
}

func dropWorkTimeModels(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.102: Dropping work-time model tables...")

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
			ALTER TABLE config.staff_work_schedules DROP CONSTRAINT IF EXISTS fk_staff_work_schedules_tenant;
			ALTER TABLE config.staff_work_schedules DROP CONSTRAINT IF EXISTS chk_sws_rotation;
			ALTER TABLE config.staff_work_schedules DROP COLUMN IF EXISTS rotation_length;
				ALTER TABLE config.staff_work_schedules DROP CONSTRAINT IF EXISTS chk_sws_week;
			ALTER TABLE config.staff_work_schedules DROP COLUMN IF EXISTS week_index;
			ALTER TABLE users.staff DROP COLUMN IF EXISTS rotation_anchor_date;
			ALTER TABLE users.staff DROP CONSTRAINT IF EXISTS fk_staff_work_time_model;
			ALTER TABLE users.staff DROP COLUMN IF EXISTS work_time_model_id;
		DROP TABLE IF EXISTS config.work_time_model_entries CASCADE;
		DROP TABLE IF EXISTS config.work_time_models CASCADE;
	`)
	if err != nil {
		return fmt.Errorf("error dropping work-time model tables: %w", err)
	}

	fmt.Println("Migration 1.15.102: Successfully rolled back")
	return tx.Commit()
}
