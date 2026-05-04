package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	createEnrollmentPhasesVersion     = "1.15.56"
	createEnrollmentPhasesDescription = "Create enrollment.phases table — replaces tenant-wide open-window settings with per-phase windows + own form schema + own care_offering set (parent-enrollment phase model)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     createEnrollmentPhasesVersion,
		Description: createEnrollmentPhasesDescription,
		DependsOn: []string{
			createEnrollmentFormSchemasVersion,
			"1.15.1", // RLS infrastructure
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 1.15.56: Creating enrollment.phases table...")

			if _, err := db.NewRaw(`
				CREATE TABLE IF NOT EXISTS enrollment.phases (
					id                          BIGSERIAL PRIMARY KEY,
					tenant_id                   BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
					name                        TEXT NOT NULL,
					kind                        TEXT NOT NULL DEFAULT 'school_year'
						CHECK (kind IN ('school_year', 'holiday', 'custom')),
					service_start_date          DATE NOT NULL,
					service_end_date            DATE NOT NULL,
					enrollment_open_at          TIMESTAMPTZ,
					enrollment_close_at         TIMESTAMPTZ,
					form_schema_id              BIGINT REFERENCES enrollment.form_schemas(id) ON DELETE SET NULL,
					show_status_reason_to_parent BOOLEAN NOT NULL DEFAULT false,
					care_overflow_mode          TEXT NOT NULL DEFAULT 'waitlist'
						CHECK (care_overflow_mode IN ('waitlist', 'reject', 'allow')),
					is_active                   BOOLEAN NOT NULL DEFAULT true,
					created_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
					updated_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
					CONSTRAINT enrollment_phases_service_dates_chk
						CHECK (service_end_date >= service_start_date),
					CONSTRAINT enrollment_phases_window_chk
						CHECK (
							enrollment_open_at IS NULL
							OR enrollment_close_at IS NULL
							OR enrollment_close_at > enrollment_open_at
						),
					CONSTRAINT enrollment_phases_unique_name
						UNIQUE (tenant_id, name)
				);
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed creating enrollment.phases: %w", err)
			}

			if _, err := db.NewRaw(`
				CREATE INDEX IF NOT EXISTS idx_enrollment_phases_tenant_active
				ON enrollment.phases (tenant_id, is_active);

				CREATE INDEX IF NOT EXISTS idx_enrollment_phases_open_window
				ON enrollment.phases (tenant_id, enrollment_open_at, enrollment_close_at)
				WHERE is_active = true;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed creating phases indexes: %w", err)
			}

			if _, err := db.NewRaw(`
				ALTER TABLE enrollment.phases ENABLE ROW LEVEL SECURITY;
				ALTER TABLE enrollment.phases FORCE ROW LEVEL SECURITY;
				CREATE POLICY tenant_isolation_enrollment_phases ON enrollment.phases
					FOR ALL
					USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
					WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed enabling RLS on phases: %w", err)
			}

			if _, err := db.NewRaw(`
				GRANT SELECT, INSERT, UPDATE, DELETE ON enrollment.phases TO phoenix_tenant;
				GRANT USAGE ON SEQUENCE enrollment.phases_id_seq TO phoenix_tenant;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed granting phases permissions: %w", err)
			}

			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 1.15.56: Dropping enrollment.phases...")
			if _, err := db.NewRaw(`DROP TABLE IF EXISTS enrollment.phases CASCADE;`).Exec(ctx); err != nil {
				return fmt.Errorf("failed dropping enrollment.phases: %w", err)
			}
			return nil
		},
	)
}
