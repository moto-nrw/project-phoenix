package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const attendanceRetentionVersion = "1.15.460"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     attendanceRetentionVersion,
		Description: "Tagesanwesenheit nach der Löschfrist der Schule löschen (#3957)",
		DependsOn: []string{
			protocolRetentionVersion, // previous subject check of audit.data_deletions
			staffRetentionVersion,    // the last migration before this one
		},
	})
	Migrations.MustRegister(attendanceRetentionUp, attendanceRetentionDown)
}

// attendanceRetentionKind is the data kind of the deletion log this migration
// adds.
const attendanceRetentionKind = "attendance_retention"

// attendanceRetentionUpSQL extends the subject check of 1.15.454 by the
// attendance kind. The tenant role already deletes attendance rows, and RLS
// keeps the nightly run to its school, so the deletion log is the only
// change.
const attendanceRetentionUpSQL = `
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
						'parent_message_retention',
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

// attendanceRetentionDownSQL restores the subject check of 1.15.454.
const attendanceRetentionDownSQL = `
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
						'parent_message_retention',
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

func attendanceRetentionUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, attendanceRetentionUpSQL); err != nil {
		return fmt.Errorf("let the deletion log record attendance retention: %w", err)
	}
	return nil
}

// attendanceRetentionDown refuses while attendance retention rows are
// logged: the restored check would reject them.
func attendanceRetentionDown(ctx context.Context, db *bun.DB) error {
	var logged int
	if err := db.NewRaw(`SELECT COUNT(*) FROM audit.data_deletions
		WHERE deletion_type = ? AND student_id IS NULL AND staff_id IS NULL`, attendanceRetentionKind).Scan(ctx, &logged); err != nil {
		return fmt.Errorf("check attendance retention deletion log rows: %w", err)
	}
	if logged > 0 {
		return fmt.Errorf("cannot restore the data_deletions subject check while %d attendance retention rows exist", logged)
	}
	if _, err := db.ExecContext(ctx, attendanceRetentionDownSQL); err != nil {
		return fmt.Errorf("restore the data_deletions subject check: %w", err)
	}
	return nil
}
