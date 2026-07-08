package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	createStudentDataChangeRequestsVersion     = "1.15.151"
	createStudentDataChangeRequestsDescription = "Create users.student_data_change_requests - parent Stammdaten direct-edit audit + change-request review"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     createStudentDataChangeRequestsVersion,
		Description: createStudentDataChangeRequestsDescription,
		DependsOn: []string{
			UsersStudentsVersion,                  // student_id FK target
			"1.0.1",                               // auth.accounts (submitted_by + reviewed_by FKs)
			repairEnrollmentChangeRequestsVersion, // latest development migration before this branch's additions
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createStudentDataChangeRequestsUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return createStudentDataChangeRequestsDown(ctx, db)
		},
	)
}

// createStudentDataChangeRequestsUp creates the single table that backs BOTH
// parent Stammdaten tracks:
//
//   - Track A (direct edit): the parent owns the data, the live record is
//     updated immediately, and an `auto_applied` row is inserted here purely as
//     the audit trail (old_value -> new_value, applied_at set).
//   - Track B (change request): high-stakes / OGS-coupled fields (child name,
//     birthday, permanent weekday Gehzeit) land here as `pending`; a staff
//     decision moves them to `approved` (and applies new_value to the live
//     record) or `rejected`.
//
// submitted_by/reviewed_by reference auth.accounts; deleting the submitting
// parent account cascades their request history (GDPR erasure), while a deleted
// reviewer only nulls the audit link.
func createStudentDataChangeRequestsUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.151: Creating users.student_data_change_requests table...")

	if _, err := db.NewRaw(`
		CREATE TABLE IF NOT EXISTS users.student_data_change_requests (
			id              BIGSERIAL PRIMARY KEY,
			tenant_id       BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			student_id      BIGINT NOT NULL REFERENCES users.students(id) ON DELETE CASCADE,
			submitted_by    BIGINT NOT NULL REFERENCES auth.accounts(id) ON DELETE CASCADE,
			target          TEXT NOT NULL
				CHECK (target IN ('person', 'student', 'guardian_profile', 'guardian_phone', 'departure')),
			target_ref_id   BIGINT,
			field_key       TEXT NOT NULL,
			old_value       JSONB,
			new_value       JSONB NOT NULL,
			status          TEXT NOT NULL DEFAULT 'pending'
				CHECK (status IN ('auto_applied', 'pending', 'approved', 'rejected')),
			review_reason   TEXT,
			reviewed_by     BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
			reviewed_at     TIMESTAMPTZ,
			applied_at      TIMESTAMPTZ,
			created_at      TIMESTAMPTZ NOT NULL DEFAULT current_timestamp,
			updated_at      TIMESTAMPTZ NOT NULL DEFAULT current_timestamp
		);
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed creating users.student_data_change_requests: %w", err)
	}

	if _, err := db.NewRaw(`
		CREATE INDEX IF NOT EXISTS idx_student_data_change_requests_tenant_status
		ON users.student_data_change_requests (tenant_id, status);

		CREATE INDEX IF NOT EXISTS idx_student_data_change_requests_student
		ON users.student_data_change_requests (student_id);

		CREATE INDEX IF NOT EXISTS idx_student_data_change_requests_submitted_by
		ON users.student_data_change_requests (submitted_by);

		CREATE UNIQUE INDEX IF NOT EXISTS uniq_student_data_change_requests_pending_field
		ON users.student_data_change_requests (tenant_id, student_id, target, field_key)
		WHERE status = 'pending';
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed creating users.student_data_change_requests indexes: %w", err)
	}

	if _, err := db.NewRaw(`
		ALTER TABLE users.student_data_change_requests ENABLE ROW LEVEL SECURITY;
		ALTER TABLE users.student_data_change_requests FORCE ROW LEVEL SECURITY;

		DO $$
		BEGIN
			IF NOT EXISTS (
				SELECT 1 FROM pg_policies
				WHERE schemaname = 'users'
					AND tablename = 'student_data_change_requests'
					AND policyname = 'tenant_isolation_student_data_change_requests'
			) THEN
				CREATE POLICY tenant_isolation_student_data_change_requests
					ON users.student_data_change_requests
					FOR ALL
					USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
					WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);
			END IF;
		END $$;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed enabling RLS on users.student_data_change_requests: %w", err)
	}

	return nil
}

func createStudentDataChangeRequestsDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.151: Dropping users.student_data_change_requests...")
	if _, err := db.NewRaw(`DROP TABLE IF EXISTS users.student_data_change_requests CASCADE;`).Exec(ctx); err != nil {
		return fmt.Errorf("failed dropping users.student_data_change_requests: %w", err)
	}
	return nil
}
