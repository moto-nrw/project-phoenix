package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	parentConferencesVersion     = "1.15.685"
	parentConferencesDescription = "Elternsprechtag: Lehrkraft legt Zeitfenster an, Eltern buchen je Kind (#4801)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     parentConferencesVersion,
		Description: parentConferencesDescription,
		DependsOn: []string{
			schoolAbsenceReportsVersion, // the last migration before this one
			schoolCalendarVersion,       // school_event_type the booked appointment carries
			parentDeclarationsVersion,   // auth.guardian_student_access as the permission store
			surveysVersion,              // the deletion kinds this extends
			holidayGuestErasureVersion,  // the school erasure functions this extends
		},
	})
	Migrations.MustRegister(parentConferencesUp, parentConferencesDown)
}

// parentConferencesUpSQL stores the Elternsprechtag (#4801).
//
//   - calendar.parent_conferences is the day a Lehrkraft offers for one of
//     her classes: date, first and last minute, slot length and breaks. The
//     slots are derived from these values, never stored one by one.
//   - calendar.parent_conference_bookings is one booked slot for one child.
//     The two unique constraints are the double-booking guard: a slot holds
//     one child, and a child holds one slot per day, also when two families
//     book at the same moment.
//   - appointment_id is the calendar appointment the booking shows as in the
//     parents' calendar and in moto schule. It carries the school event type
//     elternsprechtag, which nobody can enter by hand.
//
// Minutes count from midnight in the school's wall clock (Europe/Berlin).
const parentConferencesUpSQL = `
		CREATE TABLE IF NOT EXISTS calendar.parent_conferences (
			id                   BIGSERIAL PRIMARY KEY,
			tenant_id            BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			organizer_staff_id   BIGINT NOT NULL REFERENCES users.staff_school_memberships(id),
			organizer_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
			school_class         TEXT NOT NULL,
			conference_date      DATE NOT NULL,
			start_minute         SMALLINT NOT NULL,
			end_minute           SMALLINT NOT NULL,
			slot_minutes         SMALLINT NOT NULL,
			breaks               JSONB NOT NULL DEFAULT '[]'::jsonb,
			location             TEXT,
			created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT uq_calendar_parent_conferences_tenant_id UNIQUE (tenant_id, id),
			CONSTRAINT chk_calendar_parent_conferences_class CHECK (btrim(school_class) <> ''),
			CONSTRAINT chk_calendar_parent_conferences_window CHECK (
				start_minute >= 0 AND end_minute <= 1440 AND start_minute < end_minute
			),
			CONSTRAINT chk_calendar_parent_conferences_slot CHECK (
				slot_minutes BETWEEN 5 AND 120 AND slot_minutes <= end_minute - start_minute
			),
			CONSTRAINT chk_calendar_parent_conferences_breaks CHECK (jsonb_typeof(breaks) = 'array')
		);
		CREATE INDEX IF NOT EXISTS idx_calendar_parent_conferences_class
			ON calendar.parent_conferences (tenant_id, school_class, conference_date);
		CREATE INDEX IF NOT EXISTS idx_calendar_parent_conferences_date
			ON calendar.parent_conferences (tenant_id, conference_date);

		COMMENT ON TABLE calendar.parent_conferences IS
			'Elternsprechtag einer Lehrkraft für eine ihrer Klassen (#4801); wird mit den Buchungen nach Ende des Schuljahres gelöscht.';

		CREATE TABLE IF NOT EXISTS calendar.parent_conference_bookings (
			id                  BIGSERIAL PRIMARY KEY,
			tenant_id           BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			conference_id       BIGINT NOT NULL,
			student_id          BIGINT NOT NULL,
			guardian_profile_id BIGINT REFERENCES users.guardian_profiles(id) ON DELETE SET NULL,
			slot_start_minute   SMALLINT NOT NULL,
			appointment_id      BIGINT REFERENCES calendar.appointments(id) ON DELETE SET NULL,
			created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT fk_calendar_parent_conference_bookings_conference
				FOREIGN KEY (tenant_id, conference_id)
				REFERENCES calendar.parent_conferences (tenant_id, id)
				ON DELETE CASCADE,
			CONSTRAINT fk_calendar_parent_conference_bookings_student
				FOREIGN KEY (tenant_id, student_id)
				REFERENCES users.student_profiles (tenant_id, id)
				ON DELETE CASCADE,
			CONSTRAINT uq_calendar_parent_conference_bookings_slot
				UNIQUE (tenant_id, conference_id, slot_start_minute),
			CONSTRAINT uq_calendar_parent_conference_bookings_student
				UNIQUE (tenant_id, conference_id, student_id),
			CONSTRAINT chk_calendar_parent_conference_bookings_slot CHECK (
				slot_start_minute >= 0 AND slot_start_minute < 1440
			)
		);
		CREATE INDEX IF NOT EXISTS idx_calendar_parent_conference_bookings_student
			ON calendar.parent_conference_bookings (tenant_id, student_id);
		CREATE INDEX IF NOT EXISTS idx_calendar_parent_conference_bookings_appointment
			ON calendar.parent_conference_bookings (appointment_id)
			WHERE appointment_id IS NOT NULL;

		COMMENT ON TABLE calendar.parent_conference_bookings IS
			'Gebuchtes Zeitfenster am Elternsprechtag für ein Kind (#4801). Ein Fenster hält ein Kind, ein Kind ein Fenster je Sprechtag.';

		DROP TRIGGER IF EXISTS update_calendar_parent_conferences_updated_at ON calendar.parent_conferences;
		CREATE TRIGGER update_calendar_parent_conferences_updated_at
		BEFORE UPDATE ON calendar.parent_conferences
		FOR EACH ROW EXECUTE FUNCTION update_modified_column();

		DROP TRIGGER IF EXISTS update_calendar_parent_conference_bookings_updated_at ON calendar.parent_conference_bookings;
		CREATE TRIGGER update_calendar_parent_conference_bookings_updated_at
		BEFORE UPDATE ON calendar.parent_conference_bookings
		FOR EACH ROW EXECUTE FUNCTION update_modified_column();

		GRANT SELECT, INSERT, UPDATE, DELETE ON calendar.parent_conferences, calendar.parent_conference_bookings
			TO phoenix_tenant, phoenix_admin;
		GRANT USAGE ON SEQUENCE calendar.parent_conferences_id_seq, calendar.parent_conference_bookings_id_seq
			TO phoenix_tenant, phoenix_admin;

		ALTER TABLE calendar.appointments
			DROP CONSTRAINT IF EXISTS chk_calendar_appointments_school_event_type,
			ADD CONSTRAINT chk_calendar_appointments_school_event_type CHECK (
				school_event_type IS NULL OR school_event_type IN (
					'ausflug',
					'paedagogischer_tag',
					'unterrichtsende',
					'sonstiges',
					'elternsprechtag'
				)
			);
`

// parentConferencesGrantSQL lets the full guardians book: the roles that
// carry custody. Seeing the Sprechtag stays covered by parent_portal.access;
// pickup-only people, emergency contacts and social workers do not book.
// New relationships receive the permission from the role presets.
const parentConferencesGrantSQL = `
		UPDATE auth.guardian_student_access AS access
		SET permissions = access.permissions || '{"parent_portal.conference.book": true}'::jsonb
		FROM users.student_guardian_relationships AS relationship
		WHERE relationship.tenant_id = access.tenant_id AND relationship.id = access.relationship_id
			AND relationship.guardian_role IN ('primary_guardian', 'legal_guardian', 'co_guardian')
			AND NOT (access.permissions ? 'parent_portal.conference.book');
`

// parentConferenceDataKindsUpSQL extends the subject check of
// audit.data_deletions by the Elternsprechtag retention kind;
// parentConferenceDataKindsDownSQL restores the check of 1.15.568. The list
// is spelled out: the architecture check classifies only static migration
// SQL.
const parentConferenceDataKindsUpSQL = `
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
						'parent_conference_retention'
					)
				)
			);
`

const parentConferenceDataKindsDownSQL = `
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

const parentConferencesDownSQL = `
		UPDATE auth.guardian_student_access
		SET permissions = permissions - 'parent_portal.conference.book'
		WHERE permissions ? 'parent_portal.conference.book';

		DROP TABLE IF EXISTS calendar.parent_conference_bookings;
		DROP TABLE IF EXISTS calendar.parent_conferences;

		DELETE FROM calendar.appointments WHERE school_event_type = 'elternsprechtag';
		ALTER TABLE calendar.appointments
			DROP CONSTRAINT IF EXISTS chk_calendar_appointments_school_event_type,
			ADD CONSTRAINT chk_calendar_appointments_school_event_type CHECK (
				school_event_type IS NULL OR school_event_type IN (
					'ausflug',
					'paedagogischer_tag',
					'unterrichtsende',
					'sonstiges'
				)
			);
`

func parentConferencesUp(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, parentConferencesUpSQL); err != nil {
			return fmt.Errorf("create the parent conference tables: %w", err)
		}
		if err := provisionTenantRLS(ctx, tx,
			"calendar.parent_conferences",
			"calendar.parent_conference_bookings",
		); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, parentConferencesGrantSQL); err != nil {
			return fmt.Errorf("grant parent_portal.conference.book: %w", err)
		}
		if _, err := tx.ExecContext(ctx, parentConferenceDataKindsUpSQL); err != nil {
			return fmt.Errorf("allow the parent conference retention kind: %w", err)
		}
		// The school erasure deletes the two tables with the school.
		for _, step := range []func(context.Context, bun.Tx) error{
			installParentConferenceErasurePlan, installParentConferenceErasurePreflight, installParentConferenceErasureResidual,
			installParentConferenceErasureDelete, installParentConferenceErasurePreview,
		} {
			if err := step(ctx, tx); err != nil {
				return fmt.Errorf("install the parent conference school erasure: %w", err)
			}
		}
		return nil
	})
}

func parentConferencesDown(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		for _, step := range []func(context.Context, bun.Tx) error{
			installHolidayGuestErasurePlan, installHolidayGuestErasurePreflight, installHolidayGuestErasureResidual,
			installHolidayGuestErasureDelete, installHolidayGuestErasurePreview,
		} {
			if err := step(ctx, tx); err != nil {
				return fmt.Errorf("restore the school erasure of 1.15.681: %w", err)
			}
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM audit.data_deletions WHERE deletion_type = 'parent_conference_retention'`); err != nil {
			return fmt.Errorf("drop the parent conference retention log: %w", err)
		}
		if _, err := tx.ExecContext(ctx, parentConferenceDataKindsDownSQL); err != nil {
			return fmt.Errorf("restore the deletion kinds of 1.15.568: %w", err)
		}
		if _, err := tx.ExecContext(ctx, parentConferencesDownSQL); err != nil {
			return fmt.Errorf("drop the parent conference tables: %w", err)
		}
		return nil
	})
}
