package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const parentBillingDatevBookedDocumentsVersion = "1.15.888"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     parentBillingDatevBookedDocumentsVersion,
		Description: "Elternabrechnung: DATEV-Datei nur mit neuen Buchungen, Wiederholung im Protokoll (#6433)",
		DependsOn: []string{
			parentBillingDatevExportFilesVersion, // the DATEV format of the protocol (1.15.884), the newest on development
		},
	})
	Migrations.MustRegister(parentBillingDatevBookedDocumentsUp, parentBillingDatevBookedDocumentsDown)
}

// parentBillingDatevBookedDocumentsUpSQL records which booking went to DATEV
// first (#6433), so the next DATEV file of a run holds only new bookings:
//
//   - datev_booked_documents: one row per booking, the first DATEV file that
//     held it. A booking is a monthly invoice, the Storno of one (its own
//     row, the Storno shares the care_invoice_id of its invoice), or a
//     holiday document (invoice, Storno, credit). The unique indexes are the
//     guard against two downloads at the same time: the second one finds the
//     bookings taken and rolls back with its file number. A row goes with
//     its file and with its booking.
//   - export_files.datev_all: the person chose "all bookings again"; the
//     file repeats bookings that were already in a DATEV file.
//
// The DATEV files of #6427 held every booking of their run. The backfill
// marks those bookings with the earliest such file; a Storno counts when it
// was issued before that file was made.
const parentBillingDatevBookedDocumentsUpSQL = `
ALTER TABLE parent_billing.export_files
	ADD COLUMN datev_all BOOLEAN NOT NULL DEFAULT false,
	ADD CONSTRAINT chk_export_files_datev_all CHECK (NOT datev_all OR format = 'datev');

CREATE TABLE parent_billing.datev_booked_documents (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL,
	export_file_id BIGINT NOT NULL,
	care_invoice_id BIGINT,
	care_cancellation_id BIGINT REFERENCES parent_billing.care_invoice_cancellations(id) ON DELETE CASCADE,
	document_id BIGINT,
	CONSTRAINT fk_datev_booked_documents_file FOREIGN KEY (organization_id, export_file_id)
		REFERENCES parent_billing.export_files (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT fk_datev_booked_documents_care_invoice FOREIGN KEY (organization_id, care_invoice_id)
		REFERENCES parent_billing.care_invoices (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT fk_datev_booked_documents_document FOREIGN KEY (organization_id, document_id)
		REFERENCES parent_billing.documents (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT chk_datev_booked_documents_one CHECK (num_nonnulls(care_invoice_id, care_cancellation_id, document_id) = 1)
);
-- A booking goes to DATEV once in the standard download (#6433).
CREATE UNIQUE INDEX uq_datev_booked_documents_care_invoice ON parent_billing.datev_booked_documents (organization_id, care_invoice_id)
	WHERE care_invoice_id IS NOT NULL;
CREATE UNIQUE INDEX uq_datev_booked_documents_care_cancellation ON parent_billing.datev_booked_documents (organization_id, care_cancellation_id)
	WHERE care_cancellation_id IS NOT NULL;
CREATE UNIQUE INDEX uq_datev_booked_documents_document ON parent_billing.datev_booked_documents (organization_id, document_id)
	WHERE document_id IS NOT NULL;
CREATE INDEX idx_datev_booked_documents_file ON parent_billing.datev_booked_documents (organization_id, export_file_id);
COMMENT ON TABLE parent_billing.datev_booked_documents IS
	'Booking of the Elternabrechnung and the first DATEV file that held it (#6433); the standard DATEV download leaves it out.';

INSERT INTO parent_billing.datev_booked_documents (organization_id, export_file_id, care_invoice_id)
SELECT DISTINCT ON (link.organization_id, link.care_invoice_id) link.organization_id, link.export_file_id, link.care_invoice_id
FROM parent_billing.export_file_invoices AS link
JOIN parent_billing.export_files AS file ON file.organization_id = link.organization_id AND file.id = link.export_file_id
JOIN parent_billing.care_invoices AS invoice ON invoice.organization_id = link.organization_id AND invoice.id = link.care_invoice_id
WHERE file.format = 'datev' AND invoice.total_cents <> 0
ORDER BY link.organization_id, link.care_invoice_id, file.file_number;

INSERT INTO parent_billing.datev_booked_documents (organization_id, export_file_id, care_cancellation_id)
SELECT DISTINCT ON (cancellation.id) cancellation.organization_id, file.id, cancellation.id
FROM parent_billing.care_invoice_cancellations AS cancellation
JOIN parent_billing.export_file_invoices AS link
	ON link.organization_id = cancellation.organization_id AND link.care_invoice_id = cancellation.invoice_id
JOIN parent_billing.export_files AS file ON file.organization_id = link.organization_id AND file.id = link.export_file_id
WHERE file.format = 'datev' AND cancellation.total_cents <> 0 AND cancellation.issued_at <= file.exported_at
ORDER BY cancellation.id, file.file_number;

INSERT INTO parent_billing.datev_booked_documents (organization_id, export_file_id, document_id)
SELECT DISTINCT ON (link.organization_id, link.document_id) link.organization_id, link.export_file_id, link.document_id
FROM parent_billing.export_file_invoices AS link
JOIN parent_billing.export_files AS file ON file.organization_id = link.organization_id AND file.id = link.export_file_id
JOIN parent_billing.documents AS document ON document.organization_id = link.organization_id AND document.id = link.document_id
WHERE file.format = 'datev' AND document.total_cents <> 0
ORDER BY link.organization_id, link.document_id, file.file_number;

ALTER TABLE parent_billing.datev_booked_documents ENABLE ROW LEVEL SECURITY;
ALTER TABLE parent_billing.datev_booked_documents FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_parent_billing_datev_booked_documents ON parent_billing.datev_booked_documents FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
REVOKE ALL ON parent_billing.datev_booked_documents FROM PUBLIC;
GRANT SELECT, INSERT ON parent_billing.datev_booked_documents TO phoenix_tenant;
GRANT USAGE ON SEQUENCE parent_billing.datev_booked_documents_id_seq TO phoenix_tenant;
GRANT ALL ON parent_billing.datev_booked_documents TO phoenix_admin;
GRANT USAGE ON SEQUENCE parent_billing.datev_booked_documents_id_seq TO phoenix_admin;
`

const parentBillingDatevBookedDocumentsDownSQL = `
DROP TABLE IF EXISTS parent_billing.datev_booked_documents;
ALTER TABLE parent_billing.export_files DROP CONSTRAINT IF EXISTS chk_export_files_datev_all, DROP COLUMN IF EXISTS datev_all;
`

func parentBillingDatevBookedDocumentsUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, parentBillingDatevBookedDocumentsUpSQL); err != nil {
		return fmt.Errorf("record the bookings of the DATEV files: %w", err)
	}
	return nil
}

func parentBillingDatevBookedDocumentsDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, parentBillingDatevBookedDocumentsDownSQL); err != nil {
		return fmt.Errorf("drop the bookings of the DATEV files: %w", err)
	}
	return nil
}
