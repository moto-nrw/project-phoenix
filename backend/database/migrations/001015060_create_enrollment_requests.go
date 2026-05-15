package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	createEnrollmentRequestsVersion     = "1.15.60"
	createEnrollmentRequestsDescription = "Create enrollment.requests table for parent submission records (PR 5)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     createEnrollmentRequestsVersion,
		Description: createEnrollmentRequestsDescription,
		DependsOn: []string{
			createEnrollmentFormSchemasVersion,
			"1.15.33", // schedule.calendar_periods
			"1.0.1",   // auth.accounts (guardian_account_id FK)
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createEnrollmentRequestsUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return createEnrollmentRequestsDown(ctx, db)
		},
	)
}

func createEnrollmentRequestsUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.60: Creating enrollment.requests table...")

	if _, err := db.NewRaw(`
		CREATE TABLE IF NOT EXISTS enrollment.requests (
			id                   BIGSERIAL PRIMARY KEY,
			tenant_id            BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			schema_id            BIGINT NOT NULL REFERENCES enrollment.form_schemas(id) ON DELETE RESTRICT,
			calendar_period_id   BIGINT NOT NULL REFERENCES schedule.calendar_periods(id) ON DELETE SET NULL,
			guardian_first_name  TEXT NOT NULL,
			guardian_last_name   TEXT NOT NULL,
			guardian_email       TEXT NOT NULL,
			guardian_phone       TEXT,
			guardian_account_id  BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
			consent_flags        JSONB NOT NULL DEFAULT '{}'::jsonb,
			custom_data          JSONB NOT NULL DEFAULT '{}'::jsonb,
			status_token         TEXT NOT NULL UNIQUE,
			status_token_expires TIMESTAMPTZ,
			submitted_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			withdrawn_at         TIMESTAMPTZ
		);
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed creating enrollment.requests: %w", err)
	}

	if _, err := db.NewRaw(`
		CREATE INDEX IF NOT EXISTS idx_enrollment_requests_tenant_period
		ON enrollment.requests (tenant_id, calendar_period_id);

		CREATE INDEX IF NOT EXISTS idx_enrollment_requests_email_lower
		ON enrollment.requests (tenant_id, LOWER(guardian_email));

		CREATE INDEX IF NOT EXISTS idx_enrollment_requests_submitted_at
		ON enrollment.requests (tenant_id, submitted_at DESC);
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed creating enrollment.requests indexes: %w", err)
	}

	if _, err := db.NewRaw(`
		ALTER TABLE enrollment.requests ENABLE ROW LEVEL SECURITY;
		ALTER TABLE enrollment.requests FORCE ROW LEVEL SECURITY;
		CREATE POLICY tenant_isolation_enrollment_requests ON enrollment.requests
			FOR ALL
			USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
			WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed enabling RLS on enrollment.requests: %w", err)
	}

	return nil
}

func createEnrollmentRequestsDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.60: Dropping enrollment.requests...")
	if _, err := db.NewRaw(`DROP TABLE IF EXISTS enrollment.requests CASCADE;`).Exec(ctx); err != nil {
		return fmt.Errorf("failed dropping enrollment.requests: %w", err)
	}
	return nil
}
