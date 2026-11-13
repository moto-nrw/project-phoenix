package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const submittedOfferingTermsVersion = "1.15.964"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     submittedOfferingTermsVersion,
		Description: "Anmeldung: Angebotsbedingungen bei der Einreichung festhalten (#6837)",
		DependsOn: []string{
			traegerExportRecipientsVersion,   // the newest migration on development (1.15.963)
			requestChildStorageExpandVersion, // creates enrollment.request_child_offering_selections (1.15.375)
		},
	})
	Migrations.MustRegister(submittedOfferingTermsUp, submittedOfferingTermsDown)
}

// submittedOfferingTermsUpSQL keeps the catalog terms of an offering as the
// parents submitted and signed them (#6837): name, price, day mode, offered
// days and the linked activity. The catalog row stays editable; the approval
// compares against this snapshot and the office confirms any change. A choice
// stored before has no snapshot (NULL); nothing reconstructs it.
const submittedOfferingTermsUpSQL = `
ALTER TABLE enrollment.request_child_offering_selections ADD COLUMN offering_terms JSONB;
COMMENT ON COLUMN enrollment.request_child_offering_selections.offering_terms IS
	'Catalog terms of the offering at submission (#6837): name, price_cents, days_of_week_mode, available_days, activity_group_id. NULL for choices stored before.';
`

const submittedOfferingTermsDownSQL = `ALTER TABLE enrollment.request_child_offering_selections DROP COLUMN offering_terms;`

func submittedOfferingTermsUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, submittedOfferingTermsUpSQL); err != nil {
		return fmt.Errorf("add the submitted offering terms: %w", err)
	}
	return nil
}

func submittedOfferingTermsDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, submittedOfferingTermsDownSQL); err != nil {
		return fmt.Errorf("drop the submitted offering terms: %w", err)
	}
	return nil
}
