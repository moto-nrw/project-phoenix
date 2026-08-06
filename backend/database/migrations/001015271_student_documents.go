package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	studentDocumentsVersion     = "1.15.271"
	studentDocumentsDescription = "Create users.student_documents plus the student_documents:health and :legal permissions (#777)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     studentDocumentsVersion,
		Description: studentDocumentsDescription,
		DependsOn: []string{
			compositePKIndexesVersion, // UNIQUE users.students(tenant_id, id) backs the composite FK
		},
	})

	Migrations.MustRegister(studentDocumentsUp, studentDocumentsDown)
}

// studentDocumentsUp creates document storage for children (issue #777).
//
// Only metadata lives in the database — the bytes are stored through the
// storage backend under student-documents/{tenant_id}/{uuid} and are
// reachable exclusively through the permission-checked download handler
// (there is no static file server for backend public/).
//
// deleted_at is a soft delete on purpose: the metadata row is the record of
// what existed. The file itself is removed on delete; the row stays.
// phoenix_tenant therefore gets no DELETE grant — deletion is an UPDATE
// setting deleted_at/deleted_by.
//
// The cleanup table has NO foreign key to users.students, and that is
// load-bearing rather than an oversight. Documents cascade away when a child
// is deleted; a cleanup row that cascaded with them would leave the stored
// bytes on disk with nothing left to point at them.
//
// student_documents:health gates Attest, Impfnachweis and Medikamentenplan
// (Art. 9 GDPR health data); student_documents:legal gates the
// Sorgerechtsnachweis. Both are catalog-only like staff_documents:health
// (1.15.257): school admins match via the admin:* wildcard, and a dedicated
// role can be granted either permission explicitly without another migration.
// Every other category rides on the existing users:update permission. All
// three checks live in the document service.
func studentDocumentsUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.271: Creating users.student_documents + student document permissions...")

	if _, err := db.NewRaw(`
		CREATE TABLE IF NOT EXISTS users.student_documents (
			id BIGSERIAL PRIMARY KEY,
			tenant_id BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			student_id BIGINT NOT NULL,
			category TEXT NOT NULL CHECK (category IN (
				'betreuungsvertrag', 'abholvollmacht', 'schwimmerlaubnis',
				'attest', 'impfnachweis', 'medikamentenplan',
				'sorgerecht', 'sonstiges'
			)),
			filename_display TEXT NOT NULL,
			filename_stored TEXT NOT NULL,
			size_bytes BIGINT NOT NULL CHECK (size_bytes >= 0),
			content_type TEXT NOT NULL,
			uploaded_by BIGINT NOT NULL,
			deleted_at TIMESTAMPTZ,
			deleted_by BIGINT,
			file_deleted_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT uq_student_documents_stored UNIQUE (filename_stored),
			CONSTRAINT fk_student_documents_student
				FOREIGN KEY (tenant_id, student_id)
				REFERENCES users.students(tenant_id, id) ON DELETE CASCADE
		);

		CREATE INDEX IF NOT EXISTS idx_student_documents_student
			ON users.student_documents (tenant_id, student_id)
			WHERE deleted_at IS NULL;

		CREATE INDEX IF NOT EXISTS idx_student_documents_pending_file_cleanup
			ON users.student_documents (tenant_id, student_id)
			WHERE deleted_at IS NOT NULL AND file_deleted_at IS NULL;

		CREATE TABLE IF NOT EXISTS users.student_document_file_cleanup (
			id BIGSERIAL PRIMARY KEY,
			tenant_id BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			owner_id BIGINT NOT NULL,
			filename_stored TEXT NOT NULL,
			retry_after TIMESTAMPTZ NOT NULL DEFAULT NOW() + INTERVAL '5 minutes',
			cleaned_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT uq_student_document_file_cleanup UNIQUE (tenant_id, filename_stored)
		);

		CREATE INDEX IF NOT EXISTS idx_student_document_file_cleanup_pending
			ON users.student_document_file_cleanup (tenant_id, retry_after, owner_id)
			WHERE cleaned_at IS NULL;

		DROP TRIGGER IF EXISTS update_student_documents_updated_at ON users.student_documents;
		CREATE TRIGGER update_student_documents_updated_at
		BEFORE UPDATE ON users.student_documents
		FOR EACH ROW EXECUTE FUNCTION update_modified_column();

		DROP TRIGGER IF EXISTS update_student_document_file_cleanup_updated_at ON users.student_document_file_cleanup;
		CREATE TRIGGER update_student_document_file_cleanup_updated_at
		BEFORE UPDATE ON users.student_document_file_cleanup
		FOR EACH ROW EXECUTE FUNCTION update_modified_column();

		ALTER TABLE users.student_documents ENABLE ROW LEVEL SECURITY;
		ALTER TABLE users.student_documents FORCE ROW LEVEL SECURITY;
		ALTER TABLE users.student_document_file_cleanup ENABLE ROW LEVEL SECURITY;
		ALTER TABLE users.student_document_file_cleanup FORCE ROW LEVEL SECURITY;

		DO $$
		BEGIN
			IF NOT EXISTS (
				SELECT 1 FROM pg_policies
				WHERE schemaname = 'users' AND tablename = 'student_documents'
					AND policyname = 'tenant_isolation_users_student_documents'
			) THEN
				CREATE POLICY tenant_isolation_users_student_documents
					ON users.student_documents
					FOR ALL
					USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
					WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);
			END IF;

			IF NOT EXISTS (
				SELECT 1 FROM pg_policies
				WHERE schemaname = 'users' AND tablename = 'student_document_file_cleanup'
					AND policyname = 'tenant_isolation_users_student_document_file_cleanup'
			) THEN
				CREATE POLICY tenant_isolation_users_student_document_file_cleanup
					ON users.student_document_file_cleanup
					FOR ALL
					USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
					WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);
			END IF;
		END $$;

		GRANT SELECT, INSERT, UPDATE ON users.student_documents TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE users.student_documents_id_seq TO phoenix_tenant;
		REVOKE DELETE ON users.student_documents FROM phoenix_tenant;

		GRANT SELECT, INSERT, UPDATE ON users.student_document_file_cleanup TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE users.student_document_file_cleanup_id_seq TO phoenix_tenant;
		REVOKE DELETE ON users.student_document_file_cleanup FROM phoenix_tenant;

		INSERT INTO auth.permissions (name, description, resource, action, is_system)
		VALUES
			('student_documents:health', 'Read, upload and delete child health documents (Attest, Impfnachweis, Medikamentenplan, Art. 9 GDPR health data)', 'student_documents', 'health', TRUE),
			('student_documents:legal', 'Read, upload and delete child custody documents (Sorgerechtsnachweis)', 'student_documents', 'legal', TRUE)
		ON CONFLICT (name) DO NOTHING;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed creating student documents objects: %w", err)
	}
	return nil
}

func studentDocumentsDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.271: Dropping users.student_documents...")
	if _, err := db.NewRaw(`
		DELETE FROM auth.permissions WHERE name IN ('student_documents:health', 'student_documents:legal');
		DROP TABLE IF EXISTS users.student_document_file_cleanup CASCADE;
		DROP TABLE IF EXISTS users.student_documents CASCADE;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed rolling back student documents objects: %w", err)
	}
	return nil
}
