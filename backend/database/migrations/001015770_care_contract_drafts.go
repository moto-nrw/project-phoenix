package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const careContractDraftsVersion = "1.15.770"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     careContractDraftsVersion,
		Description: "Betreuungsvertrag: Vorlagen je Angebot, Verträge aus der Anmeldung, Versand (#4678)",
		DependsOn: []string{
			emailDeliveryEventsVersion, // the last migration before this one
		},
	})
	Migrations.MustRegister(careContractDraftsUp, careContractDraftsDown)
}

// careContractDraftsUpSQL adds the first tables of the owner Betreuungsvertrag
// (care-contract, ADR 0088, ADR 0153, #4678):
//
//   - contract_templates: the contract template of a Träger for one care
//     offering of one of its schools, with term, notice period, special
//     termination, statute confirmation and the contracting party.
//   - contracts: one contract per child of an accepted application (the
//     key of ADR 0088 section 6), at one place of care, with its status. It
//     covers every care offering (module) the child books at that school.
//     The status names every state of ADR 0088 section 2; this
//     migration's code only reaches entwurf and versendet. Only a draft may
//     be discarded by the request role.
//   - contract_versions: the frozen copy of child, monthly price, terms and
//     effective date. The request role adds versions, never changes them.
//   - contract_version_services: the booked services of a version, one row
//     per weekday and module with the module's end time on that day. A
//     weekday without a row is a day without care. last_day is the last
//     booked day of a module that ends before the version does, so the
//     contract never promises a module beyond its booking. The tariffs of #4684
//     price each weekday and module from these rows.
//   - contract_parties: the parents who are party to the contract, with an
//     optional parents' account.
//   - contract_documents: the PDF of a version, rendered once at the first
//     send and never changed.
//   - contract_mailings: every send of a document, with the outbox entry.
//   - contract_events: the trail of every read and write of the Träger-Büro
//     and of the parents' inbox. It keeps the contract ID without a foreign
//     key, so a discarded draft keeps its trail.
//
// No table references the school's application, child or offering by a
// foreign key: the contract outlives them (ADR 0088 section 9) and goes with
// the Träger. Every table carries organization_id and the Träger policy of
// ADR 0105.
const careContractDraftsUpSQL = `
CREATE SCHEMA IF NOT EXISTS care_contract;
GRANT USAGE ON SCHEMA care_contract TO phoenix_tenant, phoenix_admin;

CREATE TABLE care_contract.contract_templates (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL REFERENCES platform.organizations(id) ON DELETE CASCADE,
	school_id BIGINT NOT NULL,
	care_offering_id BIGINT NOT NULL,
	term_kind VARCHAR(16) NOT NULL,
	term_months SMALLINT,
	notice_months SMALLINT NOT NULL,
	notice_to VARCHAR(16) NOT NULL,
	special_termination BOOLEAN NOT NULL DEFAULT false,
	special_termination_note TEXT NOT NULL DEFAULT '',
	statute_required BOOLEAN NOT NULL DEFAULT false,
	statute_title TEXT NOT NULL DEFAULT '',
	statute_url TEXT NOT NULL DEFAULT '',
	contract_partner TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_by_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	CONSTRAINT uq_contract_templates_offering UNIQUE (organization_id, school_id, care_offering_id),
	CONSTRAINT uq_contract_templates_organization UNIQUE (organization_id, id),
	CONSTRAINT chk_contract_templates_term CHECK (
		(term_kind IN ('open_ended', 'school_year') AND term_months IS NULL)
		OR (term_kind = 'fixed_months' AND term_months BETWEEN 1 AND 48)),
	CONSTRAINT chk_contract_templates_notice CHECK (notice_months BETWEEN 0 AND 12
		AND notice_to IN ('month_end', 'half_year', 'school_year_end')),
	CONSTRAINT chk_contract_templates_special CHECK (char_length(special_termination_note) <= 1000
		AND (NOT special_termination OR BTRIM(special_termination_note) <> '')),
	CONSTRAINT chk_contract_templates_statute CHECK (char_length(statute_title) <= 200
		AND char_length(statute_url) <= 500 AND (NOT statute_required OR BTRIM(statute_title) <> '')),
	CONSTRAINT chk_contract_templates_partner CHECK (char_length(contract_partner) <= 300)
);
COMMENT ON TABLE care_contract.contract_templates IS
	'Contract template of a Träger for one care offering of one of its schools (#4678, ADR 0088, ADR 0153).';

CREATE TABLE care_contract.contracts (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL REFERENCES platform.organizations(id) ON DELETE CASCADE,
	school_id BIGINT NOT NULL,
	request_child_id BIGINT NOT NULL,
	student_id BIGINT,
	status VARCHAR(16) NOT NULL DEFAULT 'entwurf',
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	created_by_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	sent_at TIMESTAMPTZ,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT uq_contracts_application UNIQUE (school_id, request_child_id),
	CONSTRAINT uq_contracts_organization UNIQUE (organization_id, id),
	CONSTRAINT chk_contracts_status CHECK (status IN ('entwurf', 'versendet', 'unterschrieben', 'aktiv', 'beendet')),
	CONSTRAINT chk_contracts_sent CHECK (status = 'entwurf' OR sent_at IS NOT NULL)
);
CREATE INDEX idx_contracts_organization_status ON care_contract.contracts (organization_id, status, school_id);
COMMENT ON TABLE care_contract.contracts IS
	'Care contract of a Träger from the accepted application of a child (#4678, ADR 0088, ADR 0153).';

CREATE TABLE care_contract.contract_versions (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL,
	contract_id BIGINT NOT NULL,
	version_number SMALLINT NOT NULL,
	status VARCHAR(16) NOT NULL DEFAULT 'beantragt',
	effective_from DATE NOT NULL,
	ends_on DATE,
	child_first_name TEXT NOT NULL,
	child_last_name TEXT NOT NULL,
	child_birth_date DATE,
	school_name TEXT NOT NULL,
	phase_name TEXT NOT NULL DEFAULT '',
	price_cents BIGINT,
	term_kind VARCHAR(16) NOT NULL,
	term_months SMALLINT,
	notice_months SMALLINT NOT NULL,
	notice_to VARCHAR(16) NOT NULL,
	special_termination BOOLEAN NOT NULL,
	special_termination_note TEXT NOT NULL,
	statute_required BOOLEAN NOT NULL,
	statute_title TEXT NOT NULL,
	statute_url TEXT NOT NULL,
	contract_partner TEXT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT fk_contract_versions_contract FOREIGN KEY (organization_id, contract_id)
		REFERENCES care_contract.contracts (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT uq_contract_versions_number UNIQUE (contract_id, version_number),
	CONSTRAINT uq_contract_versions_organization UNIQUE (organization_id, id),
	CONSTRAINT chk_contract_versions_number CHECK (version_number >= 1),
	CONSTRAINT chk_contract_versions_status CHECK (status IN ('beantragt', 'wirksam', 'abgeloest', 'abgelehnt')),
	CONSTRAINT chk_contract_versions_ends CHECK (ends_on IS NULL OR ends_on >= effective_from),
	CONSTRAINT chk_contract_versions_price CHECK (price_cents IS NULL OR price_cents >= 0),
	CONSTRAINT chk_contract_versions_term CHECK (term_kind IN ('open_ended', 'school_year', 'fixed_months'))
);
COMMENT ON TABLE care_contract.contract_versions IS
	'Frozen version of a care contract with its effective date (#4678, ADR 0088, ADR 0153); never changed by the request role.';

CREATE TABLE care_contract.contract_version_services (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL,
	contract_version_id BIGINT NOT NULL,
	weekday VARCHAR(3) NOT NULL,
	care_offering_id BIGINT NOT NULL,
	module_name TEXT NOT NULL,
	ends_at TIME,
	last_day DATE,
	CONSTRAINT fk_contract_version_services_version FOREIGN KEY (organization_id, contract_version_id)
		REFERENCES care_contract.contract_versions (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT uq_contract_version_services_day UNIQUE (contract_version_id, weekday, care_offering_id),
	CONSTRAINT chk_contract_version_services_weekday CHECK (weekday IN ('mon', 'tue', 'wed', 'thu', 'fri', 'sat', 'sun')),
	CONSTRAINT chk_contract_version_services_module CHECK (BTRIM(module_name) <> '')
);
COMMENT ON TABLE care_contract.contract_version_services IS
	'Booked services of a contract version: per weekday the module and its end time (#4678, ADR 0153); a weekday without a row has no care.';

CREATE TABLE care_contract.contract_parties (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL,
	contract_id BIGINT NOT NULL,
	first_name TEXT NOT NULL,
	last_name TEXT NOT NULL,
	email TEXT NOT NULL DEFAULT '',
	account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT fk_contract_parties_contract FOREIGN KEY (organization_id, contract_id)
		REFERENCES care_contract.contracts (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT chk_contract_parties_email CHECK (char_length(email) <= 320)
);
CREATE INDEX idx_contract_parties_contract ON care_contract.contract_parties (contract_id);
CREATE INDEX idx_contract_parties_email ON care_contract.contract_parties (organization_id, lower(email));
COMMENT ON TABLE care_contract.contract_parties IS
	'Parents who are party to a care contract, with an optional parents account (#4678, ADR 0088 section 2).';

CREATE TABLE care_contract.contract_documents (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL,
	contract_id BIGINT NOT NULL,
	contract_version_id BIGINT NOT NULL,
	template_version INT NOT NULL,
	filename TEXT NOT NULL,
	pdf BYTEA NOT NULL,
	pdf_sha256 TEXT GENERATED ALWAYS AS (encode(sha256(pdf), 'hex')) STORED,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	created_by_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	CONSTRAINT fk_contract_documents_contract FOREIGN KEY (organization_id, contract_id)
		REFERENCES care_contract.contracts (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT fk_contract_documents_version FOREIGN KEY (organization_id, contract_version_id)
		REFERENCES care_contract.contract_versions (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT uq_contract_documents_version UNIQUE (contract_version_id),
	CONSTRAINT uq_contract_documents_organization UNIQUE (organization_id, id),
	CONSTRAINT chk_contract_documents_pdf CHECK (octet_length(pdf) > 0)
);
COMMENT ON TABLE care_contract.contract_documents IS
	'PDF of a contract version, rendered once at its first send and never changed (#4678, ADR 0153).';

CREATE TABLE care_contract.contract_mailings (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL,
	contract_id BIGINT NOT NULL,
	contract_document_id BIGINT NOT NULL,
	outbox_id BIGINT NOT NULL,
	recipient_email TEXT NOT NULL,
	sent_by_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	sent_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT fk_contract_mailings_contract FOREIGN KEY (organization_id, contract_id)
		REFERENCES care_contract.contracts (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT fk_contract_mailings_document FOREIGN KEY (organization_id, contract_document_id)
		REFERENCES care_contract.contract_documents (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT uq_contract_mailings_outbox UNIQUE (outbox_id),
	CONSTRAINT chk_contract_mailings_recipient CHECK (BTRIM(recipient_email) <> '')
);
CREATE INDEX idx_contract_mailings_contract ON care_contract.contract_mailings (contract_id, sent_at DESC);
CREATE INDEX idx_contract_mailings_recipient ON care_contract.contract_mailings (organization_id, lower(recipient_email));
COMMENT ON TABLE care_contract.contract_mailings IS
	'Every send of a contract document by mail and into the parents inbox (#4678, ADR 0153).';

CREATE TABLE care_contract.contract_events (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL REFERENCES platform.organizations(id) ON DELETE CASCADE,
	contract_id BIGINT,
	school_id BIGINT,
	account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	action VARCHAR(16) NOT NULL,
	details JSONB NOT NULL DEFAULT '{}'::jsonb,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT chk_contract_events_action CHECK (action IN ('read', 'create', 'send', 'discard', 'template'))
);
CREATE INDEX idx_contract_events_contract ON care_contract.contract_events (organization_id, contract_id, created_at DESC);
COMMENT ON TABLE care_contract.contract_events IS
	'Trail of every read and write of care contracts and templates (#4678, ADR 0153); keeps a discarded draft''s trail.';

CREATE TRIGGER update_care_contract_templates_updated_at
	BEFORE UPDATE ON care_contract.contract_templates
	FOR EACH ROW EXECUTE FUNCTION update_modified_column();
CREATE TRIGGER update_care_contract_contracts_updated_at
	BEFORE UPDATE ON care_contract.contracts
	FOR EACH ROW EXECUTE FUNCTION update_modified_column();

ALTER TABLE care_contract.contract_templates ENABLE ROW LEVEL SECURITY;
ALTER TABLE care_contract.contract_templates FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_care_contract_contract_templates ON care_contract.contract_templates FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
ALTER TABLE care_contract.contract_versions ENABLE ROW LEVEL SECURITY;
ALTER TABLE care_contract.contract_versions FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_care_contract_contract_versions ON care_contract.contract_versions FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
ALTER TABLE care_contract.contract_version_services ENABLE ROW LEVEL SECURITY;
ALTER TABLE care_contract.contract_version_services FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_care_contract_contract_version_services ON care_contract.contract_version_services FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
ALTER TABLE care_contract.contract_parties ENABLE ROW LEVEL SECURITY;
ALTER TABLE care_contract.contract_parties FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_care_contract_contract_parties ON care_contract.contract_parties FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
ALTER TABLE care_contract.contract_documents ENABLE ROW LEVEL SECURITY;
ALTER TABLE care_contract.contract_documents FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_care_contract_contract_documents ON care_contract.contract_documents FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
ALTER TABLE care_contract.contract_mailings ENABLE ROW LEVEL SECURITY;
ALTER TABLE care_contract.contract_mailings FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_care_contract_contract_mailings ON care_contract.contract_mailings FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
ALTER TABLE care_contract.contract_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE care_contract.contract_events FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_care_contract_contract_events ON care_contract.contract_events FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);

-- The contract itself: a Träger reads, adds and changes its own contracts;
-- it removes only a draft (ADR 0153 point 4).
ALTER TABLE care_contract.contracts ENABLE ROW LEVEL SECURITY;
ALTER TABLE care_contract.contracts FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_read_care_contract_contracts ON care_contract.contracts FOR SELECT
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
CREATE POLICY traeger_insert_care_contract_contracts ON care_contract.contracts FOR INSERT
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint AND status = 'entwurf');
CREATE POLICY traeger_update_care_contract_contracts ON care_contract.contracts FOR UPDATE
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
CREATE POLICY traeger_delete_care_contract_draft ON care_contract.contracts FOR DELETE
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint AND status = 'entwurf');

REVOKE ALL ON care_contract.contract_templates, care_contract.contracts, care_contract.contract_versions,
	care_contract.contract_version_services, care_contract.contract_parties, care_contract.contract_documents, care_contract.contract_mailings,
	care_contract.contract_events FROM PUBLIC;
GRANT SELECT, INSERT, UPDATE ON care_contract.contract_templates TO phoenix_tenant;
GRANT SELECT, INSERT, DELETE ON care_contract.contracts TO phoenix_tenant;
GRANT UPDATE (status, sent_at, student_id) ON care_contract.contracts TO phoenix_tenant;
GRANT SELECT, INSERT ON care_contract.contract_versions, care_contract.contract_version_services, care_contract.contract_parties,
	care_contract.contract_documents, care_contract.contract_mailings, care_contract.contract_events TO phoenix_tenant;
GRANT USAGE ON SEQUENCE care_contract.contract_templates_id_seq, care_contract.contracts_id_seq,
	care_contract.contract_versions_id_seq, care_contract.contract_version_services_id_seq, care_contract.contract_parties_id_seq, care_contract.contract_documents_id_seq,
	care_contract.contract_mailings_id_seq, care_contract.contract_events_id_seq TO phoenix_tenant;
GRANT ALL ON ALL TABLES IN SCHEMA care_contract TO phoenix_admin;
GRANT ALL ON ALL SEQUENCES IN SCHEMA care_contract TO phoenix_admin;
`

const careContractDraftsDownSQL = `DROP SCHEMA IF EXISTS care_contract CASCADE;`

func careContractDraftsUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, careContractDraftsUpSQL); err != nil {
		return fmt.Errorf("create care contract drafts: %w", err)
	}
	return nil
}

func careContractDraftsDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, careContractDraftsDownSQL); err != nil {
		return fmt.Errorf("drop care contract drafts: %w", err)
	}
	return nil
}
