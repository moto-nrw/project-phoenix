package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const messageAttachmentsVersion = "1.15.936"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     messageAttachmentsVersion,
		Description: "Nachrichten: Fotos und Dateien als Anhänge in Eltern- und Teamchats (#6685)",
		DependsOn: []string{
			holidayHostOwnContactsVersion,   // the newest on development (1.15.934)
			"1.15.723",                      // client_message_id on the three message tables
			"1.15.811",                      // the school erasure functions this migration extends
			enrollmentRequestUploadsVersion, // the file phase of the school erasure as it stands
		},
	})
	Migrations.MustRegister(messageAttachmentsUp, messageAttachmentsDown)
}

// messageAttachmentsUpSQL stores the photos and files of a chat message
// (#6685).
//
//   - users.message_attachments holds the metadata. The bytes live in the
//     private object store under message-attachments/{tenant_id}/{uuid}; no
//     public URL reaches them. A row starts unclaimed with its uploader;
//     sending the message claims it for exactly one message of the same chat
//     family (scope parent: 1:1 and group threads with guardians, scope
//     staff: the team chat). An unclaimed row expires after a day.
//   - The uploader is kept for the claim only: an account that goes leaves
//     its sent files with their messages and its unsent ones to the expiry.
//   - A claimed row has no lifetime of its own: it goes with its message
//     (ON DELETE CASCADE), so the message retention, a deleted conversation
//     and a deleted child remove the file with the text.
//   - Every deleted row queues its object in users.message_attachment_cleanup
//     through a trigger, inside the deleting transaction. The sweep removes
//     the bytes and settles the intent. An intent outlives its row; its only
//     foreign key is the school. The trigger queues nothing while the school
//     itself is deleted: the erasure removes the school's object directory.
//   - The school erasure learns both tables: the attachments go before
//     their messages and their cleanup intents after them
//     (message_attachment_school_erasure_*.go, copies of the functions of
//     1.15.811 and 1.15.758 with the two tables added).
//   - The three message tables carry a snapshot of their attachments
//     (attachments JSONB), so a conversation loads with its files in the
//     statement that loads the messages. A message may now consist of
//     attachments only: the body check accepts an empty text beside at
//     least one attachment.
const messageAttachmentsUpSQL = `
		CREATE TABLE users.message_attachments (
			id BIGSERIAL PRIMARY KEY,
			tenant_id BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			scope TEXT NOT NULL CHECK (scope IN ('parent', 'staff')),
			uploaded_by_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
			parent_message_id BIGINT REFERENCES users.parent_messages(id) ON DELETE CASCADE,
			parent_group_message_id BIGINT REFERENCES users.parent_group_messages(id) ON DELETE CASCADE,
			staff_message_id BIGINT REFERENCES users.staff_messages(id) ON DELETE CASCADE,
			filename_display TEXT NOT NULL,
			filename_stored TEXT NOT NULL,
			size_bytes BIGINT NOT NULL CHECK (size_bytes > 0),
			content_type TEXT NOT NULL,
			width INTEGER CHECK (width > 0),
			height INTEGER CHECK (height > 0),
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			claimed_at TIMESTAMPTZ,
			CONSTRAINT uq_message_attachments_stored UNIQUE (filename_stored),
			CONSTRAINT chk_message_attachments_claim CHECK (
				(claimed_at IS NULL AND num_nonnulls(parent_message_id, parent_group_message_id, staff_message_id) = 0)
				OR (claimed_at IS NOT NULL AND num_nonnulls(parent_message_id, parent_group_message_id, staff_message_id) = 1)
			),
			CONSTRAINT chk_message_attachments_scope CHECK (
				(scope = 'parent' AND staff_message_id IS NULL)
				OR (scope = 'staff' AND parent_message_id IS NULL AND parent_group_message_id IS NULL)
			)
		);

		CREATE INDEX idx_message_attachments_parent_message
			ON users.message_attachments (parent_message_id) WHERE parent_message_id IS NOT NULL;
		CREATE INDEX idx_message_attachments_parent_group_message
			ON users.message_attachments (parent_group_message_id) WHERE parent_group_message_id IS NOT NULL;
		CREATE INDEX idx_message_attachments_staff_message
			ON users.message_attachments (staff_message_id) WHERE staff_message_id IS NOT NULL;
		CREATE INDEX idx_message_attachments_unclaimed
			ON users.message_attachments (tenant_id, uploaded_by_account_id, created_at) WHERE claimed_at IS NULL;

		CREATE TABLE users.message_attachment_cleanup (
			id BIGSERIAL PRIMARY KEY,
			tenant_id BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			filename_stored TEXT NOT NULL,
			cleaned_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT uq_message_attachment_cleanup UNIQUE (tenant_id, filename_stored)
		);

		CREATE INDEX idx_message_attachment_cleanup_pending
			ON users.message_attachment_cleanup (tenant_id, id) WHERE cleaned_at IS NULL;

		CREATE FUNCTION users.queue_message_attachment_cleanup()
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
			INSERT INTO users.message_attachment_cleanup (tenant_id, filename_stored)
			VALUES (OLD.tenant_id, OLD.filename_stored)
			ON CONFLICT (tenant_id, filename_stored) DO UPDATE SET cleaned_at = NULL;
			RETURN OLD;
		END;
		$$;
		REVOKE ALL ON FUNCTION users.queue_message_attachment_cleanup() FROM PUBLIC;

		CREATE TRIGGER trg_message_attachments_queue_cleanup
		AFTER DELETE ON users.message_attachments
		FOR EACH ROW EXECUTE FUNCTION users.queue_message_attachment_cleanup();

		GRANT SELECT, INSERT, UPDATE, DELETE ON users.message_attachments TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE users.message_attachments_id_seq TO phoenix_tenant;
		GRANT SELECT, INSERT, UPDATE, DELETE ON users.message_attachment_cleanup TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE users.message_attachment_cleanup_id_seq TO phoenix_tenant;
		GRANT ALL ON users.message_attachments, users.message_attachment_cleanup TO phoenix_admin;
		GRANT ALL ON SEQUENCE users.message_attachments_id_seq, users.message_attachment_cleanup_id_seq TO phoenix_admin;

		ALTER TABLE users.parent_messages
			ADD COLUMN attachments JSONB NOT NULL DEFAULT '[]'::jsonb,
			ADD CONSTRAINT chk_parent_messages_attachments_array CHECK (jsonb_typeof(attachments) = 'array'),
			DROP CONSTRAINT chk_parent_messages_body_not_blank,
			ADD CONSTRAINT chk_parent_messages_body_not_blank
				CHECK (length(btrim(body)) > 0 OR jsonb_array_length(attachments) > 0);
		ALTER TABLE users.parent_group_messages
			ADD COLUMN attachments JSONB NOT NULL DEFAULT '[]'::jsonb,
			ADD CONSTRAINT chk_parent_group_messages_attachments_array CHECK (jsonb_typeof(attachments) = 'array'),
			DROP CONSTRAINT chk_parent_group_messages_body_not_blank,
			ADD CONSTRAINT chk_parent_group_messages_body_not_blank
				CHECK (length(btrim(body)) > 0 OR jsonb_array_length(attachments) > 0);
		ALTER TABLE users.staff_messages
			ADD COLUMN attachments JSONB NOT NULL DEFAULT '[]'::jsonb,
			ADD CONSTRAINT chk_staff_messages_attachments_array CHECK (jsonb_typeof(attachments) = 'array'),
			DROP CONSTRAINT chk_staff_messages_body_not_blank,
			ADD CONSTRAINT chk_staff_messages_body_not_blank
				CHECK (length(btrim(body)) > 0 OR jsonb_array_length(attachments) > 0);
`

// messageAttachmentsDownSQL drops the attachments and restores the body
// checks. Objects still on disk stay; the down path is for development
// databases. A message that consists of attachments only would violate the
// restored check and goes with its attachments.
const messageAttachmentsDownSQL = `
		DELETE FROM users.parent_messages WHERE length(btrim(body)) = 0;
		DELETE FROM users.parent_group_messages WHERE length(btrim(body)) = 0;
		DELETE FROM users.staff_messages WHERE length(btrim(body)) = 0;
		ALTER TABLE users.parent_messages
			DROP CONSTRAINT chk_parent_messages_body_not_blank,
			DROP CONSTRAINT chk_parent_messages_attachments_array,
			DROP COLUMN attachments,
			ADD CONSTRAINT chk_parent_messages_body_not_blank CHECK (length(btrim(body)) > 0);
		ALTER TABLE users.parent_group_messages
			DROP CONSTRAINT chk_parent_group_messages_body_not_blank,
			DROP CONSTRAINT chk_parent_group_messages_attachments_array,
			DROP COLUMN attachments,
			ADD CONSTRAINT chk_parent_group_messages_body_not_blank CHECK (length(btrim(body)) > 0);
		ALTER TABLE users.staff_messages
			DROP CONSTRAINT chk_staff_messages_body_not_blank,
			DROP CONSTRAINT chk_staff_messages_attachments_array,
			DROP COLUMN attachments,
			ADD CONSTRAINT chk_staff_messages_body_not_blank CHECK (length(btrim(body)) > 0);
		DROP TABLE users.message_attachments CASCADE;
		DROP TABLE users.message_attachment_cleanup CASCADE;
		DROP FUNCTION users.queue_message_attachment_cleanup();
`

// messageAttachmentErasureFilePhaseSQL lets the file phase of the school
// erasure record the private message-attachments namespace as the eleventh
// reviewed file kind; the function is otherwise the one of 1.15.491.
const messageAttachmentErasureFilePhaseSQL = `
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
 IF (SELECT count(*) FROM jsonb_each(p_counts))<>11 OR EXISTS (
 SELECT FROM jsonb_each(p_counts) e WHERE e.key NOT IN ('files','announcement-attachments','student-documents','staff-documents','student-photos','login-images','enrollment-legal-documents','enrollment-form-legal-documents','enrollment-uploads','avatars','message-attachments')
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

// messageAttachmentErasureFilePhaseDownSQL restores the file phase of
// 1.15.491 with its ten file kinds.
const messageAttachmentErasureFilePhaseDownSQL = `
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

func messageAttachmentsUp(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, messageAttachmentsUpSQL); err != nil {
			return fmt.Errorf("create message attachments: %w", err)
		}
		if err := provisionTenantRLS(ctx, tx, "users.message_attachments", "users.message_attachment_cleanup"); err != nil {
			return err
		}
		for _, step := range []func(context.Context, bun.Tx) error{
			installMessageAttachmentErasurePlan, installMessageAttachmentErasurePreflight,
			installMessageAttachmentErasureResidual, installMessageAttachmentErasureDelete,
			installMessageAttachmentErasurePreview,
		} {
			if err := step(ctx, tx); err != nil {
				return fmt.Errorf("add the message attachments to the school erasure: %w", err)
			}
		}
		if _, err := tx.ExecContext(ctx, messageAttachmentErasureFilePhaseSQL); err != nil {
			return fmt.Errorf("extend school erasure file phase: %w", err)
		}
		return nil
	})
}

func messageAttachmentsDown(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		for _, step := range []func(context.Context, bun.Tx) error{
			installAttendanceRetentionMarkErasurePlan, installEmailDeliveryEventErasurePreflight,
			installAttendanceRetentionMarkErasureResidual, installAttendanceRetentionMarkErasureDelete,
			installAttendanceRetentionMarkErasurePreview,
		} {
			if err := step(ctx, tx); err != nil {
				return fmt.Errorf("remove the message attachments from the school erasure: %w", err)
			}
		}
		if _, err := tx.ExecContext(ctx, messageAttachmentErasureFilePhaseDownSQL); err != nil {
			return fmt.Errorf("restore school erasure file phase: %w", err)
		}
		if _, err := tx.ExecContext(ctx, messageAttachmentsDownSQL); err != nil {
			return fmt.Errorf("drop message attachments: %w", err)
		}
		return nil
	})
}
