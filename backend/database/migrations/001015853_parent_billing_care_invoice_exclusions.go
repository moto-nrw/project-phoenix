package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const parentBillingCareInvoiceExclusionsVersion = "1.15.853"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     parentBillingCareInvoiceExclusionsVersion,
		Description: "Elternabrechnung: Ausschluss im Monatslauf je Rechnung, Bestandsausschlüsse gelten für Betreuung und Mensa (#6346)",
		DependsOn: []string{
			demoVisitorErasureOutboxKindsVersion, // the newest migration before this one
			parentBillingCareReviewVersion,       // the trail actions exclude and include
		},
	})
	Migrations.MustRegister(parentBillingCareInvoiceExclusionsUp, parentBillingCareInvoiceExclusionsDown)
}

// Since #6346 an exclusion or return of the review of a monthly run names
// one invoice: its contract and its service ("services": ["care"] or
// ["meal"]). An entry written before named the contract only and held back
// every invoice of it, the care invoice and a Mensa invoice of its own
// alike. The up migration writes that meaning into the entry: both
// services. The reader then needs no rule for entries without services, and
// an exclusion of before keeps exactly the invoices it held back. The trail
// stays append only for the request role; this runs as the migration role.
const parentBillingCareInvoiceExclusionsUpSQL = `
UPDATE parent_billing.care_run_events
SET details = details || jsonb_build_object('services', jsonb_build_array('care', 'meal'))
WHERE action IN ('exclude', 'include') AND NOT details ? 'services';
`

// The down migration takes the services off every entry again: the code of
// before reads an entry by its contract, so each exclusion holds back the
// whole contract again, as it did before #6346.
const parentBillingCareInvoiceExclusionsDownSQL = `
UPDATE parent_billing.care_run_events
SET details = details - 'services'
WHERE action IN ('exclude', 'include') AND details ? 'services';
`

func parentBillingCareInvoiceExclusionsUp(ctx context.Context, db *bun.DB) error {
	return mapCareExclusionsOntoServices(ctx, db)
}

// mapCareExclusionsOntoServices is the up migration on any handle, so its
// test runs it in a transaction of its own.
func mapCareExclusionsOntoServices(ctx context.Context, db bun.IDB) error {
	if _, err := db.ExecContext(ctx, parentBillingCareInvoiceExclusionsUpSQL); err != nil {
		return fmt.Errorf("map the exclusions of the monthly runs onto both services: %w", err)
	}
	return nil
}

func parentBillingCareInvoiceExclusionsDown(ctx context.Context, db *bun.DB) error {
	return takeServicesOffCareExclusions(ctx, db)
}

func takeServicesOffCareExclusions(ctx context.Context, db bun.IDB) error {
	if _, err := db.ExecContext(ctx, parentBillingCareInvoiceExclusionsDownSQL); err != nil {
		return fmt.Errorf("take the services off the exclusions of the monthly runs: %w", err)
	}
	return nil
}
