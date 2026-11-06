package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const traegerSharedDocumentsVersion = "1.15.760"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     traegerSharedDocumentsVersion,
		Description: "Träger-Portal: vorhandene Dokumente für Eltern und Schulen freigeben (#4692)",
		DependsOn: []string{
			emailDeliveryEventsVersion,      // the last migration before this one
			traegerDocumentTemplatesVersion, // the Träger tables of ADR 0105 it follows
		},
	})
	Migrations.MustRegister(traegerSharedDocumentsUp, traegerSharedDocumentsDown)
}

// traegerSharedDocumentsUpSQL adds the documents a Träger shares with the
// parents and the schools of its schools (#4692, spec #4611 user stories 98
// and 99):
//
//   - platform.traeger_documents: a file the Träger-Büro uploads (PDF, Word,
//     Excel, PowerPoint or an image) with its title. A document never changes; the only
//     update allowed is the ON DELETE SET NULL of the account that uploaded
//     it. Like the master data it belongs to the Träger, follows the Träger
//     policy of ADR 0105 and goes with the Träger.
//   - platform.traeger_document_releases: who a document is released for.
//     The parents of all schools, of one school or of one group of a school,
//     or the schools: all or one. A release is revoked, never deleted, and
//     only revoked_at and revoked_by ever change, once. One active release
//     exists per document and audience. A school must belong to the Träger;
//     the trigger checks it, because the Träger transaction cannot read
//     platform.schools.
//   - platform.traeger_document_reads: every time a parent or a school opens
//     a document, with the account; kept three years like the master data
//     log.
//
// A school transaction reads no row of these tables. Its access comes only
// through the SECURITY DEFINER functions below: they answer for the school
// of app.current_tenant_id with the documents an active release covers, and
// nothing outside a school transaction. A revoked release ends the access at
// once. Opening a document records the read in the same statement.
const traegerSharedDocumentsUpSQL = `
CREATE TABLE platform.traeger_documents (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL REFERENCES platform.organizations(id) ON DELETE CASCADE,
	title TEXT NOT NULL,
	file_name TEXT NOT NULL,
	content_type VARCHAR(100) NOT NULL,
	size_bytes INTEGER NOT NULL,
	content BYTEA NOT NULL,
	created_by BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT uq_traeger_documents_organization UNIQUE (organization_id, id),
	CONSTRAINT chk_traeger_documents_title CHECK (BTRIM(title) <> '' AND char_length(title) <= 120),
	CONSTRAINT chk_traeger_documents_file_name CHECK (BTRIM(file_name) <> '' AND char_length(file_name) <= 255),
	CONSTRAINT chk_traeger_documents_content_type CHECK (content_type IN (
		'application/pdf', 'image/png', 'image/jpeg',
		'application/vnd.openxmlformats-officedocument.wordprocessingml.document',
		'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
		'application/vnd.openxmlformats-officedocument.presentationml.presentation'
	)),
	CONSTRAINT chk_traeger_documents_size CHECK (size_bytes > 0 AND size_bytes <= 10485760 AND octet_length(content) = size_bytes)
);
CREATE INDEX idx_traeger_documents_organization ON platform.traeger_documents (organization_id, created_at DESC);
COMMENT ON TABLE platform.traeger_documents IS
	'Files a Träger shares with parents and schools (#4692); a document never changes; read and written in the Träger transaction (ADR 0105).';

CREATE TABLE platform.traeger_document_releases (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL,
	document_id BIGINT NOT NULL,
	audience VARCHAR(16) NOT NULL,
	school_id BIGINT REFERENCES platform.schools(id) ON DELETE CASCADE,
	group_id BIGINT,
	group_name TEXT,
	released_by BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	released_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	revoked_by BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	revoked_at TIMESTAMPTZ,
	CONSTRAINT fk_traeger_document_releases_document FOREIGN KEY (organization_id, document_id)
		REFERENCES platform.traeger_documents (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT chk_traeger_document_releases_audience CHECK (audience IN ('parents', 'schools')),
	CONSTRAINT chk_traeger_document_releases_group CHECK (
		(group_id IS NULL AND group_name IS NULL)
		OR (audience = 'parents' AND school_id IS NOT NULL AND group_id IS NOT NULL
			AND BTRIM(group_name) <> '' AND char_length(group_name) <= 200)
	),
	CONSTRAINT chk_traeger_document_releases_revoked CHECK (revoked_at IS NOT NULL OR revoked_by IS NULL)
);
CREATE UNIQUE INDEX uq_traeger_document_releases_active ON platform.traeger_document_releases
	(document_id, audience, COALESCE(school_id, 0), COALESCE(group_id, 0)) WHERE revoked_at IS NULL;
CREATE INDEX idx_traeger_document_releases_document ON platform.traeger_document_releases (organization_id, document_id);
CREATE INDEX idx_traeger_document_releases_school ON platform.traeger_document_releases (school_id) WHERE revoked_at IS NULL;
COMMENT ON TABLE platform.traeger_document_releases IS
	'Releases of a Träger document for the parents (all, school, group) or the schools (all, one) (#4692); revoked, never deleted.';
COMMENT ON COLUMN platform.traeger_document_releases.group_name IS
	'Name of the group when it was released, so the Träger and the school read whom it was released for without a read of School Structure (#4692).';

CREATE FUNCTION platform.check_traeger_document_release() RETURNS trigger
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
REVOKE ALL ON FUNCTION platform.check_traeger_document_release() FROM PUBLIC;

CREATE TRIGGER check_platform_traeger_document_release
	BEFORE INSERT OR UPDATE ON platform.traeger_document_releases
	FOR EACH ROW EXECUTE FUNCTION platform.check_traeger_document_release();

CREATE FUNCTION platform.refuse_traeger_document_change() RETURNS trigger
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
REVOKE ALL ON FUNCTION platform.refuse_traeger_document_change() FROM PUBLIC;

CREATE TRIGGER refuse_platform_traeger_document_change
	BEFORE UPDATE ON platform.traeger_documents
	FOR EACH ROW EXECUTE FUNCTION platform.refuse_traeger_document_change();

CREATE TABLE platform.traeger_document_reads (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL,
	document_id BIGINT NOT NULL,
	account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	reader VARCHAR(16) NOT NULL,
	school_id BIGINT REFERENCES platform.schools(id) ON DELETE SET NULL,
	read_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT fk_traeger_document_reads_document FOREIGN KEY (organization_id, document_id)
		REFERENCES platform.traeger_documents (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT chk_traeger_document_reads_reader CHECK (reader IN ('parent', 'school'))
);
CREATE INDEX idx_traeger_document_reads_document ON platform.traeger_document_reads (organization_id, document_id, read_at DESC);
COMMENT ON TABLE platform.traeger_document_reads IS
	'Every opening of a Träger document by a parent or a school, with the account (#4692); kept three years.';

ALTER TABLE platform.traeger_documents ENABLE ROW LEVEL SECURITY;
ALTER TABLE platform.traeger_documents FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_platform_traeger_documents ON platform.traeger_documents FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);

ALTER TABLE platform.traeger_document_releases ENABLE ROW LEVEL SECURITY;
ALTER TABLE platform.traeger_document_releases FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_platform_traeger_document_releases ON platform.traeger_document_releases FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);

ALTER TABLE platform.traeger_document_reads ENABLE ROW LEVEL SECURITY;
ALTER TABLE platform.traeger_document_reads FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_platform_traeger_document_reads ON platform.traeger_document_reads FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);

REVOKE ALL ON platform.traeger_documents, platform.traeger_document_releases, platform.traeger_document_reads FROM PUBLIC, phoenix_auth;
GRANT SELECT, INSERT ON platform.traeger_documents TO phoenix_tenant;
GRANT SELECT, INSERT ON platform.traeger_document_releases TO phoenix_tenant;
GRANT UPDATE (revoked_at, revoked_by) ON platform.traeger_document_releases TO phoenix_tenant;
GRANT SELECT, INSERT, DELETE ON platform.traeger_document_reads TO phoenix_tenant;
GRANT USAGE ON SEQUENCE platform.traeger_documents_id_seq, platform.traeger_document_releases_id_seq,
	platform.traeger_document_reads_id_seq TO phoenix_tenant;
GRANT ALL ON platform.traeger_documents, platform.traeger_document_releases, platform.traeger_document_reads TO phoenix_admin;
GRANT ALL ON SEQUENCE platform.traeger_documents_id_seq, platform.traeger_document_releases_id_seq,
	platform.traeger_document_reads_id_seq TO phoenix_admin;

-- platform.traeger_school_document_releases is the active releases that
-- cover the school of app.current_tenant_id, empty outside a school
-- transaction.
CREATE FUNCTION platform.traeger_school_document_releases()
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
COMMENT ON FUNCTION platform.traeger_school_document_releases() IS
	'Active releases of Träger documents for the school of app.current_tenant_id or the parents at it (#4692); no content.';

-- platform.open_traeger_school_document returns the file of a document an
-- active release covers for the school of app.current_tenant_id and records
-- the read with the account; nothing for any other document.
CREATE FUNCTION platform.open_traeger_school_document(p_document_id BIGINT, p_account_id BIGINT)
	RETURNS TABLE (file_name TEXT, content_type VARCHAR, content BYTEA)
	LANGUAGE plpgsql
	VOLATILE
	SECURITY DEFINER
	SET search_path = pg_catalog, pg_temp
	AS $$
	DECLARE
		v_school_id BIGINT := NULLIF(current_setting('app.current_tenant_id', true), '')::bigint;
		v_organization_id BIGINT;
	BEGIN
		SELECT release.organization_id INTO v_organization_id
		FROM platform.traeger_school_document_releases() AS release
		WHERE release.document_id = p_document_id
		LIMIT 1;
		IF v_organization_id IS NULL THEN
			RETURN;
		END IF;
		INSERT INTO platform.traeger_document_reads (organization_id, document_id, account_id, reader, school_id)
		VALUES (v_organization_id, p_document_id, p_account_id, 'school', v_school_id);
		RETURN QUERY
			SELECT document.file_name, document.content_type, document.content
			FROM platform.traeger_documents AS document
			WHERE document.organization_id = v_organization_id AND document.id = p_document_id;
	END;
	$$;
COMMENT ON FUNCTION platform.open_traeger_school_document(BIGINT, BIGINT) IS
	'File of a Träger document an active release covers for the school of app.current_tenant_id; records the read (#4692).';

REVOKE ALL ON FUNCTION platform.traeger_school_document_releases() FROM PUBLIC;
REVOKE ALL ON FUNCTION platform.open_traeger_school_document(BIGINT, BIGINT) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION platform.traeger_school_document_releases() TO phoenix_tenant;
GRANT EXECUTE ON FUNCTION platform.open_traeger_school_document(BIGINT, BIGINT) TO phoenix_tenant;
`

const traegerSharedDocumentsDownSQL = `
DROP FUNCTION IF EXISTS platform.open_traeger_school_document(BIGINT, BIGINT);
DROP FUNCTION IF EXISTS platform.traeger_school_document_releases();
DROP TABLE IF EXISTS platform.traeger_document_reads;
DROP TABLE IF EXISTS platform.traeger_document_releases;
DROP TABLE IF EXISTS platform.traeger_documents;
DROP FUNCTION IF EXISTS platform.check_traeger_document_release();
DROP FUNCTION IF EXISTS platform.refuse_traeger_document_change();
`

func traegerSharedDocumentsUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, traegerSharedDocumentsUpSQL); err != nil {
		return fmt.Errorf("create Träger shared documents: %w", err)
	}
	return nil
}

func traegerSharedDocumentsDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, traegerSharedDocumentsDownSQL); err != nil {
		return fmt.Errorf("drop Träger shared documents: %w", err)
	}
	return nil
}
