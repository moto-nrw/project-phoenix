package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const parentBillingDatevExportFilesVersion = "1.15.881"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     parentBillingDatevExportFilesVersion,
		Description: "Elternabrechnung: DATEV-Buchungsstapel im Exportprotokoll (#4694)",
		DependsOn: []string{
			parentBillingExportFilesVersion, // the protocol whose formats this one extends (1.15.875)
		},
	})
	Migrations.MustRegister(parentBillingDatevExportFilesUp, parentBillingDatevExportFilesDown)
}

// parentBillingDatevExportFilesUpSQL lets the export protocol of #4693 record
// the DATEV booking batch (#4694) as a format of its own. The file takes the
// file number of the Träger like every other format; it has no reference and
// no submission.
const parentBillingDatevExportFilesUpSQL = `
ALTER TABLE parent_billing.export_files DROP CONSTRAINT chk_export_files_format,
	ADD CONSTRAINT chk_export_files_format CHECK (format IN ('sollstellung', 'pain008', 'datev'));
`

// The down step removes the DATEV entries; their invoice links go with them.
const parentBillingDatevExportFilesDownSQL = `
DELETE FROM parent_billing.export_files WHERE format = 'datev';
ALTER TABLE parent_billing.export_files DROP CONSTRAINT chk_export_files_format,
	ADD CONSTRAINT chk_export_files_format CHECK (format IN ('sollstellung', 'pain008'));
`

func parentBillingDatevExportFilesUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, parentBillingDatevExportFilesUpSQL); err != nil {
		return fmt.Errorf("let the export protocol record DATEV files: %w", err)
	}
	return nil
}

func parentBillingDatevExportFilesDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, parentBillingDatevExportFilesDownSQL); err != nil {
		return fmt.Errorf("drop the DATEV format of the export protocol: %w", err)
	}
	return nil
}
