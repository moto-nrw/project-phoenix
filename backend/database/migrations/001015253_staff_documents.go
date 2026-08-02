package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	staffDocumentsVersion     = "1.15.253"
	staffDocumentsDescription = "Create users.staff_documents (Dokumente tab phase 1) and the staff_documents:health permission (#1424)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     staffDocumentsVersion,
		Description: staffDocumentsDescription,
		DependsOn: []string{
			staffStammdatenVersion,    // users.staff FK conventions + audit section shape (#1423)
			compositePKIndexesVersion, // UNIQUE users.staff(tenant_id, id) backs the composite FK
		},
	})

	Migrations.MustRegister(staffDocumentsUp, staffDocumentsDown)
}

// staffDocumentsUp creates the staff document metadata storage for issue
// #1424 (Dokumente tab phase 1).
//
// Only metadata lives in the database — the file bytes are stored on the
// filesystem under public/uploads/staff-documents/{tenant_id}/ with a UUID
// filename and are reachable exclusively through the permission-checked
// download handler (there is no static file server for backend public/).
//
// deleted_at is a soft delete on purpose: the metadata row is the audit
// trail of what existed. The file itself is removed on delete; the row
// stays. phoenix_tenant therefore gets no DELETE grant at all — deletion is
// an UPDATE setting deleted_at/deleted_by.
//
// staff_documents:health gates the AU-Bescheinigung category (Art. 9 GDPR
// special category — health data). It is catalog-only like staff:financial
// (1.15.251): school admins match via the admin:* wildcard, and a dedicated
// role can be granted the permission explicitly without another migration.
// The Lohnabrechnung category reuses the existing staff:financial
// permission; both checks live in the document service.
func staffDocumentsUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.253: Creating users.staff_documents + staff_documents:health permission...")

	if _, err := db.NewRaw(`
		CREATE TABLE IF NOT EXISTS users.staff_documents (
			id BIGSERIAL PRIMARY KEY,
			tenant_id BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			staff_id BIGINT NOT NULL,
			category TEXT NOT NULL CHECK (category IN (
				'arbeitsvertrag', 'zeugnis', 'lohnabrechnung',
				'bewerbung', 'au_bescheinigung', 'sonstiges'
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
			CONSTRAINT uq_staff_documents_stored UNIQUE (filename_stored),
			CONSTRAINT fk_staff_documents_staff
				FOREIGN KEY (tenant_id, staff_id)
				REFERENCES users.staff(tenant_id, id) ON DELETE CASCADE
		);

		CREATE INDEX IF NOT EXISTS idx_staff_documents_staff
			ON users.staff_documents (tenant_id, staff_id)
			WHERE deleted_at IS NULL;

		CREATE INDEX IF NOT EXISTS idx_staff_documents_pending_file_cleanup
			ON users.staff_documents (tenant_id, staff_id)
			WHERE deleted_at IS NOT NULL AND file_deleted_at IS NULL;

		DROP TRIGGER IF EXISTS update_staff_documents_updated_at ON users.staff_documents;
		CREATE TRIGGER update_staff_documents_updated_at
		BEFORE UPDATE ON users.staff_documents
		FOR EACH ROW EXECUTE FUNCTION update_modified_column();

		ALTER TABLE users.staff_documents ENABLE ROW LEVEL SECURITY;
		ALTER TABLE users.staff_documents FORCE ROW LEVEL SECURITY;

		DO $$
		BEGIN
			IF NOT EXISTS (
				SELECT 1 FROM pg_policies
				WHERE schemaname = 'users' AND tablename = 'staff_documents'
					AND policyname = 'tenant_isolation_users_staff_documents'
			) THEN
				CREATE POLICY tenant_isolation_users_staff_documents
					ON users.staff_documents
					FOR ALL
					USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
					WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);
			END IF;
		END $$;

		GRANT SELECT, INSERT, UPDATE ON users.staff_documents TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE users.staff_documents_id_seq TO phoenix_tenant;
		REVOKE DELETE ON users.staff_documents FROM phoenix_tenant;

		INSERT INTO auth.permissions (name, description, resource, action, is_system)
		VALUES ('staff_documents:health', 'Read, upload and delete staff sick-note documents (AU-Bescheinigungen, Art. 9 GDPR health data)', 'staff_documents', 'health', TRUE)
		ON CONFLICT (name) DO NOTHING;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed creating staff documents objects: %w", err)
	}
	return nil
}

func staffDocumentsDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.253: Dropping users.staff_documents...")
	if _, err := db.NewRaw(`
		DELETE FROM auth.permissions WHERE name = 'staff_documents:health';
		DROP TABLE IF EXISTS users.staff_documents CASCADE;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed rolling back staff documents objects: %w", err)
	}
	return nil
}
