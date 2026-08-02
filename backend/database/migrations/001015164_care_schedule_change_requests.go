package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	careScheduleChangeRequestsVersion     = "1.15.164"
	careScheduleChangeRequestsDescription = "Create schedule.care_schedule_change_requests - parent care-schedule change requests reviewed on the Änderungsanfragen page (#1803)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     careScheduleChangeRequestsVersion,
		Description: careScheduleChangeRequestsDescription,
		DependsOn: []string{
			UsersStudentsVersion,      // student_id FK target
			"1.0.1",                   // auth.accounts (submitted_by + reviewed_by FKs)
			workTimeStartTimesVersion, // latest development migration before this branch's additions
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return careScheduleChangeRequestsUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return careScheduleChangeRequestsDown(ctx, db)
		},
	)
}

// careScheduleChangeRequestsUp creates the dedicated store for parent
// care-schedule change requests (#1803). These previously lived as
// kind='request' rows inside users.parent_messages, coupling the request
// lifecycle to the chat. One row per request carrying the full weekly-plan
// payload — the request is an atomic weekly plan (approve/reject as a whole),
// so per-field rows like users.student_data_change_requests would only force
// reassembly at apply time.
//
// payload is the canonical {"weekdays":[{"weekday","mode","arrival","pickup"}]}
// shape (empty aspect = unchanged; apply merges onto the live plan).
//
// The partial unique index enforces at most ONE open request per student —
// a parent revises by withdrawing and re-submitting, staff never has to
// reconcile stacked requests.
func careScheduleChangeRequestsUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.164: Creating schedule.care_schedule_change_requests...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schedule.care_schedule_change_requests (
			id              BIGSERIAL PRIMARY KEY,
			tenant_id       BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			student_id      BIGINT NOT NULL REFERENCES users.students(id) ON DELETE CASCADE,
			submitted_by    BIGINT NOT NULL REFERENCES auth.accounts(id) ON DELETE CASCADE,
			payload         JSONB NOT NULL,
			status          TEXT NOT NULL DEFAULT 'pending'
			                CHECK (status IN ('pending', 'approved', 'rejected', 'withdrawn')),
			decision_reason TEXT,
			reviewed_by     BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
			reviewed_at     TIMESTAMPTZ,
			applied_at      TIMESTAMPTZ,
			created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);

		CREATE INDEX IF NOT EXISTS idx_care_schedule_change_requests_tenant_status
			ON schedule.care_schedule_change_requests (tenant_id, status);
		CREATE INDEX IF NOT EXISTS idx_care_schedule_change_requests_student
			ON schedule.care_schedule_change_requests (student_id);
		CREATE INDEX IF NOT EXISTS idx_care_schedule_change_requests_submitted_by
			ON schedule.care_schedule_change_requests (submitted_by);
		CREATE UNIQUE INDEX IF NOT EXISTS uniq_care_schedule_change_requests_pending
			ON schedule.care_schedule_change_requests (tenant_id, student_id)
			WHERE status = 'pending';

		DROP TRIGGER IF EXISTS update_care_schedule_change_requests_updated_at ON schedule.care_schedule_change_requests;
		CREATE TRIGGER update_care_schedule_change_requests_updated_at
		BEFORE UPDATE ON schedule.care_schedule_change_requests
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();

		ALTER TABLE schedule.care_schedule_change_requests ENABLE ROW LEVEL SECURITY;
		ALTER TABLE schedule.care_schedule_change_requests FORCE ROW LEVEL SECURITY;

		DROP POLICY IF EXISTS tenant_isolation_schedule_care_schedule_change_requests ON schedule.care_schedule_change_requests;
		CREATE POLICY tenant_isolation_schedule_care_schedule_change_requests ON schedule.care_schedule_change_requests
			FOR ALL
			USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
			WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);

		GRANT SELECT, INSERT, UPDATE, DELETE ON schedule.care_schedule_change_requests TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE schedule.care_schedule_change_requests_id_seq TO phoenix_tenant;
	`)
	if err != nil {
		return fmt.Errorf("error creating schedule.care_schedule_change_requests: %w", err)
	}

	return tx.Commit()
}

func careScheduleChangeRequestsDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.164: Dropping schedule.care_schedule_change_requests...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	_, err = tx.ExecContext(ctx, `
		DROP TRIGGER IF EXISTS update_care_schedule_change_requests_updated_at ON schedule.care_schedule_change_requests;
		DROP TABLE IF EXISTS schedule.care_schedule_change_requests CASCADE;
	`)
	if err != nil {
		return fmt.Errorf("error dropping schedule.care_schedule_change_requests: %w", err)
	}
	return tx.Commit()
}
