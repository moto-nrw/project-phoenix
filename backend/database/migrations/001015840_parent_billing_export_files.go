package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const parentBillingExportFilesVersion = "1.15.840"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     parentBillingExportFilesVersion,
		Description: "Elternabrechnung: Exportprotokoll, Dateinummer je Träger, Sollstellung mit Feldzuordnung, Debitorennummer der Familie (#4693)",
		DependsOn: []string{
			accountCredentialGenerationBindingVersion, // the newest migration on development
			parentBillingSEPADebitsVersion,            // the PAIN.008 file the protocol records too
			parentBillingCareNoticesVersion,           // the Jahresbescheide and the demo erasure of 1.15.823 this one extends
		},
	})
	Migrations.MustRegister(parentBillingExportFilesUp, parentBillingExportFilesDown)
}

// parentBillingExportFilesUpSQL adds the frame of the finance exports of the
// Elternabrechnung (#4693):
//
//   - export_files: one row per file a person of the Träger downloaded, in
//     the transaction that made it: the file number of the Träger, the
//     format, the run, the file name, a fingerprint of the content, the
//     count and sum, who and when. A repeated download is a new row with a
//     new number. The row never changes; only the confirmation that a
//     PAIN.008 file went to the bank fills submitted_at once, and the
//     erasure of an account clears its reference.
//   - export_file_invoices: the invoices, holiday documents and
//     Jahresbescheide of each file (after the pattern of sepa_debits), the
//     history per invoice. A row goes with its invoice.
//   - export_mappings: the field mapping of a format per Träger.
//   - number_range_counters: service 'export' counts the files of a Träger
//     across the formats of the Elternabrechnung (year 0). Its row may start
//     at 0, so a start value of 1 is recorded as set.
//   - family_billing.debtor_number: the debtor number of the family in the
//     finance system of the Träger; the release copies it onto the monthly
//     invoice (care_invoices.debtor_number), which keeps it.
const parentBillingExportFilesUpSQL = `
ALTER TABLE parent_billing.number_range_counters DROP CONSTRAINT chk_number_range_counters_service,
	ADD CONSTRAINT chk_number_range_counters_service CHECK (service IN ('holiday', 'care', 'export'));
ALTER TABLE parent_billing.number_range_counters DROP CONSTRAINT chk_number_range_counters_last,
	ADD CONSTRAINT chk_number_range_counters_last CHECK (last_number >= 1 OR (service = 'export' AND year = 0 AND last_number >= 0));

ALTER TABLE parent_billing.family_billing
	ADD COLUMN debtor_number VARCHAR(35),
	ADD CONSTRAINT chk_family_billing_debtor_number CHECK (
		debtor_number IS NULL OR (debtor_number <> '' AND debtor_number = btrim(debtor_number) AND debtor_number !~ '[\r\n]'));
COMMENT ON COLUMN parent_billing.family_billing.debtor_number IS
	'Debtor number of the family in the finance system of the Träger (#4693); the release copies it onto the monthly invoice.';
ALTER TABLE parent_billing.care_invoices
	ADD COLUMN debtor_number VARCHAR(35),
	ADD CONSTRAINT chk_care_invoices_debtor_number CHECK (
		debtor_number IS NULL OR (debtor_number <> '' AND debtor_number = btrim(debtor_number) AND debtor_number !~ '[\r\n]'));
COMMENT ON COLUMN parent_billing.care_invoices.debtor_number IS
	'Copy of the debtor number of the family at the release (#4693); every export uses it.';

CREATE TABLE parent_billing.export_mappings (
	organization_id BIGINT NOT NULL REFERENCES platform.organizations(id) ON DELETE CASCADE,
	format VARCHAR(32) NOT NULL,
	settings JSONB NOT NULL,
	updated_by_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY (organization_id, format),
	CONSTRAINT chk_export_mappings_format CHECK (format IN ('sollstellung')),
	CONSTRAINT chk_export_mappings_settings CHECK (jsonb_typeof(settings) = 'object')
);
COMMENT ON TABLE parent_billing.export_mappings IS
	'Field mapping of an export format of a Träger (#4693): columns, separators, date format and file name; no person.';

CREATE TABLE parent_billing.export_files (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL REFERENCES platform.organizations(id) ON DELETE CASCADE,
	file_number BIGINT NOT NULL,
	format VARCHAR(32) NOT NULL,
	run_kind VARCHAR(16) NOT NULL,
	run_id BIGINT NOT NULL,
	file_name VARCHAR(255) NOT NULL,
	content_sha256 CHAR(64) NOT NULL,
	invoice_count INTEGER NOT NULL,
	total_cents BIGINT NOT NULL,
	reference VARCHAR(35),
	exported_by_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	exported_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	submitted_at TIMESTAMPTZ,
	submitted_by_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	CONSTRAINT uq_export_files_organization UNIQUE (organization_id, id),
	-- One number names one file of a Träger (#4693, E-d).
	CONSTRAINT uq_export_files_number UNIQUE (organization_id, file_number),
	CONSTRAINT chk_export_files_number CHECK (file_number >= 1),
	CONSTRAINT chk_export_files_format CHECK (format IN ('sollstellung', 'pain008')),
	CONSTRAINT chk_export_files_run CHECK (run_kind IN ('care', 'holiday', 'notice') AND run_id > 0),
	CONSTRAINT chk_export_files_name CHECK (file_name <> '' AND file_name !~ '[/\\\r\n]'),
	CONSTRAINT chk_export_files_sha CHECK (content_sha256 ~ '^[0-9a-f]{64}$'),
	CONSTRAINT chk_export_files_count CHECK (invoice_count >= 0),
	-- Only a PAIN.008 file carries a message ID and goes to the bank.
	CONSTRAINT chk_export_files_reference CHECK ((format = 'pain008') = (reference IS NOT NULL)),
	CONSTRAINT chk_export_files_submitted CHECK (submitted_at IS NULL OR format = 'pain008')
);
CREATE INDEX idx_export_files_run ON parent_billing.export_files (organization_id, run_kind, run_id, exported_at DESC);
CREATE INDEX idx_export_files_exported ON parent_billing.export_files (organization_id, exported_at DESC, id DESC);
COMMENT ON TABLE parent_billing.export_files IS
	'Protocol of the finance exports of the Elternabrechnung (#4693): one row per downloaded file with its file number, format, run, fingerprint, who and when; never changes.';

CREATE TABLE parent_billing.export_file_invoices (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL,
	export_file_id BIGINT NOT NULL,
	care_invoice_id BIGINT,
	document_id BIGINT,
	care_notice_id BIGINT,
	CONSTRAINT fk_export_file_invoices_file FOREIGN KEY (organization_id, export_file_id)
		REFERENCES parent_billing.export_files (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT fk_export_file_invoices_care_invoice FOREIGN KEY (organization_id, care_invoice_id)
		REFERENCES parent_billing.care_invoices (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT fk_export_file_invoices_document FOREIGN KEY (organization_id, document_id)
		REFERENCES parent_billing.documents (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT fk_export_file_invoices_care_notice FOREIGN KEY (organization_id, care_notice_id)
		REFERENCES parent_billing.care_notices (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT chk_export_file_invoices_one CHECK (num_nonnulls(care_invoice_id, document_id, care_notice_id) = 1)
);
CREATE UNIQUE INDEX uq_export_file_invoices_care_invoice ON parent_billing.export_file_invoices (export_file_id, care_invoice_id)
	WHERE care_invoice_id IS NOT NULL;
CREATE UNIQUE INDEX uq_export_file_invoices_document ON parent_billing.export_file_invoices (export_file_id, document_id)
	WHERE document_id IS NOT NULL;
CREATE UNIQUE INDEX uq_export_file_invoices_care_notice ON parent_billing.export_file_invoices (export_file_id, care_notice_id)
	WHERE care_notice_id IS NOT NULL;
CREATE INDEX idx_export_file_invoices_care_invoice ON parent_billing.export_file_invoices (organization_id, care_invoice_id)
	WHERE care_invoice_id IS NOT NULL;
CREATE INDEX idx_export_file_invoices_document ON parent_billing.export_file_invoices (organization_id, document_id)
	WHERE document_id IS NOT NULL;
CREATE INDEX idx_export_file_invoices_care_notice ON parent_billing.export_file_invoices (organization_id, care_notice_id)
	WHERE care_notice_id IS NOT NULL;
COMMENT ON TABLE parent_billing.export_file_invoices IS
	'Invoice, holiday document or Jahresbescheid in an exported file (#4693): the export history per invoice; goes with its invoice.';

-- A protocol entry keeps what the download wrote. The confirmation of a
-- PAIN.008 file sets submitted_at and its account once; the erasure of an
-- account clears its references (ON DELETE SET NULL).
CREATE FUNCTION parent_billing.refuse_export_file_rewrite() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
	IF (to_jsonb(NEW) - ARRAY['exported_by_account_id', 'submitted_at', 'submitted_by_account_id'])
		IS DISTINCT FROM (to_jsonb(OLD) - ARRAY['exported_by_account_id', 'submitted_at', 'submitted_by_account_id']) THEN
		RAISE EXCEPTION 'an export file of the protocol never changes' USING ERRCODE = '55000';
	END IF;
	IF NEW.exported_by_account_id IS NOT NULL AND NEW.exported_by_account_id IS DISTINCT FROM OLD.exported_by_account_id THEN
		RAISE EXCEPTION 'an export file keeps who downloaded it' USING ERRCODE = '55000';
	END IF;
	IF OLD.submitted_at IS NOT NULL AND (NEW.submitted_at IS DISTINCT FROM OLD.submitted_at
		OR (NEW.submitted_by_account_id IS NOT NULL AND NEW.submitted_by_account_id IS DISTINCT FROM OLD.submitted_by_account_id)) THEN
		RAISE EXCEPTION 'a submitted export file stays submitted' USING ERRCODE = '55000';
	END IF;
	IF OLD.submitted_at IS NULL AND NEW.submitted_at IS NULL AND NEW.submitted_by_account_id IS NOT NULL THEN
		RAISE EXCEPTION 'only a submission names its account' USING ERRCODE = '55000';
	END IF;
	RETURN NEW;
END;
$$;
REVOKE ALL ON FUNCTION parent_billing.refuse_export_file_rewrite() FROM PUBLIC;
CREATE TRIGGER refuse_parent_billing_export_file_rewrite
	BEFORE UPDATE ON parent_billing.export_files
	FOR EACH ROW EXECUTE FUNCTION parent_billing.refuse_export_file_rewrite();

ALTER TABLE parent_billing.export_mappings ENABLE ROW LEVEL SECURITY;
ALTER TABLE parent_billing.export_mappings FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_parent_billing_export_mappings ON parent_billing.export_mappings FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
ALTER TABLE parent_billing.export_files ENABLE ROW LEVEL SECURITY;
ALTER TABLE parent_billing.export_files FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_parent_billing_export_files ON parent_billing.export_files FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
ALTER TABLE parent_billing.export_file_invoices ENABLE ROW LEVEL SECURITY;
ALTER TABLE parent_billing.export_file_invoices FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_parent_billing_export_file_invoices ON parent_billing.export_file_invoices FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);

REVOKE ALL ON parent_billing.export_mappings FROM PUBLIC;
REVOKE ALL ON parent_billing.export_files FROM PUBLIC;
REVOKE ALL ON parent_billing.export_file_invoices FROM PUBLIC;
GRANT SELECT, INSERT, UPDATE, DELETE ON parent_billing.export_mappings TO phoenix_tenant;
GRANT SELECT, INSERT, UPDATE ON parent_billing.export_files TO phoenix_tenant;
GRANT SELECT, INSERT ON parent_billing.export_file_invoices TO phoenix_tenant;
GRANT USAGE ON SEQUENCE parent_billing.export_files_id_seq TO phoenix_tenant;
GRANT USAGE ON SEQUENCE parent_billing.export_file_invoices_id_seq TO phoenix_tenant;
GRANT ALL ON parent_billing.export_mappings, parent_billing.export_files, parent_billing.export_file_invoices TO phoenix_admin;
`

const parentBillingExportFilesDownSQL = `
DROP TABLE IF EXISTS parent_billing.export_file_invoices;
DROP TABLE IF EXISTS parent_billing.export_files;
DROP FUNCTION IF EXISTS parent_billing.refuse_export_file_rewrite();
DROP TABLE IF EXISTS parent_billing.export_mappings;
ALTER TABLE parent_billing.care_invoices DROP CONSTRAINT IF EXISTS chk_care_invoices_debtor_number, DROP COLUMN IF EXISTS debtor_number;
ALTER TABLE parent_billing.family_billing DROP CONSTRAINT IF EXISTS chk_family_billing_debtor_number, DROP COLUMN IF EXISTS debtor_number;
DELETE FROM parent_billing.number_range_counters WHERE service = 'export';
ALTER TABLE parent_billing.number_range_counters DROP CONSTRAINT chk_number_range_counters_last,
	ADD CONSTRAINT chk_number_range_counters_last CHECK (last_number >= 1);
ALTER TABLE parent_billing.number_range_counters DROP CONSTRAINT chk_number_range_counters_service,
	ADD CONSTRAINT chk_number_range_counters_service CHECK (service IN ('holiday', 'care'));
`

func parentBillingExportFilesUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, parentBillingExportFilesUpSQL); err != nil {
		return fmt.Errorf("add the export files of the parent billing: %w", err)
	}
	// The protocol names who downloaded a file; the demo erasure takes the
	// files of a retired demo school along (#6015).
	return installExportFileErasure(ctx, db)
}

func parentBillingExportFilesDown(ctx context.Context, db *bun.DB) error {
	if err := installCareNoticeErasure(ctx, db); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, parentBillingExportFilesDownSQL); err != nil {
		return fmt.Errorf("drop the export files of the parent billing: %w", err)
	}
	return nil
}
