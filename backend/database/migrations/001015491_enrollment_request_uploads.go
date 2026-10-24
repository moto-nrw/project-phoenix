package migrations

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/uptrace/bun"
)

const enrollmentRequestUploadsVersion = "1.15.491"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     enrollmentRequestUploadsVersion,
		Description: "Anmeldung: Dateien der Eltern (Einkommensnachweise) mit Löschfrist und eigenem Recht (#4783)",
		DependsOn: []string{
			"1.15.490",                      // the last migration before this one (school erasure files)
			schoolContentRetentionVersion,   // previous subject check of audit.data_deletions
			createEnrollmentRequestsVersion, // enrollment.requests
			schoolErasureVersion,            // the erasure plan this migration extends
		},
	})
	Migrations.MustRegister(enrollmentRequestUploadsUp, enrollmentRequestUploadsDown)
}

// enrollmentRequestUploadsUpSQL stores the files parents attach to an
// enrollment request (#4783).
//
//   - enrollment.request_uploads holds the metadata. The bytes live in the
//     private object store under enrollment-uploads/{tenant_id}/{uuid}; no
//     public URL reaches them. A row starts unclaimed with the hash of the
//     token the uploader holds; the submission claims it for its request and
//     clears the hash. Unclaimed rows expire after a day, claimed rows after
//     the school's window once the request is decided.
//   - Every deleted row, whether deleted directly or cascaded with its
//     request or phase, queues its object in
//     enrollment.request_upload_cleanup through a trigger, inside the
//     deleting transaction. The sweep removes the bytes and settles the
//     intent. An intent outlives the row and the request it came from; its
//     only foreign key is the school, the write fence of the school erasure.
//     The trigger runs as its owner and queues nothing while a school is
//     itself deleted: the erasure removes the school's object directory.
//   - The school erasure plan of 1.15.481 learns both tables: the reinstalled
//     plan, preflight, delete, preview and residual functions lock, check and remove the uploads before their
//     requests and phases, and the cleanup intents after the uploads whose
//     deletion fills them.
//   - The upload bucket of the public form counts per IP next to the
//     submission buckets (key type upload_ip).
//   - enrollment_uploads:read gates opening a file. Like
//     student_documents:health it is catalog-only: school admins match via
//     the admin:* wildcard, other roles get it explicitly.
const enrollmentRequestUploadsUpSQL = `
		CREATE TABLE IF NOT EXISTS enrollment.request_uploads (
			id BIGSERIAL PRIMARY KEY,
			tenant_id BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			phase_id BIGINT NOT NULL REFERENCES enrollment.phases(id) ON DELETE CASCADE,
			field_key TEXT NOT NULL,
			request_id BIGINT REFERENCES enrollment.requests(id) ON DELETE CASCADE,
			token_hash TEXT,
			filename_display TEXT NOT NULL,
			filename_stored TEXT NOT NULL,
			size_bytes BIGINT NOT NULL CHECK (size_bytes > 0),
			content_type TEXT NOT NULL CHECK (content_type IN ('application/pdf', 'image/jpeg', 'image/png')),
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			claimed_at TIMESTAMPTZ,
			CONSTRAINT uq_request_uploads_stored UNIQUE (filename_stored),
			CONSTRAINT uq_request_uploads_token UNIQUE (token_hash),
			CONSTRAINT chk_request_uploads_claim CHECK (
				(request_id IS NULL AND claimed_at IS NULL AND token_hash IS NOT NULL)
				OR (request_id IS NOT NULL AND claimed_at IS NOT NULL AND token_hash IS NULL)
			)
		);

		CREATE INDEX IF NOT EXISTS idx_request_uploads_request
			ON enrollment.request_uploads (tenant_id, request_id)
			WHERE request_id IS NOT NULL;
		CREATE INDEX IF NOT EXISTS idx_request_uploads_unclaimed
			ON enrollment.request_uploads (tenant_id, created_at)
			WHERE request_id IS NULL;

		CREATE TABLE IF NOT EXISTS enrollment.request_upload_cleanup (
			id BIGSERIAL PRIMARY KEY,
			tenant_id BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			filename_stored TEXT NOT NULL,
			cleaned_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT uq_request_upload_cleanup UNIQUE (tenant_id, filename_stored)
		);

		CREATE INDEX IF NOT EXISTS idx_request_upload_cleanup_pending
			ON enrollment.request_upload_cleanup (tenant_id, id)
			WHERE cleaned_at IS NULL;

		CREATE OR REPLACE FUNCTION enrollment.queue_request_upload_cleanup()
		RETURNS TRIGGER
		LANGUAGE plpgsql
		SECURITY DEFINER
		SET search_path = pg_catalog
		AS $$
		BEGIN
			-- A school that is going away takes its whole object directory
			-- with it (data lifecycle layout tenantDir); its rows queue nothing.
			IF NOT EXISTS (SELECT 1 FROM platform.schools WHERE id = OLD.tenant_id) THEN
				RETURN OLD;
			END IF;
			INSERT INTO enrollment.request_upload_cleanup (tenant_id, filename_stored)
			VALUES (OLD.tenant_id, OLD.filename_stored)
			ON CONFLICT (tenant_id, filename_stored) DO UPDATE SET cleaned_at = NULL;
			RETURN OLD;
		END;
		$$;
		REVOKE ALL ON FUNCTION enrollment.queue_request_upload_cleanup() FROM PUBLIC;

		DROP TRIGGER IF EXISTS trg_request_uploads_queue_cleanup ON enrollment.request_uploads;
		CREATE TRIGGER trg_request_uploads_queue_cleanup
		AFTER DELETE ON enrollment.request_uploads
		FOR EACH ROW EXECUTE FUNCTION enrollment.queue_request_upload_cleanup();

		ALTER TABLE enrollment.request_uploads ENABLE ROW LEVEL SECURITY;
		ALTER TABLE enrollment.request_uploads FORCE ROW LEVEL SECURITY;
		ALTER TABLE enrollment.request_upload_cleanup ENABLE ROW LEVEL SECURITY;
		ALTER TABLE enrollment.request_upload_cleanup FORCE ROW LEVEL SECURITY;

		DROP POLICY IF EXISTS tenant_isolation_enrollment_request_uploads ON enrollment.request_uploads;
		CREATE POLICY tenant_isolation_enrollment_request_uploads
			ON enrollment.request_uploads
			FOR ALL
			USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
			WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);
		DROP POLICY IF EXISTS tenant_isolation_enrollment_request_upload_cleanup ON enrollment.request_upload_cleanup;
		CREATE POLICY tenant_isolation_enrollment_request_upload_cleanup
			ON enrollment.request_upload_cleanup
			FOR ALL
			USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
			WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);

		GRANT SELECT, INSERT, UPDATE, DELETE ON enrollment.request_uploads TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE enrollment.request_uploads_id_seq TO phoenix_tenant;
		GRANT SELECT, INSERT, UPDATE, DELETE ON enrollment.request_upload_cleanup TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE enrollment.request_upload_cleanup_id_seq TO phoenix_tenant;
		GRANT ALL ON enrollment.request_uploads, enrollment.request_upload_cleanup TO phoenix_admin;
		GRANT ALL ON SEQUENCE enrollment.request_uploads_id_seq, enrollment.request_upload_cleanup_id_seq TO phoenix_admin;

		ALTER TABLE enrollment.submission_rate_limits
			DROP CONSTRAINT IF EXISTS submission_rate_limits_key_type_check;
		ALTER TABLE enrollment.submission_rate_limits
			ADD CONSTRAINT submission_rate_limits_key_type_check CHECK (key_type IN ('ip', 'email', 'upload_ip'));

		INSERT INTO auth.permissions (name, description, resource, action, is_system)
		VALUES ('enrollment_uploads:read', 'Open the files parents attach to an enrollment request, such as income proofs', 'enrollment_uploads', 'read', TRUE)
		ON CONFLICT (name) DO NOTHING;

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
						'setting_audit_retention',
						'enrollment_upload_retention'
					)
				)
			);
`

// enrollmentRequestUploadsDownSQL restores the subject check of 1.15.465 and
// drops the tables. Objects still on disk stay; the down path is for
// development databases.
const enrollmentRequestUploadsDownSQL = `
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

		DELETE FROM enrollment.submission_rate_limits WHERE key_type = 'upload_ip';
		ALTER TABLE enrollment.submission_rate_limits
			DROP CONSTRAINT IF EXISTS submission_rate_limits_key_type_check;
		ALTER TABLE enrollment.submission_rate_limits
			ADD CONSTRAINT submission_rate_limits_key_type_check CHECK (key_type IN ('ip', 'email'));

		DELETE FROM auth.role_permissions
		WHERE permission_id IN (SELECT id FROM auth.permissions WHERE name = 'enrollment_uploads:read');
		DELETE FROM auth.permissions WHERE name = 'enrollment_uploads:read';
		DROP TABLE IF EXISTS enrollment.request_uploads CASCADE;
		DROP TABLE IF EXISTS enrollment.request_upload_cleanup CASCADE;
		DROP FUNCTION IF EXISTS enrollment.queue_request_upload_cleanup();
`

// enrollmentUploadErasureFilePhaseSQL lets the file phase of the school
// erasure (1.15.490) record the private enrollment-uploads namespace as the
// tenth reviewed file kind; the function is otherwise unchanged.
const enrollmentUploadErasureFilePhaseSQL = `
 CREATE OR REPLACE FUNCTION platform.school_erasure_file_phase(p_id BIGINT,p_phase TEXT,p_counts JSONB)
 RETURNS JSONB LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $body$
 DECLARE v_order platform.school_erasures%ROWTYPE;
 BEGIN
 SELECT * INTO v_order FROM platform.school_erasures WHERE id=p_id FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'existing erasure required' USING ERRCODE='22023'; END IF;
 IF v_order.completed_at IS NOT NULL THEN RETURN to_jsonb(v_order); END IF;
 IF p_phase='inventory' THEN
 IF v_order.files_inventory_at IS NULL THEN
 IF v_order.db_done_at IS NOT NULL OR p_counts IS NULL OR jsonb_typeof(p_counts)<>'object' THEN
 RAISE EXCEPTION 'inventory required before database deletion' USING ERRCODE='22023'; END IF;
 IF (SELECT count(*) FROM jsonb_each(p_counts))<>10 OR EXISTS (
 SELECT FROM jsonb_each(p_counts) e WHERE e.key NOT IN ('files','announcement-attachments','student-documents','staff-documents','student-photos','login-images','enrollment-legal-documents','enrollment-form-legal-documents','enrollment-uploads','avatars')
 OR jsonb_typeof(e.value)<>'number' OR e.value::text !~ '^[0-9]+$') THEN
 RAISE EXCEPTION 'bounded file counts required' USING ERRCODE='22023'; END IF;
 UPDATE platform.school_erasures SET files_inventory_at=clock_timestamp(),file_counts=p_counts,procedure_version='1.15.490' WHERE id=p_id;
 END IF;
 ELSIF p_phase='failed' THEN
 UPDATE platform.school_erasures SET status='failed',failed_at=clock_timestamp(),last_error_code='F0001' WHERE id=p_id;
 ELSIF p_phase='files_done' AND v_order.db_done_at IS NOT NULL AND v_order.files_inventory_at IS NOT NULL THEN
 UPDATE platform.school_erasures SET status='files_done',files_done_at=COALESCE(files_done_at,clock_timestamp()),failed_at=NULL,last_error_code=NULL WHERE id=p_id;
 ELSIF p_phase='completed' AND v_order.status='files_done' THEN
 UPDATE platform.school_erasures SET status='completed',completed_at=clock_timestamp() WHERE id=p_id;
 ELSE RAISE EXCEPTION 'invalid file phase transition' USING ERRCODE='22023'; END IF;
 RETURN platform.read_school_erasure(p_id);
 END $body$;
`

func enrollmentRequestUploadsUp(ctx context.Context, db *bun.DB) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer rollbackEnrollmentRequestUploads(ctx, tx)
	if _, err := tx.ExecContext(ctx, enrollmentRequestUploadsUpSQL); err != nil {
		return fmt.Errorf("create enrollment request uploads: %w", err)
	}
	if err := installSchoolErasurePlan(ctx, tx); err != nil {
		return fmt.Errorf("extend school erasure plan: %w", err)
	}
	if err := installSchoolErasureDelete(ctx, tx); err != nil {
		return fmt.Errorf("extend school erasure delete: %w", err)
	}
	if err := installSchoolErasurePreflight(ctx, tx); err != nil {
		return fmt.Errorf("extend school erasure preflight: %w", err)
	}
	if err := installSchoolErasurePreview(ctx, tx); err != nil {
		return fmt.Errorf("extend school erasure preview: %w", err)
	}
	if err := installSchoolErasureResidual(ctx, tx); err != nil {
		return fmt.Errorf("extend school erasure residual check: %w", err)
	}
	if _, err := tx.ExecContext(ctx, enrollmentUploadErasureFilePhaseSQL); err != nil {
		return fmt.Errorf("extend school erasure file phase: %w", err)
	}
	return tx.Commit()
}

// enrollmentRequestUploadsDown refuses while upload retention rows are
// logged: the restored check would reject them.
func enrollmentRequestUploadsDown(ctx context.Context, db *bun.DB) error {
	var logged int
	if err := db.NewRaw(`SELECT COUNT(*) FROM audit.data_deletions
		WHERE deletion_type = 'enrollment_upload_retention' AND student_id IS NULL AND staff_id IS NULL`).Scan(ctx, &logged); err != nil {
		return fmt.Errorf("check enrollment upload retention deletion log rows: %w", err)
	}
	if logged > 0 {
		return fmt.Errorf("cannot restore the data_deletions subject check while %d enrollment upload retention rows exist", logged)
	}
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer rollbackEnrollmentRequestUploads(ctx, tx)
	if _, err := tx.ExecContext(ctx, enrollmentRequestUploadsDownSQL); err != nil {
		return fmt.Errorf("revert enrollment request uploads: %w", err)
	}
	return tx.Commit()
}

func rollbackEnrollmentRequestUploads(ctx context.Context, tx bun.Tx) {
	if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
		logRollbackFailure(ctx, err)
	}
}
