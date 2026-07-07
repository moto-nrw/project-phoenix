package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	shiftTypesVersion     = "1.15.170"
	shiftTypesDescription = "Create schedule.shift_types (Schichtarten) and link staff_shifts via shift_type_id (#1836)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     shiftTypesVersion,
		Description: shiftTypesDescription,
		DependsOn:   []string{staffShiftsVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return shiftTypesUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return shiftTypesDown(ctx, db)
		},
	)
}

func shiftTypesUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.170: Creating schedule.shift_types + staff_shifts.shift_type_id...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// Tenant-defined shift types (Schichtarten): the *kind* of duty a planned
	// shift represents. Schools manage their own set — nothing is hard-wired.
	// UNIQUE(tenant_id, id) backs the composite FK from staff_shifts so a shift
	// can only reference a type from its own tenant. A case-insensitive unique
	// name per tenant keeps the list clean and lets example seeding be
	// idempotent.
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schedule.shift_types (
			id          BIGSERIAL PRIMARY KEY,
			tenant_id   BIGINT NOT NULL REFERENCES platform.schools(id),
			name        TEXT NOT NULL,
			color       TEXT NOT NULL,
			description TEXT,
			is_active   BOOLEAN NOT NULL DEFAULT TRUE,
			created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT uniq_shift_types_tenant_id UNIQUE (tenant_id, id)
		);

		CREATE UNIQUE INDEX IF NOT EXISTS uniq_shift_types_tenant_name
			ON schedule.shift_types (tenant_id, LOWER(name));

		DROP TRIGGER IF EXISTS update_shift_types_updated_at ON schedule.shift_types;
		CREATE TRIGGER update_shift_types_updated_at
		BEFORE UPDATE ON schedule.shift_types
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();

		ALTER TABLE schedule.shift_types ENABLE ROW LEVEL SECURITY;
		ALTER TABLE schedule.shift_types FORCE ROW LEVEL SECURITY;

		DROP POLICY IF EXISTS tenant_isolation_schedule_shift_types ON schedule.shift_types;
		CREATE POLICY tenant_isolation_schedule_shift_types ON schedule.shift_types
			FOR ALL
			USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
			WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);

		GRANT SELECT, INSERT, UPDATE, DELETE ON schedule.shift_types TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE schedule.shift_types_id_seq TO phoenix_tenant;

		-- Link planned shifts to an optional shift type. MATCH SIMPLE (default):
		-- a NULL shift_type_id skips the composite FK check, so untyped shifts
		-- stay valid. ON DELETE SET NULL (shift_type_id) clears ONLY the type
		-- reference when a type is removed — a plain SET NULL would also null
		-- the shared tenant_id column (NOT NULL) and fail the delete.
		ALTER TABLE schedule.staff_shifts
			ADD COLUMN IF NOT EXISTS shift_type_id BIGINT;

		ALTER TABLE schedule.staff_shifts
			DROP CONSTRAINT IF EXISTS fk_staff_shifts_shift_type;
		ALTER TABLE schedule.staff_shifts
			ADD CONSTRAINT fk_staff_shifts_shift_type
				FOREIGN KEY (tenant_id, shift_type_id)
				REFERENCES schedule.shift_types(tenant_id, id)
				ON DELETE SET NULL (shift_type_id);

		CREATE INDEX IF NOT EXISTS idx_staff_shifts_shift_type
			ON schedule.staff_shifts (tenant_id, shift_type_id);
	`)
	if err != nil {
		return fmt.Errorf("error creating schedule.shift_types: %w", err)
	}

	return tx.Commit()
}

func shiftTypesDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.170: Dropping schedule.shift_types + staff_shifts.shift_type_id...")

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
		ALTER TABLE schedule.staff_shifts DROP CONSTRAINT IF EXISTS fk_staff_shifts_shift_type;
		DROP INDEX IF EXISTS schedule.idx_staff_shifts_shift_type;
		ALTER TABLE schedule.staff_shifts DROP COLUMN IF EXISTS shift_type_id;

		DROP TRIGGER IF EXISTS update_shift_types_updated_at ON schedule.shift_types;
		DROP TABLE IF EXISTS schedule.shift_types CASCADE;
	`)
	if err != nil {
		return fmt.Errorf("error dropping schedule.shift_types: %w", err)
	}
	return tx.Commit()
}
