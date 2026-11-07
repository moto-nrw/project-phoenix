package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const traegerDocumentDeletionVersion = "1.15.795"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     traegerDocumentDeletionVersion,
		Description: "Träger-Portal: Träger-Dokumente löschen (#5993)",
		DependsOn: []string{
			careContractAmendmentsVersion,   // the last migration before this one
			traegerDocumentDownloadsVersion, // the download ledger the deletion keeps
		},
	})
	Migrations.MustRegister(traegerDocumentDeletionUp, traegerDocumentDeletionDown)
}

// traegerDocumentDeletionUpSQL lets the Träger-Büro delete a shared document
// (#5993, ADR 0157):
//
//   - The deletion removes the file at once (content becomes NULL) and keeps
//     the row with deleted_at and deleted_by: who deleted which document and
//     when. Releases, reads and the download ledger name the row, so no
//     cascade removes them; each keeps its own retention. The row, title and
//     file name stay as their referent until the Träger is erased, like a
//     revoked release.
//   - A deletion happens once and changes nothing else; refuse_traeger_document_change
//     allows exactly that next to the ON DELETE SET NULL of the accounts.
//   - A deleted document takes no new release; the release trigger refuses
//     it, and the SECURITY DEFINER functions of the schools never answer for
//     it. The service revokes the active releases in the same transaction
//     and locks the document row, so a release committing at the same time
//     either is revoked or is refused.
const traegerDocumentDeletionUpSQL = `
ALTER TABLE platform.traeger_documents
	ADD COLUMN deleted_at TIMESTAMPTZ,
	ADD COLUMN deleted_by BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	ALTER COLUMN content DROP NOT NULL,
	ADD CONSTRAINT chk_traeger_documents_deleted CHECK (
		(deleted_at IS NULL AND content IS NOT NULL AND deleted_by IS NULL)
		OR (deleted_at IS NOT NULL AND content IS NULL)
	);
COMMENT ON COLUMN platform.traeger_documents.deleted_at IS
	'When the Träger-Büro deleted the document (#5993); the file is gone, the row stays as the referent of its releases, reads and downloads.';
COMMENT ON COLUMN platform.traeger_documents.deleted_by IS
	'Account that deleted the document (#5993); SET NULL when the account is deleted.';

GRANT UPDATE (content, deleted_at, deleted_by) ON platform.traeger_documents TO phoenix_tenant;

CREATE OR REPLACE FUNCTION platform.refuse_traeger_document_change() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
	-- The deletion: once, the file goes, nothing else changes.
	IF OLD.deleted_at IS NULL AND NEW.deleted_at IS NOT NULL AND NEW.content IS NULL
		AND (NEW.id, NEW.organization_id, NEW.title, NEW.file_name, NEW.content_type, NEW.size_bytes, NEW.created_at)
			IS NOT DISTINCT FROM (OLD.id, OLD.organization_id, OLD.title, OLD.file_name, OLD.content_type, OLD.size_bytes, OLD.created_at)
		AND (NEW.created_by IS NOT DISTINCT FROM OLD.created_by OR NEW.created_by IS NULL) THEN
		RETURN NEW;
	END IF;
	-- ON DELETE SET NULL of the uploading or the deleting account.
	IF (NEW.id, NEW.organization_id, NEW.title, NEW.file_name, NEW.content_type, NEW.size_bytes, NEW.content, NEW.created_at, NEW.deleted_at)
			IS NOT DISTINCT FROM (OLD.id, OLD.organization_id, OLD.title, OLD.file_name, OLD.content_type, OLD.size_bytes, OLD.content, OLD.created_at, OLD.deleted_at)
		AND (NEW.created_by IS NOT DISTINCT FROM OLD.created_by OR NEW.created_by IS NULL)
		AND (NEW.deleted_by IS NOT DISTINCT FROM OLD.deleted_by OR NEW.deleted_by IS NULL) THEN
		RETURN NEW;
	END IF;
	RAISE EXCEPTION 'a Träger document never changes; it is only deleted, once'
		USING ERRCODE = '55000';
END;
$$;

CREATE OR REPLACE FUNCTION platform.check_traeger_document_release() RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, pg_temp
AS $$
BEGIN
	IF TG_OP = 'UPDATE' THEN
		IF OLD.revoked_at IS NULL AND NEW.revoked_at IS NOT NULL
			AND (NEW.id, NEW.organization_id, NEW.document_id, NEW.audience, NEW.school_id, NEW.group_id, NEW.group_name, NEW.released_at)
				IS NOT DISTINCT FROM (OLD.id, OLD.organization_id, OLD.document_id, OLD.audience, OLD.school_id, OLD.group_id, OLD.group_name, OLD.released_at)
			AND (NEW.released_by IS NOT DISTINCT FROM OLD.released_by OR NEW.released_by IS NULL) THEN
			RETURN NEW;
		END IF;
		IF (NEW.id, NEW.organization_id, NEW.document_id, NEW.audience, NEW.school_id, NEW.group_id, NEW.group_name, NEW.released_at, NEW.revoked_at)
				IS NOT DISTINCT FROM (OLD.id, OLD.organization_id, OLD.document_id, OLD.audience, OLD.school_id, OLD.group_id, OLD.group_name, OLD.released_at, OLD.revoked_at)
			AND (NEW.released_by IS NOT DISTINCT FROM OLD.released_by OR NEW.released_by IS NULL)
			AND (NEW.revoked_by IS NOT DISTINCT FROM OLD.revoked_by OR NEW.revoked_by IS NULL) THEN
			RETURN NEW;
		END IF;
		RAISE EXCEPTION 'a release of a Träger document is only revoked, once'
			USING ERRCODE = '55000';
	END IF;
	IF NEW.revoked_at IS NOT NULL THEN
		RAISE EXCEPTION 'a release of a Träger document starts active'
			USING ERRCODE = '23514';
	END IF;
	IF EXISTS (
		SELECT 1 FROM platform.traeger_documents AS document
		WHERE document.organization_id = NEW.organization_id AND document.id = NEW.document_id
			AND document.deleted_at IS NOT NULL
	) THEN
		RAISE EXCEPTION 'a deleted Träger document is released for nobody'
			USING ERRCODE = '55000';
	END IF;
	IF NEW.school_id IS NOT NULL AND NOT EXISTS (
		SELECT 1 FROM platform.schools AS school
		WHERE school.id = NEW.school_id AND school.organization_id = NEW.organization_id
	) THEN
		RAISE EXCEPTION 'the school does not belong to the Träger of the document'
			USING ERRCODE = '23503';
	END IF;
	RETURN NEW;
END;
$$;

CREATE OR REPLACE FUNCTION platform.traeger_school_document_releases()
	RETURNS TABLE (
		document_id BIGINT, organization_id BIGINT, title TEXT, file_name TEXT, content_type VARCHAR,
		size_bytes INTEGER, uploaded_at TIMESTAMPTZ, release_id BIGINT, audience VARCHAR,
		school_id BIGINT, group_id BIGINT, group_name TEXT, released_at TIMESTAMPTZ
	)
	LANGUAGE sql
	STABLE
	SECURITY DEFINER
	SET search_path = pg_catalog, pg_temp
	AS $$
	SELECT document.id, document.organization_id, document.title, document.file_name, document.content_type,
		document.size_bytes, document.created_at, release.id, release.audience,
		release.school_id, release.group_id, release.group_name, release.released_at
	FROM platform.schools AS school
	JOIN platform.traeger_document_releases AS release
		ON release.organization_id = school.organization_id
		AND (release.school_id IS NULL OR release.school_id = school.id)
		AND release.revoked_at IS NULL
	JOIN platform.traeger_documents AS document
		ON document.organization_id = release.organization_id AND document.id = release.document_id
		AND document.deleted_at IS NULL
	WHERE school.id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint
		AND school.organization_id IS NOT NULL
	$$;
`

// traegerDocumentDeletionDownSQL restores 1.15.780. It fails while a deleted
// document exists, because its file is gone and content becomes NOT NULL.
const traegerDocumentDeletionDownSQL = `
CREATE OR REPLACE FUNCTION platform.traeger_school_document_releases()
	RETURNS TABLE (
		document_id BIGINT, organization_id BIGINT, title TEXT, file_name TEXT, content_type VARCHAR,
		size_bytes INTEGER, uploaded_at TIMESTAMPTZ, release_id BIGINT, audience VARCHAR,
		school_id BIGINT, group_id BIGINT, group_name TEXT, released_at TIMESTAMPTZ
	)
	LANGUAGE sql
	STABLE
	SECURITY DEFINER
	SET search_path = pg_catalog, pg_temp
	AS $$
	SELECT document.id, document.organization_id, document.title, document.file_name, document.content_type,
		document.size_bytes, document.created_at, release.id, release.audience,
		release.school_id, release.group_id, release.group_name, release.released_at
	FROM platform.schools AS school
	JOIN platform.traeger_document_releases AS release
		ON release.organization_id = school.organization_id
		AND (release.school_id IS NULL OR release.school_id = school.id)
		AND release.revoked_at IS NULL
	JOIN platform.traeger_documents AS document
		ON document.organization_id = release.organization_id AND document.id = release.document_id
	WHERE school.id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint
		AND school.organization_id IS NOT NULL
	$$;

CREATE OR REPLACE FUNCTION platform.check_traeger_document_release() RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, pg_temp
AS $$
BEGIN
	IF TG_OP = 'UPDATE' THEN
		IF OLD.revoked_at IS NULL AND NEW.revoked_at IS NOT NULL
			AND (NEW.id, NEW.organization_id, NEW.document_id, NEW.audience, NEW.school_id, NEW.group_id, NEW.group_name, NEW.released_at)
				IS NOT DISTINCT FROM (OLD.id, OLD.organization_id, OLD.document_id, OLD.audience, OLD.school_id, OLD.group_id, OLD.group_name, OLD.released_at)
			AND (NEW.released_by IS NOT DISTINCT FROM OLD.released_by OR NEW.released_by IS NULL) THEN
			RETURN NEW;
		END IF;
		IF (NEW.id, NEW.organization_id, NEW.document_id, NEW.audience, NEW.school_id, NEW.group_id, NEW.group_name, NEW.released_at, NEW.revoked_at)
				IS NOT DISTINCT FROM (OLD.id, OLD.organization_id, OLD.document_id, OLD.audience, OLD.school_id, OLD.group_id, OLD.group_name, OLD.released_at, OLD.revoked_at)
			AND (NEW.released_by IS NOT DISTINCT FROM OLD.released_by OR NEW.released_by IS NULL)
			AND (NEW.revoked_by IS NOT DISTINCT FROM OLD.revoked_by OR NEW.revoked_by IS NULL) THEN
			RETURN NEW;
		END IF;
		RAISE EXCEPTION 'a release of a Träger document is only revoked, once'
			USING ERRCODE = '55000';
	END IF;
	IF NEW.revoked_at IS NOT NULL THEN
		RAISE EXCEPTION 'a release of a Träger document starts active'
			USING ERRCODE = '23514';
	END IF;
	IF NEW.school_id IS NOT NULL AND NOT EXISTS (
		SELECT 1 FROM platform.schools AS school
		WHERE school.id = NEW.school_id AND school.organization_id = NEW.organization_id
	) THEN
		RAISE EXCEPTION 'the school does not belong to the Träger of the document'
			USING ERRCODE = '23503';
	END IF;
	RETURN NEW;
END;
$$;

CREATE OR REPLACE FUNCTION platform.refuse_traeger_document_change() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
	IF NEW.created_by IS NULL
		AND (NEW.id, NEW.organization_id, NEW.title, NEW.file_name, NEW.content_type, NEW.size_bytes, NEW.content, NEW.created_at)
			IS NOT DISTINCT FROM (OLD.id, OLD.organization_id, OLD.title, OLD.file_name, OLD.content_type, OLD.size_bytes, OLD.content, OLD.created_at) THEN
		RETURN NEW;
	END IF;
	RAISE EXCEPTION 'a Träger document never changes'
		USING ERRCODE = '55000';
END;
$$;

REVOKE UPDATE (content, deleted_at, deleted_by) ON platform.traeger_documents FROM phoenix_tenant;
ALTER TABLE platform.traeger_documents
	DROP CONSTRAINT IF EXISTS chk_traeger_documents_deleted,
	ALTER COLUMN content SET NOT NULL,
	DROP COLUMN IF EXISTS deleted_by,
	DROP COLUMN IF EXISTS deleted_at;
`

func traegerDocumentDeletionUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, traegerDocumentDeletionUpSQL); err != nil {
		return fmt.Errorf("add Träger document deletion: %w", err)
	}
	return nil
}

func traegerDocumentDeletionDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, traegerDocumentDeletionDownSQL); err != nil {
		return fmt.Errorf("drop Träger document deletion: %w", err)
	}
	return nil
}
