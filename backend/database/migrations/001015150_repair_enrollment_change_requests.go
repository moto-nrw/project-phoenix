package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	repairEnrollmentChangeRequestsVersion     = "1.15.150"
	repairEnrollmentChangeRequestsDescription = "Idempotently (re)create the enrollment change-request tables. Repairs environments where the original 001015148_enrollment_change_requests never ran because the bun migration name 001015148 was already burned by an out-of-band parent-messaging deploy."
)

// Why this migration exists
//
// bun tracks applied migrations by the numeric filename prefix (extractMigrationName
// in uptrace/bun's migrate package returns the leading digits as the migration name).
// Two migrations were authored with the prefix 001015148:
// 001015148_parent_messaging and 001015148_enrollment_change_requests. An out-of-band
// deploy ran parent messaging first and marked bun name "001015148" applied; bun then
// permanently skips the enrollment migration in that environment, so
// enrollment.change_requests / enrollment.change_request_messages were never created
// there.
//
// Parent messaging is renumbered to 001015149 (and made re-run-safe) so it stops
// colliding. The enrollment migration keeps 001015148 — on a fresh DB it still creates
// the tables. This migration carries a brand-new bun name (001015150) that no
// environment has applied, so it runs everywhere and idempotently brings the
// enrollment change-request schema up: present-and-correct on fresh DBs (no-op),
// created on the damaged environment. The DDL mirrors 001015148_enrollment_change_requests
// exactly and is safe on both.
func init() {
	MigrationRegistry.Register(&Migration{
		Version:     repairEnrollmentChangeRequestsVersion,
		Description: repairEnrollmentChangeRequestsDescription,
		DependsOn: []string{
			enrollmentChangeRequestsVersion, // 1.15.148 — the original definition this repairs
			parentMessagingVersion,          // 1.15.149 — order after the renumbered messaging migration
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 1.15.150: Repairing enrollment.change_requests tables (idempotent)...")
			if _, err := db.NewRaw(`
				CREATE TABLE IF NOT EXISTS enrollment.change_requests (
					id BIGSERIAL PRIMARY KEY,
					tenant_id BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
					request_id BIGINT NOT NULL REFERENCES enrollment.requests(id) ON DELETE CASCADE,
					request_child_id BIGINT REFERENCES enrollment.request_children(id) ON DELETE SET NULL,
					status TEXT NOT NULL DEFAULT 'pending_review'
						CHECK (status IN ('pending_review', 'needs_parent_response', 'approved', 'rejected', 'cancelled')),
					parent_note TEXT,
					admin_decision_note TEXT,
					base_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
					proposed_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
					diff_json JSONB NOT NULL DEFAULT '{}'::jsonb,
					created_by_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
					reviewed_by_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
					reviewed_at TIMESTAMPTZ,
					created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
					updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
				);

				CREATE INDEX IF NOT EXISTS idx_enrollment_change_requests_request
					ON enrollment.change_requests (tenant_id, request_id, created_at DESC);
				CREATE INDEX IF NOT EXISTS idx_enrollment_change_requests_status
					ON enrollment.change_requests (tenant_id, status, created_at DESC);
				CREATE INDEX IF NOT EXISTS idx_enrollment_change_requests_child
					ON enrollment.change_requests (tenant_id, request_child_id, created_at DESC)
					WHERE request_child_id IS NOT NULL;

				CREATE TABLE IF NOT EXISTS enrollment.change_request_messages (
					id BIGSERIAL PRIMARY KEY,
					tenant_id BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
					change_request_id BIGINT NOT NULL REFERENCES enrollment.change_requests(id) ON DELETE CASCADE,
					author_type TEXT NOT NULL CHECK (author_type IN ('parent', 'staff', 'system')),
					author_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
					body TEXT NOT NULL,
					internal_only BOOLEAN NOT NULL DEFAULT FALSE,
					created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
					updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
				);

				CREATE INDEX IF NOT EXISTS idx_enrollment_change_request_messages_request
					ON enrollment.change_request_messages (tenant_id, change_request_id, created_at ASC);

				ALTER TABLE enrollment.change_requests ENABLE ROW LEVEL SECURITY;
				ALTER TABLE enrollment.change_requests FORCE ROW LEVEL SECURITY;
				ALTER TABLE enrollment.change_request_messages ENABLE ROW LEVEL SECURITY;
				ALTER TABLE enrollment.change_request_messages FORCE ROW LEVEL SECURITY;

				DO $$
				BEGIN
					IF NOT EXISTS (
						SELECT 1 FROM pg_policies
						WHERE schemaname = 'enrollment'
							AND tablename = 'change_requests'
							AND policyname = 'tenant_isolation_enrollment_change_requests'
					) THEN
						CREATE POLICY tenant_isolation_enrollment_change_requests
							ON enrollment.change_requests
							FOR ALL
							USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
							WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);
					END IF;

					IF NOT EXISTS (
						SELECT 1 FROM pg_policies
						WHERE schemaname = 'enrollment'
							AND tablename = 'change_request_messages'
							AND policyname = 'tenant_isolation_enrollment_change_request_messages'
					) THEN
						CREATE POLICY tenant_isolation_enrollment_change_request_messages
							ON enrollment.change_request_messages
							FOR ALL
							USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
							WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);
					END IF;
				END $$;

				GRANT SELECT, INSERT, UPDATE ON enrollment.change_requests TO phoenix_tenant;
				GRANT SELECT, INSERT ON enrollment.change_request_messages TO phoenix_tenant;
				GRANT USAGE ON SEQUENCE enrollment.change_requests_id_seq TO phoenix_tenant;
				GRANT USAGE ON SEQUENCE enrollment.change_request_messages_id_seq TO phoenix_tenant;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed repairing enrollment change-request tables: %w", err)
			}
			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			// Intentional no-op. This is a forward repair, not a schema owner: the
			// enrollment change-request tables are conceptually owned by
			// 001015148_enrollment_change_requests, whose own down migration drops
			// them. Dropping them here too would double-drop on a full reverse
			// migration and would destroy tables this migration may only have
			// found (not created) on a fresh DB.
			fmt.Println("Rolling back migration 1.15.150: no-op (enrollment change-request tables are owned by 1.15.148)")
			return nil
		},
	)
}
