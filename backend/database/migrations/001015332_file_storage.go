package migrations

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/uptrace/bun"
)

const (
	fileStorageVersion     = "1.15.332"
	fileStorageDescription = "Create the documents schema (folders, files, visibility, cleanup intents), audit.file_events and the files:manage permission (#2596)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     fileStorageVersion,
		Description: fileStorageDescription,
		DependsOn: []string{
			"1.15.1",                     // RLS infrastructure
			createTenantRolesVersion,     // phoenix_tenant / phoenix_admin roles
			auditAppendOnlyGrantsVersion, // audit schema is append-only for phoenix_tenant
		},
	})

	Migrations.MustRegister(fileStorageUp, fileStorageDown)
}

// fileStorageUp creates the school-wide file storage (issue #2596).
//
// Only metadata lives in the database. The bytes go through the storage
// backend under files/{tenant_id}/{uuid} and are reachable exclusively
// through the visibility-checked download handler.
//
// Visibility is a property of the FOLDER, never of a file. A folder is either
// open to every account with an active mapping to the school (all_staff),
// restricted to admins (admins), or shared with an explicit set of roles and
// accounts (selected, resolved through documents.folder_roles and
// documents.folder_accounts). Files inherit their folder's visibility. Keeping
// the rule on one level is deliberate: per-file rights are what turns a shared
// drive into a support queue ("why can't I see X?").
//
// documents.files reuses the shared document row shape (models/documents), so
// the cleanup-intent protocol, the soft delete and the scheduler sweep are the
// same code that already guards child and staff documents. The composite
// foreign key (tenant_id, folder_id) cascades: deleting a folder removes its
// file rows, and the cleanup intents queued beforehand (no FK on purpose)
// outlive that cascade so the bytes are still reclaimed.
//
// files:manage is catalog-only like the document permissions: school admins
// match through the admin:* wildcard, and a dedicated role can be granted it
// explicitly without another migration. Reading is not a permission at all,
// it is the folder visibility.
func fileStorageUp(ctx context.Context, db *bun.DB) error {
	slog.Info("migration 1.15.332: creating documents schema and file storage tables")

	if _, err := db.NewRaw(`CREATE SCHEMA IF NOT EXISTS documents;`).Exec(ctx); err != nil {
		return fmt.Errorf("failed creating documents schema: %w", err)
	}

	// Same schema-level grant shape as display (1.15.175) and enrollment
	// (1.15.59). The table-level REVOKE below narrows documents.files back to
	// soft delete only.
	if _, err := db.NewRaw(`
		GRANT USAGE ON SCHEMA documents TO phoenix_tenant, phoenix_admin;
		ALTER DEFAULT PRIVILEGES IN SCHEMA documents
			GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO phoenix_tenant;
		ALTER DEFAULT PRIVILEGES IN SCHEMA documents
			GRANT USAGE ON SEQUENCES TO phoenix_tenant;
		ALTER DEFAULT PRIVILEGES IN SCHEMA documents
			GRANT ALL ON TABLES TO phoenix_admin;
		ALTER DEFAULT PRIVILEGES IN SCHEMA documents
			GRANT ALL ON SEQUENCES TO phoenix_admin;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed granting permissions on documents schema: %w", err)
	}

	if _, err := db.NewRaw(`
		CREATE TABLE IF NOT EXISTS documents.folders (
			id BIGSERIAL PRIMARY KEY,
			tenant_id BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			name TEXT NOT NULL CHECK (char_length(btrim(name)) BETWEEN 1 AND 120),
			visibility TEXT NOT NULL CHECK (visibility IN ('all_staff', 'admins', 'selected')),
			created_by BIGINT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT uq_documents_folders_tenant_id UNIQUE (tenant_id, id),
			CONSTRAINT uq_documents_folders_name UNIQUE (tenant_id, name)
		);

		CREATE TABLE IF NOT EXISTS documents.folder_roles (
			tenant_id BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			folder_id BIGINT NOT NULL,
			role_id BIGINT NOT NULL REFERENCES auth.roles(id) ON DELETE CASCADE,
			PRIMARY KEY (tenant_id, folder_id, role_id),
			CONSTRAINT fk_documents_folder_roles_folder
				FOREIGN KEY (tenant_id, folder_id)
				REFERENCES documents.folders(tenant_id, id) ON DELETE CASCADE
		);

		CREATE TABLE IF NOT EXISTS documents.folder_accounts (
			tenant_id BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			folder_id BIGINT NOT NULL,
			account_id BIGINT NOT NULL REFERENCES auth.accounts(id) ON DELETE CASCADE,
			PRIMARY KEY (tenant_id, folder_id, account_id),
			CONSTRAINT fk_documents_folder_accounts_folder
				FOREIGN KEY (tenant_id, folder_id)
				REFERENCES documents.folders(tenant_id, id) ON DELETE CASCADE
		);

		CREATE TABLE IF NOT EXISTS documents.files (
			id BIGSERIAL PRIMARY KEY,
			tenant_id BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			folder_id BIGINT NOT NULL,
			category TEXT NOT NULL DEFAULT 'file' CHECK (category = 'file'),
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
			CONSTRAINT uq_documents_files_stored UNIQUE (filename_stored),
			CONSTRAINT fk_documents_files_folder
				FOREIGN KEY (tenant_id, folder_id)
				REFERENCES documents.folders(tenant_id, id) ON DELETE CASCADE
		);

		CREATE INDEX IF NOT EXISTS idx_documents_files_folder
			ON documents.files (tenant_id, folder_id)
			WHERE deleted_at IS NULL;

		CREATE INDEX IF NOT EXISTS idx_documents_files_pending_file_cleanup
			ON documents.files (tenant_id, folder_id)
			WHERE deleted_at IS NOT NULL AND file_deleted_at IS NULL;

		CREATE TABLE IF NOT EXISTS documents.file_cleanup (
			id BIGSERIAL PRIMARY KEY,
			tenant_id BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			owner_id BIGINT NOT NULL,
			filename_stored TEXT NOT NULL,
			retry_after TIMESTAMPTZ NOT NULL DEFAULT NOW() + INTERVAL '5 minutes',
			cleaned_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT uq_documents_file_cleanup UNIQUE (tenant_id, filename_stored)
		);

		CREATE INDEX IF NOT EXISTS idx_documents_file_cleanup_pending
			ON documents.file_cleanup (tenant_id, retry_after, owner_id)
			WHERE cleaned_at IS NULL;

		DROP TRIGGER IF EXISTS update_documents_folders_updated_at ON documents.folders;
		CREATE TRIGGER update_documents_folders_updated_at
		BEFORE UPDATE ON documents.folders
		FOR EACH ROW EXECUTE FUNCTION update_modified_column();

		DROP TRIGGER IF EXISTS update_documents_files_updated_at ON documents.files;
		CREATE TRIGGER update_documents_files_updated_at
		BEFORE UPDATE ON documents.files
		FOR EACH ROW EXECUTE FUNCTION update_modified_column();

		DROP TRIGGER IF EXISTS update_documents_file_cleanup_updated_at ON documents.file_cleanup;
		CREATE TRIGGER update_documents_file_cleanup_updated_at
		BEFORE UPDATE ON documents.file_cleanup
		FOR EACH ROW EXECUTE FUNCTION update_modified_column();

		-- Soft delete only: deleting a file is an UPDATE stamping deleted_at.
		-- Rows removed through the folder cascade need no privilege.
		REVOKE DELETE ON documents.files FROM phoenix_tenant;
		REVOKE DELETE ON documents.file_cleanup FROM phoenix_tenant;

		-- Append-only trail of what happened in the storage: uploads,
		-- deletions, folder changes. The audit schema's default ACL is closed
		-- (1.15.225), so the grant is explicit and stays at SELECT + INSERT.
		CREATE TABLE IF NOT EXISTS audit.file_events (
			id BIGSERIAL PRIMARY KEY,
			tenant_id BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			folder_id BIGINT,
			file_id BIGINT,
			action TEXT NOT NULL CHECK (action IN (
				'folder_created', 'folder_updated', 'folder_deleted',
				'file_uploaded', 'file_deleted'
			)),
			actor_account_id BIGINT,
			actor_name TEXT NOT NULL,
			detail TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);

		CREATE INDEX IF NOT EXISTS idx_audit_file_events_tenant_created
			ON audit.file_events (tenant_id, created_at DESC);

		GRANT SELECT, INSERT ON audit.file_events TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE audit.file_events_id_seq TO phoenix_tenant;
		GRANT ALL ON audit.file_events TO phoenix_admin;
		GRANT ALL ON SEQUENCE audit.file_events_id_seq TO phoenix_admin;

		INSERT INTO auth.permissions (name, description, resource, action, is_system)
		VALUES
			('files:manage', 'Manage the school file storage: create folders, set their visibility, upload and delete any file', 'files', 'manage', TRUE)
		ON CONFLICT (name) DO NOTHING;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed creating file storage objects: %w", err)
	}

	if err := provisionTenantRLS(ctx, db,
		"documents.folders",
		"documents.folder_roles",
		"documents.folder_accounts",
		"documents.files",
		"documents.file_cleanup",
		"audit.file_events",
	); err != nil {
		return fmt.Errorf("failed provisioning RLS for file storage: %w", err)
	}
	return nil
}

func fileStorageDown(ctx context.Context, db *bun.DB) error {
	slog.Info("migration 1.15.332: rolling back, dropping documents schema")
	if _, err := db.NewRaw(`
		DELETE FROM auth.permissions WHERE name = 'files:manage';
		DROP TABLE IF EXISTS audit.file_events CASCADE;
		DROP TABLE IF EXISTS documents.file_cleanup CASCADE;
		DROP TABLE IF EXISTS documents.files CASCADE;
		DROP TABLE IF EXISTS documents.folder_accounts CASCADE;
		DROP TABLE IF EXISTS documents.folder_roles CASCADE;
		DROP TABLE IF EXISTS documents.folders CASCADE;
		DROP SCHEMA IF EXISTS documents CASCADE;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed rolling back file storage objects: %w", err)
	}
	return nil
}
