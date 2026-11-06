package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const careContractSignaturesVersion = "1.15.776"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     careContractSignaturesVersion,
		Description: "Betreuungsvertrag: Unterschrift, einmaliger Mail-Link, Papiervertrag, Erinnerung (#4679)",
		DependsOn: []string{
			careContractDraftsVersion, // the tables of the contract (#4678)
		},
	})
	Migrations.MustRegister(careContractSignaturesUp, careContractSignaturesDown)
}

// careContractSignaturesUpSQL adds the signature of a care contract (#4679,
// ADR 0088 section 3, ADR 0153 section 9):
//
//   - audit.traeger_signatures: the signature ledger of the Audit Platform
//     (#4785) at the Träger. It stores the drawn or typed signature with its
//     signer, time, the SHA-256 of the signed PDF and a record hash, append
//     only, under the Träger policy. audit.signatures stays bound to a
//     school; a contract lives at the Träger (ADR 0105), so the Träger gets
//     its own ledger with the same checks. The signature goes with its
//     contract version.
//   - care_contract.contract_signatures: the conclusion of one contract
//     version. Online (drawn or typed) it points to the ledger entry; on
//     paper it holds the uploaded PDF and who recorded it. It keeps the
//     checksum of the structured version and of the PDF that was signed,
//     the way of identification (parents account, one-time mail link,
//     office), when the withdrawal notice was given and whether the parents
//     asked for care before the withdrawal period ends (§ 356, § 357a BGB).
//     One signature per version; a changed version signs again (#4816).
//   - care_contract.contract_signing_links: the one-time mail link. Only the
//     SHA-256 of the token is stored. A link names one contract and the PDF
//     it was sent with; used_at closes it for good.
//   - care_contract.contract_reminders: every reminder that a contract is
//     not signed yet, at most one per number and contract.
//
// The request role adds signatures, links and reminders and never changes a
// signature; of a link it may only set used_at. A signed contract becomes
// aktiv and its version wirksam, the transition #4680 (1.15.760) guards with
// its trigger once both are merged; the grant on the status of a version is
// the same in both.
const careContractSignaturesUpSQL = `
CREATE TABLE audit.traeger_signatures (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL REFERENCES platform.organizations(id) ON DELETE CASCADE,
	purpose TEXT NOT NULL CHECK (purpose IN ('care_contract')),
	care_contract_version_id BIGINT NOT NULL,
	signer_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	signer_name TEXT NOT NULL CHECK (btrim(signer_name) <> '' AND char_length(signer_name) <= 300),
	method TEXT NOT NULL CHECK (method IN ('drawn','typed')),
	typed_name TEXT,
	image_png BYTEA NOT NULL CHECK (octet_length(image_png) BETWEEN 1 AND 204800),
	image_sha256 TEXT NOT NULL CHECK (image_sha256 ~ '^[0-9a-f]{64}$'),
	document_sha256 TEXT NOT NULL CHECK (document_sha256 ~ '^[0-9a-f]{64}$'),
	record_hash TEXT NOT NULL CHECK (record_hash ~ '^[0-9a-f]{64}$'),
	signed_at TIMESTAMPTZ NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	CONSTRAINT uq_traeger_signatures_organization UNIQUE (organization_id, id),
	CONSTRAINT fk_traeger_signatures_contract_version FOREIGN KEY (organization_id, care_contract_version_id)
		REFERENCES care_contract.contract_versions (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT chk_traeger_signatures_typed_name CHECK ((method = 'typed') = (typed_name IS NOT NULL))
);
CREATE INDEX idx_traeger_signatures_version ON audit.traeger_signatures (organization_id, care_contract_version_id);
CREATE INDEX idx_traeger_signatures_signer_account ON audit.traeger_signatures (signer_account_id)
	WHERE signer_account_id IS NOT NULL;
COMMENT ON TABLE audit.traeger_signatures IS
	'Signature ledger of the Audit Platform at the Träger (#4785, #4679): append only, Träger policy.';

CREATE TABLE care_contract.contract_signatures (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL,
	contract_id BIGINT NOT NULL,
	contract_version_id BIGINT NOT NULL,
	contract_document_id BIGINT,
	method VARCHAR(8) NOT NULL,
	channel VARCHAR(16) NOT NULL,
	audit_signature_id BIGINT,
	signer_name TEXT NOT NULL,
	signer_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	signing_link_id BIGINT,
	version_sha256 TEXT NOT NULL,
	document_sha256 TEXT NOT NULL,
	withdrawal_notice_at TIMESTAMPTZ,
	early_start_requested BOOLEAN NOT NULL DEFAULT false,
	paper_filename TEXT,
	paper_pdf BYTEA,
	paper_pdf_sha256 TEXT GENERATED ALWAYS AS (CASE WHEN paper_pdf IS NULL THEN NULL ELSE encode(sha256(paper_pdf), 'hex') END) STORED,
	signed_on DATE NOT NULL,
	signed_at TIMESTAMPTZ NOT NULL,
	recorded_by_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT fk_contract_signatures_contract FOREIGN KEY (organization_id, contract_id)
		REFERENCES care_contract.contracts (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT fk_contract_signatures_version FOREIGN KEY (organization_id, contract_version_id)
		REFERENCES care_contract.contract_versions (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT fk_contract_signatures_document FOREIGN KEY (organization_id, contract_document_id)
		REFERENCES care_contract.contract_documents (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT fk_contract_signatures_audit FOREIGN KEY (organization_id, audit_signature_id)
		REFERENCES audit.traeger_signatures (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT uq_contract_signatures_version UNIQUE (contract_version_id),
	CONSTRAINT chk_contract_signatures_method CHECK (method IN ('drawn', 'typed', 'paper')),
	CONSTRAINT chk_contract_signatures_channel CHECK (channel IN ('parent_account', 'mail_link', 'office')),
	-- Online the ledger holds the signature and the PDF it was given on;
	-- on paper the upload is the proof and the office recorded it.
	CONSTRAINT chk_contract_signatures_proof CHECK (
		(method IN ('drawn', 'typed') AND channel IN ('parent_account', 'mail_link') AND audit_signature_id IS NOT NULL
			AND contract_document_id IS NOT NULL AND paper_pdf IS NULL AND withdrawal_notice_at IS NOT NULL)
		OR (method = 'paper' AND channel = 'office' AND audit_signature_id IS NULL AND paper_pdf IS NOT NULL
			AND recorded_by_account_id IS NOT NULL)),
	CONSTRAINT chk_contract_signatures_link CHECK ((channel = 'mail_link') = (signing_link_id IS NOT NULL)),
	CONSTRAINT chk_contract_signatures_hashes CHECK (version_sha256 ~ '^[0-9a-f]{64}$' AND document_sha256 ~ '^[0-9a-f]{64}$'),
	CONSTRAINT chk_contract_signatures_signer CHECK (BTRIM(signer_name) <> '' AND char_length(signer_name) <= 300),
	CONSTRAINT chk_contract_signatures_paper CHECK (paper_pdf IS NULL OR (octet_length(paper_pdf) BETWEEN 5 AND 10485760
		AND paper_filename IS NOT NULL AND char_length(paper_filename) <= 255))
);
CREATE INDEX idx_contract_signatures_contract ON care_contract.contract_signatures (organization_id, contract_id);
COMMENT ON TABLE care_contract.contract_signatures IS
	'Conclusion of a care contract version: online signature or paper contract (#4679, ADR 0088 section 3); never changed.';

CREATE TABLE care_contract.contract_signing_links (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL,
	contract_id BIGINT NOT NULL,
	contract_document_id BIGINT NOT NULL,
	token_sha256 TEXT NOT NULL,
	expires_at TIMESTAMPTZ NOT NULL,
	used_at TIMESTAMPTZ,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT fk_contract_signing_links_contract FOREIGN KEY (organization_id, contract_id)
		REFERENCES care_contract.contracts (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT fk_contract_signing_links_document FOREIGN KEY (organization_id, contract_document_id)
		REFERENCES care_contract.contract_documents (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT uq_contract_signing_links_token UNIQUE (token_sha256),
	CONSTRAINT uq_contract_signing_links_organization UNIQUE (organization_id, id),
	CONSTRAINT chk_contract_signing_links_token CHECK (token_sha256 ~ '^[0-9a-f]{64}$'),
	CONSTRAINT chk_contract_signing_links_expiry CHECK (expires_at > created_at)
);
CREATE INDEX idx_contract_signing_links_contract ON care_contract.contract_signing_links (organization_id, contract_id);
COMMENT ON TABLE care_contract.contract_signing_links IS
	'One-time mail link to read and sign one care contract (#4679); stores only the SHA-256 of the token.';

ALTER TABLE care_contract.contract_signatures ADD CONSTRAINT fk_contract_signatures_link
	FOREIGN KEY (organization_id, signing_link_id)
	REFERENCES care_contract.contract_signing_links (organization_id, id) ON DELETE CASCADE;

CREATE TABLE care_contract.contract_reminders (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL,
	contract_id BIGINT NOT NULL,
	reminder_number SMALLINT NOT NULL,
	outbox_id BIGINT NOT NULL,
	recipient_email TEXT NOT NULL,
	sent_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT fk_contract_reminders_contract FOREIGN KEY (organization_id, contract_id)
		REFERENCES care_contract.contracts (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT uq_contract_reminders_number UNIQUE (contract_id, reminder_number),
	CONSTRAINT chk_contract_reminders_number CHECK (reminder_number BETWEEN 1 AND 10),
	CONSTRAINT chk_contract_reminders_recipient CHECK (BTRIM(recipient_email) <> '')
);
COMMENT ON TABLE care_contract.contract_reminders IS
	'Reminder mails of a care contract that is not signed yet (#4679).';

ALTER TABLE care_contract.contract_events DROP CONSTRAINT chk_contract_events_action;
ALTER TABLE care_contract.contract_events ADD CONSTRAINT chk_contract_events_action
	CHECK (action IN ('read', 'create', 'send', 'discard', 'template', 'conclude', 'sign', 'remind'));

ALTER TABLE audit.traeger_signatures ENABLE ROW LEVEL SECURITY;
ALTER TABLE audit.traeger_signatures FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_audit_traeger_signatures ON audit.traeger_signatures FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
ALTER TABLE care_contract.contract_signatures ENABLE ROW LEVEL SECURITY;
ALTER TABLE care_contract.contract_signatures FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_care_contract_contract_signatures ON care_contract.contract_signatures FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
ALTER TABLE care_contract.contract_signing_links ENABLE ROW LEVEL SECURITY;
ALTER TABLE care_contract.contract_signing_links FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_care_contract_contract_signing_links ON care_contract.contract_signing_links FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
ALTER TABLE care_contract.contract_reminders ENABLE ROW LEVEL SECURITY;
ALTER TABLE care_contract.contract_reminders FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_care_contract_contract_reminders ON care_contract.contract_reminders FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);

REVOKE ALL ON audit.traeger_signatures, care_contract.contract_signatures, care_contract.contract_signing_links,
	care_contract.contract_reminders FROM PUBLIC;
GRANT SELECT, INSERT ON audit.traeger_signatures, care_contract.contract_signatures, care_contract.contract_signing_links,
	care_contract.contract_reminders TO phoenix_tenant;
GRANT UPDATE (used_at) ON care_contract.contract_signing_links TO phoenix_tenant;
GRANT UPDATE (status) ON care_contract.contract_versions TO phoenix_tenant;
GRANT USAGE ON SEQUENCE audit.traeger_signatures_id_seq, care_contract.contract_signatures_id_seq,
	care_contract.contract_signing_links_id_seq, care_contract.contract_reminders_id_seq TO phoenix_tenant;
GRANT ALL ON audit.traeger_signatures, care_contract.contract_signatures, care_contract.contract_signing_links,
	care_contract.contract_reminders TO phoenix_admin;
GRANT ALL ON SEQUENCE audit.traeger_signatures_id_seq, care_contract.contract_signatures_id_seq,
	care_contract.contract_signing_links_id_seq, care_contract.contract_reminders_id_seq TO phoenix_admin;
`

const careContractSignaturesDownSQL = `
DROP TABLE IF EXISTS care_contract.contract_reminders;
DROP TABLE IF EXISTS care_contract.contract_signatures;
DROP TABLE IF EXISTS care_contract.contract_signing_links;
DROP TABLE IF EXISTS audit.traeger_signatures;
DELETE FROM care_contract.contract_events WHERE action IN ('sign', 'remind');
ALTER TABLE care_contract.contract_events DROP CONSTRAINT IF EXISTS chk_contract_events_action;
ALTER TABLE care_contract.contract_events ADD CONSTRAINT chk_contract_events_action
	CHECK (action IN ('read', 'create', 'send', 'discard', 'template', 'conclude'));
`

func careContractSignaturesUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, careContractSignaturesUpSQL); err != nil {
		return fmt.Errorf("create care contract signatures: %w", err)
	}
	return nil
}

func careContractSignaturesDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, careContractSignaturesDownSQL); err != nil {
		return fmt.Errorf("drop care contract signatures: %w", err)
	}
	return nil
}
