package migrations

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/uptrace/bun"
)

const classHomeworkVersion = "1.15.510"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     classHomeworkVersion,
		Description: "Hausaufgaben: die Lehrkraft trägt sie für ihre Klasse ein, die OGS meldet je Kind zurück (#4796)",
		DependsOn: []string{
			"1.15.499",               // the last migration before this one
			morningAttendanceVersion, // previous subject check of audit.data_deletions
			lehrkraftRoleVersion,     // the role the write permission goes to
			schoolPortalRolesVersion, // auth.roles.school_portal, the school roles that get it too
			schoolErasureVersion,     // the erasure plan this migration extends
		},
	})
	Migrations.MustRegister(classHomeworkUp, classHomeworkDown)
}

// homeworkRetentionKind is the data kind of the deletion log this migration
// adds.
const homeworkRetentionKind = "homework_retention"

// classHomeworkUpSQL creates the two homework tables (#4796):
//
//   - education.class_homework: what a class has to do on one day, entered
//     by its Lehrkraft. The subject is optional, the text is not. Several
//     entries per class and day are allowed (one per subject). The class is
//     matched via LOWER(BTRIM(...)) like education.class_arrival_exceptions.
//     created_by_staff_id is the author; only the author changes or deletes
//     an entry, and a removed staff member leaves it without one.
//   - education.homework_feedback: the OGS feedback of one child on one day,
//     "done", "not_done" or "difficult", with an optional short note. One row
//     per child and day; a second feedback replaces the first. It goes with
//     its child through ON DELETE CASCADE.
//
// Parents read neither table. The nightly retention deletes both after the
// school's window gdpr.homework_retention_days and logs each run as one row
// of the new data kind. The school erasure plan of 1.15.481 learns both
// tables: the reinstalled plan, preflight, delete, preview and residual
// functions lock, check and remove their rows before the children and staff
// memberships they name.
const classHomeworkUpSQL = `
		CREATE TABLE IF NOT EXISTS education.class_homework (
			id                  BIGSERIAL PRIMARY KEY,
			tenant_id           BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			school_class        TEXT NOT NULL,
			date                DATE NOT NULL,
			subject             TEXT,
			description         TEXT NOT NULL,
			created_by_staff_id BIGINT REFERENCES users.staff_school_memberships(id) ON DELETE SET NULL,
			created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT chk_class_homework_school_class CHECK (BTRIM(school_class) <> ''),
			CONSTRAINT chk_class_homework_subject CHECK (subject IS NULL OR (BTRIM(subject) <> '' AND char_length(subject) <= 60)),
			CONSTRAINT chk_class_homework_description CHECK (BTRIM(description) <> '' AND char_length(description) <= 2000)
		);

		COMMENT ON TABLE education.class_homework IS
			'Homework a Lehrkraft enters for one class and day; the OGS sees it in Lernzeit blocks (#4796)';

		CREATE INDEX IF NOT EXISTS idx_class_homework_class_date
			ON education.class_homework (tenant_id, LOWER(BTRIM(school_class)), date);
		CREATE INDEX IF NOT EXISTS idx_class_homework_tenant_date
			ON education.class_homework (tenant_id, date);
		CREATE INDEX IF NOT EXISTS idx_class_homework_created_by
			ON education.class_homework (created_by_staff_id) WHERE created_by_staff_id IS NOT NULL;

		DROP TRIGGER IF EXISTS update_class_homework_updated_at ON education.class_homework;
		CREATE TRIGGER update_class_homework_updated_at
		BEFORE UPDATE ON education.class_homework
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();

		CREATE TABLE IF NOT EXISTS education.homework_feedback (
			id         BIGSERIAL PRIMARY KEY,
			tenant_id  BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			student_id BIGINT NOT NULL,
			date       DATE NOT NULL,
			status     VARCHAR(16) NOT NULL,
			note       TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT fk_homework_feedback_student FOREIGN KEY (tenant_id, student_id)
				REFERENCES users.student_profiles(tenant_id, id) ON DELETE CASCADE,
			CONSTRAINT chk_homework_feedback_status CHECK (status IN ('done', 'not_done', 'difficult')),
			CONSTRAINT chk_homework_feedback_note CHECK (note IS NULL OR (BTRIM(note) <> '' AND char_length(note) <= 200))
		);

		COMMENT ON TABLE education.homework_feedback IS
			'OGS feedback per child and day on the homework of its class: done, not_done or difficult (#4796)';

		CREATE UNIQUE INDEX IF NOT EXISTS uniq_homework_feedback_student_date
			ON education.homework_feedback (tenant_id, student_id, date);
		CREATE INDEX IF NOT EXISTS idx_homework_feedback_tenant_date
			ON education.homework_feedback (tenant_id, date);

		DROP TRIGGER IF EXISTS update_homework_feedback_updated_at ON education.homework_feedback;
		CREATE TRIGGER update_homework_feedback_updated_at
		BEFORE UPDATE ON education.homework_feedback
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();

		GRANT SELECT, INSERT, UPDATE, DELETE ON education.class_homework TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE education.class_homework_id_seq TO phoenix_tenant;
		GRANT SELECT, INSERT, UPDATE, DELETE ON education.homework_feedback TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE education.homework_feedback_id_seq TO phoenix_tenant;

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
						'enrollment_upload_retention'
					)
				)
			);
`

// classHomeworkDownSQL restores the subject check of 1.15.492 and drops the
// tables.
const classHomeworkDownSQL = `
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
						'setting_audit_retention',
						'enrollment_upload_retention'
					)
				)
			);

		DROP TABLE IF EXISTS education.homework_feedback;
		DROP TABLE IF EXISTS education.class_homework;
`

// classHomeworkGrantSQL grants class_day:homework_write to the system role
// lehrkraft and to every school role made from the template „Wie Lehrkraft“
// (auth.roles.school_portal, #4574). A tenant role that merely shares the
// name lehrkraft gets nothing: grantPermissionToRoles matches by name only.
const classHomeworkGrantSQL = `
		INSERT INTO auth.role_permissions (role_id, permission_id)
		SELECT r.id, p.id
		FROM auth.roles r
		CROSS JOIN auth.permissions p
		WHERE p.name = ?
		  AND ((r.tenant_id IS NULL AND LOWER(BTRIM(r.name)) = 'lehrkraft') OR r.school_portal)
		ON CONFLICT (role_id, permission_id) DO NOTHING
`

// classHomeworkUp creates the tables with tenant RLS, extends the deletion
// log by the retention kind and grants class_day:homework_write. Like
// class_day:morning_attendance_write it opens a door without widening a
// scope: the route re-checks the caller's education.class_teachers
// assignment, and only the author changes or deletes an entry.
func classHomeworkUp(ctx context.Context, db *bun.DB) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer rollbackClassHomework(ctx, tx)
	if _, err := tx.ExecContext(ctx, classHomeworkUpSQL); err != nil {
		return fmt.Errorf("create homework tables: %w", err)
	}
	if err := installSchoolErasureFunctions(ctx, tx); err != nil {
		return fmt.Errorf("extend school erasure: %w", err)
	}
	if err := installSchoolErasurePreview(ctx, tx); err != nil {
		return fmt.Errorf("extend school erasure preview: %w", err)
	}
	// Tenant isolation commits with the tables, never after their grants.
	if err := provisionTenantRLS(ctx, tx, "education.class_homework", "education.homework_feedback"); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO auth.permissions (name, description, resource, action)
		VALUES (?, ?, 'class_day', 'homework_write')
		ON CONFLICT (name) DO NOTHING
	`, "class_day:homework_write", "Hausaufgaben für eine zugewiesene Klasse eintragen (moto schule)"); err != nil {
		return fmt.Errorf("insert class_day:homework_write: %w", err)
	}
	if _, err := tx.ExecContext(ctx, classHomeworkGrantSQL, "class_day:homework_write"); err != nil {
		return fmt.Errorf("grant class_day:homework_write: %w", err)
	}
	return tx.Commit()
}

// classHomeworkDown refuses while homework retention rows are logged: the
// restored check would reject them.
func classHomeworkDown(ctx context.Context, db *bun.DB) error {
	var logged int
	if err := db.NewRaw(`SELECT COUNT(*) FROM audit.data_deletions
		WHERE deletion_type = ? AND student_id IS NULL AND staff_id IS NULL`, homeworkRetentionKind).Scan(ctx, &logged); err != nil {
		return fmt.Errorf("check homework retention deletion log rows: %w", err)
	}
	if logged > 0 {
		return fmt.Errorf("cannot restore the data_deletions subject check while %d homework retention rows exist", logged)
	}
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer rollbackClassHomework(ctx, tx)
	if _, err := tx.ExecContext(ctx, classHomeworkDownSQL); err != nil {
		return fmt.Errorf("drop homework tables: %w", err)
	}
	if err := restoreSchoolErasureBeforeHomework(ctx, tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return dropPermission(ctx, db, "class_day:homework_write")
}

func rollbackClassHomework(ctx context.Context, tx bun.Tx) {
	if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
		logRollbackFailure(ctx, err)
	}
}
