package migrations

import (
	"context"
	"fmt"
	"strings"

	"github.com/uptrace/bun"
)

// exportFileErasureAnchor is where 1.15.840 adds to the erasure of 1.15.823:
// right before the monthly invoices of the demo school go.
const exportFileErasureAnchor = `					DELETE FROM parent_billing.care_invoices
					WHERE school_id = ANY (hidden_schools);
`

// exportFileErasureStatement is what 1.15.840 adds (#4693). A file of the
// export protocol names who downloaded it and the invoices it held; a file
// that held a monthly invoice or a Jahresbescheid of the school goes with
// it, before the invoices and their links. The demo Träger keeps no books
// (#6015).
const exportFileErasureStatement = `					-- A file of the export protocol that held an invoice or a
					-- Jahresbescheid of the school goes with it (#4693).
					DELETE FROM parent_billing.export_files AS file
					USING parent_billing.export_file_invoices AS held
					WHERE held.organization_id = file.organization_id
						AND held.export_file_id = file.id
						AND (held.care_invoice_id IN (
								SELECT invoice.id FROM parent_billing.care_invoices AS invoice
								WHERE invoice.school_id = ANY (hidden_schools))
							OR held.care_notice_id IN (
								SELECT notice.id FROM parent_billing.care_notices AS notice
								WHERE notice.school_id = ANY (hidden_schools)));
`

// demoVisitorErasureExportFilesSQL is platform.erase_closed_demo_visitors of
// 1.15.823 with exportFileErasureStatement before exportFileErasureAnchor;
// the rest is unchanged, TestExportFileErasureExtendsTheErasureOf823 keeps it
// that way.
var demoVisitorErasureExportFilesSQL = strings.Replace(demoVisitorErasureCareNoticesSQL, exportFileErasureAnchor,
	exportFileErasureStatement+exportFileErasureAnchor, 1)

// installExportFileErasure installs the erasure of 1.15.840.
func installExportFileErasure(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, demoVisitorErasureExportFilesSQL); err != nil {
		return fmt.Errorf("erase the export files of retired demo schools: %w", err)
	}
	return nil
}
