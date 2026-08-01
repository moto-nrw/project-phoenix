package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	offeringChangeRequestsVersion     = "1.15.246"
	offeringChangeRequestsDescription = "Create enrollment.offering_change_requests for post-enrollment care/AG change requests from the parents portal."
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     offeringChangeRequestsVersion,
		Description: offeringChangeRequestsDescription,
		DependsOn:   []string{notificationPreferencesBackfillVersion},
	})

	Migrations.MustRegister(offeringChangeRequestsUp, offeringChangeRequestsDown)
}

// The table is deliberately narrow and separate from enrollment.change_requests
// (#1665). Those rows are whole-form corrections of a submission whose review
// window is still open; these are a switch of the booked offerings for a child
// that is already enrolled and attending, carrying an effective date instead of
// rewriting the submission. Mixing them would put a form snapshot on every
// mid-year switch and force the submission-window rules onto it.
//
// Shape follows schedule.care_schedule_change_requests (#1803): one canonical
// payload per row, decided atomically, at most one pending row per child.
func offeringChangeRequestsUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.246: Creating enrollment.offering_change_requests...")
	if _, err := db.NewRaw(`
		CREATE TABLE IF NOT EXISTS enrollment.offering_change_requests (
			id BIGSERIAL PRIMARY KEY,
			tenant_id BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			student_id BIGINT NOT NULL REFERENCES users.students(id) ON DELETE CASCADE,
			request_child_id BIGINT NOT NULL REFERENCES enrollment.request_children(id) ON DELETE CASCADE,
			submitted_by BIGINT NOT NULL REFERENCES auth.accounts(id) ON DELETE CASCADE,
			payload JSONB NOT NULL,
			effective_from DATE NOT NULL,
			parent_note TEXT,
			status TEXT NOT NULL DEFAULT 'pending'
				CHECK (status IN ('pending', 'approved', 'rejected', 'withdrawn')),
			decision_reason TEXT,
			reviewed_by BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
			reviewed_at TIMESTAMPTZ,
			applied_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);

		-- One open request per child: a second one would race the first on
		-- apply and leave the family unsure which switch is pending.
		CREATE UNIQUE INDEX IF NOT EXISTS uq_offering_change_requests_pending
			ON enrollment.offering_change_requests (tenant_id, student_id)
			WHERE status = 'pending';

		CREATE INDEX IF NOT EXISTS idx_offering_change_requests_review
			ON enrollment.offering_change_requests (tenant_id, status, effective_from, created_at DESC);
		CREATE INDEX IF NOT EXISTS idx_offering_change_requests_student
			ON enrollment.offering_change_requests (tenant_id, student_id, created_at DESC);

		ALTER TABLE enrollment.offering_change_requests ENABLE ROW LEVEL SECURITY;
		ALTER TABLE enrollment.offering_change_requests FORCE ROW LEVEL SECURITY;

		DO $$
		BEGIN
			IF NOT EXISTS (
				SELECT 1 FROM pg_policies
				WHERE schemaname = 'enrollment'
					AND tablename = 'offering_change_requests'
					AND policyname = 'tenant_isolation_enrollment_offering_change_requests'
			) THEN
				CREATE POLICY tenant_isolation_enrollment_offering_change_requests
					ON enrollment.offering_change_requests
					FOR ALL
					USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
					WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);
			END IF;
		END $$;

		GRANT SELECT, INSERT, UPDATE ON enrollment.offering_change_requests TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE enrollment.offering_change_requests_id_seq TO phoenix_tenant;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed creating enrollment.offering_change_requests: %w", err)
	}
	return nil
}

func offeringChangeRequestsDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.246: Dropping enrollment.offering_change_requests...")
	if _, err := db.NewRaw(`
		DROP TABLE IF EXISTS enrollment.offering_change_requests CASCADE;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed dropping enrollment.offering_change_requests: %w", err)
	}
	return nil
}
