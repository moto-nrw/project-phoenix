package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	excusedAbsenceRequestsVersion     = "1.15.177"
	excusedAbsenceRequestsDescription = "Create active.excused_absence_requests - optional office approval gate for parent-submitted excused absences (#1845)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     excusedAbsenceRequestsVersion,
		Description: excusedAbsenceRequestsDescription,
		DependsOn: []string{
			UsersStudentsVersion,          // student_id FK target
			"1.0.1",                       // auth.accounts (submitted_by + reviewed_by FKs)
			repairSchulhofIsSystemVersion, // latest development migration before this branch
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return excusedAbsenceRequestsUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return excusedAbsenceRequestsDown(ctx, db)
		},
	)
}

// excusedAbsenceRequestsUp creates the store for parent-submitted EXCUSED
// absence requests that a school has chosen to gate behind office approval
// (operations.parent_excused_requires_approval, #1845).
//
// A "Krankmeldung" is always written straight to active.student_status_days.
// An "entschuldigte Abmeldung" is written straight there too UNLESS the school
// turned the approval gate on — then it becomes a PENDING row here that the
// Änderungsanfragen review queue must confirm. On approval the request is
// applied as excused active.student_status_days rows exactly like the direct
// path; a rejected/withdrawn request never touches the status days, so until a
// decision is made the child stays "expected" (no absence recorded).
//
// One row per request carries the whole set of dates plus the (now mandatory)
// note. Deliberately NO one-pending-per-student unique index (unlike
// schedule.care_schedule_change_requests): a parent may legitimately have
// several open excused requests for different day ranges at once.
func excusedAbsenceRequestsUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.177: Creating active.excused_absence_requests...")

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
		CREATE TABLE IF NOT EXISTS active.excused_absence_requests (
			id              BIGSERIAL PRIMARY KEY,
			tenant_id       BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			student_id      BIGINT NOT NULL REFERENCES users.students(id) ON DELETE CASCADE,
			submitted_by    BIGINT NOT NULL REFERENCES auth.accounts(id) ON DELETE CASCADE,
			dates           JSONB NOT NULL,
			note            TEXT NOT NULL,
			status          TEXT NOT NULL DEFAULT 'pending'
			                CHECK (status IN ('pending', 'approved', 'rejected', 'withdrawn')),
			decision_reason TEXT,
			reviewed_by     BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
			reviewed_at     TIMESTAMPTZ,
			applied_at      TIMESTAMPTZ,
			created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);

		CREATE INDEX IF NOT EXISTS idx_excused_absence_requests_tenant_status
			ON active.excused_absence_requests (tenant_id, status);
		CREATE INDEX IF NOT EXISTS idx_excused_absence_requests_student
			ON active.excused_absence_requests (student_id);
		CREATE INDEX IF NOT EXISTS idx_excused_absence_requests_submitted_by
			ON active.excused_absence_requests (submitted_by);

		DROP TRIGGER IF EXISTS update_excused_absence_requests_updated_at ON active.excused_absence_requests;
		CREATE TRIGGER update_excused_absence_requests_updated_at
		BEFORE UPDATE ON active.excused_absence_requests
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();

		ALTER TABLE active.excused_absence_requests ENABLE ROW LEVEL SECURITY;
		ALTER TABLE active.excused_absence_requests FORCE ROW LEVEL SECURITY;

		DROP POLICY IF EXISTS tenant_isolation_active_excused_absence_requests ON active.excused_absence_requests;
		CREATE POLICY tenant_isolation_active_excused_absence_requests ON active.excused_absence_requests
			FOR ALL
			USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
			WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);

		GRANT SELECT, INSERT, UPDATE, DELETE ON active.excused_absence_requests TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE active.excused_absence_requests_id_seq TO phoenix_tenant;
	`)
	if err != nil {
		return fmt.Errorf("error creating active.excused_absence_requests: %w", err)
	}

	return tx.Commit()
}

func excusedAbsenceRequestsDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.177: Dropping active.excused_absence_requests...")

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
		DROP TRIGGER IF EXISTS update_excused_absence_requests_updated_at ON active.excused_absence_requests;
		DROP TABLE IF EXISTS active.excused_absence_requests CASCADE;
	`)
	if err != nil {
		return fmt.Errorf("error dropping active.excused_absence_requests: %w", err)
	}
	return tx.Commit()
}
