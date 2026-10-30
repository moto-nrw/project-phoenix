package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	groupWeekProtocolsVersion     = "1.15.718"
	groupWeekProtocolsDescription = "Wochenprotokoll: je OGS-Gruppe (oder Schule) und Woche ein Protokoll aus den Tagesdaten, ergänzt und abgeschlossen von der Gruppenleitung (#4769)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     groupWeekProtocolsVersion,
		Description: groupWeekProtocolsDescription,
		DependsOn: []string{
			studentAccidentReportsVersion, // the last migration before this one
			surveysVersion,                // the retention kinds this extends
			visitRevisionEventsVersion,    // the school erasure this extends
		},
	})
	Migrations.MustRegister(groupWeekProtocolsUp, groupWeekProtocolsDown)
}

// groupWeekProtocolsUpSQL stores the weekly protocol of a group (#4769).
//
//   - schedule.group_week_protocols: one protocol per OGS group and week
//     (Monday to Sunday), or one for the whole school when it works without
//     fixed groups (scope 'school'). It is 'collecting' while the evenings of
//     the week are recorded, becomes a 'draft' on Friday evening, and is
//     'closed' by the group leadership or, after the following week, by the
//     worker. group_id carries no foreign key and group_name is kept: a
//     deleted or renamed group must not change a past protocol.
//   - schedule.group_week_protocol_days: what one day looked like, recorded
//     on its evening because the student record keeps only the current
//     group, so a day cannot be rebuilt later after a group change. facts is
//     the recorded day (counts, missing children with their reason, blocks
//     with their status, the day's staff notices); a day without facts was
//     not recorded. note is the leadership's free text for the day.
//
// The PDF is rendered from the closed protocol, which no longer changes;
// both tables go after gdpr.attendance_retention_days, counted from the
// week's Sunday.
const groupWeekProtocolsUpSQL = `
CREATE TABLE IF NOT EXISTS schedule.group_week_protocols (
	id BIGSERIAL PRIMARY KEY,
	tenant_id BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
	scope VARCHAR(10) NOT NULL,
	group_id BIGINT,
	group_name TEXT NOT NULL,
	week_start DATE NOT NULL,
	status VARCHAR(12) NOT NULL DEFAULT 'collecting',
	week_note TEXT NOT NULL DEFAULT '',
	drafted_at TIMESTAMPTZ,
	closed_at TIMESTAMPTZ,
	closed_by_account_id BIGINT,
	closed_by_name TEXT,
	closed_automatically BOOLEAN NOT NULL DEFAULT FALSE,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	CONSTRAINT chk_group_week_protocols_scope CHECK (
		(scope = 'group' AND group_id IS NOT NULL) OR (scope = 'school' AND group_id IS NULL)),
	CONSTRAINT chk_group_week_protocols_monday CHECK (EXTRACT(ISODOW FROM week_start) = 1),
	CONSTRAINT chk_group_week_protocols_status CHECK (status IN ('collecting', 'draft', 'closed')),
	CONSTRAINT chk_group_week_protocols_lifecycle CHECK (
		(status = 'collecting' AND drafted_at IS NULL AND closed_at IS NULL)
		OR (status = 'draft' AND drafted_at IS NOT NULL AND closed_at IS NULL)
		OR (status = 'closed' AND closed_at IS NOT NULL
			AND (closed_automatically OR closed_by_account_id IS NOT NULL))),
	CONSTRAINT chk_group_week_protocols_week_note CHECK (char_length(week_note) <= 4000)
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_group_week_protocols_group_week
	ON schedule.group_week_protocols (tenant_id, group_id, week_start) WHERE scope = 'group';
CREATE UNIQUE INDEX IF NOT EXISTS uq_group_week_protocols_school_week
	ON schedule.group_week_protocols (tenant_id, week_start) WHERE scope = 'school';
CREATE INDEX IF NOT EXISTS idx_group_week_protocols_open
	ON schedule.group_week_protocols (tenant_id, status, week_start) WHERE status <> 'closed';
COMMENT ON TABLE schedule.group_week_protocols IS
	'Weekly protocol of an OGS group or of the whole school (#4769); deleted after gdpr.attendance_retention_days from the week''s Sunday.';

CREATE TABLE IF NOT EXISTS schedule.group_week_protocol_days (
	id BIGSERIAL PRIMARY KEY,
	tenant_id BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
	protocol_id BIGINT NOT NULL REFERENCES schedule.group_week_protocols(id) ON DELETE CASCADE,
	day DATE NOT NULL,
	captured_at TIMESTAMPTZ,
	facts JSONB,
	note TEXT NOT NULL DEFAULT '',
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	CONSTRAINT uq_group_week_protocol_days_day UNIQUE (protocol_id, day),
	CONSTRAINT chk_group_week_protocol_days_capture CHECK ((captured_at IS NULL) = (facts IS NULL)),
	CONSTRAINT chk_group_week_protocol_days_note CHECK (char_length(note) <= 2000)
);
CREATE INDEX IF NOT EXISTS idx_group_week_protocol_days_tenant
	ON schedule.group_week_protocol_days (tenant_id);

DROP TRIGGER IF EXISTS update_group_week_protocols_updated_at ON schedule.group_week_protocols;
CREATE TRIGGER update_group_week_protocols_updated_at
	BEFORE UPDATE ON schedule.group_week_protocols
	FOR EACH ROW EXECUTE FUNCTION update_modified_column();
DROP TRIGGER IF EXISTS update_group_week_protocol_days_updated_at ON schedule.group_week_protocol_days;
CREATE TRIGGER update_group_week_protocol_days_updated_at
	BEFORE UPDATE ON schedule.group_week_protocol_days
	FOR EACH ROW EXECUTE FUNCTION update_modified_column();

REVOKE ALL ON schedule.group_week_protocols, schedule.group_week_protocol_days FROM PUBLIC;
GRANT SELECT, INSERT, UPDATE, DELETE ON schedule.group_week_protocols, schedule.group_week_protocol_days TO phoenix_tenant;
GRANT USAGE ON SEQUENCE schedule.group_week_protocols_id_seq, schedule.group_week_protocol_days_id_seq TO phoenix_tenant;
`

// weekProtocolDataKindsUpSQL extends the subject check of
// audit.data_deletions by the weekly protocols' retention kind; the down
// migration restores the check of 1.15.568. The list is spelled out: the
// architecture check classifies only static migration SQL.
const weekProtocolDataKindsUpSQL = `
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
						'week_protocol_retention'
					)
				)
			);
`

// weekProtocolDataKindsDownSQL restores the subject check of 1.15.568.
const weekProtocolDataKindsDownSQL = `
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
						'staff_survey_retention'
					)
				)
			);
`

func groupWeekProtocolsUp(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, groupWeekProtocolsUpSQL); err != nil {
			return fmt.Errorf("create the weekly protocol tables: %w", err)
		}
		if err := provisionTenantRLS(ctx, tx, "schedule.group_week_protocols", "schedule.group_week_protocol_days"); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, weekProtocolDataKindsUpSQL); err != nil {
			return fmt.Errorf("add the weekly protocol retention kind: %w", err)
		}
		for _, step := range []func(context.Context, bun.Tx) error{
			installWeekProtocolErasurePlan, installWeekProtocolErasurePreflight, installWeekProtocolErasureResidual,
			installWeekProtocolErasureDelete, installWeekProtocolErasurePreview,
		} {
			if err := step(ctx, tx); err != nil {
				return fmt.Errorf("install the erasure with the weekly protocols: %w", err)
			}
		}
		return nil
	})
}

// groupWeekProtocolsDown reinstalls the 1.15.705 erasure before it drops the
// tables, so the installed plan never names a missing table.
func groupWeekProtocolsDown(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		for _, step := range []func(context.Context, bun.Tx) error{
			installVisitRevisionErasurePlan, installVisitRevisionErasurePreflight, installVisitRevisionErasureResidual,
			installVisitRevisionErasureDelete, installVisitRevisionErasurePreview,
		} {
			if err := step(ctx, tx); err != nil {
				return fmt.Errorf("restore the erasure without the weekly protocols: %w", err)
			}
		}
		if _, err := tx.ExecContext(ctx, `
			DELETE FROM audit.data_deletions WHERE deletion_type = 'week_protocol_retention';
			DROP TABLE IF EXISTS schedule.group_week_protocol_days;
			DROP TABLE IF EXISTS schedule.group_week_protocols;`); err != nil {
			return fmt.Errorf("drop the weekly protocol tables: %w", err)
		}
		if _, err := tx.ExecContext(ctx, weekProtocolDataKindsDownSQL); err != nil {
			return fmt.Errorf("restore the retention kinds of 1.15.568: %w", err)
		}
		return nil
	})
}
