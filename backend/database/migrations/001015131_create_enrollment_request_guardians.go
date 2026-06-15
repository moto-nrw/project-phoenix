package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	createEnrollmentRequestGuardiansVersion     = "1.15.131"
	createEnrollmentRequestGuardiansDescription = "Create enrollment.request_guardians table - additional guardians (Erziehungsberechtigte) submitted alongside the primary guardian, materialized on approval"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     createEnrollmentRequestGuardiansVersion,
		Description: createEnrollmentRequestGuardiansDescription,
		DependsOn: []string{
			createEnrollmentRequestsVersion, // request_id FK target
			UsersGuardianProfilesVersion,    // guardian_profile_id FK target
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createEnrollmentRequestGuardiansUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return createEnrollmentRequestGuardiansDown(ctx, db)
		},
	)
}

func createEnrollmentRequestGuardiansUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.131: Creating enrollment.request_guardians table...")

	// One row per ADDITIONAL guardian on a submission. The primary
	// guardian stays flattened on enrollment.requests (status link,
	// dedup, NOT NULL email all hang off it); this table is purely
	// additive for co-guardians.
	//
	// email/phone are NULLABLE on purpose: a co-guardian (e.g. a
	// grandparent) may be a contact-only record with no email. Only the
	// primary guardian's email is required.
	//
	// guardian_profile_id is stamped on the FIRST child approval so that
	// all subsequently-approved children of the same request reuse the
	// same users.guardian_profiles row instead of creating duplicates
	// (children are approved one at a time, and an email-less co-guardian
	// cannot be deduped by email). ON DELETE SET NULL: deleting the
	// profile later must not delete the submission record.
	if _, err := db.NewRaw(`
		CREATE TABLE IF NOT EXISTS enrollment.request_guardians (
			id                  BIGSERIAL PRIMARY KEY,
			tenant_id           BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			request_id          BIGINT NOT NULL REFERENCES enrollment.requests(id) ON DELETE CASCADE,
			first_name          TEXT NOT NULL,
			last_name           TEXT NOT NULL,
			email               TEXT,
			phone               TEXT,
			guardian_profile_id BIGINT REFERENCES users.guardian_profiles(id) ON DELETE SET NULL,
			sort_order          INT NOT NULL DEFAULT 0,
			created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed creating enrollment.request_guardians: %w", err)
	}

	if _, err := db.NewRaw(`
		CREATE INDEX IF NOT EXISTS idx_enrollment_request_guardians_request
		ON enrollment.request_guardians (request_id, sort_order);

		CREATE INDEX IF NOT EXISTS idx_enrollment_request_guardians_profile
		ON enrollment.request_guardians (guardian_profile_id)
		WHERE guardian_profile_id IS NOT NULL;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed creating enrollment.request_guardians indexes: %w", err)
	}

	if _, err := db.NewRaw(`
		ALTER TABLE enrollment.request_guardians ENABLE ROW LEVEL SECURITY;
		ALTER TABLE enrollment.request_guardians FORCE ROW LEVEL SECURITY;
		CREATE POLICY tenant_isolation_enrollment_request_guardians ON enrollment.request_guardians
			FOR ALL
			USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
			WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed enabling RLS on enrollment.request_guardians: %w", err)
	}

	return nil
}

func createEnrollmentRequestGuardiansDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.131: Dropping enrollment.request_guardians...")
	if _, err := db.NewRaw(`DROP TABLE IF EXISTS enrollment.request_guardians CASCADE;`).Exec(ctx); err != nil {
		return fmt.Errorf("failed dropping enrollment.request_guardians: %w", err)
	}
	return nil
}
