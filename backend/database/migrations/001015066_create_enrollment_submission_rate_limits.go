package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	createEnrollmentRateLimitsVersion     = "1.15.66"
	createEnrollmentRateLimitsDescription = "Create enrollment.submission_rate_limits for per-IP and per-email throttling on the public submit endpoint (parent-enrollment PR 7)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     createEnrollmentRateLimitsVersion,
		Description: createEnrollmentRateLimitsDescription,
		DependsOn: []string{
			createEnrollmentFormSchemasVersion, // ensures the enrollment schema exists
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 1.15.66: Creating enrollment.submission_rate_limits table...")
			if _, err := db.NewRaw(`
				CREATE TABLE IF NOT EXISTS enrollment.submission_rate_limits (
					id            BIGSERIAL PRIMARY KEY,
					tenant_id     BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
					key_type      TEXT   NOT NULL CHECK (key_type IN ('ip', 'email')),
					key_value     TEXT   NOT NULL,
					attempts      INT    NOT NULL DEFAULT 0,
					window_start  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
					updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
					UNIQUE (tenant_id, key_type, key_value)
				);
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed creating enrollment.submission_rate_limits: %w", err)
			}

			if _, err := db.NewRaw(`
				CREATE INDEX IF NOT EXISTS idx_enrollment_submission_rate_limits_window
				ON enrollment.submission_rate_limits (window_start);
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed creating index on enrollment.submission_rate_limits: %w", err)
			}

			// Tenant isolation: rate-limit rows carry guardian emails as
			// key_value, so cross-tenant reads/writes would both leak PII
			// and let a tenant clear another tenant's counter to bypass
			// throttling.
			if _, err := db.NewRaw(`
				ALTER TABLE enrollment.submission_rate_limits ENABLE ROW LEVEL SECURITY;
				ALTER TABLE enrollment.submission_rate_limits FORCE ROW LEVEL SECURITY;
				CREATE POLICY tenant_isolation_submission_rate_limits ON enrollment.submission_rate_limits
					FOR ALL
					USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
					WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed enabling RLS on enrollment.submission_rate_limits: %w", err)
			}

			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 1.15.66: Dropping enrollment.submission_rate_limits...")
			if _, err := db.NewRaw(`DROP TABLE IF EXISTS enrollment.submission_rate_limits CASCADE;`).Exec(ctx); err != nil {
				return fmt.Errorf("failed dropping enrollment.submission_rate_limits: %w", err)
			}
			return nil
		},
	)
}
