package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	createCareOfferingsVersion     = "1.15.52"
	createCareOfferingsDescription = "Create enrollment.care_offerings table for parent-enrollment care catalog (PR 6)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     createCareOfferingsVersion,
		Description: createCareOfferingsDescription,
		DependsOn: []string{
			createEnrollmentFormSchemasVersion, // enrollment schema + grants
			"1.15.33",                          // schedule.calendar_periods
			ActivitiesGroupsVersion,            // activities.groups (activity_group_id FK target)
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createCareOfferingsUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return createCareOfferingsDown(ctx, db)
		},
	)
}

func createCareOfferingsUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.52: Creating enrollment.care_offerings table...")

	if _, err := db.NewRaw(`
		CREATE TABLE IF NOT EXISTS enrollment.care_offerings (
			id                       BIGSERIAL PRIMARY KEY,
			tenant_id                BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			calendar_period_id       BIGINT REFERENCES schedule.calendar_periods(id) ON DELETE SET NULL,
			activity_group_id        BIGINT REFERENCES activities.groups(id) ON DELETE SET NULL,
			name                     TEXT NOT NULL,
			description              TEXT,
			days_of_week_mode        TEXT NOT NULL DEFAULT 'fixed'
				CHECK (days_of_week_mode IN ('fixed', 'parent_choice')),
			available_days           JSONB NOT NULL DEFAULT '["mon","tue","wed","thu","fri"]'::jsonb,
			includes_holiday_care    BOOLEAN NOT NULL DEFAULT FALSE,
			includes_lunch           BOOLEAN NOT NULL DEFAULT FALSE,
			capacity                 INT,
			price_cents              INT,
			application_window_start TIMESTAMPTZ,
			application_window_end   TIMESTAMPTZ,
			service_start_date       DATE,
			service_end_date         DATE,
			is_active                BOOLEAN NOT NULL DEFAULT TRUE,
			sort_order               INT NOT NULL DEFAULT 0,
			created_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT chk_care_offerings_capacity_nonneg
				CHECK (capacity IS NULL OR capacity >= 0),
			CONSTRAINT chk_care_offerings_price_nonneg
				CHECK (price_cents IS NULL OR price_cents >= 0),
			CONSTRAINT chk_care_offerings_window_order
				CHECK (
					application_window_end IS NULL
					OR application_window_start IS NULL
					OR application_window_end > application_window_start
				),
			CONSTRAINT chk_care_offerings_service_order
				CHECK (
					service_end_date IS NULL
					OR service_start_date IS NULL
					OR service_end_date >= service_start_date
				)
		);
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed creating enrollment.care_offerings: %w", err)
	}

	if _, err := db.NewRaw(`
		CREATE INDEX IF NOT EXISTS idx_care_offerings_tenant_period
			ON enrollment.care_offerings (tenant_id, calendar_period_id);
		CREATE INDEX IF NOT EXISTS idx_care_offerings_active
			ON enrollment.care_offerings (tenant_id, is_active)
			WHERE is_active = TRUE;
		CREATE INDEX IF NOT EXISTS idx_care_offerings_window
			ON enrollment.care_offerings (tenant_id, application_window_start, application_window_end)
			WHERE is_active = TRUE;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed creating care_offerings indexes: %w", err)
	}

	if _, err := db.NewRaw(`
		ALTER TABLE enrollment.care_offerings ENABLE ROW LEVEL SECURITY;
		ALTER TABLE enrollment.care_offerings FORCE ROW LEVEL SECURITY;
		CREATE POLICY tenant_isolation_enrollment_care_offerings ON enrollment.care_offerings
			FOR ALL
			USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
			WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed enabling RLS on care_offerings: %w", err)
	}

	return nil
}

func createCareOfferingsDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.52: Dropping enrollment.care_offerings...")
	if _, err := db.NewRaw(`DROP TABLE IF EXISTS enrollment.care_offerings CASCADE;`).Exec(ctx); err != nil {
		return fmt.Errorf("failed dropping enrollment.care_offerings: %w", err)
	}
	return nil
}
