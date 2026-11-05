package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const traegerDocumentTemplateInvoiceLayoutVersion = "1.15.753"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     traegerDocumentTemplateInvoiceLayoutVersion,
		Description: "Träger-Portal: Darstellung der Rechnungstabelle je Vorlagenversion (#5691)",
		DependsOn: []string{
			traegerDocumentTemplatesVersion, // the last migration before this one and the table it extends
		},
	})
	Migrations.MustRegister(traegerDocumentTemplateInvoiceLayoutUp, traegerDocumentTemplateInvoiceLayoutDown)
}

// traegerDocumentTemplateInvoiceLayoutUpSQL stores with every version of a
// document template how its table of amounts lists the positions (#5691):
// grouped by child with a subtotal, and without the week or the time model
// of a booking. false everywhere is the standard table every version saved
// before kept. The layout belongs to the version, so the trigger that
// refuses every change of a version now compares it too.
const traegerDocumentTemplateInvoiceLayoutUpSQL = `
ALTER TABLE platform.traeger_document_templates
	ADD COLUMN invoice_group_by_child BOOLEAN NOT NULL DEFAULT false,
	ADD COLUMN invoice_hide_weeks BOOLEAN NOT NULL DEFAULT false,
	ADD COLUMN invoice_hide_time_models BOOLEAN NOT NULL DEFAULT false;
COMMENT ON COLUMN platform.traeger_document_templates.invoice_group_by_child IS
	'The table of amounts names each child once above its positions and closes them with a subtotal (#5691).';
COMMENT ON COLUMN platform.traeger_document_templates.invoice_hide_weeks IS
	'The table of amounts leaves the week of a booking out (#5691).';
COMMENT ON COLUMN platform.traeger_document_templates.invoice_hide_time_models IS
	'The table of amounts leaves the time model of a booking out (#5691).';

CREATE OR REPLACE FUNCTION platform.refuse_traeger_document_template_change() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
	IF NEW.created_by IS NULL
		AND (NEW.organization_id, NEW.kind, NEW.version, NEW.subject, NEW.salutation, NEW.body, NEW.closing, NEW.created_at,
			NEW.invoice_group_by_child, NEW.invoice_hide_weeks, NEW.invoice_hide_time_models)
			IS NOT DISTINCT FROM (OLD.organization_id, OLD.kind, OLD.version, OLD.subject, OLD.salutation, OLD.body, OLD.closing, OLD.created_at,
			OLD.invoice_group_by_child, OLD.invoice_hide_weeks, OLD.invoice_hide_time_models) THEN
		RETURN NEW;
	END IF;
	RAISE EXCEPTION 'a version of a document template never changes'
		USING ERRCODE = '55000';
END;
$$;
`

const traegerDocumentTemplateInvoiceLayoutDownSQL = `
CREATE OR REPLACE FUNCTION platform.refuse_traeger_document_template_change() RETURNS trigger
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
ALTER TABLE platform.traeger_document_templates
	DROP COLUMN IF EXISTS invoice_group_by_child,
	DROP COLUMN IF EXISTS invoice_hide_weeks,
	DROP COLUMN IF EXISTS invoice_hide_time_models;
`

func traegerDocumentTemplateInvoiceLayoutUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, traegerDocumentTemplateInvoiceLayoutUpSQL); err != nil {
		return fmt.Errorf("add invoice layout to Träger document templates: %w", err)
	}
	return nil
}

func traegerDocumentTemplateInvoiceLayoutDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, traegerDocumentTemplateInvoiceLayoutDownSQL); err != nil {
		return fmt.Errorf("drop invoice layout of Träger document templates: %w", err)
	}
	return nil
}
