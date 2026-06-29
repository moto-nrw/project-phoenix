package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	lateInvitesAndRequestSourcesVersion     = "1.15.153"
	lateInvitesAndRequestSourcesDescription = "Add enrollment request sources and late invite tokens for closed-phase parent submissions"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     lateInvitesAndRequestSourcesVersion,
		Description: lateInvitesAndRequestSourcesDescription,
		DependsOn:   []string{repairEnrollmentChangeRequestsVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 1.15.153: Adding enrollment late invites and request source fields...")
			if _, err := db.NewRaw(`
				ALTER TABLE enrollment.requests
					ADD COLUMN IF NOT EXISTS submission_source TEXT NOT NULL DEFAULT 'public',
					ADD COLUMN IF NOT EXISTS source_metadata JSONB NOT NULL DEFAULT '{}'::jsonb;

				DO $$
				BEGIN
					IF NOT EXISTS (
						SELECT 1 FROM pg_constraint
						WHERE conname = 'chk_enrollment_requests_submission_source'
							AND conrelid = 'enrollment.requests'::regclass
					) THEN
						ALTER TABLE enrollment.requests
							ADD CONSTRAINT chk_enrollment_requests_submission_source
							CHECK (submission_source IN ('public', 'late_invite', 'admin_manual'));
					END IF;
				END $$;

				CREATE TABLE IF NOT EXISTS enrollment.late_invites (
					id BIGSERIAL PRIMARY KEY,
					tenant_id BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
					phase_id BIGINT NOT NULL REFERENCES enrollment.phases(id) ON DELETE CASCADE,
					token_hash TEXT NOT NULL,
					guardian_email TEXT NOT NULL,
					guardian_first_name TEXT,
					guardian_last_name TEXT,
					expires_at TIMESTAMPTZ NOT NULL,
					used_at TIMESTAMPTZ,
					used_request_id BIGINT REFERENCES enrollment.requests(id) ON DELETE SET NULL,
					created_by BIGINT NOT NULL REFERENCES auth.accounts(id) ON DELETE RESTRICT,
					reason TEXT,
					created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
					updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
					CONSTRAINT uq_enrollment_late_invites_token_hash UNIQUE (tenant_id, token_hash),
					CONSTRAINT chk_enrollment_late_invites_email CHECK (length(btrim(guardian_email)) > 0),
					CONSTRAINT chk_enrollment_late_invites_expiry CHECK (expires_at > created_at)
				);

				CREATE INDEX IF NOT EXISTS idx_enrollment_late_invites_phase
					ON enrollment.late_invites (tenant_id, phase_id, created_at DESC);
				CREATE INDEX IF NOT EXISTS idx_enrollment_late_invites_usable
					ON enrollment.late_invites (tenant_id, phase_id, token_hash, expires_at)
					WHERE used_at IS NULL;
				CREATE INDEX IF NOT EXISTS idx_enrollment_late_invites_email
					ON enrollment.late_invites (tenant_id, phase_id, lower(guardian_email));

				ALTER TABLE enrollment.late_invites ENABLE ROW LEVEL SECURITY;
				ALTER TABLE enrollment.late_invites FORCE ROW LEVEL SECURITY;

				DO $$
				BEGIN
					IF NOT EXISTS (
						SELECT 1 FROM pg_policies
						WHERE schemaname = 'enrollment'
							AND tablename = 'late_invites'
							AND policyname = 'tenant_isolation_enrollment_late_invites'
					) THEN
						CREATE POLICY tenant_isolation_enrollment_late_invites
							ON enrollment.late_invites
							FOR ALL
							USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
							WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);
					END IF;
				END $$;

				GRANT SELECT, INSERT, UPDATE ON enrollment.late_invites TO phoenix_tenant;
				GRANT USAGE ON SEQUENCE enrollment.late_invites_id_seq TO phoenix_tenant;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed adding enrollment late invites: %w", err)
			}
			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 1.15.153: Dropping enrollment late invites and request source fields...")
			if _, err := db.NewRaw(`
				DROP TABLE IF EXISTS enrollment.late_invites;
				ALTER TABLE enrollment.requests
					DROP CONSTRAINT IF EXISTS chk_enrollment_requests_submission_source,
					DROP COLUMN IF EXISTS source_metadata,
					DROP COLUMN IF EXISTS submission_source;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed rolling back enrollment late invites: %w", err)
			}
			return nil
		},
	)
}
