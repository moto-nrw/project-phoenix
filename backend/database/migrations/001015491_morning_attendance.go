package migrations

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/uptrace/bun"
)

const morningAttendanceVersion = "1.15.491"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     morningAttendanceVersion,
		Description: "Anwesenheit am Vormittag: die Lehrkraft trägt je Kind da oder fehlt ein (#4790)",
		DependsOn: []string{
			"1.15.490",                    // the last migration before this one; its erasure preview is replaced
			schoolContentRetentionVersion, // previous subject check of audit.data_deletions
			lehrkraftRoleVersion,          // the role the write permission goes to
			classListEntriesVersion,       // the class-list-only children a row may name
			schoolErasureVersion,          // the erasure phases frozen anew with the table
		},
	})
	Migrations.MustRegister(morningAttendanceUp, morningAttendanceDown)
}

// morningAttendanceRetentionKind is the data kind of the deletion log this
// migration adds.
const morningAttendanceRetentionKind = "morning_attendance_retention"

// morningAttendanceUpSQL creates active.morning_attendance (#4790): what the
// Lehrkraft of a class marked for one child on one school morning, "present"
// or "absent". It is a table of its own on purpose: the OGS attendance
// (active.attendance) records care check-ins, and a morning mark must never
// change one.
//
//   - A row names exactly one child: a student with an OGS record (bound to
//     the row's school through the composite key), or a class-list-only
//     entry (#2382) without one. Both go with their child through ON DELETE
//     CASCADE.
//   - One row per child and date; a correction on the same day updates it.
//   - recorded_by_staff_id keeps who marked last; a removed staff member
//     leaves the mark without a name.
//   - The nightly retention deletes rows dated before the school's window
//     gdpr.morning_attendance_retention_days and logs each run as one row of
//     the new data kind.
const morningAttendanceUpSQL = `
		CREATE TABLE IF NOT EXISTS active.morning_attendance (
			id                   BIGSERIAL PRIMARY KEY,
			tenant_id            BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			date                 DATE NOT NULL,
			student_id           BIGINT,
			class_list_entry_id  BIGINT REFERENCES users.class_list_entries(id) ON DELETE CASCADE,
			status               VARCHAR(16) NOT NULL,
			recorded_by_staff_id BIGINT REFERENCES users.staff_school_memberships(id) ON DELETE SET NULL,
			created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT fk_morning_attendance_student FOREIGN KEY (tenant_id, student_id)
				REFERENCES users.student_profiles(tenant_id, id) ON DELETE CASCADE,
			CONSTRAINT chk_morning_attendance_subject CHECK ((student_id IS NULL) <> (class_list_entry_id IS NULL)),
			CONSTRAINT chk_morning_attendance_status CHECK (status IN ('present', 'absent'))
		);

		COMMENT ON TABLE active.morning_attendance IS
			'Morning attendance a Lehrkraft marks per child and school day, separate from the OGS attendance (#4790)';

		CREATE UNIQUE INDEX IF NOT EXISTS uniq_morning_attendance_student_date
			ON active.morning_attendance (tenant_id, student_id, date) WHERE student_id IS NOT NULL;
		CREATE UNIQUE INDEX IF NOT EXISTS uniq_morning_attendance_list_entry_date
			ON active.morning_attendance (tenant_id, class_list_entry_id, date) WHERE class_list_entry_id IS NOT NULL;
		CREATE INDEX IF NOT EXISTS idx_morning_attendance_tenant_date
			ON active.morning_attendance (tenant_id, date);
		CREATE INDEX IF NOT EXISTS idx_morning_attendance_recorded_by
			ON active.morning_attendance (recorded_by_staff_id) WHERE recorded_by_staff_id IS NOT NULL;

		DROP TRIGGER IF EXISTS update_morning_attendance_updated_at ON active.morning_attendance;
		CREATE TRIGGER update_morning_attendance_updated_at
		BEFORE UPDATE ON active.morning_attendance
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();

		GRANT SELECT, INSERT, UPDATE, DELETE ON active.morning_attendance TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE active.morning_attendance_id_seq TO phoenix_tenant;

		ALTER TABLE audit.data_deletions
			DROP CONSTRAINT IF EXISTS chk_data_deletions_subject;
		ALTER TABLE audit.data_deletions
			ADD CONSTRAINT chk_data_deletions_subject CHECK (
				(student_id IS NULL OR staff_id IS NULL)
				AND (
					student_id IS NOT NULL
					OR staff_id IS NOT NULL
					OR deletion_type IN (
						'unregistered_tag_scan_retention',
						'status_day_retention',
						'attendance_retention',
						'morning_attendance_retention',
						'guardian_retention',
						'parent_message_retention',
						'parent_announcement_retention',
						'staff_notice_retention',
						'appointment_retention',
						'appointment_tombstone_retention',
						'grade_transition_retention',
						'removed_file_retention',
						'auth_event_retention',
						'data_access_log_retention',
						'deletion_log_retention',
						'delivery_retention',
						'file_cleanup_retention',
						'staff_cleanup_retention',
						'student_document_cleanup_retention',
						'care_withdrawal_retention',
						'submission_rate_limit_retention',
						'settled_enrollment_retention',
						'staff_sick_note_retention',
						'student_deletion_log_retention',
						'enrollment_deletion_log_retention',
						'enrollment_restoration_log_retention',
						'time_tracking_deletion_log_retention',
						'staff_master_data_change_retention',
						'personnel_number_change_retention',
						'guardian_financial_change_retention',
						'class_list_change_retention',
						'substitution_change_retention',
						'file_event_retention',
						'export_transfer_retention',
						'setting_audit_retention'
					)
				)
			);
`

// morningAttendanceDownSQL restores the subject check of 1.15.465 and drops
// the table.
const morningAttendanceDownSQL = `
		ALTER TABLE audit.data_deletions
			DROP CONSTRAINT IF EXISTS chk_data_deletions_subject;
		ALTER TABLE audit.data_deletions
			ADD CONSTRAINT chk_data_deletions_subject CHECK (
				(student_id IS NULL OR staff_id IS NULL)
				AND (
					student_id IS NOT NULL
					OR staff_id IS NOT NULL
					OR deletion_type IN (
						'unregistered_tag_scan_retention',
						'status_day_retention',
						'attendance_retention',
						'guardian_retention',
						'parent_message_retention',
						'parent_announcement_retention',
						'staff_notice_retention',
						'appointment_retention',
						'appointment_tombstone_retention',
						'grade_transition_retention',
						'removed_file_retention',
						'auth_event_retention',
						'data_access_log_retention',
						'deletion_log_retention',
						'delivery_retention',
						'file_cleanup_retention',
						'staff_cleanup_retention',
						'student_document_cleanup_retention',
						'care_withdrawal_retention',
						'submission_rate_limit_retention',
						'settled_enrollment_retention',
						'staff_sick_note_retention',
						'student_deletion_log_retention',
						'enrollment_deletion_log_retention',
						'enrollment_restoration_log_retention',
						'time_tracking_deletion_log_retention',
						'staff_master_data_change_retention',
						'personnel_number_change_retention',
						'guardian_financial_change_retention',
						'class_list_change_retention',
						'substitution_change_retention',
						'file_event_retention',
						'export_transfer_retention',
						'setting_audit_retention'
					)
				)
			);

		DROP TABLE IF EXISTS active.morning_attendance;
`

// morningAttendanceUp creates the table with tenant RLS, extends the deletion
// log by the retention kind and grants class_day:morning_attendance_write to
// the lehrkraft role. Like class_day:arrival_exception_write it opens a door
// without widening a scope: the route re-checks the class assignment, and the
// school's setting operations.school_portal_morning_attendance decides
// whether the door is unlocked.
func morningAttendanceUp(ctx context.Context, db *bun.DB) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer rollbackMorningAttendance(ctx, tx)
	if _, err := tx.ExecContext(ctx, morningAttendanceUpSQL); err != nil {
		return fmt.Errorf("create active.morning_attendance: %w", err)
	}
	if err := installMorningAttendanceErasure(ctx, tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	if err := provisionTenantRLS(ctx, db, "active.morning_attendance"); err != nil {
		return err
	}
	return grantPermissionToRoles(ctx, db, permissionSpec{
		Name:        "class_day:morning_attendance_write",
		Description: "Anwesenheit am Vormittag für eine zugewiesene Klasse eintragen (moto schule)",
		Resource:    "class_day",
		Action:      "morning_attendance_write",
	}, "lehrkraft")
}

// morningAttendanceDown refuses while morning attendance retention rows are
// logged: the restored check would reject them.
func morningAttendanceDown(ctx context.Context, db *bun.DB) error {
	var logged int
	if err := db.NewRaw(`SELECT COUNT(*) FROM audit.data_deletions
		WHERE deletion_type = ? AND student_id IS NULL AND staff_id IS NULL`, morningAttendanceRetentionKind).Scan(ctx, &logged); err != nil {
		return fmt.Errorf("check morning attendance retention deletion log rows: %w", err)
	}
	if logged > 0 {
		return fmt.Errorf("cannot restore the data_deletions subject check while %d morning attendance retention rows exist", logged)
	}
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer rollbackMorningAttendance(ctx, tx)
	if err := restoreSchoolErasurePhases(ctx, tx); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, morningAttendanceDownSQL); err != nil {
		return fmt.Errorf("drop active.morning_attendance: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return dropPermission(ctx, db, "class_day:morning_attendance_write")
}

func rollbackMorningAttendance(ctx context.Context, tx bun.Tx) {
	if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
		logRollbackFailure(ctx, err)
	}
}
