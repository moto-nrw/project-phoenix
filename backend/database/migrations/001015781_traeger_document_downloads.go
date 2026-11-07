package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const traegerDocumentDownloadsVersion = "1.15.781"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     traegerDocumentDownloadsVersion,
		Description: "Träger-Portal: Downloads der Träger-Dokumente protokollieren (#4692)",
		DependsOn: []string{
			traegerSharedDocumentsVersion, // the documents it records
		},
	})
	Migrations.MustRegister(traegerDocumentDownloadsUp, traegerDocumentDownloadsDown)
}

// traegerDocumentDownloadsUpSQL adds audit.traeger_document_downloads, the
// one place of every download of a shared document by the Träger-Büro
// (#4692, ADR 0157): actor, document, time and the client's address and
// browser, never the title or the file name. Like audit.traeger_account_changes
// (Audit Platform) it carries the Träger and no school, because a document
// belongs to the Träger, released or not, with or without schools.
//
// The download writes the entry in the Träger transaction that reads the file,
// so a failed entry fails the download. The Träger policy of ADR 0105 keeps
// each Träger to its own rows; only the hourly retention deletes, with the
// cutoff of the Träger account ledger, and every row goes with the Träger.
const traegerDocumentDownloadsUpSQL = `
CREATE TABLE audit.traeger_document_downloads (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL REFERENCES platform.organizations(id) ON DELETE CASCADE,
	document_id BIGINT NOT NULL,
	actor_account_id BIGINT NOT NULL REFERENCES auth.accounts(id) ON DELETE CASCADE,
	client_ip TEXT,
	user_agent TEXT,
	downloaded_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT fk_traeger_document_downloads_document FOREIGN KEY (organization_id, document_id)
		REFERENCES platform.traeger_documents (organization_id, id) ON DELETE CASCADE
);
CREATE INDEX idx_traeger_document_downloads_organization ON audit.traeger_document_downloads (organization_id, downloaded_at);
CREATE INDEX idx_traeger_document_downloads_document ON audit.traeger_document_downloads (organization_id, document_id);
COMMENT ON TABLE audit.traeger_document_downloads IS
	'Every download of a shared Träger document by the Träger-Büro (#4692, ADR 0157): actor, document, time and client, no title or file name.';

ALTER TABLE audit.traeger_document_downloads ENABLE ROW LEVEL SECURITY;
ALTER TABLE audit.traeger_document_downloads FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_audit_traeger_document_downloads ON audit.traeger_document_downloads FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);

REVOKE ALL ON audit.traeger_document_downloads FROM PUBLIC, phoenix_auth, phoenix_tenant, phoenix_admin;
GRANT SELECT, INSERT ON audit.traeger_document_downloads TO phoenix_tenant;
GRANT USAGE ON SEQUENCE audit.traeger_document_downloads_id_seq TO phoenix_tenant;
GRANT SELECT, INSERT, DELETE ON audit.traeger_document_downloads TO phoenix_admin;
GRANT USAGE ON SEQUENCE audit.traeger_document_downloads_id_seq TO phoenix_admin;
`

const traegerDocumentDownloadsDownSQL = `
DROP TABLE IF EXISTS audit.traeger_document_downloads;
`

func traegerDocumentDownloadsUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, traegerDocumentDownloadsUpSQL); err != nil {
		return fmt.Errorf("create Träger document downloads: %w", err)
	}
	return nil
}

func traegerDocumentDownloadsDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, traegerDocumentDownloadsDownSQL); err != nil {
		return fmt.Errorf("drop Träger document downloads: %w", err)
	}
	return nil
}
