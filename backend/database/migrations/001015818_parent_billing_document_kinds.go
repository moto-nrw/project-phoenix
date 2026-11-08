package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const parentBillingDocumentKindsVersion = "1.15.818"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     parentBillingDocumentKindsVersion,
		Description: "Elternabrechnung: Dokumentart je Leistung, festgehalten bei der Ausstellung, und Rechtsbehelfsbelehrung des Gebührenbescheids (#4688)",
		DependsOn: []string{
			parentBillingCostShareScopeVersion,          // the newest parent-billing migration before this one
			traegerDocumentTemplateInvoiceLayoutVersion, // the trigger of the template versions it extends
		},
	})
	Migrations.MustRegister(parentBillingDocumentKindsUp, parentBillingDocumentKindsDown)
}

// parentBillingDocumentKindsUpSQL lets a Träger choose per service the kind
// of document its billing issues (#4688, ADR 0119 addendum 2026-11-08):
//
//   - parent_billing.document_kinds: the kind a service issues now. The
//     holiday care issues the confirmation letter as invoice, an invoice or a
//     fee notice; the monthly care an invoice or a fee notice. Without a row
//     a service keeps the kind it issued before: confirmation and invoice.
//     The row names no person.
//   - documents.document_kind and care_invoices.document_kind: the kind the
//     release froze on the document. The trigger that refuses every change
//     of an issued document keeps it; a later choice changes only the next
//     release. A Storno or a Gutschrift copies the kind of its invoice.
//     Existing rows keep the kind they were issued as.
//   - traeger_document_templates.legal_remedy: the Rechtsbehelfsbelehrung
//     the Träger writes into the template of the fee notice. It belongs to
//     the version, so the trigger that refuses every change of a version
//     compares it too. Only the kind "bescheid" carries one.
const parentBillingDocumentKindsUpSQL = `
CREATE TABLE parent_billing.document_kinds (
	organization_id BIGINT NOT NULL REFERENCES platform.organizations(id) ON DELETE CASCADE,
	service VARCHAR(16) NOT NULL,
	document_kind VARCHAR(16) NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY (organization_id, service),
	CONSTRAINT chk_document_kinds_choice CHECK (
		(service = 'holiday' AND document_kind IN ('confirmation', 'invoice', 'fee_notice'))
		OR (service = 'care' AND document_kind IN ('invoice', 'fee_notice')))
);
COMMENT ON TABLE parent_billing.document_kinds IS
	'Kind of document a service of a Träger issues (#4688): invoice, fee notice or confirmation letter as invoice; read and written in the Träger transaction. The release freezes it on the document.';
CREATE TRIGGER update_parent_billing_document_kinds_updated_at
	BEFORE UPDATE ON parent_billing.document_kinds
	FOR EACH ROW EXECUTE FUNCTION update_modified_column();
ALTER TABLE parent_billing.document_kinds ENABLE ROW LEVEL SECURITY;
ALTER TABLE parent_billing.document_kinds FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_parent_billing_document_kinds ON parent_billing.document_kinds FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
REVOKE ALL ON parent_billing.document_kinds FROM PUBLIC;
GRANT SELECT, INSERT, UPDATE ON parent_billing.document_kinds TO phoenix_tenant;
GRANT ALL ON parent_billing.document_kinds TO phoenix_admin;

ALTER TABLE parent_billing.documents
	ADD COLUMN document_kind VARCHAR(16) NOT NULL DEFAULT 'confirmation',
	ADD CONSTRAINT chk_documents_document_kind CHECK (document_kind IN ('confirmation', 'invoice', 'fee_notice'));
COMMENT ON COLUMN parent_billing.documents.document_kind IS
	'Kind of document the release froze (#4688): confirmation letter as invoice, invoice or fee notice; a Storno or a Gutschrift has the kind of its invoice.';
ALTER TABLE parent_billing.care_invoices
	ADD COLUMN document_kind VARCHAR(16) NOT NULL DEFAULT 'invoice',
	ADD CONSTRAINT chk_care_invoices_document_kind CHECK (document_kind IN ('invoice', 'fee_notice'));
COMMENT ON COLUMN parent_billing.care_invoices.document_kind IS
	'Kind of document the release froze (#4688): invoice or fee notice; its Storno has the same kind.';

ALTER TABLE platform.traeger_document_templates
	ADD COLUMN legal_remedy TEXT NOT NULL DEFAULT '',
	ADD CONSTRAINT chk_traeger_document_templates_legal_remedy
		CHECK (char_length(legal_remedy) <= 5000 AND (kind = 'bescheid' OR legal_remedy = ''));
COMMENT ON COLUMN platform.traeger_document_templates.legal_remedy IS
	'Rechtsbehelfsbelehrung the Träger writes into its fee notice (#4688); no fee notice is issued without one.';

CREATE OR REPLACE FUNCTION platform.refuse_traeger_document_template_change() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
	IF NEW.created_by IS NULL
		AND (NEW.organization_id, NEW.kind, NEW.version, NEW.subject, NEW.salutation, NEW.body, NEW.closing, NEW.created_at,
			NEW.invoice_group_by_child, NEW.invoice_hide_time_models, NEW.legal_remedy)
			IS NOT DISTINCT FROM (OLD.organization_id, OLD.kind, OLD.version, OLD.subject, OLD.salutation, OLD.body, OLD.closing, OLD.created_at,
			OLD.invoice_group_by_child, OLD.invoice_hide_time_models, OLD.legal_remedy) THEN
		RETURN NEW;
	END IF;
	RAISE EXCEPTION 'a version of a document template never changes'
		USING ERRCODE = '55000';
END;
$$;
`

const parentBillingDocumentKindsDownSQL = `
CREATE OR REPLACE FUNCTION platform.refuse_traeger_document_template_change() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
	IF NEW.created_by IS NULL
		AND (NEW.organization_id, NEW.kind, NEW.version, NEW.subject, NEW.salutation, NEW.body, NEW.closing, NEW.created_at,
			NEW.invoice_group_by_child, NEW.invoice_hide_time_models)
			IS NOT DISTINCT FROM (OLD.organization_id, OLD.kind, OLD.version, OLD.subject, OLD.salutation, OLD.body, OLD.closing, OLD.created_at,
			OLD.invoice_group_by_child, OLD.invoice_hide_time_models) THEN
		RETURN NEW;
	END IF;
	RAISE EXCEPTION 'a version of a document template never changes'
		USING ERRCODE = '55000';
END;
$$;
ALTER TABLE platform.traeger_document_templates DROP COLUMN IF EXISTS legal_remedy;
ALTER TABLE parent_billing.care_invoices DROP COLUMN IF EXISTS document_kind;
ALTER TABLE parent_billing.documents DROP COLUMN IF EXISTS document_kind;
DROP TABLE IF EXISTS parent_billing.document_kinds;
`

func parentBillingDocumentKindsUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, parentBillingDocumentKindsUpSQL); err != nil {
		return fmt.Errorf("add document kinds of parent billing: %w", err)
	}
	return nil
}

func parentBillingDocumentKindsDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, parentBillingDocumentKindsDownSQL); err != nil {
		return fmt.Errorf("drop document kinds of parent billing: %w", err)
	}
	return nil
}
