package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	surveysVersion     = "1.15.514"
	surveysDescription = "Umfragen an Eltern oder Mitarbeitende, wahlweise anonym: Teilnahme und Antwort getrennt gespeichert (#4781)"

	surveysManagePermissionName = "surveys:manage"

	parentSurveyRetentionKind = "parent_survey_retention"
	staffSurveyRetentionKind  = "staff_survey_retention"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     surveysVersion,
		Description: surveysDescription,
		DependsOn: []string{
			schoolPortalClassTripsVersion, // previous head
			signaturesVersion,             // its erasure plan entries stay in the refreshed plan
			classHandoverVersion,          // previous head; its erasure plan entries stay in the refreshed plan
			"1.15.501",                    // previous head
			"1.15.499",                    // previous head
			"1.15.497",                    // previous head
			morningAttendanceVersion,      // previous subject check of audit.data_deletions
			parentGroupThreadsVersion,     // the erasure plan refreshed here names its tables
			parentAnnouncementsVersion,    // a parent survey is carried by a system announcement
			schoolErasureVersion,          // the erasure plan this migration extends
		},
	})
	Migrations.MustRegister(surveysUp, surveysDown)
}

// surveysTablesSQL creates the surveys (#4781).
//
// A survey asks either the guardians of a school (audience 'parents', reached
// through a system-authored parent announcement that carries the targets,
// the feed entry and the delivery) or its staff (audience 'staff').
//
// The anonymity of an anonymous survey rests on the shape of the tables, not
// on the screens:
//
//   - survey_participations says only THAT an account took part. It has no
//     serial id and no timestamp, so neither its order nor its time can be
//     matched against an answer. It blocks a second submission and selects
//     the reminder audience.
//   - survey_responses and survey_answers hold the answers. Their ids are
//     random UUIDs, they carry no timestamp, and a response of an anonymous
//     survey has no account (the trigger below refuses one). Nothing links a
//     response to a participation.
//   - Accepted limit: a participation and its response are written in one
//     transaction, so both rows carry the same xmin and sit near each other in
//     the heap. Only database access sees either; the app and its logs never
//     read them, and the tenant export leaves the participations, responses
//     and answers out (see the data lifecycle catalog).
const surveysTablesSQL = `
		ALTER TABLE users.parent_announcements
			DROP CONSTRAINT IF EXISTS chk_parent_announcements_system_kind,
			ADD CONSTRAINT chk_parent_announcements_system_kind
				CHECK (system_kind IS NULL OR system_kind IN ('care_cancellation', 'survey'));

		CREATE TABLE IF NOT EXISTS users.surveys (
			id                BIGSERIAL PRIMARY KEY,
			tenant_id         BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			title             TEXT NOT NULL,
			intro             TEXT NOT NULL DEFAULT '',
			audience          TEXT NOT NULL,
			anonymous         BOOLEAN NOT NULL,
			send_email        BOOLEAN NOT NULL DEFAULT FALSE,
			targets           JSONB NOT NULL DEFAULT '[]'::jsonb,
			closes_at         TIMESTAMPTZ NOT NULL,
			published_at      TIMESTAMPTZ,
			announcement_id   BIGINT REFERENCES users.parent_announcements(id) ON DELETE SET NULL,
			created_by        BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
			created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT chk_surveys_audience CHECK (audience IN ('parents', 'staff')),
			CONSTRAINT chk_surveys_title_not_blank CHECK (length(btrim(title)) > 0),
			CONSTRAINT chk_surveys_targets_array CHECK (jsonb_typeof(targets) = 'array'),
			CONSTRAINT chk_surveys_staff_without_carrier CHECK (audience = 'parents' OR (announcement_id IS NULL AND send_email = FALSE))
		);
		CREATE INDEX IF NOT EXISTS idx_surveys_tenant
			ON users.surveys (tenant_id, audience, closes_at DESC);
		CREATE UNIQUE INDEX IF NOT EXISTS uq_surveys_announcement
			ON users.surveys (announcement_id) WHERE announcement_id IS NOT NULL;

		CREATE TABLE IF NOT EXISTS users.survey_questions (
			id        BIGSERIAL PRIMARY KEY,
			tenant_id BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			survey_id BIGINT NOT NULL REFERENCES users.surveys(id) ON DELETE CASCADE,
			position  INTEGER NOT NULL,
			kind      TEXT NOT NULL,
			prompt    TEXT NOT NULL,
			required  BOOLEAN NOT NULL DEFAULT TRUE,
			CONSTRAINT chk_survey_questions_kind CHECK (kind IN ('single', 'multiple', 'scale', 'text')),
			CONSTRAINT chk_survey_questions_prompt_not_blank CHECK (length(btrim(prompt)) > 0),
			CONSTRAINT uq_survey_questions_position UNIQUE (survey_id, position)
		);

		CREATE TABLE IF NOT EXISTS users.survey_question_options (
			id          BIGSERIAL PRIMARY KEY,
			tenant_id   BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			question_id BIGINT NOT NULL REFERENCES users.survey_questions(id) ON DELETE CASCADE,
			position    INTEGER NOT NULL,
			label       TEXT NOT NULL,
			CONSTRAINT chk_survey_question_options_label_not_blank CHECK (length(btrim(label)) > 0),
			CONSTRAINT uq_survey_question_options_position UNIQUE (question_id, position)
		);

		-- Who took part, nothing else: no id, no timestamp.
		CREATE TABLE IF NOT EXISTS users.survey_participations (
			tenant_id  BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			survey_id  BIGINT NOT NULL REFERENCES users.surveys(id) ON DELETE CASCADE,
			account_id BIGINT NOT NULL REFERENCES auth.accounts(id) ON DELETE CASCADE,
			PRIMARY KEY (survey_id, account_id)
		);
		CREATE INDEX IF NOT EXISTS idx_survey_participations_account
			ON users.survey_participations (tenant_id, account_id);

		-- What was answered: random ids, no timestamp, an account only for a
		-- named survey.
		CREATE TABLE IF NOT EXISTS users.survey_responses (
			id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			tenant_id  BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			survey_id  BIGINT NOT NULL REFERENCES users.surveys(id) ON DELETE CASCADE,
			account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL
		);
		CREATE INDEX IF NOT EXISTS idx_survey_responses_survey
			ON users.survey_responses (tenant_id, survey_id);

		CREATE TABLE IF NOT EXISTS users.survey_answers (
			id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			tenant_id   BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			response_id UUID NOT NULL REFERENCES users.survey_responses(id) ON DELETE CASCADE,
			question_id BIGINT NOT NULL REFERENCES users.survey_questions(id) ON DELETE CASCADE,
			option_id   BIGINT REFERENCES users.survey_question_options(id) ON DELETE CASCADE,
			scale_value SMALLINT,
			text_value  TEXT,
			CONSTRAINT chk_survey_answers_one_value CHECK (num_nonnulls(option_id, scale_value, text_value) = 1),
			CONSTRAINT chk_survey_answers_scale CHECK (scale_value IS NULL OR scale_value BETWEEN 1 AND 5),
			CONSTRAINT chk_survey_answers_text_not_blank CHECK (text_value IS NULL OR length(btrim(text_value)) > 0)
		);
		CREATE INDEX IF NOT EXISTS idx_survey_answers_question
			ON users.survey_answers (tenant_id, question_id);
		CREATE INDEX IF NOT EXISTS idx_survey_answers_response
			ON users.survey_answers (response_id);

		DROP TRIGGER IF EXISTS update_surveys_updated_at ON users.surveys;
		CREATE TRIGGER update_surveys_updated_at
		BEFORE UPDATE ON users.surveys
		FOR EACH ROW EXECUTE FUNCTION update_modified_column();

		-- A response of an anonymous survey never names an account, whatever
		-- the application sends.
		CREATE OR REPLACE FUNCTION users.survey_responses_keep_anonymous() RETURNS trigger
		LANGUAGE plpgsql AS $$
		BEGIN
			IF NEW.account_id IS NOT NULL AND EXISTS (
				SELECT 1 FROM users.surveys s WHERE s.id = NEW.survey_id AND s.anonymous
			) THEN
				RAISE EXCEPTION 'an anonymous survey stores no account with its answers' USING ERRCODE = '23514';
			END IF;
			RETURN NEW;
		END;
		$$;
		DROP TRIGGER IF EXISTS survey_responses_keep_anonymous ON users.survey_responses;
		CREATE TRIGGER survey_responses_keep_anonymous
		BEFORE INSERT OR UPDATE ON users.survey_responses
		FOR EACH ROW EXECUTE FUNCTION users.survey_responses_keep_anonymous();

		-- Once published, a survey keeps its anonymity setting and its audience:
		-- the people who answered relied on what they were told.
		CREATE OR REPLACE FUNCTION users.surveys_freeze_published() RETURNS trigger
		LANGUAGE plpgsql AS $$
		BEGIN
			IF OLD.published_at IS NOT NULL
				AND (NEW.anonymous IS DISTINCT FROM OLD.anonymous OR NEW.audience IS DISTINCT FROM OLD.audience) THEN
				RAISE EXCEPTION 'a published survey keeps its anonymity and audience' USING ERRCODE = '23514';
			END IF;
			RETURN NEW;
		END;
		$$;
		DROP TRIGGER IF EXISTS surveys_freeze_published ON users.surveys;
		CREATE TRIGGER surveys_freeze_published
		BEFORE UPDATE ON users.surveys
		FOR EACH ROW EXECUTE FUNCTION users.surveys_freeze_published();

		GRANT SELECT, INSERT, UPDATE, DELETE ON
			users.surveys, users.survey_questions, users.survey_question_options,
			users.survey_participations, users.survey_responses, users.survey_answers
			TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE
			users.surveys_id_seq, users.survey_questions_id_seq, users.survey_question_options_id_seq
			TO phoenix_tenant;
`

// surveyTables are the tables of this migration, parents first.
var surveyTables = []string{
	"users.surveys",
	"users.survey_questions",
	"users.survey_question_options",
	"users.survey_participations",
	"users.survey_responses",
	"users.survey_answers",
}

func surveysUp(ctx context.Context, db *bun.DB) error {
	if err := db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, surveysTablesSQL); err != nil {
			return fmt.Errorf("create survey tables: %w", err)
		}
		// Tenant isolation commits with the tables, never after their grants.
		if err := provisionTenantRLS(ctx, tx, surveyTables...); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, surveyDataKindsUpSQL); err != nil {
			return fmt.Errorf("extend data_deletions subject check: %w", err)
		}
		if err := installSchoolErasureFunctions(ctx, tx); err != nil {
			return fmt.Errorf("refresh school erasure functions for surveys: %w", err)
		}
		if err := installSchoolErasurePreview(ctx, tx); err != nil {
			return fmt.Errorf("refresh school erasure preview for surveys: %w", err)
		}
		return nil
	}); err != nil {
		return err
	}
	return grantPermissionToRoles(ctx, db, permissionSpec{
		Name:        surveysManagePermissionName,
		Description: "Umfragen anlegen, veröffentlichen und auswerten",
		Resource:    "surveys",
		Action:      "manage",
	}, "admin")
}

// surveysDown refuses while survey retention runs are logged: the restored
// subject check would reject them.
func surveysDown(ctx context.Context, db *bun.DB) error {
	var logged int
	if err := db.NewRaw(`SELECT COUNT(*) FROM audit.data_deletions
		WHERE deletion_type IN (?, ?) AND student_id IS NULL AND staff_id IS NULL`,
		parentSurveyRetentionKind, staffSurveyRetentionKind).Scan(ctx, &logged); err != nil {
		return fmt.Errorf("check survey retention deletion log rows: %w", err)
	}
	if logged > 0 {
		return fmt.Errorf("cannot restore the data_deletions subject check while %d survey retention rows exist", logged)
	}
	if err := db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, surveyDataKindsDownSQL); err != nil {
			return fmt.Errorf("restore data_deletions subject check: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `
			DROP TABLE IF EXISTS users.survey_answers;
			DROP TABLE IF EXISTS users.survey_responses;
			DROP TABLE IF EXISTS users.survey_participations;
			DROP TABLE IF EXISTS users.survey_question_options;
			DROP TABLE IF EXISTS users.survey_questions;
			DROP TABLE IF EXISTS users.surveys;
			DROP FUNCTION IF EXISTS users.survey_responses_keep_anonymous();
			DROP FUNCTION IF EXISTS users.surveys_freeze_published();
			DELETE FROM users.parent_announcements WHERE system_kind = 'survey';
			ALTER TABLE users.parent_announcements
				DROP CONSTRAINT IF EXISTS chk_parent_announcements_system_kind,
				ADD CONSTRAINT chk_parent_announcements_system_kind
					CHECK (system_kind IS NULL OR system_kind IN ('care_cancellation'));
		`); err != nil {
			return fmt.Errorf("drop survey tables: %w", err)
		}
		return nil
	}); err != nil {
		return err
	}
	return dropPermission(ctx, db, surveysManagePermissionName)
}

// surveyDataKindsUpSQL extends the subject check of audit.data_deletions by
// the two survey retention kinds; surveyDataKindsDownSQL restores the check
// of 1.15.492. The list is spelled out: the architecture check classifies
// only static migration SQL.
const surveyDataKindsUpSQL = `
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
						'enrollment_upload_retention',
						'parent_survey_retention',
						'staff_survey_retention'
					)
				)
			);
`

const surveyDataKindsDownSQL = `
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
`
