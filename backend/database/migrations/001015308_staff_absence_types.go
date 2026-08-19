package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	staffAbsenceTypesVersion     = "1.15.308"
	staffAbsenceTypesDescription = "Create active.staff_absence_types (schulbezogene Abwesenheitsarten) and link active.staff_absences via absence_type_id (#2403)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     staffAbsenceTypesVersion,
		Description: staffAbsenceTypesDescription,
		DependsOn:   []string{absenceCompTimeTypeVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return staffAbsenceTypesUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return staffAbsenceTypesDown(ctx, db)
		},
	)
}

func staffAbsenceTypesUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.308: Creating active.staff_absence_types + staff_absences.absence_type_id...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// Tenant-defined absence names (#2403). Only a school's OWN additions live
	// here — the five standard types stay code constants, so every tenant has
	// them by construction and none can be renamed, deleted or duplicated.
	//
	// base_type pins the canonical absence type whose arithmetic the entry
	// inherits. The CHECK mirrors chk_sa_type (migration 1.15.220) so the two
	// value sets cannot drift; v1 only ever writes 'other'.
	//
	// UNIQUE(tenant_id, id) backs the composite FK from staff_absences, so an
	// absence can only reference a type from its own tenant. The
	// case-insensitive unique name keeps "Ferienzeit" and "ferienzeit" from
	// both existing.
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS active.staff_absence_types (
			id         BIGSERIAL PRIMARY KEY,
			tenant_id  BIGINT NOT NULL REFERENCES platform.schools(id),
			name       TEXT NOT NULL,
			base_type  VARCHAR(20) NOT NULL DEFAULT 'other',
			is_active  BOOLEAN NOT NULL DEFAULT TRUE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT uniq_staff_absence_types_tenant_id UNIQUE (tenant_id, id),
			CONSTRAINT chk_sat_base_type CHECK (base_type IN ('sick','vacation','training','other','comp_time'))
		);

		CREATE UNIQUE INDEX IF NOT EXISTS uniq_staff_absence_types_tenant_name
			ON active.staff_absence_types (tenant_id, LOWER(name));

		DROP TRIGGER IF EXISTS update_staff_absence_types_updated_at ON active.staff_absence_types;
		CREATE TRIGGER update_staff_absence_types_updated_at
		BEFORE UPDATE ON active.staff_absence_types
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();

		ALTER TABLE active.staff_absence_types ENABLE ROW LEVEL SECURITY;
		ALTER TABLE active.staff_absence_types FORCE ROW LEVEL SECURITY;

		DROP POLICY IF EXISTS tenant_isolation_active_staff_absence_types ON active.staff_absence_types;
		CREATE POLICY tenant_isolation_active_staff_absence_types ON active.staff_absence_types
			FOR ALL
			USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
			WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);

		GRANT SELECT, INSERT, UPDATE, DELETE ON active.staff_absence_types TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE active.staff_absence_types_id_seq TO phoenix_tenant;

		-- Optional named subtype of an absence. NULL = one of the five standard
		-- types, identified by absence_type as before; every existing row stays
		-- untouched and every existing reader keeps working.
		ALTER TABLE active.staff_absences
			ADD COLUMN IF NOT EXISTS absence_type_id BIGINT;

		-- MATCH SIMPLE (default): a NULL absence_type_id skips the composite FK
		-- check. ON DELETE RESTRICT, not SET NULL: a used art must stay readable
		-- as history, so retirement is is_active = FALSE, never a delete. The
		-- API offers no delete at all; this is the structural backstop.
		ALTER TABLE active.staff_absences
			DROP CONSTRAINT IF EXISTS fk_staff_absences_absence_type;
		ALTER TABLE active.staff_absences
			ADD CONSTRAINT fk_staff_absences_absence_type
				FOREIGN KEY (tenant_id, absence_type_id)
				REFERENCES active.staff_absence_types(tenant_id, id)
				ON DELETE RESTRICT;

		-- A custom name may only ride on the base type it inherits. Without this
		-- an "Regenerationstag" label could be pinned to a 'vacation' row and
		-- quietly consume the Urlaubskontingent under a name that hides it.
		ALTER TABLE active.staff_absences
			DROP CONSTRAINT IF EXISTS chk_sa_custom_type_is_other;
		ALTER TABLE active.staff_absences
			ADD CONSTRAINT chk_sa_custom_type_is_other
				CHECK (absence_type_id IS NULL OR absence_type = 'other');

		CREATE INDEX IF NOT EXISTS idx_staff_absences_absence_type_id
			ON active.staff_absences (tenant_id, absence_type_id);
	`)
	if err != nil {
		return fmt.Errorf("error creating active.staff_absence_types: %w", err)
	}

	return tx.Commit()
}

func staffAbsenceTypesDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.308: Dropping active.staff_absence_types + staff_absences.absence_type_id...")

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
		ALTER TABLE active.staff_absences DROP CONSTRAINT IF EXISTS chk_sa_custom_type_is_other;
		ALTER TABLE active.staff_absences DROP CONSTRAINT IF EXISTS fk_staff_absences_absence_type;
		DROP INDEX IF EXISTS active.idx_staff_absences_absence_type_id;
		ALTER TABLE active.staff_absences DROP COLUMN IF EXISTS absence_type_id;

		DROP TRIGGER IF EXISTS update_staff_absence_types_updated_at ON active.staff_absence_types;
		DROP TABLE IF EXISTS active.staff_absence_types CASCADE;
	`)
	if err != nil {
		return fmt.Errorf("error dropping active.staff_absence_types: %w", err)
	}
	return tx.Commit()
}
