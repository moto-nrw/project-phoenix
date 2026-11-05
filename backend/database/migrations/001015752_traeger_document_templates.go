package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const traegerDocumentTemplatesVersion = "1.15.752"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     traegerDocumentTemplatesVersion,
		Description: "Träger-Portal: Vorlagen je Dokumentart mit Platzhaltern und Versionen (#4677)",
		DependsOn: []string{
			holidaySiblingCancelAuditVersion, // the last migration before this one
			traegerMasterDataVersion,         // the master data log the saves write
			parentBillingLettersVersion,      // the letters that name the version they used
		},
	})
	Migrations.MustRegister(traegerDocumentTemplatesUp, traegerDocumentTemplatesDown)
}

// traegerDocumentTemplatesUpSQL adds the document templates of a Träger
// (#4677, spec #4611 user stories 91 and 92, ADR 0152):
//
//   - platform.traeger_document_templates: every version of the template of
//     a kind of document (subject, salutation, body, closing with
//     placeholders). A save adds the next version; no version ever
//     changes, so a document keeps the text it was written with. The only
//     update allowed is the ON DELETE SET NULL of the account that saved it.
//     Like the master data, the rows belong to the Träger, follow the Träger
//     policy of ADR 0105 and go with the Träger.
//   - platform.traeger_master_data_audit admits the subject 'template'.
//   - parent_billing.run_letters records the template version its letters
//     were written with; 0 is the standard template, which every letter
//     created before this migration used.
const traegerDocumentTemplatesUpSQL = `
CREATE TABLE platform.traeger_document_templates (
	organization_id BIGINT NOT NULL REFERENCES platform.organizations(id) ON DELETE CASCADE,
	kind VARCHAR(16) NOT NULL,
	version INTEGER NOT NULL,
	subject TEXT NOT NULL,
	salutation TEXT NOT NULL DEFAULT '',
	body TEXT NOT NULL,
	closing TEXT NOT NULL DEFAULT '',
	created_by BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY (organization_id, kind, version),
	CONSTRAINT chk_traeger_document_templates_kind CHECK (
		kind IN ('vertrag', 'rechnung', 'bescheid', 'bestaetigung', 'kuendigung', 'mahnung', 'bescheinigung')
	),
	CONSTRAINT chk_traeger_document_templates_version CHECK (version > 0),
	CONSTRAINT chk_traeger_document_templates_text CHECK (
		BTRIM(subject) <> '' AND char_length(subject) <= 200
		AND char_length(salutation) <= 200
		AND BTRIM(body) <> '' AND char_length(body) <= 10000
		AND char_length(closing) <= 500
	)
);
COMMENT ON TABLE platform.traeger_document_templates IS
	'Versions of the document templates of a Träger with placeholders (#4677, ADR 0152); a version never changes; read and written in the Träger transaction (ADR 0105).';

CREATE FUNCTION platform.refuse_traeger_document_template_change() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
	IF NEW.created_by IS NULL
		AND (NEW.organization_id, NEW.kind, NEW.version, NEW.subject, NEW.salutation, NEW.body, NEW.closing, NEW.created_at)
			IS NOT DISTINCT FROM (OLD.organization_id, OLD.kind, OLD.version, OLD.subject, OLD.salutation, OLD.body, OLD.closing, OLD.created_at) THEN
		RETURN NEW;
	END IF;
	RAISE EXCEPTION 'a version of a document template never changes'
		USING ERRCODE = '55000';
END;
$$;
REVOKE ALL ON FUNCTION platform.refuse_traeger_document_template_change() FROM PUBLIC;

CREATE TRIGGER refuse_platform_traeger_document_template_change
	BEFORE UPDATE ON platform.traeger_document_templates
	FOR EACH ROW EXECUTE FUNCTION platform.refuse_traeger_document_template_change();

ALTER TABLE platform.traeger_document_templates ENABLE ROW LEVEL SECURITY;
ALTER TABLE platform.traeger_document_templates FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_platform_traeger_document_templates ON platform.traeger_document_templates FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);

REVOKE ALL ON platform.traeger_document_templates FROM PUBLIC, phoenix_auth;
GRANT SELECT, INSERT ON platform.traeger_document_templates TO phoenix_tenant;
GRANT ALL ON platform.traeger_document_templates TO phoenix_admin;

ALTER TABLE platform.traeger_master_data_audit DROP CONSTRAINT chk_traeger_master_data_audit_subject;
ALTER TABLE platform.traeger_master_data_audit ADD CONSTRAINT chk_traeger_master_data_audit_subject
	CHECK (subject IN ('master_data', 'contact', 'logo', 'bank_account', 'template'));

ALTER TABLE parent_billing.run_letters ADD COLUMN template_version INTEGER NOT NULL DEFAULT 0
	CONSTRAINT chk_run_letters_template_version CHECK (template_version >= 0);
COMMENT ON COLUMN parent_billing.run_letters.template_version IS
	'Version of the confirmation letter template of the Träger the letters were written with (#4677); 0 is the standard template.';

CREATE OR REPLACE FUNCTION parent_billing.refuse_run_letters_change() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
	IF NEW.created_by_account_id IS NULL
		AND (NEW.organization_id, NEW.run_id, NEW.payment_due_on, NEW.notes, NEW.emergency_contacts, NEW.created_at, NEW.template_version)
			IS NOT DISTINCT FROM (OLD.organization_id, OLD.run_id, OLD.payment_due_on, OLD.notes, OLD.emergency_contacts, OLD.created_at, OLD.template_version) THEN
		RETURN NEW;
	END IF;
	RAISE EXCEPTION 'an issued invoice of the Elternabrechnung never changes (%)', TG_TABLE_NAME
		USING ERRCODE = '55000';
END;
$$;
`

const traegerDocumentTemplatesDownSQL = `
CREATE OR REPLACE FUNCTION parent_billing.refuse_run_letters_change() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
	IF NEW.created_by_account_id IS NULL
		AND (NEW.organization_id, NEW.run_id, NEW.payment_due_on, NEW.notes, NEW.emergency_contacts, NEW.created_at)
			IS NOT DISTINCT FROM (OLD.organization_id, OLD.run_id, OLD.payment_due_on, OLD.notes, OLD.emergency_contacts, OLD.created_at) THEN
		RETURN NEW;
	END IF;
	RAISE EXCEPTION 'an issued invoice of the Elternabrechnung never changes (%)', TG_TABLE_NAME
		USING ERRCODE = '55000';
END;
$$;
ALTER TABLE parent_billing.run_letters DROP COLUMN IF EXISTS template_version;
DELETE FROM platform.traeger_master_data_audit WHERE subject = 'template';
ALTER TABLE platform.traeger_master_data_audit DROP CONSTRAINT chk_traeger_master_data_audit_subject;
ALTER TABLE platform.traeger_master_data_audit ADD CONSTRAINT chk_traeger_master_data_audit_subject
	CHECK (subject IN ('master_data', 'contact', 'logo', 'bank_account'));
DROP TABLE IF EXISTS platform.traeger_document_templates;
DROP FUNCTION IF EXISTS platform.refuse_traeger_document_template_change();
`

func traegerDocumentTemplatesUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, traegerDocumentTemplatesUpSQL); err != nil {
		return fmt.Errorf("create Träger document templates: %w", err)
	}
	return nil
}

func traegerDocumentTemplatesDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, traegerDocumentTemplatesDownSQL); err != nil {
		return fmt.Errorf("drop Träger document templates: %w", err)
	}
	return nil
}
