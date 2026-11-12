package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	MigrationRegistry.Register(&Migration{Version: "1.15.895", Description: "Feedback, App-Nutzung und Team-Chat im Löschprotokoll", DependsOn: []string{parentConferencesVersion}})
	Migrations.MustRegister(auxiliaryRetentionAuditUp, auxiliaryRetentionAuditDown)
}

func auxiliaryRetentionAuditUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, auxiliaryRetentionAuditSQL); err != nil {
		return fmt.Errorf("extend retention audit kinds: %w", err)
	}
	return nil
}

func auxiliaryRetentionAuditDown(context.Context, *bun.DB) error {
	// Existing audit entries are append-only. Keep the additive constraint on rollback.
	return nil
}

const auxiliaryRetentionAuditSQL = `
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
						'homework_retention',
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
						'setting_audit_retention',
						'enrollment_upload_retention',
						'parent_survey_retention',
						'staff_survey_retention',
						'week_protocol_retention',
						'parent_conference_retention',
 'feedback_retention', 'pwa_usage_retention', 'staff_message_retention'
					)
				)
			);
`
