package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	repairStudentDataChangeRequestsVersion     = "1.15.154"
	repairStudentDataChangeRequestsDescription = "Idempotently (re)create users.student_data_change_requests. Repairs environments where the original 001015151_create_student_data_change_requests never ran because the bun migration name 001015151 was already burned by an earlier late-invites deploy."
)

// Why this migration exists
//
// bun tracks applied migrations by the numeric filename prefix (extractMigrationName
// in uptrace/bun's migrate package returns the leading digits as the migration name).
// Two migrations were authored with the prefix 001015151:
// 001015151_late_invites_and_request_sources and
// 001015151_create_student_data_change_requests. Staging deployed the late-invites
// migration first (group #85) and marked bun name "001015151" applied; bun then
// permanently skips the student-data migration in that environment, so
// users.student_data_change_requests was never created there and the "Änderungsanfragen"
// review screen fails with 42P01 (relation does not exist).
//
// The late-invites migration was renumbered to 001015153 to clear the registry version
// collision, but renumbering does not un-burn the 001015151 bun name on environments
// that already applied it — the new occupant of 001015151 (the student-data table)
// stays skipped. This is the exact failure mode that 001015150 repairs for the
// 001015148 prefix.
//
// The student-data migration keeps 001015151 — on a fresh DB it still creates the
// table. This migration carries a brand-new bun name (001015154) that no environment
// has applied, so it runs everywhere and idempotently brings the table up:
// present-and-correct on fresh DBs (no-op via IF NOT EXISTS / guarded policy), created
// on the damaged environment. The DDL mirrors 001015151_create_student_data_change_requests
// exactly. As on a fresh DB, the table is created by the postgres superuser, so the
// schema-wide ALTER DEFAULT PRIVILEGES from 1.14.1 auto-grants phoenix_tenant the table
// and sequence privileges — no explicit GRANT is needed (and none appears in the
// original migration).
func init() {
	MigrationRegistry.Register(&Migration{
		Version:     repairStudentDataChangeRequestsVersion,
		Description: repairStudentDataChangeRequestsDescription,
		DependsOn: []string{
			createStudentDataChangeRequestsVersion, // 1.15.151 — the original definition this repairs
			lateInvitesAndRequestSourcesVersion,    // 1.15.153 — order after the renumbered migration that burned 001015151
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 1.15.154: Repairing users.student_data_change_requests table (idempotent)...")

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
				return fmt.Errorf("failed repairing users.student_data_change_requests: %w", err)
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
				return fmt.Errorf("failed repairing users.student_data_change_requests indexes: %w", err)
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
				return fmt.Errorf("failed repairing RLS on users.student_data_change_requests: %w", err)
			}

			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			// Intentional no-op. This is a forward repair, not a schema owner: the
			// users.student_data_change_requests table is conceptually owned by
			// 001015151_create_student_data_change_requests, whose own down migration
			// drops it. Dropping it here too would double-drop on a full reverse
			// migration and would destroy a table this migration may only have found
			// (not created) on a fresh DB.
			fmt.Println("Rolling back migration 1.15.154: no-op (users.student_data_change_requests is owned by 1.15.151)")
			return nil
		},
	)
}
